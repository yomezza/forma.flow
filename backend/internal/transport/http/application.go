package httptransport

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yomezza/forma.flow/backend/internal/absence"
	"github.com/yomezza/forma.flow/backend/internal/admin"
	"github.com/yomezza/forma.flow/backend/internal/auth"
	"github.com/yomezza/forma.flow/backend/internal/booking"
	"github.com/yomezza/forma.flow/backend/internal/catalog"
	"github.com/yomezza/forma.flow/backend/internal/client"
	"github.com/yomezza/forma.flow/backend/internal/config"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"github.com/yomezza/forma.flow/backend/internal/room"
	"github.com/yomezza/forma.flow/backend/internal/schedule"
	"github.com/yomezza/forma.flow/backend/internal/specialist"
	"github.com/yomezza/forma.flow/backend/internal/storage/postgres"
)

type application struct {
	server    *Server
	cfg       config.Config
	studio    string
	store     *postgres.Store
	auth      *auth.Service
	bookings  *booking.Service
	catalog   *catalog.Service
	clients   *client.Service
	schedule  *schedule.Service
	staff     *specialist.Service
	rooms     *room.Service
	absences  *absence.Service
	admin     *admin.Service
	mu        sync.Mutex
	rates     map[string]rateEntry
	hashSlots chan struct{}
}
type rateEntry struct {
	Start time.Time
	Count int
}

