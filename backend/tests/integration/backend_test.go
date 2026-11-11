package integration

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	osExec "os/exec"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/yomezza/forma.flow/backend/internal/auth"
	"github.com/yomezza/forma.flow/backend/internal/booking"
	"github.com/yomezza/forma.flow/backend/internal/config"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"github.com/yomezza/forma.flow/backend/internal/schedule"
	"github.com/yomezza/forma.flow/backend/internal/storage/postgres"
	httptransport "github.com/yomezza/forma.flow/backend/internal/transport/http"
)

const origin = "http://forma.test"
const password = "test-owner-password-long"

type testClock struct{ n atomic.Int64 }

func (c *testClock) Now() time.Time  { return time.Unix(c.n.Load(), 0).UTC() }
func (c *testClock) Set(v time.Time) { c.n.Store(v.Unix()) }

type fixture struct {
	store              *postgres.Store
	dsn, studio        string
	cfg                config.Config
	clock              *testClock
	actor              domain.Actor
	specialists, rooms []string
	api                *httptest.Server
}

func setup(t *testing.T, n int) *fixture {
	t.Helper()
	base := os.Getenv("INTEGRATION_DATABASE_URL")
	if base == "" {
		t.Skip("INTEGRATION_DATABASE_URL required for real PostgreSQL tests")
	}
	ctx := context.Background()
	admin, e := pgx.Connect(ctx, base)
	if e != nil {
		t.Fatal(e)
	}
	name := "forma_test_" + strings.ReplaceAll(domain.ID(), "-", "")
	if _, e = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{name}.Sanitize()); e != nil {
		t.Fatal(e)
	}
	u, e := url.Parse(base)
	if e != nil {
		t.Fatal(e)
	}
	u.Path = "/" + name
	dsn := u.String()
	t.Cleanup(func() {
		admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)")
		admin.Close(ctx)
	})
	db, e := sql.Open("pgx", dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	goose.SetDialect("postgres")
	if e = goose.UpContext(ctx, db, filepath.Join("..", "..", "migrations")); e != nil {
		t.Fatal(e)
	}
	store, e := postgres.Open(ctx, dsn, 30)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(store.Pool.Close)
	hash, e := auth.Hash(password)
	if e != nil {
		t.Fatal(e)
	}
	if e = postgres.Bootstrap(ctx, store, "Etc/UTC", "owner@forma.test", hash); e != nil {
		t.Fatal(e)
	}
	studio, e := store.Studio(ctx, "Etc/UTC")
	if e != nil {
		t.Fatal(e)
	}
	f := &fixture{store: store, dsn: dsn, studio: studio, cfg: config.Config{DatabaseURL: dsn, PublicOrigin: origin, StudioTimeZone: "Etc/UTC", SolverNodes: 100000, MaxComponent: 100, PoolMax: 30, SessionTTL: time.Hour}, clock: &testClock{}}
	f.clock.Set(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	account, e := postgres.FindAccount(ctx, store.Pool, "owner@forma.test")
	if e != nil {
		t.Fatal(e)
	}
	f.actor, e = postgres.ActorRoles(ctx, store.Pool, account.ID)
	if e != nil {
		t.Fatal(e)
	}
	exec(t, store, `UPDATE simple_schedule SET work_days='{0,1,2,3,4,5,6}',day_start='09:00',day_end='18:00',step_minutes=30 WHERE resource_id=$1`, studio)
	for _, id := range []string{"A", "B"} {
		exec(t, store, `INSERT INTO services(id,name,description,duration_minutes,price_amount) VALUES($1,$1,'test',60,3500)`, id)
	}
	rows, e := postgres.Rooms(ctx, store.Pool, studio, true)
	if e != nil {
		t.Fatal(e)
	}
	f.rooms = append(f.rooms, rows[0].String("id"))
	for i := 1; i < n; i++ {
		r, e := postgres.SaveRoom(ctx, store.Pool, studio, "", domain.M{"name": fmt.Sprintf("Room %d", i), "sortOrder": i})
		if e != nil {
			t.Fatal(e)
		}
		f.rooms = append(f.rooms, r.String("id"))
	}
	for i := 0; i < n; i++ {
		var id string
		e = store.Pool.QueryRow(ctx, "INSERT INTO specialists(resource_id,full_name) VALUES($1,$2) RETURNING id::text", studio, fmt.Sprintf("Specialist %d", i)).Scan(&id)
		if e != nil {
			t.Fatal(e)
		}
		f.specialists = append(f.specialists, id)
		for day := 0; day < 7; day++ {
			exec(t, store, "INSERT INTO specialist_weekly_intervals VALUES($1,$2,540,1140)", id, day)
		}
		exec(t, store, "INSERT INTO specialist_services VALUES($1,'A'),($1,'B')", id)
	}
	f.api = f.server(t)
	return f
}
func exec(t *testing.T, s *postgres.Store, q string, args ...any) {
	t.Helper()
	if _, e := s.Pool.Exec(context.Background(), q, args...); e != nil {
		t.Fatal(e)
	}
}
func (f *fixture) server(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(httptransport.API(f.store, slog.New(slog.NewTextHandler(io.Discard, nil)), f.cfg, f.studio, f.clock))
	t.Cleanup(server.Close)
	return server
}

type session struct {
	cookie *http.Cookie
	csrf   string
}

func request(t *testing.T, server *httptest.Server, method, path string, body any, sess *session, extra map[string]string) (int, domain.M) {
	t.Helper()
	var raw []byte
	if body != nil {
		raw, _ = json.Marshal(body)
	}
	req, e := http.NewRequest(method, server.URL+path, bytes.NewReader(raw))
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", origin)
	if sess != nil {
		req.AddCookie(sess.cookie)
		req.Header.Set("X-CSRF-Token", sess.csrf)
	}
	for k, v := range extra {
		req.Header.Set(k, v)
	}
	httpClient := server.Client()
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 20 * time.Second}
	}
	resp, e := httpClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	m := domain.M{}
	if resp.StatusCode != 204 {
		if e = json.NewDecoder(resp.Body).Decode(&m); e != nil {
			t.Fatalf("%s %s status %d: invalid JSON: %v", method, path, resp.StatusCode, e)
		}
	}
	return resp.StatusCode, m
}
func login(t *testing.T, f *fixture, email, pass string) (*session, domain.M) {
	t.Helper()
	b, _ := json.Marshal(domain.M{"email": email, "password": pass})
	req, _ := http.NewRequest("POST", f.api.URL+"/api/v1/auth/sessions", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", origin)
	resp, e := f.api.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	var m domain.M
	json.NewDecoder(resp.Body).Decode(&m)
	if resp.StatusCode != 201 {
		t.Fatalf("login %d %+v", resp.StatusCode, m)
	}
	cookies := resp.Cookies()
	if len(cookies) != 1 || !cookies[0].HttpOnly {
		t.Fatal("invalid session cookie")
	}
	return &session{cookie: cookies[0], csrf: m.String("csrfToken")}, m
}
func payload(clock string) domain.M {
	return domain.M{"serviceId": "A", "date": "2027-01-02", "time": clock, "fullName": "Тестовый Клиент", "phone": "+79991234567", "consentAccepted": true, "consentVersion": "v1"}
}
func create(t *testing.T, f *fixture, clock string) domain.M {
	t.Helper()
	status, m := request(t, f.api, "POST", "/api/v1/booking-requests", payload(clock), nil, map[string]string{"Idempotency-Key": domain.ID()})
	if status != 201 {
		t.Fatalf("create status %d %+v", status, m)
	}
	return m
}
func assertStatus(t *testing.T, status, want int, m domain.M) {
	t.Helper()
	if status != want {
		t.Fatalf("status %d want %d: %+v", status, want, m)
	}
}
func TestConcurrentBookingAcrossInstances(t *testing.T) {
	for _, capacity := range []int{1, 2} {
		t.Run(fmt.Sprint(capacity), func(t *testing.T) {
			f := setup(t, capacity)
			second := f.server(t)
			servers := []*httptest.Server{f.api, second}
			var wg sync.WaitGroup
			gate := make(chan struct{})
			var accepted, rejected atomic.Int32
			for i := 0; i < 20; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-gate
					status, m := request(t, servers[i%2], "POST", "/api/v1/booking-requests", payload("10:00"), nil, map[string]string{"Idempotency-Key": domain.ID()})
					if status == 201 {
						accepted.Add(1)
					} else if status == 409 && m.Object("error").String("code") == "SLOT_UNAVAILABLE" {
						rejected.Add(1)
					} else {
						t.Errorf("unexpected concurrent response %d %+v", status, m)
					}
				}(i)
			}
			close(gate)
			wg.Wait()
			if int(accepted.Load()) != capacity || int(rejected.Load()) != 20-capacity {
				t.Fatalf("accepted %d rejected %d", accepted.Load(), rejected.Load())
			}
			var count int
			if e := f.store.Pool.QueryRow(context.Background(), "SELECT count(*) FROM bookings").Scan(&count); e != nil || count != capacity {
				t.Fatalf("database count %d: %v", count, e)
			}
		})
	}
}
func TestIdempotencyAndSharedPhone(t *testing.T) {
	f := setup(t, 1)
	second := f.server(t)
	var wg sync.WaitGroup
	key := domain.ID()
	ids := make(chan string, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			server := f.api
			if i%2 == 1 {
				server = second
			}
			status, m := request(t, server, "POST", "/api/v1/booking-requests", payload("10:00"), nil, map[string]string{"Idempotency-Key": key})
			if status != 200 && status != 201 {
				t.Errorf("idempotency %d %+v", status, m)
				return
			}
			ids <- m.String("id")
		}(i)
	}
	wg.Wait()
	close(ids)
	first := ""
	for id := range ids {
		if first == "" {
			first = id
		}
		if first != id {
			t.Fatal("duplicate booking")
		}
	}
	if first == "" {
		t.Fatal("no successful idempotent response")
	}
	status, m := request(t, f.api, "POST", "/api/v1/booking-requests", payload("11:00"), nil, map[string]string{"Idempotency-Key": key})
	assertStatus(t, status, 409, m)
	admin, _ := login(t, f, "owner@forma.test", password)
	status, m = request(t, f.api, "POST", "/api/v1/admin/bookings/"+first+"/confirm", nil, admin, nil)
	assertStatus(t, status, 409, m)
	if m.Object("error").String("code") != "CLIENT_LINK_REQUIRED" {
		t.Fatal(m)
	}
	clientIDs := []string{}
	for _, name := range []string{"Иван Петров", "Мария Петрова", "Иван Петров"} {
		status, m = request(t, f.api, "POST", "/api/v1/admin/clients", domain.M{"fullName": name, "phone": "+79991234567"}, admin, nil)
		assertStatus(t, status, 201, m)
		clientIDs = append(clientIDs, m.String("id"))
	}
	if clientIDs[0] == clientIDs[2] {
		t.Fatal("identity merged")
	}
	status, m = request(t, f.api, "GET", "/api/v1/admin/clients/by-phone?phone=%2B79991234567", nil, admin, nil)
	assertStatus(t, status, 200, m)
	if len(m["items"].([]any)) != 3 {
		t.Fatal(m)
	}
	status, m = request(t, f.api, "PUT", "/api/v1/admin/bookings/"+first+"/client", domain.M{"clientId": clientIDs[0]}, admin, nil)
	assertStatus(t, status, 200, m)
	status, m = request(t, f.api, "POST", "/api/v1/admin/bookings/"+first+"/confirm", nil, admin, nil)
	assertStatus(t, status, 200, m)
	if m["specialistId"] != nil {
		t.Fatal("confirmation assigned staff")
	}
	status, m = request(t, f.api, "POST", "/api/v1/booking-requests", payload("10:00"), nil, map[string]string{"Idempotency-Key": key})
	assertStatus(t, status, 200, m)
	if m.String("status") != "pending_confirmation" || m.Has("clientPhone") || m.Has("clientId") || m.Has("specialistId") {
		t.Fatal("receipt changed or leaked PII", m)
	}
	secondBooking := create(t, f, "12:00")
	status, m = request(t, f.api, "PUT", "/api/v1/admin/bookings/"+secondBooking.String("id")+"/client", domain.M{"clientId": clientIDs[1]}, admin, nil)
	assertStatus(t, status, 200, m)
	status, m = request(t, f.api, "GET", "/api/v1/admin/clients/"+clientIDs[0]+"/bookings", nil, admin, nil)
	assertStatus(t, status, 200, m)
	if len(m["items"].([]any)) != 1 {
		t.Fatal("mixed client histories", m)
	}
	_ = booking.Receipt
	_ = schedule.Engine{}
}

