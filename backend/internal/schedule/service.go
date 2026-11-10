package schedule

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"github.com/yomezza/forma.flow/backend/internal/storage/postgres"
	"reflect"
	"sort"
	"time"
)

type Service struct {
	Store    *postgres.Store
	StudioID string
	Engine   Engine
}

func Range(from, to string, max int) ([]string, error) {
	a, e := Date(from)
	if e != nil {
		return nil, e
	}
	b, e := Date(to)
	if e != nil || b.Before(a) || b.Sub(a) > time.Duration(max-1)*24*time.Hour {
		return nil, domain.Validation("date range")
	}
	out := []string{}
	for d := a; !d.After(b); d = d.AddDate(0, 0, 1) {
		out = append(out, d.Format("2006-01-02"))
	}
	return out, nil
}
func (s *Service) Availability(ctx context.Context, from, to, serviceID string, slots bool) (out domain.M, err error) {
	dates, e := Range(from, to, 62)
	if e != nil {
		return nil, e
	}
	err = s.Store.ReadTx(ctx, func(db postgres.DB) error {
		d, e := postgres.Snapshot(ctx, db, s.StudioID, from, to)
		if e != nil {
			return e
		}
		var service domain.M
		if serviceID != "" {
			service, e = postgres.Service(ctx, db, serviceID)
			if e != nil {
				return e
			}
			if !service.Bool("isActive") {
				return domain.NotFound
			}
		}
		states := domain.M{}
		for _, date := range dates {
			if service == nil {
				_, state := ActiveRule(d, date)
				loc, _ := time.LoadLocation(d.Zone)
				if date < s.Engine.Clock.Now().In(loc).Format("2006-01-02") {
					state = "past"
				}
				states[date] = state
				continue
			}
			available, state, e := s.Engine.Slots(ctx, d, date, service)
			if e != nil {
				return e
			}
			if slots {
				out = domain.M{"date": date, "serviceId": serviceID, "slots": available}
				return nil
			}
			states[date] = state
		}
		out = domain.M{"states": states}
		return nil
	})
	return
}
func snapshot(d domain.ScheduleData) domain.M {
	return domain.M{"mode": d.Mode, "simple": d.Simple, "vacations": d.Vacations, "days": d.Days}
}
func (s *Service) Read(ctx context.Context, operation, date, stage, from, to string) (out domain.M, err error) {
	if from == "" {
		from = s.Engine.Clock.Now().UTC().Format("2006-01-02")
	}
	if to == "" {
		to = from
	}
	if date != "" {
		from = date
		to = date
	}
	if _, e := Range(from, to, 62); e != nil {
		return nil, e
	}
	err = s.Store.ReadTx(ctx, func(db postgres.DB) error {
		d, e := postgres.Snapshot(ctx, db, s.StudioID, from, to)
		if operation == "warnings" {
			d, e = postgres.ActiveSnapshot(ctx, db, s.StudioID, s.Engine.Clock.Now())
		}
		if e != nil {
			return e
		}
		switch operation {
		case "mode":
			out = domain.M{"mode": d.Mode, "revision": d.Revision}
		case "simple":
			out = domain.Clone(d.Simple)
			out["revision"] = d.Revision
		case "days":
			if stage != "draft" && stage != "published" {
				return domain.Validation("stage")
			}
			days := domain.M{}
			for _, r := range d.Days {
				key := r.String("date")
				if key >= from && key <= to && r.String("stage") == stage {
					days[key] = Rule(d, key, stage)
				}
			}
			out = domain.M{"days": days, "revision": d.Revision}
		case "day":
			rule, _ := ActiveRule(d, date)
			out = domain.M{"active": rule, "draft": Rule(d, date, "draft"), "published": Rule(d, date, "published"), "revision": d.Revision}
		case "warnings":
			ids := []string{}
			for _, b := range d.Bookings {
				if outside(d, b) {
					ids = append(ids, b.String("id"))
				}
			}
			out = domain.M{"bookingIds": ids}
		}
		return nil
	})
	return
}
func outside(d domain.ScheduleData, b domain.M) bool {
	if b.String("status") != "pending_confirmation" && b.String("status") != "confirmed" {
		return false
	}
	rule, _ := ActiveRule(d, b.String("date"))
	m, _ := Minute(b.String("time"))
	return !Fits(rule, m, b.Int("durationMinutes"))
}
func parseRule(rule domain.M) ([][2]int, error) {
	if rule == nil {
		return nil, domain.Validation("rule")
	}
	kind := rule.String("kind")
	if kind != "working" && kind != "off" && kind != "unplanned" {
		return nil, domain.Validation("kind")
	}
	intervals := [][2]int{}
	last := -1
	for _, i := range Objects(rule["intervals"]) {
		a, e := Minute(i.String("start"))
		if e != nil {
			return nil, e
		}
		b, e := Minute(i.String("end"))
		if e != nil {
			return nil, e
		}
		if a >= b || a < last {
			return nil, domain.Validation("intervals")
		}
		last = b
		intervals = append(intervals, [2]int{a, b})
	}
	if (kind == "working") != (len(intervals) > 0) {
		return nil, domain.Validation("interval count")
	}
	if kind != "working" && rule["stepOverride"] != nil {
		return nil, domain.Validation("stepOverride")
	}
	return intervals, nil
}
func exactIDs(a, b []string) bool {
	x := append([]string{}, a...)
	y := append([]string{}, b...)
	sort.Strings(x)
	sort.Strings(y)
	return reflect.DeepEqual(x, y)
}
func conflictIDs(d domain.ScheduleData) []string {
	out := []string{}
	for _, b := range d.Bookings {
		if outside(d, b) {
			out = append(out, b.String("id"))
		}
	}
	sort.Strings(out)
	return out
}
func (s *Service) Mutate(ctx context.Context, a domain.Actor, operation, date, id string, in domain.M) (out domain.M, err error) {
	if err = a.Admin(); err != nil {
		return
	}
	from, to := in.String("from"), in.String("to")
	if from == "" {
		from = s.Engine.Clock.Now().UTC().Format("2006-01-02")
	}
	if to == "" {
		to = from
	}
	if date != "" {
		from = date
		to = date
	}
	dates, e := Range(from, to, 366)
	if e != nil {
		return nil, e
	}
	err = s.Store.InTx(ctx, func(db postgres.DB) error {
		rev, _, e := postgres.LockStudio(ctx, db, s.StudioID)
		if e != nil {
			return e
		}
		if rev != int64(in.Int("expectedRevision")) {
			return domain.Fail("VERSION_CONFLICT", 409, "Расписание изменилось.")
		}
		before, e := postgres.Snapshot(ctx, db, s.StudioID, from, to)
		if operation == "mode" || operation == "simple" {
			before, e = postgres.ActiveSnapshot(ctx, db, s.StudioID, s.Engine.Clock.Now())
		}
		if e != nil {
			return e
		}
		capacity := true
		put := func(day, stage string, r domain.M) error {
			intervals, e := parseRule(r)
			if e != nil {
				return e
			}
			return postgres.PutDay(ctx, db, s.StudioID, day, stage, r, intervals)
		}
		switch operation {
		case "simple":
			start, e := Minute(in.String("start"))
			if e != nil {
				return e
			}
			end, e := Minute(in.String("end"))
			if e != nil || start >= end {
				return domain.Validation("working hours")
			}
			if e = postgres.PutSimple(ctx, db, s.StudioID, in); e != nil {
				return e
			}
		case "mode":
			if e = postgres.SetMode(ctx, db, s.StudioID, in.String("mode")); e != nil {
				return e
			}
		case "draft", "range":
			capacity = false
			for _, day := range dates {
				r := in.Object("rule")
				if week := in.Object("week"); week != nil {
					d, _ := Date(day)
					r = week.Object(string(rune('0' + int(d.Weekday()))))
					if r == nil {
						continue
					}
				}
				if e = put(day, "draft", r); e != nil {
					return e
				}
			}
		case "copy":
			capacity = false
			for _, day := range dates {
				r, _ := SimpleRule(before, day)
				if e = put(day, "draft", r); e != nil {
					return e
				}
			}
		case "publish":
			for _, day := range dates {
				r := Rule(before, day, "draft")
				if r == nil {
					continue
				}
				if e = put(day, "published", r); e != nil {
					return e
				}
				if _, e = db.Exec(ctx, "DELETE FROM schedule_days WHERE resource_id=$1 AND day=$2::date AND stage='draft'", s.StudioID, day); e != nil {
					return e
				}
			}
		case "restore":
			capacity = false
			entry, e := postgres.HistoryEntry(ctx, db, id)
			if e != nil {
				return e
			}
			old := entry.Object("before")
			historic := before
			historic.Mode = old.String("mode")
			historic.Simple = old.Object("simple")
			historic.Days = Objects(old["days"])
			historic.Vacations = Objects(old["vacations"])
			allDates := map[string]bool{}
			for _, r := range append(append([]domain.M{}, before.Days...), historic.Days...) {
				allDates[r.String("date")] = true
			}
			for _, day := range dates {
				allDates[day] = true
			}
			for day := range allDates {
				r, _ := ActiveRule(historic, day)
				if e = put(day, "draft", r); e != nil {
					return e
				}
			}
		default:
			return domain.Validation("schedule operation")
		}
		after, e := postgres.Snapshot(ctx, db, s.StudioID, from, to)
		if operation == "mode" || operation == "simple" {
			after, e = postgres.ActiveSnapshot(ctx, db, s.StudioID, s.Engine.Clock.Now())
		}
		if e != nil {
			return e
		}
		if operation == "mode" || operation == "publish" {
			ids := conflictIDs(after)
			if !exactIDs(ids, in.Strings("acknowledgedBookingIds")) {
				return &domain.Error{Status: 409, Code: "CONFLICT_ACK_REQUIRED", Message: "Подтвердите конфликтующие записи.", Details: domain.M{"bookingIds": ids}}
			}
		}
		if e = postgres.RecordSchedule(ctx, db, s.StudioID, a.AccountID, operation, snapshot(before), snapshot(after), capacity); e != nil {
			return e
		}
		out = domain.M{"revision": rev + 1}
		if operation == "mode" {
			out["mode"] = in.String("mode")
		}
		if operation == "simple" {
			out = domain.Clone(after.Simple)
			out["revision"] = rev + 1
		}
		return nil
	})
	return
}