func API(store *postgres.Store, log *slog.Logger, cfg config.Config, studio string, clock domain.Clock) http.Handler {
	if clock == nil {
		clock = domain.RealClock{}
	}
	engine := schedule.Engine{Clock: clock, MaxNodes: cfg.SolverNodes, MaxComponent: cfg.MaxComponent}
	s := &Server{Store: store, Log: log}
	staff := &specialist.Service{Store: store, StudioID: studio, Engine: engine}
	a := &application{server: s, cfg: cfg, store: store, studio: studio, auth: auth.New(store, cfg.SessionTTL), bookings: &booking.Service{Store: store, StudioID: studio, Engine: engine}, catalog: &catalog.Service{Store: store, StudioID: studio}, clients: &client.Service{Store: store}, schedule: &schedule.Service{Store: store, StudioID: studio, TimeZone: cfg.StudioTimeZone, Engine: engine}, staff: staff, rooms: &room.Service{Store: store, StudioID: studio, Staff: staff}, absences: &absence.Service{Store: store, StudioID: studio, Engine: engine}, admin: &admin.Service{Store: store, StudioID: studio, Engine: engine}, rates: map[string]rateEntry{}, hashSlots: make(chan struct{}, 4)}
	mux := http.NewServeMux()
	for _, route := range routes {
		rt := route
		mux.HandleFunc(rt.Method+" "+rt.Path, func(w http.ResponseWriter, r *http.Request) { a.handle(w, r, rt) })
	}
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) { JSON(w, 200, domain.M{"status": "ok"}) })
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if e := store.Pool.Ping(ctx); e != nil {
			JSON(w, 503, domain.M{"status": "unavailable"})
			return
		}
		JSON(w, 200, domain.M{"status": "ready"})
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { s.Error(w, r, domain.NotFound) })
	return s.middleware(mux)
}
func (a *application) rate(r *http.Request, login bool) bool {
	ip, _, _ := net.SplitHostPort(r.RemoteAddr)
	key := ip + ":booking"
	limit := 30
	if login {
		key = ip + ":auth"
		limit = 8
	}
	now := time.Now()
	a.mu.Lock()
	defer a.mu.Unlock()
	entry := a.rates[key]
	if now.Sub(entry.Start) >= time.Minute {
		entry = rateEntry{Start: now}
	}
	if len(a.rates) > 10000 {
		for k, v := range a.rates {
			if now.Sub(v.Start) > time.Minute {
				delete(a.rates, k)
			}
		}
		if len(a.rates) > 10000 {
			return false
		}
	}
	entry.Count++
	a.rates[key] = entry
	return entry.Count <= limit
}
func (a *application) cookie(w http.ResponseWriter, actor domain.Actor) {
	http.SetCookie(w, &http.Cookie{Name: "ft_session", Value: actor.Token, Path: "/api/v1", HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode, Expires: actor.ExpiresAt, MaxAge: int(time.Until(actor.ExpiresAt).Seconds())})
}
func (a *application) handle(w http.ResponseWriter, r *http.Request, rt route) {
	ctx, cancel := context.WithTimeout(r.Context(), 12*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	actor := domain.Actor{}
	if rt.Role != "" {
		cookie, e := r.Cookie("ft_session")
		if e != nil {
			a.server.Error(w, r, domain.AuthRequired)
			return
		}
		actor, e = a.auth.Authenticate(ctx, cookie.Value)
		if e != nil {
			a.server.Error(w, r, e)
			return
		}
		if actor.MustChangePassword && rt.Operation != "staffGetCurrentSession" && rt.Operation != "staffDeleteCurrentSession" && rt.Operation != "staffChangePassword" {
			a.server.Error(w, r, domain.Fail("FIRST_LOGIN_PASSWORD_CHANGE_REQUIRED", 403, "Сначала смените временный пароль."))
			return
		}
		var roleError error
		switch rt.Role {
		case "owner":
			if !actor.Has("owner") {
				roleError = domain.Forbidden
			}
		case "admin":
			roleError = actor.Admin()
		case "specialist":
			roleError = actor.Specialist()
		}
		if roleError != nil {
			a.server.Error(w, r, roleError)
			return
		}
	}
	if r.Method != "GET" {
		if r.Header.Get("Origin") != a.cfg.PublicOrigin {
			a.server.Error(w, r, domain.Forbidden)
			return
		}
		if rt.Role != "" && subtle.ConstantTimeCompare([]byte(auth.CSRF(actor.Token)), []byte(r.Header.Get("X-CSRF-Token"))) != 1 {
			a.server.Error(w, r, domain.Forbidden)
			return
		}
	}
	if rt.Operation == "staffCreateSession" || rt.Operation == "createBookingRequest" {
		if !a.rate(r, rt.Operation == "staffCreateSession") {
			w.Header().Set("Retry-After", "60")
			a.server.Error(w, r, domain.Fail("RATE_LIMITED", 429, "Слишком много запросов."))
			return
		}
	}
	if rt.Operation == "staffCreateSession" || rt.Operation == "staffChangePassword" || rt.Operation == "adminInviteStaff" {
		select {
		case a.hashSlots <- struct{}{}:
			defer func() { <-a.hashSlots }()
		default:
			a.server.Error(w, r, domain.Fail("RATE_LIMITED", 429, "Повторите запрос позднее."))
			return
		}
	}
	listOperations := map[string]bool{"listServices": true, "adminListServices": true, "adminListClients": true, "adminListClientsByPhone": true, "adminListBookings": true, "adminClientBookings": true, "specialistMyBookings": true, "adminGetAssignmentEvents": true, "specialistOwnReassignmentEvents": true, "adminListSpecialists": true, "adminListRooms": true, "adminListAbsences": true, "specialistMyAbsences": true, "adminListVacations": true, "adminGetScheduleHistory": true}
	if listOperations[rt.Operation] {
		p, e := pagination(r)
		if e != nil {
			a.server.Error(w, r, e)
			return
		}
		ctx = postgres.WithPage(ctx, p)
		r = r.WithContext(ctx)
	}
	if e := validateQuery(r, rt); e != nil {
		a.server.Error(w, r, e)
		return
	}
	in := domain.M{}
	if rt.Body {
		var e error
		in, e = readBody(w, r, rt.Schema)
		if e != nil {
			a.server.Error(w, r, e)
			return
		}
	}
	result, status, e := a.execute(ctx, w, r, rt, actor, in)
	if e != nil {
		a.server.Error(w, r, e)
		return
	}
	JSON(w, status, result)
}
func pagination(r *http.Request) (postgres.PageSpec, error) {
	p := postgres.PageSpec{Limit: 25}
	q := r.URL.Query()
	if raw := q.Get("limit"); raw != "" {
		n, e := strconv.Atoi(raw)
		if e != nil || n < 1 || n > 100 {
			return p, domain.Validation("limit")
		}
		p.Limit = n
	}
	if raw := q.Get("cursor"); raw != "" {
		b, e := base64.RawURLEncoding.DecodeString(raw)
		if e != nil {
			return p, domain.Validation("cursor")
		}
		n, e := strconv.Atoi(string(b))
		if e != nil || n < 0 || n > 10000000 {
			return p, domain.Validation("cursor")
		}
		p.Offset = n
	}
	return p, nil
}
func page(r *http.Request, items []domain.M) (domain.M, error) {
	p, e := pagination(r)
	if e != nil {
		return nil, e
	}
	var next any
	if len(items) > p.Limit {
		next = base64.RawURLEncoding.EncodeToString([]byte(strconv.Itoa(p.Offset + p.Limit)))
		items = items[:p.Limit]
	}
	if items == nil {
		items = []domain.M{}
	}
	return domain.M{"items": items, "nextCursor": next}, nil
}
func (a *application) execute(ctx context.Context, w http.ResponseWriter, r *http.Request, rt route, actor domain.Actor, in domain.M) (any, int, error) {
	q := r.URL.Query()
	id, date := r.PathValue("id"), r.PathValue("date")
	status := rt.Status
	ret := func(v any, e error) (any, int, error) { return v, status, e }
	list := func(v []domain.M, e error) (any, int, error) {
		if e != nil {
			return nil, status, e
		}
		p, e := page(r, v)
		return p, status, e
	}
	switch rt.Operation {
	case "staffCreateSession":
		session, e := a.auth.Login(ctx, in.String("email"), in.String("password"))
		if e != nil {
			return ret(nil, e)
		}
		a.cookie(w, session)
		return ret(auth.Session(session), nil)
	case "staffGetCurrentSession":
		return ret(auth.Session(actor), nil)
	case "staffDeleteCurrentSession":
		e := a.auth.Logout(ctx, actor)
		http.SetCookie(w, &http.Cookie{Name: "ft_session", Path: "/api/v1", MaxAge: -1, HttpOnly: true, Secure: a.cfg.CookieSecure, SameSite: http.SameSiteLaxMode})
		return ret(nil, e)
	case "staffChangePassword":
		session, e := a.auth.ChangePassword(ctx, actor, in.String("currentPassword"), in.String("newPassword"))
		if e != nil {
			return ret(nil, e)
		}
		a.cookie(w, session)
		return ret(auth.Session(session), nil)
	case "adminInviteStaff":
		v, e := a.auth.Invite(ctx, actor, in)
		return ret(v, e)
	case "listServices", "adminListServices":
		v, e := a.catalog.List(ctx, actor, rt.Operation == "adminListServices")
		return list(v, e)
	case "getService":
		v, e := a.catalog.Get(ctx, id)
		return ret(v, e)
	case "adminCreateService", "adminUpdateService":
		v, e := a.catalog.Save(ctx, actor, id, in)
		return ret(v, e)
	case "adminArchiveService":
		return ret(nil, a.catalog.Archive(ctx, actor, id))
	case "getPublicSiteSettings", "adminGetSettings":
		v, e := a.admin.Settings(ctx)
		return ret(v, e)
	case "getPublicSiteContent":
		v, e := a.admin.Content(ctx)
		return ret(v, e)
	case "adminUpdateSettings", "adminUpdateSiteContent":
		v, e := a.admin.SaveSite(ctx, actor, in, rt.Operation == "adminUpdateSettings")
		return ret(v, e)
	case "getSlots":
		date = q.Get("date")
		v, e := a.schedule.Availability(ctx, date, date, q.Get("serviceId"), true)
		if q.Get("serviceId") == "" {
			e = domain.Validation("serviceId")
		}
		return ret(v, e)
	case "getDateStates":
		v, e := a.schedule.Availability(ctx, q.Get("from"), q.Get("to"), q.Get("serviceId"), false)
		return ret(v, e)
	case "createBookingRequest":
		v, replay, e := a.bookings.Create(ctx, in, r.Header.Get("Idempotency-Key"))
		if replay {
			status = 200
		}
		return ret(v, e)
	case "adminListClients", "adminListClientsByPhone":
		phone := q.Get("phone")
		if rt.Operation == "adminListClientsByPhone" && phone == "" {
			return ret(nil, domain.Validation("phone"))
		}
		v, e := a.clients.List(ctx, actor, phone, q.Get("search"))
		return list(v, e)
	case "adminGetClient":
		v, e := a.clients.Get(ctx, actor, id)
		return ret(v, e)
	case "adminCreateClient", "adminUpdateClient":
		v, e := a.clients.Save(ctx, actor, id, in)
		return ret(v, e)
	case "adminAssignBookingClient":
		v, e := a.bookings.LinkClient(ctx, actor, id, in)
		return ret(booking.PublicBooking(v), e)
	case "adminConfirmBooking", "adminCancelBooking", "adminRestoreBooking", "specialistCompleteBooking", "specialistUndoCompletion":
		action := "confirm"
		switch rt.Operation {
		case "adminCancelBooking":
			action = "cancel"
		case "adminRestoreBooking":
			action = "restore"
		case "specialistCompleteBooking":
			action = "complete"
		case "specialistUndoCompletion":
			action = "undo-completion"
		}
		v, e := a.bookings.Transition(ctx, actor, id, action, in.String("note"))
		if e != nil {
			return ret(nil, e)
		}
		if strings.HasPrefix(rt.Operation, "specialist") {
			d, e := postgres.Snapshot(ctx, a.store.Pool, a.studio, v.String("date"), v.String("date"))
			return ret(booking.SpecialistBooking(d, v), e)
		}
		return ret(booking.PublicBooking(v), nil)
	case "adminListBookings", "adminClientBookings", "specialistMyBookings":
		clientID, specialistID := "", ""
		if rt.Operation == "adminClientBookings" {
			clientID = id
			if _, e := a.clients.Get(ctx, actor, id); e != nil {
				return ret(nil, e)
			}
		}
		if rt.Operation == "specialistMyBookings" {
			specialistID = actor.SpecialistID
		}
		v, e := postgres.BookingList(ctx, a.store.Pool, a.studio, clientID, q.Get("date"), q.Get("status"), specialistID)
		if e != nil {
			return ret(nil, e)
		}
		out := []domain.M{}
		var d domain.ScheduleData
		if specialistID != "" {
			today := time.Now().UTC().Format("2006-01-02")
			d, e = postgres.Snapshot(ctx, a.store.Pool, a.studio, today, today)
			if e != nil {
				return ret(nil, e)
			}
		}
		for _, b := range v {
			if specialistID != "" {
				out = append(out, booking.SpecialistBooking(d, b))
			} else {
				out = append(out, booking.PublicBooking(b))
			}
		}
		return list(out, nil)
	case "getDashboard":
		v, e := a.admin.Dashboard(ctx, actor)
		return ret(v, e)
	case "adminAttention":
		v, e := a.admin.Attention(ctx, actor)
		return ret(v, e)
	case "adminExportPII":
		v, e := a.admin.Export(ctx, actor)
		return ret(v, e)
	case "adminAssignmentCandidates":
		v, e := a.bookings.Candidates(ctx, actor, id)
		return ret(v, e)
	case "adminPreviewAssignment":
		v, e := a.bookings.Preview(ctx, actor, id, in)
		return ret(v, e)
	case "adminAssignBooking":
		v, e := a.bookings.Assign(ctx, actor, id, in)
		return ret(booking.PublicBooking(v), e)
	case "adminGetAssignmentEvents", "specialistOwnReassignmentEvents":
		sid := ""
		if rt.Operation == "specialistOwnReassignmentEvents" {
			sid = actor.SpecialistID
		}
		v, e := postgres.AssignmentEvents(ctx, a.store.Pool, id, sid)
		if sid != "" {
			for _, event := range v {
				for _, k := range []string{"fromSpecialistId", "toSpecialistId", "fromRoomId", "toRoomId"} {
					delete(event, k)
				}
			}
		}
		return list(v, e)
	case "adminListSpecialists":
		v, e := postgres.Specialists(ctx, a.store.Pool, a.studio, q.Get("includeArchived") == "true")
		return list(v, e)
	case "adminGetSpecialist":
		v, e := postgres.Specialist(ctx, a.store.Pool, id)
		return ret(v, e)
	case "adminCreateSpecialist", "adminUpdateSpecialist", "adminArchiveSpecialist":
		v, e := a.staff.Save(ctx, actor, id, in, rt.Operation == "adminArchiveSpecialist")
		return ret(v, e)
	case "adminGetSpecialistSchedule", "specialistMySchedule":
		v, e := a.staff.GetSchedule(ctx, actor, id, rt.Operation == "specialistMySchedule")
		return ret(v, e)
	case "adminSetSpecialistSchedule":
		v, e := a.staff.SetSchedule(ctx, actor, id, in)
		return ret(v, e)
	case "adminListRooms":
		v, e := postgres.Rooms(ctx, a.store.Pool, a.studio, q.Get("includeInactive") == "true")
		return list(v, e)
	case "adminCreateRoom", "adminUpdateRoom":
		v, e := a.rooms.Save(ctx, actor, id, in)
		return ret(v, e)
	case "adminListAbsences", "specialistMyAbsences":
		sid := ""
		if rt.Operation == "specialistMyAbsences" {
			sid = actor.SpecialistID
		}
		v, e := postgres.Absences(ctx, a.store.Pool, sid)
		return list(v, e)
	case "specialistSubmitAbsence":
		v, e := a.absences.Submit(ctx, actor, in)
		return ret(v, e)
	case "adminPreviewAbsenceApproval":
		v, e := a.absences.Preview(ctx, actor, id)
		return ret(v, e)
	case "adminApproveAbsence", "adminRejectAbsence", "specialistWithdrawAbsence":
		action := "approve"
		if rt.Operation == "adminRejectAbsence" {
			action = "reject"
		}
		if rt.Operation == "specialistWithdrawAbsence" {
			action = "withdraw"
		}
		v, e := a.absences.Decide(ctx, actor, id, action, in)
		return ret(v, e)
	case "adminListVacations":
		v, e := postgres.Vacations(ctx, a.store.Pool, a.studio, q.Get("contains"))
		return list(v, e)
	case "adminCreateVacation", "adminUpdateVacation", "adminDeleteVacation":
		if rt.Operation == "adminDeleteVacation" {
			revision, e := strconv.Atoi(r.Header.Get("If-Match"))
			if e != nil {
				return ret(nil, domain.Validation("If-Match"))
			}
			in["expectedRevision"] = revision
		}
		v, e := a.schedule.Vacation(ctx, actor, id, in, rt.Operation == "adminDeleteVacation")
		return ret(v, e)
	case "adminGetSimpleSchedule", "adminGetScheduleMode", "adminListScheduleDays", "adminGetScheduleDay", "getBookingWarnings":
		op := "simple"
		switch rt.Operation {
		case "adminGetScheduleMode":
			op = "mode"
		case "adminListScheduleDays":
			op = "days"
		case "adminGetScheduleDay":
			op = "day"
		case "getBookingWarnings":
			op = "warnings"
		}
		v, e := a.schedule.Read(ctx, op, date, q.Get("stage"), q.Get("from"), q.Get("to"))
		return ret(v, e)
	case "adminSetSimpleSchedule", "adminSetScheduleMode", "adminSetDraftDay", "adminSetDraftRange", "adminCopySimpleToDraft", "adminPublishSchedule", "adminRestoreVersionToDraft":
		ops := map[string]string{"adminSetSimpleSchedule": "simple", "adminSetScheduleMode": "mode", "adminSetDraftDay": "draft", "adminSetDraftRange": "range", "adminCopySimpleToDraft": "copy", "adminPublishSchedule": "publish", "adminRestoreVersionToDraft": "restore"}
		v, e := a.schedule.Mutate(ctx, actor, ops[rt.Operation], date, id, in)
		return ret(v, e)
	case "adminPreviewModeChange", "adminPreviewPublication", "adminPreviewRestore":
		op := "mode"
		if rt.Operation == "adminPreviewPublication" {
			op = "publish"
		}
		if rt.Operation == "adminPreviewRestore" {
			op = "restore"
		}
		v, e := a.schedule.Preview(ctx, actor, op, id, in)
		return ret(v, e)
	case "adminGetScheduleHistory":
		v, e := postgres.History(ctx, a.store.Pool, a.studio)
		return list(v, e)
	}
	return nil, 500, fmt.Errorf("operation wiring missing: %s", rt.Operation)
}