func TestStaffSecurityAssignmentCompletionAndAbsence(t *testing.T) {
	f := setup(t, 2)
	admin, _ := login(t, f, "owner@forma.test", password)
	b := create(t, f, "10:00")
	path := "/api/v1/admin/bookings/" + b.String("id")
	status, m := request(t, f.api, "GET", "/api/v1/admin/clients", nil, nil, nil)
	assertStatus(t, status, 401, m)
	status, m = request(t, f.api, "POST", "/api/v1/admin/clients", domain.M{"fullName": "Иван Петров", "phone": "+79991234567"}, admin, map[string]string{"X-CSRF-Token": "wrong"})
	assertStatus(t, status, 403, m)
	status, m = request(t, f.api, "POST", "/api/v1/admin/staff-invitations", domain.M{"email": "specialist@forma.test", "roles": []string{"specialist"}, "specialistId": f.specialists[0]}, admin, nil)
	assertStatus(t, status, 201, m)
	temporary := m.String("temporaryPassword")
	specialSession, first := login(t, f, "specialist@forma.test", temporary)
	if !first.Bool("mustChangePassword") {
		t.Fatal("missing first login gate")
	}
	status, m = request(t, f.api, "GET", "/api/v1/specialist/me/bookings", nil, specialSession, nil)
	assertStatus(t, status, 403, m)
	if m.Object("error").String("code") != "FIRST_LOGIN_PASSWORD_CHANGE_REQUIRED" {
		t.Fatal(m)
	}
	// Change password and retain rotated cookie from the HTTP response.
	raw, _ := json.Marshal(domain.M{"currentPassword": temporary, "newPassword": "specialist-new-password-long"})
	req, _ := http.NewRequest("POST", f.api.URL+"/api/v1/auth/password/change", bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", origin)
	req.Header.Set("X-CSRF-Token", specialSession.csrf)
	req.AddCookie(specialSession.cookie)
	resp, e := f.api.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	var changed domain.M
	json.NewDecoder(resp.Body).Decode(&changed)
	resp.Body.Close()
	assertStatus(t, resp.StatusCode, 200, changed)
	if changed.Bool("mustChangePassword") {
		t.Fatal("gate not cleared")
	}
	newSession := &session{cookie: resp.Cookies()[0], csrf: changed.String("csrfToken")}
	status, m = request(t, f.api, "GET", "/api/v1/auth/sessions/current", nil, specialSession, nil)
	assertStatus(t, status, 401, m)
	status, m = request(t, f.api, "PUT", path+"/client", domain.M{"newClient": domain.M{"fullName": "Иван Петров", "phone": "+79991234567"}}, admin, nil)
	assertStatus(t, status, 200, m)
	status, m = request(t, f.api, "POST", path+"/confirm", nil, admin, nil)
	assertStatus(t, status, 200, m)
	status, m = request(t, f.api, "GET", path+"/assignment/candidates", nil, admin, nil)
	assertStatus(t, status, 200, m)
	revision := m.Int("capacityRevision")
	assign := domain.M{"specialistId": f.specialists[0], "expectedCapacityRevision": revision}
	status, m = request(t, f.api, "POST", path+"/assignment/preview", assign, admin, nil)
	assertStatus(t, status, 200, m)
	if !m.Bool("allowed") {
		t.Fatal(m)
	}
	status, m = request(t, f.api, "PUT", path+"/assignment", assign, admin, nil)
	assertStatus(t, status, 200, m)
	status, m = request(t, f.api, "GET", "/api/v1/specialist/me/bookings", nil, newSession, nil)
	assertStatus(t, status, 200, m)
	raw, _ = json.Marshal(m)
	if strings.Contains(string(raw), "clientPhone") || strings.Contains(string(raw), "clientId") || strings.Contains(string(raw), "79991234567") || len(m["items"].([]any)) != 1 {
		t.Fatal("specialist privacy", m)
	}
	status, m = request(t, f.api, "GET", "/api/v1/admin/clients", nil, newSession, nil)
	assertStatus(t, status, 403, m)
	own := "/api/v1/specialist/me/bookings/" + b.String("id")
	status, m = request(t, f.api, "POST", own+"/complete", nil, admin, nil)
	assertStatus(t, status, 403, m)
	status, m = request(t, f.api, "POST", own+"/complete", nil, newSession, nil)
	assertStatus(t, status, 409, m)
	if m.Object("error").String("code") != "NOT_ENDED_YET" {
		t.Fatal(m)
	}
	f.clock.Set(time.Date(2027, 1, 2, 11, 0, 0, 0, time.UTC))
	status, m = request(t, f.api, "POST", own+"/complete", nil, newSession, nil)
	assertStatus(t, status, 200, m)
	f.clock.Set(time.Date(2027, 2, 2, 11, 0, 0, 0, time.UTC))
	status, m = request(t, f.api, "POST", own+"/undo-completion", domain.M{"note": "исправление"}, newSession, nil)
	assertStatus(t, status, 200, m)
	if m.String("status") != "confirmed" {
		t.Fatal(m)
	}
	f.clock.Set(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	status, m = request(t, f.api, "POST", "/api/v1/specialist/me/absence-requests", domain.M{"kind": "day_off", "scope": "partial_day", "startDate": "2027-01-02", "endDate": "2027-01-02", "startTime": "10:00", "endTime": "12:00"}, newSession, nil)
	assertStatus(t, status, 201, m)
	absenceID := m.String("id")
	status, m = request(t, f.api, "POST", "/api/v1/admin/absence-requests/"+absenceID+"/preview", nil, admin, nil)
	assertStatus(t, status, 200, m)
	revision = m.Int("capacityRevision")
	if len(m["items"].([]any)) != 1 {
		t.Fatal("missing conflict", m)
	}
	status, m = request(t, f.api, "POST", "/api/v1/admin/absence-requests/"+absenceID+"/approve", domain.M{"expectedCapacityRevision": revision}, admin, nil)
	assertStatus(t, status, 409, m)
	status, m = request(t, f.api, "POST", "/api/v1/admin/absence-requests/"+absenceID+"/approve", domain.M{"expectedCapacityRevision": revision, "acknowledgedBookingIds": []string{b.String("id")}}, admin, nil)
	assertStatus(t, status, 200, m)
	status, m = request(t, f.api, "GET", "/api/v1/admin/attention", nil, admin, nil)
	assertStatus(t, status, 200, m)
	if len(m["items"].([]any)) == 0 {
		t.Fatal("missing attention")
	}
	// Existing appointment survives leave; new requests in its overlap component fail.
	status, m = request(t, f.api, "POST", "/api/v1/booking-requests", payload("10:30"), nil, map[string]string{"Idempotency-Key": domain.ID()})
	assertStatus(t, status, 409, m)
	status, m = request(t, f.api, "POST", "/api/v1/booking-requests", payload("14:00"), nil, map[string]string{"Idempotency-Key": domain.ID()})
	assertStatus(t, status, 201, m)
}

func TestIndirectAssignmentAndDirectSQLConstraints(t *testing.T) {
	f := setup(t, 2)
	admin, _ := login(t, f, "owner@forma.test", password)
	exec(t, f.store, "DELETE FROM specialist_services WHERE specialist_id=$1 AND service_id='B'", f.specialists[1])
	a := create(t, f, "10:00")
	input := payload("10:00")
	input["serviceId"] = "B"
	status, b := request(t, f.api, "POST", "/api/v1/booking-requests", input, nil, map[string]string{"Idempotency-Key": domain.ID()})
	assertStatus(t, status, 201, b)
	path := "/api/v1/admin/bookings/" + a.String("id") + "/assignment"
	status, m := request(t, f.api, "GET", path+"/candidates", nil, admin, nil)
	assertStatus(t, status, 200, m)
	revision := m.Int("capacityRevision")
	in := domain.M{"specialistId": f.specialists[0], "expectedCapacityRevision": revision}
	status, m = request(t, f.api, "POST", path+"/preview", in, admin, nil)
	assertStatus(t, status, 200, m)
	if !m.Bool("requiresConfirmation") || len(m["affectedBookingIds"].([]any)) != 1 {
		t.Fatal("indirect conflict missing", m)
	}
	status, m = request(t, f.api, "PUT", path, in, admin, nil)
	assertStatus(t, status, 409, m)
	in["acknowledgedBookingIds"] = []string{b.String("id")}
	status, m = request(t, f.api, "PUT", path, in, admin, nil)
	assertStatus(t, status, 200, m)
	status, m = request(t, f.api, "POST", "/api/v1/booking-requests", payload("10:30"), nil, map[string]string{"Idempotency-Key": domain.ID()})
	assertStatus(t, status, 409, m)
	// DB rejects direct specialist and room collisions regardless of service layer.
	ctx := context.Background()
	for _, pair := range [][2]string{{f.specialists[0], f.rooms[1]}, {f.specialists[1], f.rooms[0]}} {
		_, e := f.store.Pool.Exec(ctx, "UPDATE bookings SET specialist_id=$2,room_id=$3 WHERE id=$1", b.String("id"), pair[0], pair[1])
		if e == nil {
			t.Fatal("EXCLUDE bypassed")
		}
		var state interface{ SQLState() string }
		if !errors.As(e, &state) || state.SQLState() != "23P01" {
			t.Fatalf("wanted 23P01 got %v", e)
		}
	}
	for _, query := range []string{"UPDATE bookings SET ends_at=ends_at+interval '1 minute' WHERE id=$1", "UPDATE bookings SET blocked_until=blocked_until+interval '1 minute' WHERE id=$1", "UPDATE bookings SET room_id=NULL WHERE id=$1"} {
		if _, e := f.store.Pool.Exec(ctx, query, a.String("id")); e == nil {
			t.Fatal("CHECK bypassed", query)
		}
	}
	// Cancelling the displaced booking leaves unrelated later capacity available.
	status, m = request(t, f.api, "POST", "/api/v1/admin/bookings/"+b.String("id")+"/cancel", domain.M{}, admin, nil)
	assertStatus(t, status, 200, m)
}

func TestOpenAPIConformance(t *testing.T) {
	f := setup(t, 2)
	control := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var m struct {
			Now string `json:"now"`
		}
		if e := json.NewDecoder(r.Body).Decode(&m); e != nil {
			http.Error(w, "bad JSON", 400)
			return
		}
		now, e := time.Parse(time.RFC3339, m.Now)
		if e != nil {
			http.Error(w, "bad time", 400)
			return
		}
		f.clock.Set(now)
		w.WriteHeader(204)
	}))
	defer control.Close()
	cmd := osExec.Command("python3", "conformance.py")
	cmd.Env = append(os.Environ(), "CONFORMANCE_URL="+f.api.URL, "CLOCK_CONTROL_URL="+control.URL)
	out, e := cmd.CombinedOutput()
	if e != nil {
		t.Fatalf("OpenAPI conformance: %v\n%s", e, out)
	}
	t.Log(strings.TrimSpace(string(out)))
}