func (s *Service) Preview(ctx context.Context, a domain.Actor, operation, id string, in domain.M) (out domain.M, err error) {
	if err = a.Admin(); err != nil {
		return
	}
	from, to := in.String("from"), in.String("to")
	if from == "" {
		from = s.Engine.Clock.Now().UTC().Format("2006-01-02")
		to = from
	}
	dates, e := Range(from, to, 366)
	if e != nil {
		return nil, e
	}
	err = s.Store.ReadTx(ctx, func(db postgres.DB) error {
		d, e := postgres.Snapshot(ctx, db, s.StudioID, from, to)
		if e != nil {
			return e
		}
		proposed := d
		switch operation {
		case "mode":
			proposed.Mode = in.String("mode")
		case "publish":
			proposed.Days = append([]domain.M{}, d.Days...)
			for _, day := range dates {
				r := Rule(d, day, "draft")
				if r == nil {
					continue
				}
				next := []domain.M{}
				for _, old := range proposed.Days {
					if old.String("date") != day || old.String("stage") != "published" {
						next = append(next, old)
					}
				}
				if r.String("kind") != "unplanned" {
					r["date"] = day
					r["stage"] = "published"
					next = append(next, r)
				}
				proposed.Days = next
			}
		case "restore":
			entry, e := postgres.HistoryEntry(ctx, db, id)
			if e != nil {
				return e
			}
			old := entry.Object("before")
			proposed.Mode = old.String("mode")
			proposed.Simple = old.Object("simple")
			proposed.Days = Objects(old["days"])
			proposed.Vacations = Objects(old["vacations"])
		}
		conflicts := []domain.M{}
		for _, b := range proposed.Bookings {
			if outside(proposed, b) {
				conflicts = append(conflicts, domain.M{"id": b["id"], "date": b["date"], "time": b["time"]})
			}
		}
		if operation == "mode" {
			changed := 0
			for _, day := range dates {
				old, _ := ActiveRule(d, day)
				next, _ := ActiveRule(proposed, day)
				if !reflect.DeepEqual(old, next) {
					changed++
				}
			}
			out = domain.M{"changed": changed, "conflicts": len(conflicts), "revision": d.Revision}
			return nil
		}
		working, off := 0, 0
		unavailable := []string{}
		for _, day := range dates {
			r, state := ActiveRule(proposed, day)
			if r.String("kind") == "working" {
				working++
			} else if state == "off" {
				off++
			} else {
				unavailable = append(unavailable, day)
			}
		}
		out = domain.M{"total": len(dates), "working": working, "off": off, "unavailable": unavailable, "conflicts": conflicts, "revision": d.Revision}
		return nil
	})
	return
}
func (s *Service) Vacation(ctx context.Context, a domain.Actor, id string, in domain.M, remove bool) (out domain.M, err error) {
	if err = a.Admin(); err != nil {
		return
	}
	if !remove {
		if _, e := Range(in.String("startDate"), in.String("endDate"), 366); e != nil {
			return nil, e
		}
	}
	err = s.Store.InTx(ctx, func(db postgres.DB) error {
		rev, _, e := postgres.LockStudio(ctx, db, s.StudioID)
		if e != nil {
			return e
		}
		if rev != int64(in.Int("expectedRevision")) {
			return domain.Fail("VERSION_CONFLICT", 409, "Расписание изменилось.")
		}
		from := in.String("startDate")
		to := in.String("endDate")
		if remove {
			from = s.Engine.Clock.Now().UTC().Format("2006-01-02")
			to = from
		}
		before, e := postgres.Snapshot(ctx, db, s.StudioID, from, to)
		if e != nil {
			return e
		}
		if remove {
			if e = postgres.DeleteVacation(ctx, db, s.StudioID, id); e != nil {
				return e
			}
		} else {
			ids := []string{}
			if before.Mode == "simple" {
				for _, b := range before.Bookings {
					if b.String("date") >= from && b.String("date") <= to && (b.String("status") == "pending_confirmation" || b.String("status") == "confirmed") {
						ids = append(ids, b.String("id"))
					}
				}
			}
			if !exactIDs(ids, in.Strings("acknowledgedBookingIds")) {
				return &domain.Error{Code: "VACATION_CONFLICT", Status: 409, Message: "Подтвердите записи в период отпуска.", Details: domain.M{"bookingIds": ids}}
			}
			out, e = postgres.SaveVacation(ctx, db, s.StudioID, id, in)
			if e != nil {
				return e
			}
		}
		after, e := postgres.Snapshot(ctx, db, s.StudioID, from, to)
		if e != nil {
			return e
		}
		return postgres.RecordSchedule(ctx, db, s.StudioID, a.AccountID, "vacation", snapshot(before), snapshot(after), true)
	})
	return
}
