# Go testing strategy and critical cases

Use go test standard table tests for pure domain/schedule engine; httptest for HTTP; PostgreSQL 16–18 ephemeral integration DB per suite for constraints and transactions. Never rely only on mocks for concurrent correctness.

## Unit / table-driven examples

~~~go
package schedule
import ("testing"; "time")
func TestOverlap(t *testing.T) {
    base := time.Date(2026,11,10,9,0,0,0,time.UTC)
    cases := []struct{name string; a0,a1,b0,b1 int; want bool}{
        {"overlap",0,75,60,120,true},
        {"touching boundary",0,60,60,120,false},
        {"buffer overlap",0,75,60,120,true},
        {"disjoint",0,60,90,150,false},
    }
    for _,tt:=range cases { t.Run(tt.name,func(t *testing.T) {
        a0:=base.Add(time.Duration(tt.a0)*time.Minute)
        a1:=base.Add(time.Duration(tt.a1)*time.Minute)
        b0:=base.Add(time.Duration(tt.b0)*time.Minute)
        b1:=base.Add(time.Duration(tt.b1)*time.Minute)
        if got:=Overlaps(a0,a1,b0,b1);got!=tt.want {t.Fatalf("got %v want %v",got,tt.want)}
    })}
}
~~~

Test matrix:
- Simple 09:00–12:00, step 30, duration 60, no bookings: 09:00,09:30,10:00,10:30,11:00.
- 10:00–11:00 occupied with break 0: 09:00 and 11:00 still allowed, 09:30/10:00/10:30 blocked.
- Same booked span with break 15: 09:00 and 11:00 also blocked because buffer makes intervals intersect.
- Work intervals [09:00,10:00) and [11:00,12:00), duration 90: zero slots, do not combine intervals.
- Advanced without published day: unplanned, no slots even when simple weekly day is open.
- Advanced draft working not published: no public slots; publish one date only changes that date.
- BlockedDate or vacation in simple: no slots; two inclusive vacation periods touching same day are rejected.
- 2026-03-29 Europe/Berlin at 02:30 local: gap rejected; 2026-10-25 at 02:30: fold rejected. Other normal dates unaffected.
- Date/current time cutoff uses cabinet IANA zone and injected Clock, not host process location.
- Pending and confirmed reserve; cancelled frees; completed remains protected as historical event.
- Client repeat phone normalized from 8/+7/10 digits links same client; malicious different fullName never overwrites canonical client name.
- Status: each allowed transition, all forbidden pairs, cancelled restore collision, status no-op.
- Change active mode retains independent settings, preview conflicts, out-of-schedule warnings don't change status.
- Publish range with stale expectedRevision returns 409 and writes nothing; restore history goes to draft only.
- Data validation: overlapping intervals, bad step, invalid date (Feb 30), start>=end, negative price, invalid phone, missing consent/version.

## PostgreSQL concurrency integration

Start 20 goroutines with separate pgxpool connections + barrier, unique idempotency keys, same resource/date/time; expect exactly 1 committed booking, 19 SLOT_UNAVAILABLE; verify SQLSTATE 23P01 translation. Repeat across two independent server instances. Same key and identical body requested by 20 goroutines returns same booking ID exactly once; same key changed payload => IDEMPOTENCY_CONFLICT. Force rollback after client UPSERT or booking INSERT: no orphan committed client/idempotency rows. Verify cancelled time reusable and restore conflict rejected. Validate exclusion with a different start but overlapping buffer, not just identical start.

## HTTP tests

httptest.NewRecorder / httptest.NewServer: JSON decode and body limit, auth cookie/CSRF on all admin writes, 401/403/422/409 envelope, 201/200 idempotency, no PII in public GET, pagination bounds and cursor, correct request timeouts and request ID. Build fixed-clock golden tests for OpenAPI examples.

## Migration and operations tests

Apply goose up into empty database, replay up no-op, validate indices/FKs/exclusion, test down only on throwaway DB. Verify backup restoration and representative 100k booking rows EXPLAIN plans. Test concurrent publish vs booking (schedule_state lock) and serializing conflicting vacation/schedule changes. Run go test -race ./... for code races, but do not confuse this with SQL concurrency tests.