// The helper runs in a separate OS process, with its own pgxpool and memory.
func TestServerProcess(t *testing.T) {
	if os.Getenv("FORMA_TEST_PROCESS") != "1" {
		t.Skip("subprocess helper")
	}
	ctx := context.Background()
	store, e := postgres.Open(ctx, os.Getenv("FORMA_PROCESS_DSN"), 15)
	if e != nil {
		t.Fatal(e)
	}
	defer store.Pool.Close()
	studio, e := store.Studio(ctx, "Etc/UTC")
	if e != nil {
		t.Fatal(e)
	}
	cfg := config.Config{PublicOrigin: origin, StudioTimeZone: "Etc/UTC", SolverNodes: 100000, MaxComponent: 100, SessionTTL: time.Hour}
	c := &testClock{}
	c.Set(time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC))
	ln, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		t.Fatal(e)
	}
	fmt.Println("FORMA_PROCESS_URL=http://" + ln.Addr().String())
	server := &http.Server{Handler: httptransport.API(store, slog.New(slog.NewTextHandler(io.Discard, nil)), cfg, studio, c), ReadHeaderTimeout: time.Second}
	if e = server.Serve(ln); e != nil {
		t.Fatal(e)
	}
}
func startProcess(t *testing.T, f *fixture) *httptest.Server {
	t.Helper()
	executable, e := os.Executable()
	if e != nil {
		t.Fatal(e)
	}
	cmd := osExec.Command(executable, "-test.run=^TestServerProcess$", "-test.v")
	cmd.Env = append(os.Environ(), "FORMA_TEST_PROCESS=1", "FORMA_PROCESS_DSN="+f.dsn)
	pipe, e := cmd.StdoutPipe()
	if e != nil {
		t.Fatal(e)
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if e = cmd.Start(); e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { cmd.Process.Kill(); cmd.Wait() })
	lines := make(chan string, 1)
	go func() {
		scanner := bufio.NewScanner(pipe)
		for scanner.Scan() {
			if strings.HasPrefix(scanner.Text(), "FORMA_PROCESS_URL=") {
				lines <- strings.TrimPrefix(scanner.Text(), "FORMA_PROCESS_URL=")
				return
			}
		}
		close(lines)
	}()
	select {
	case endpoint := <-lines:
		if endpoint == "" {
			t.Fatalf("subprocess did not start: %s", stderr.String())
		}
		return &httptest.Server{URL: endpoint}
	case <-time.After(20 * time.Second):
		t.Fatal("subprocess startup timeout")
	}
	return nil
}
func TestTwoIndependentProcesses(t *testing.T) {
	f := setup(t, 1)
	servers := []*httptest.Server{startProcess(t, f), startProcess(t, f)}
	gate := make(chan struct{})
	var wg sync.WaitGroup
	var successes atomic.Int32
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-gate
			status, m := request(t, servers[i%2], "POST", "/api/v1/booking-requests", payload("10:00"), nil, map[string]string{"Idempotency-Key": domain.ID()})
			if status == 201 {
				successes.Add(1)
			} else if status != 409 {
				t.Errorf("subprocess %d %+v", status, m)
			}
		}(i)
	}
	close(gate)
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatal("multi-process overbooking", successes.Load())
	}
	key := domain.ID()
	ids := make(chan string, 20)
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			status, m := request(t, servers[i%2], "POST", "/api/v1/booking-requests", payload("14:00"), nil, map[string]string{"Idempotency-Key": key})
			if status != 200 && status != 201 {
				t.Errorf("multi-process replay %d %+v", status, m)
				return
			}
			ids <- m.String("id")
		}(i)
	}
	wg.Wait()
	close(ids)
	first := ""
	count := 0
	for id := range ids {
		if first == "" {
			first = id
		}
		if id != first {
			t.Fatal("multiple effects for one idempotency key across OS processes")
		}
		count++
	}
	if count != 20 {
		t.Fatalf("replayed %d/20 requests", count)
	}

}

