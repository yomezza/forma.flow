package booking

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"github.com/yomezza/forma.flow/backend/internal/assignment"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"github.com/yomezza/forma.flow/backend/internal/schedule"
	"github.com/yomezza/forma.flow/backend/internal/storage/postgres"
	"sort"
	"time"
)

type Service struct {
	Store    *postgres.Store
	StudioID string
	Engine   schedule.Engine
}

func Receipt(b domain.M) domain.M {
	out := domain.M{}
	for _, k := range []string{"id", "serviceId", "name", "durationMinutes", "priceAmount", "currency", "date", "time", "status"} {
		out[k] = b[k]
	}
	return out
}
func PublicBooking(b domain.M) domain.M {
	out := domain.Clone(b)
	for _, k := range []string{"startsAt", "endsAt", "blockedUntil", "bufferMinutes", "completedBySpecialistId"} {
		delete(out, k)
	}
	for _, k := range []string{"confirmedAt", "cancelledAt", "cancelNote"} {
		if out[k] == nil {
			delete(out, k)
		}
	}
	return out
}
func SpecialistBooking(d domain.ScheduleData, b domain.M) domain.M {
	room := ""
	for _, r := range d.Rooms {
		if r.String("id") == b.String("roomId") {
			room = r.String("name")
		}
	}
	return domain.M{"id": b["id"], "clientName": b["clientName"], "serviceName": b["name"], "date": b["date"], "time": b["time"], "durationMinutes": b["durationMinutes"], "status": b["status"], "roomName": room}
}
func outcome(r assignment.Result) error {
	if r.Outcome == assignment.UNKNOWN {
		return domain.Fail("CAPACITY_CHECK_LIMIT", 503, "Не удалось завершить расчёт вместимости.")
	}
	if r.Outcome != assignment.SAT {
		return domain.Fail("SLOT_UNAVAILABLE", 409, "Это время недоступно.")
	}
	return nil
}
func (s *Service) Create(ctx context.Context, in domain.M, key string) (out domain.M, replayed bool, err error) {
	if len(key) < 16 || len(key) > 128 {
		return nil, false, domain.Validation("Idempotency-Key")
	}
	name, phone, e := domain.Identity(in)
	if e != nil {
		return nil, false, e
	}
	if !in.Bool("consentAccepted") || !domain.Text(in.String("consentVersion"), 1, 80) {
		return nil, false, domain.Validation("consent")
	}
	if _, e = schedule.Date(in.String("date")); e != nil {
		return nil, false, e
	}
	if _, e = schedule.Minute(in.String("time")); e != nil {
		return nil, false, e
	}
	canonical := domain.Clone(in)
	canonical["fullName"] = name
	canonical["phone"] = phone
	raw, _ := json.Marshal(canonical)
	h := sha256.Sum256(raw)
	hash := hex.EncodeToString(h[:])
	err = s.Store.InTx(ctx, func(db postgres.DB) error {
		saved, response, e := postgres.ReserveKey(ctx, db, key, hash)
		if e != nil {
			return e
		}
		if saved != hash {
			return domain.Fail("IDEMPOTENCY_CONFLICT", 409, "Этот ключ использован для другой заявки.")
		}
		if response != nil {
			out = response
			replayed = true
			return nil
		}
		if _, _, e = postgres.LockStudio(ctx, db, s.StudioID); e != nil {
			return e
		}
		service, e := postgres.Service(ctx, db, in.String("serviceId"))
		if e != nil {
			return e
		}
		if !service.Bool("isActive") {
			return domain.NotFound
		}
		d, e := postgres.Snapshot(ctx, db, s.StudioID, in.String("date"), in.String("date"))
		if e != nil {
			return e
		}
		slots, _, e := s.Engine.Slots(ctx, d, in.String("date"), service)
		if e != nil {
			return e
		}
		if !contains(slots, in.String("time")) {
			return domain.Fail("SLOT_UNAVAILABLE", 409, "Это время уже занято. Выберите другое время.")
		}
		loc, e := time.LoadLocation(d.Zone)
		if e != nil {
			return e
		}
		instant, e := schedule.Instant(in.String("date"), in.String("time"), loc)
		if e != nil {
			return e
		}
		m := domain.Clone(service)
		m["serviceId"] = service["id"]
		m["clientName"] = name
		m["clientPhone"] = phone
		m["consentVersion"] = in["consentVersion"]
		m["date"] = in["date"]
		m["time"] = in["time"]
		m["startsAt"] = instant.Format(time.RFC3339Nano)
		m["endsAt"] = instant.Add(time.Duration(service.Int("durationMinutes")) * time.Minute).Format(time.RFC3339Nano)
		m["bufferMinutes"] = d.Simple.Int("breakMinutes")
		m["blockedUntil"] = instant.Add(time.Duration(service.Int("durationMinutes")+m.Int("bufferMinutes")) * time.Minute).Format(time.RFC3339Nano)
		b, e := postgres.CreateBooking(ctx, db, s.StudioID, m)
		if e != nil {
			return e
		}
		if e = postgres.BookingEvent(ctx, db, b.String("id"), "", "", "pending_confirmation", ""); e != nil {
			return e
		}
		out = Receipt(b)
		if e = postgres.SaveKey(ctx, db, key, b.String("id"), out); e != nil {
			return e
		}
		return postgres.BumpCapacity(ctx, db, s.StudioID)
	})
	return
}
func contains(a []string, v string) bool {
	for _, x := range a {
		if x == v {
			return true
		}
	}
	return false
}
func active(b domain.M) bool {
	return b.String("status") == "pending_confirmation" || b.String("status") == "confirmed"
}
func (s *Service) LinkClient(ctx context.Context, a domain.Actor, id string, in domain.M) (out domain.M, err error) {
	if err = a.Admin(); err != nil {
		return
	}
	err = s.Store.InTx(ctx, func(db postgres.DB) error {
		b, e := postgres.Booking(ctx, db, id, true)
		if e != nil {
			return e
		}
		clientID := in.String("clientId")
		if (clientID != "") == (in.Object("newClient") != nil) {
			return domain.Validation("client association")
		}
		if clientID != "" {
			if _, e = postgres.Client(ctx, db, clientID); e != nil {
				return e
			}
		} else {
			n, p, e := domain.Identity(in.Object("newClient"))
			if e != nil {
				return e
			}
			c, e := postgres.CreateClient(ctx, db, n, p)
			if e != nil {
				return e
			}
			clientID = c.String("id")
		}
		if e = postgres.LinkClient(ctx, db, id, clientID); e != nil {
			return e
		}
		if e = postgres.Audit(ctx, db, a.AccountID, "booking.client_linked", "booking", id, domain.M{"previousClientId": b["clientId"], "clientId": clientID}); e != nil {
			return e
		}
		out, e = postgres.Booking(ctx, db, id, false)
		return e
	})
	return
}
func (s *Service) Transition(ctx context.Context, a domain.Actor, id, action, note string) (out domain.M, err error) {
	if !domain.Text(note, 0, 500) {
		return nil, domain.Validation("note")
	}
	special := action == "complete" || action == "undo-completion"
	if special {
		err = a.Specialist()
	} else {
		err = a.Admin()
	}
	if err != nil {
		return
	}
	err = s.Store.InTx(ctx, func(db postgres.DB) error {
		if _, _, e := postgres.LockStudio(ctx, db, s.StudioID); e != nil {
			return e
		}
		b, e := postgres.Booking(ctx, db, id, true)
		if e != nil {
			return e
		}
		previous := b.String("status")
		next := ""
		switch action {
		case "confirm":
			if previous == "confirmed" {
				out = b
				return nil
			}
			if previous != "pending_confirmation" {
				break
			}
			if b.String("clientId") == "" {
				return domain.Fail("CLIENT_LINK_REQUIRED", 409, "Сначала привяжите карточку клиента.")
			}
			next = "confirmed"
		case "cancel":
			if previous == "cancelled" {
				out = b
				return nil
			}
			if active(b) {
				next = "cancelled"
			}
		case "restore":
			if previous != "cancelled" {
				break
			}
			next = "pending_confirmation"
			d, e := postgres.Snapshot(ctx, db, s.StudioID, b.String("date"), b.String("date"))
			if e != nil {
				return e
			}
			rule, _ := schedule.ActiveRule(d, b.String("date"))
			minute, _ := schedule.Minute(b.String("time"))
			instant, _ := time.Parse(time.RFC3339Nano, b.String("startsAt"))
			if !instant.After(s.Engine.Clock.Now()) || !schedule.Fits(rule, minute, b.Int("durationMinutes")) {
				return domain.Fail("SLOT_UNAVAILABLE", 409, "Восстановление невозможно.")
			}
			result, _, e := s.Engine.Check(ctx, d, b, false)
			if e != nil {
				return e
			}
			if e = outcome(result); e != nil {
				return e
			}
		case "complete":
			if b.String("specialistId") != a.SpecialistID {
				return domain.NotFound
			}
			if previous != "confirmed" {
				break
			}
			end, _ := time.Parse(time.RFC3339Nano, b.String("endsAt"))
			if s.Engine.Clock.Now().Before(end) {
				return domain.Fail("NOT_ENDED_YET", 409, "Сеанс ещё не окончен.")
			}
			next = "completed"
		case "undo-completion":
			if b.String("completedBySpecialistId") != a.SpecialistID {
				return domain.Forbidden
			}
			if previous == "completed" {
				next = "confirmed"
			}
		}
		if next == "" {
			return domain.Fail("INVALID_STATUS_TRANSITION", 409, "Недопустимый переход статуса.")
		}
		if e = postgres.Status(ctx, db, id, next, note, a.SpecialistID); e != nil {
			return e
		}
		if e = postgres.BookingEvent(ctx, db, id, a.AccountID, previous, next, note); e != nil {
			return e
		}
		if special {
			verb := "completed"
			if action == "undo-completion" {
				verb = "undone"
			}
			if e = postgres.CompletionEvent(ctx, db, id, a.SpecialistID, verb, note); e != nil {
				return e
			}
		}
		if e = postgres.BumpCapacity(ctx, db, s.StudioID); e != nil {
			return e
		}
		if e = postgres.Audit(ctx, db, a.AccountID, "booking."+action, "booking", id, domain.M{"previousStatus": previous, "status": next}); e != nil {
			return e
		}
		if action == "cancel" {
			if e = postgres.ResolveException(ctx, db, id); e != nil {
				return e
			}
		}
		if action == "cancel" {
			d, e := postgres.Snapshot(ctx, db, s.StudioID, b.String("date"), b.String("date"))
			if e != nil {
				return e
			}
			if e = s.Resolve(ctx, db, d); e != nil {
				return e
			}
		}
		out, e = postgres.Booking(ctx, db, id, false)
		return e
	})
	return
}

// EqualIDs requires an exact set, rejecting duplicate acknowledgements.
func EqualIDs(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	x := append([]string{}, a...)
	y := append([]string{}, b...)
	sort.Strings(x)
	sort.Strings(y)
	for i := range x {
		if x[i] != y[i] || (i > 0 && y[i] == y[i-1]) {
			return false
		}
	}
	return true
}