func TestAtomicRollbackAndMigrationCycle(t *testing.T) {
	f := setup(t, 1)
	ctx := context.Background()
	exec(t, f.store, `CREATE FUNCTION fail_booking_event() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test rollback'; END $$; CREATE TRIGGER fail_event BEFORE INSERT ON booking_events FOR EACH ROW EXECUTE FUNCTION fail_booking_event()`)
	status, m := request(t, f.api, "POST", "/api/v1/booking-requests", payload("10:00"), nil, map[string]string{"Idempotency-Key": domain.ID()})
	assertStatus(t, status, 500, m)
	for _, table := range []string{"bookings", "idempotency_keys"} {
		var count int
		if e := f.store.Pool.QueryRow(ctx, "SELECT count(*) FROM "+table).Scan(&count); e != nil || count != 0 {
			t.Fatal("partial transaction", table, count, e)
		}
	}
	exec(t, f.store, "DROP TRIGGER fail_event ON booking_events; DROP FUNCTION fail_booking_event()")
	b := create(t, f, "10:00")
	admin, _ := login(t, f, "owner@forma.test", password)
	exec(t, f.store, `CREATE FUNCTION fail_booking_update() RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN RAISE EXCEPTION 'test rollback'; END $$; CREATE TRIGGER fail_update BEFORE UPDATE ON bookings FOR EACH ROW EXECUTE FUNCTION fail_booking_update()`)
	status, m = request(t, f.api, "PUT", "/api/v1/admin/bookings/"+b.String("id")+"/client", domain.M{"newClient": domain.M{"fullName": "Иван Петров", "phone": "+79991234567"}}, admin, nil)
	assertStatus(t, status, 500, m)
	var count int
	if e := f.store.Pool.QueryRow(ctx, "SELECT count(*) FROM clients").Scan(&count); e != nil || count != 0 {
		t.Fatal("orphan client", count, e)
	}
	exec(t, f.store, "DROP TRIGGER fail_update ON bookings; DROP FUNCTION fail_booking_update()")
	db, e := sql.Open("pgx", f.dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	dir := filepath.Join("..", "..", "migrations")
	if e = goose.UpContext(ctx, db, dir); e != nil {
		t.Fatal(e)
	}
	if e = goose.DownToContext(ctx, db, dir, 0); e != nil {
		t.Fatal(e)
	}
	if e = goose.UpContext(ctx, db, dir); e != nil {
		t.Fatal(e)
	}
	if e = f.store.CheckSchema(ctx); e != nil {
		t.Fatal(e)
	}
}

func prepareAdvanced(t *testing.T, f *fixture, admin *session) {
	t.Helper()
	rule := domain.M{"kind": "working", "intervals": []domain.M{{"start": "09:00", "end": "18:00"}}, "stepOverride": nil}
	status, m := request(t, f.api, "PUT", "/api/v1/admin/schedule/drafts/2027-01-02", domain.M{"rule": rule, "expectedRevision": 0}, admin, nil)
	assertStatus(t, status, 200, m)
	status, m = request(t, f.api, "POST", "/api/v1/admin/schedule/publications", domain.M{"from": "2027-01-02", "to": "2027-01-02", "expectedRevision": 1}, admin, nil)
	assertStatus(t, status, 200, m)
	status, m = request(t, f.api, "PUT", "/api/v1/admin/schedule/mode", domain.M{"mode": "advanced", "expectedRevision": 2}, admin, nil)
	assertStatus(t, status, 200, m)
}
func TestPublishedScheduleConflictsAndRestore(t *testing.T) {
	f := setup(t, 1)
	admin, _ := login(t, f, "owner@forma.test", password)
	prepareAdvanced(t, f, admin)
	b := create(t, f, "10:00")
	off := domain.M{"kind": "off", "intervals": []domain.M{}, "stepOverride": nil}
	status, m := request(t, f.api, "PUT", "/api/v1/admin/schedule/drafts/2027-01-02", domain.M{"rule": off, "expectedRevision": 3}, admin, nil)
	assertStatus(t, status, 200, m)
	// Drafts do not change public availability.
	status, m = request(t, f.api, "GET", "/api/v1/availability/slots?date=2027-01-02&serviceId=A", nil, nil, nil)
	assertStatus(t, status, 200, m)
	if len(m["slots"].([]any)) == 0 {
		t.Fatal("draft leaked into availability")
	}
	publication := domain.M{"from": "2027-01-02", "to": "2027-01-02", "expectedRevision": 3}
	status, m = request(t, f.api, "POST", "/api/v1/admin/schedule/publications", publication, admin, nil)
	assertStatus(t, status, 409, m)
	if m.Object("error").String("code") != "VERSION_CONFLICT" {
		t.Fatal(m)
	}
	publication["expectedRevision"] = 4
	status, m = request(t, f.api, "POST", "/api/v1/admin/schedule/publications", publication, admin, nil)
	assertStatus(t, status, 409, m)
	publication["acknowledgedBookingIds"] = []string{b.String("id")}
	status, m = request(t, f.api, "POST", "/api/v1/admin/schedule/publications", publication, admin, nil)
	assertStatus(t, status, 200, m)
	status, m = request(t, f.api, "GET", "/api/v1/admin/schedule/warnings", nil, admin, nil)
	assertStatus(t, status, 200, m)
	if len(m["bookingIds"].([]any)) != 1 {
		t.Fatal("missing outside-schedule warning", m)
	}
	status, m = request(t, f.api, "GET", "/api/v1/admin/schedule/history", nil, admin, nil)
	assertStatus(t, status, 200, m)
	entry := m["items"].([]any)[0].(map[string]any)["id"].(string)
	status, m = request(t, f.api, "POST", "/api/v1/admin/schedule/history/"+entry+"/restore", domain.M{"expectedRevision": 5}, admin, nil)
	assertStatus(t, status, 200, m)
	status, m = request(t, f.api, "GET", "/api/v1/availability/slots?date=2027-01-02&serviceId=A", nil, nil, nil)
	assertStatus(t, status, 200, m)
	if len(m["slots"].([]any)) != 0 {
		t.Fatal("history restore published automatically")
	}
}
func TestPublicationVsBookingRace(t *testing.T) {
	f := setup(t, 1)
	admin, _ := login(t, f, "owner@forma.test", password)
	prepareAdvanced(t, f, admin)
	status, m := request(t, f.api, "PUT", "/api/v1/admin/schedule/drafts/2027-01-02", domain.M{"rule": domain.M{"kind": "off", "intervals": []domain.M{}, "stepOverride": nil}, "expectedRevision": 3}, admin, nil)
	assertStatus(t, status, 200, m)
	gate := make(chan struct{})
	var wg sync.WaitGroup
	statuses := make(chan int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-gate
			if i == 0 {
				status, _ := request(t, f.api, "POST", "/api/v1/booking-requests", payload("10:00"), nil, map[string]string{"Idempotency-Key": domain.ID()})
				statuses <- status
			} else {
				status, _ := request(t, f.api, "POST", "/api/v1/admin/schedule/publications", domain.M{"from": "2027-01-02", "to": "2027-01-02", "expectedRevision": 4}, admin, nil)
				statuses <- status
			}
		}(i)
	}
	close(gate)
	wg.Wait()
	close(statuses)
	successes, conflicts := 0, 0
	for status := range statuses {
		if status == 200 || status == 201 {
			successes++
		} else if status == 409 {
			conflicts++
		} else {
			t.Fatal(status)
		}
	}
	if successes != 1 || conflicts != 1 {
		t.Fatalf("publication raced past studio lock: successes=%d conflicts=%d", successes, conflicts)
	}
}
func TestUnknownCapacityFailsClosed(t *testing.T) {
	f := setup(t, 1)
	f.cfg.SolverNodes = 0
	f.api = f.server(t)
	status, m := request(t, f.api, "POST", "/api/v1/booking-requests", payload("10:00"), nil, map[string]string{"Idempotency-Key": domain.ID()})
	assertStatus(t, status, 503, m)
	if m.Object("error").String("code") != "CAPACITY_CHECK_LIMIT" {
		t.Fatal(m)
	}
	var count int
	if e := f.store.Pool.QueryRow(context.Background(), "SELECT count(*) FROM bookings").Scan(&count); e != nil || count != 0 {
		t.Fatal("UNKNOWN accepted a booking")
	}
}

func TestBackupRestoreDrill(t *testing.T) {
	f := setup(t, 1)
	container := os.Getenv("PG_TEST_CONTAINER")
	if container == "" {
		t.Skip("PG_TEST_CONTAINER required for pg_dump/pg_restore drill")
	}
	key := domain.ID()
	status, original := request(t, f.api, "POST", "/api/v1/booking-requests", payload("10:00"), nil, map[string]string{"Idempotency-Key": key})
	assertStatus(t, status, 201, original)
	u, _ := url.Parse(f.dsn)
	source := strings.TrimPrefix(u.Path, "/")
	dump, e := osExec.Command("docker", "exec", container, "pg_dump", "-U", "forma", "-d", source, "--format=custom", "--no-owner", "--no-acl").Output()
	if e != nil {
		t.Fatalf("pg_dump: %v", e)
	}
	if len(dump) == 0 {
		t.Fatal("empty backup")
	}
	ctx := context.Background()
	admin, e := pgx.Connect(ctx, f.dsn)
	if e != nil {
		t.Fatal(e)
	}
	defer admin.Close(ctx)
	target := "forma_restore_" + strings.ReplaceAll(domain.ID(), "-", "")
	if _, e = admin.Exec(ctx, "CREATE DATABASE "+pgx.Identifier{target}.Sanitize()); e != nil {
		t.Fatal(e)
	}
	defer admin.Exec(ctx, "DROP DATABASE "+pgx.Identifier{target}.Sanitize()+" WITH (FORCE)")
	restore := osExec.Command("docker", "exec", "-i", container, "pg_restore", "-U", "forma", "-d", target, "--exit-on-error", "--no-owner", "--no-acl")
	restore.Stdin = bytes.NewReader(dump)
	if out, e := restore.CombinedOutput(); e != nil {
		t.Fatalf("pg_restore: %v %s", e, out)
	}
	u.Path = "/" + target
	restored, e := postgres.Open(ctx, u.String(), 10)
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Pool.Close()
	if e = restored.CheckSchema(ctx); e != nil {
		t.Fatal(e)
	}
	server := httptest.NewServer(httptransport.API(restored, slog.New(slog.NewTextHandler(io.Discard, nil)), f.cfg, f.studio, f.clock))
	defer server.Close()
	status, replayed := request(t, server, "POST", "/api/v1/booking-requests", payload("10:00"), nil, map[string]string{"Idempotency-Key": key})
	assertStatus(t, status, 200, replayed)
	if original.String("id") != replayed.String("id") {
		t.Fatal("restore lost idempotency mapping")
	}
	t.Logf("pg_dump/pg_restore verified, archive %d bytes", len(dump))
}

func TestIndexedPaginationWith100kBookings(t *testing.T) {
	f := setup(t, 1)
	ctx := context.Background()
	tx, e := f.store.Pool.Begin(ctx)
	if e != nil {
		t.Fatal(e)
	}
	defer tx.Rollback(ctx)
	if _, e = tx.Exec(ctx, "SET LOCAL statement_timeout='60s'"); e != nil {
		t.Fatal(e)
	}
	started := time.Now()
	_, e = tx.Exec(ctx, `INSERT INTO bookings(resource_id,service_id,client_name,client_phone,consent_version,service_name,duration_minutes,price_amount,currency,local_date,local_time,starts_at,ends_at,blocked_until) SELECT $1,'A','Synthetic Client','+79991234567','test','A',60,3500,'RUB',start_at::date,start_at::time,start_at,start_at+interval '1 hour',start_at+interval '1 hour' FROM (SELECT '2000-01-01 00:00:00+00'::timestamptz + i*interval '2 hours' AS start_at FROM generate_series(1,100000) AS i) data`, f.studio)
	if e != nil {
		t.Fatal(e)
	}
	if e = tx.Commit(ctx); e != nil {
		t.Fatal(e)
	}
	exec(t, f.store, "ANALYZE bookings")
	var plan []byte
	e = f.store.Pool.QueryRow(ctx, `EXPLAIN (ANALYZE,BUFFERS,FORMAT JSON) SELECT id FROM bookings WHERE resource_id=$1 AND starts_at>='2027-01-01' ORDER BY starts_at,id LIMIT 100`, f.studio).Scan(&plan)
	if e != nil {
		t.Fatal(e)
	}
	if !strings.Contains(string(plan), "Index") {
		t.Fatal("indexed lookup expected", string(plan))
	}
	admin, _ := login(t, f, "owner@forma.test", password)
	status, m := request(t, f.api, "GET", "/api/v1/admin/bookings?limit=5", nil, admin, nil)
	assertStatus(t, status, 200, m)
	if len(m["items"].([]any)) != 5 || m.String("nextCursor") == "" {
		t.Fatal("unbounded or incomplete pagination", m)
	}
	status, next := request(t, f.api, "GET", "/api/v1/admin/bookings?limit=5&cursor="+m.String("nextCursor"), nil, admin, nil)
	assertStatus(t, status, 200, next)
	if m["items"].([]any)[0].(map[string]any)["id"] == next["items"].([]any)[0].(map[string]any)["id"] {
		t.Fatal("cursor did not advance")
	}
	t.Logf("100k rows, analyzed index lookup and HTTP pagination verified in %s", time.Since(started))
}

func TestAbsencePreviewLeavesUnrelatedConflictAlone(t *testing.T) {
	f := setup(t, 2)
	admin, _ := login(t, f, "owner@forma.test", password)
	b := create(t, f, "10:00")
	exec(t, f.store, "UPDATE bookings SET specialist_id=$2,room_id=$3 WHERE id=$1", b.String("id"), f.specialists[0], f.rooms[0])
	exec(t, f.store, "UPDATE specialists SET state='archived' WHERE id=$1", f.specialists[0])
	// A pre-existing conflict at 10:00 must not enter the 14:00 leave acknowledgement.
	ctx := context.Background()
	account, e := postgres.FindAccount(ctx, f.store.Pool, "owner@forma.test")
	if e != nil {
		t.Fatal(e)
	}
	input := domain.M{"kind": "day_off", "scope": "partial_day", "startDate": "2027-01-02", "endDate": "2027-01-02", "reason": "test"}
	leave, e := postgres.CreateAbsence(ctx, f.store.Pool, account.ID, f.specialists[1], input, 840, 900)
	if e != nil {
		t.Fatal(e)
	}
	status, m := request(t, f.api, "POST", "/api/v1/admin/absence-requests/"+leave.String("id")+"/preview", nil, admin, nil)
	assertStatus(t, status, 200, m)
	if len(m["items"].([]any)) != 0 {
		t.Fatal("unrelated overlap component included in leave preview", m)
	}
}

func TestConcurrentAssignedExclusions(t *testing.T) {
	f := setup(t, 2)
	a := create(t, f, "10:00")
	b := create(t, f, "10:00")
	ids := []string{a.String("id"), b.String("id")}
	for _, mode := range []string{"specialist", "room"} {
		t.Run(mode, func(t *testing.T) {
			exec(t, f.store, "UPDATE bookings SET specialist_id=NULL,room_id=NULL")
			gate := make(chan struct{})
			outcomes := make(chan error, 2)
			var wg sync.WaitGroup
			for i := 0; i < 2; i++ {
				wg.Add(1)
				go func(i int) {
					defer wg.Done()
					<-gate
					sid, rid := f.specialists[i], f.rooms[i]
					if mode == "specialist" {
						sid = f.specialists[0]
					} else {
						rid = f.rooms[0]
					}
					_, err := f.store.Pool.Exec(context.Background(), "UPDATE bookings SET specialist_id=$2,room_id=$3 WHERE id=$1", ids[i], sid, rid)
					outcomes <- err
				}(i)
			}
			close(gate)
			wg.Wait()
			close(outcomes)
			successes, collisions := 0, 0
			for err := range outcomes {
				if err == nil {
					successes++
					continue
				}
				var state interface{ SQLState() string }
				if errors.As(err, &state) && state.SQLState() == "23P01" {
					collisions++
				} else {
					t.Fatal(err)
				}
			}
			if successes != 1 || collisions != 1 {
				t.Fatalf("EXCLUDE race: successes=%d collisions=%d", successes, collisions)
			}
		})
	}
}
