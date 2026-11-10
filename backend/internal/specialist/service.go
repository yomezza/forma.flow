package specialist

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"github.com/yomezza/forma.flow/backend/internal/schedule"
	"github.com/yomezza/forma.flow/backend/internal/storage/postgres"
	"strconv"
)

type Service struct {
	Store    *postgres.Store
	StudioID string
	Engine   schedule.Engine
}

func (s *Service) Save(ctx context.Context, a domain.Actor, id string, in domain.M, archive bool) (out domain.M, err error) {
	if err = a.Admin(); err != nil {
		return
	}
	err = s.Store.InTx(ctx, func(db postgres.DB) error {
		if _, _, e := postgres.LockStudio(ctx, db, s.StudioID); e != nil {
			return e
		}
		m := domain.Clone(in)
		if id != "" {
			old, e := postgres.Specialist(ctx, db, id)
			if e != nil {
				return e
			}
			for k, v := range m {
				old[k] = v
			}
			m = old
		}
		if archive {
			if e := postgres.ArchiveSpecialist(ctx, db, id); e != nil {
				return e
			}
		} else {
			if !domain.Text(m.String("fullName"), 2, 120) {
				return domain.Validation("fullName")
			}
			if m.String("accountId") != "" {
				account, e := postgres.ActorRoles(ctx, db, m.String("accountId"))
				if e != nil {
					return e
				}
				if !account.Has("specialist") {
					return domain.Validation("accountId")
				}
			}
			var e error
			id, e = postgres.SaveSpecialist(ctx, db, s.StudioID, id, m)
			if e != nil {
				return e
			}
		}
		if e := s.Refresh(ctx, db, a); e != nil {
			return e
		}
		if e := postgres.BumpCapacity(ctx, db, s.StudioID); e != nil {
			return e
		}
		if e := postgres.Audit(ctx, db, a.AccountID, "specialist.saved", "specialist", id, nil); e != nil {
			return e
		}
		var e error
		out, e = postgres.Specialist(ctx, db, id)
		return e
	})
	return
}
func Intervals(v any) ([][2]int, error) {
	out := [][2]int{}
	last := -1
	for _, m := range schedule.Objects(v) {
		start, e := schedule.Minute(m.String("start"))
		if e != nil {
			return nil, e
		}
		end, e := schedule.Minute(m.String("end"))
		if e != nil {
			return nil, e
		}
		if start >= end || start < last {
			return nil, domain.Validation("intervals")
		}
		last = end
		out = append(out, [2]int{start, end})
	}
	return out, nil
}
func Personal(d domain.ScheduleData, id string) (domain.M, error) {
	for _, staff := range d.Specialists {
		if staff.String("id") != id {
			continue
		}
		week := domain.M{}
		for day := 0; day < 7; day++ {
			week[strconv.Itoa(day)] = []domain.M{}
		}
		for _, i := range schedule.Objects(staff["week"]) {
			key := strconv.Itoa(i.Int("weekday"))
			week[key] = append(week[key].([]domain.M), domain.M{"start": schedule.Clock(i.Int("startMinute")), "end": schedule.Clock(i.Int("endMinute"))})
		}
		overrides := domain.M{}
		for _, o := range schedule.Objects(staff["overrides"]) {
			intervals := []domain.M{}
			for _, i := range schedule.Objects(o["intervals"]) {
				intervals = append(intervals, domain.M{"start": schedule.Clock(i.Int("startMinute")), "end": schedule.Clock(i.Int("endMinute"))})
			}
			overrides[o.String("date")] = domain.M{"kind": o["kind"], "intervals": intervals}
		}
		return domain.M{"specialistId": id, "revision": d.CapacityRevision, "week": week, "overrides": overrides}, nil
	}
	return nil, domain.NotFound
}
func (s *Service) GetSchedule(ctx context.Context, a domain.Actor, id string, mine bool) (out domain.M, err error) {
	if mine {
		err = a.Specialist()
		id = a.SpecialistID
	} else {
		err = a.Admin()
	}
	if err != nil {
		return
	}
	err = s.Store.ReadTx(ctx, func(db postgres.DB) error {
		today := s.Engine.Clock.Now().UTC().Format("2006-01-02")
		d, e := postgres.Snapshot(ctx, db, s.StudioID, today, today)
		if e != nil {
			return e
		}
		out, e = Personal(d, id)
		return e
	})
	return
}
func (s *Service) SetSchedule(ctx context.Context, a domain.Actor, id string, in domain.M) (out domain.M, err error) {
	if err = a.Admin(); err != nil {
		return
	}
	if in.String("specialistId") != id {
		return nil, domain.Validation("specialistId")
	}
	week := map[int][][2]int{}
	for key, v := range in.Object("week") {
		day, e := strconv.Atoi(key)
		if e != nil || day < 0 || day > 6 {
			return nil, domain.Validation("weekday")
		}
		intervals, e := Intervals(v)
		if e != nil {
			return nil, e
		}
		week[day] = intervals
	}
	overrides := map[string]postgres.PersonalOverride{}
	for date, v := range in.Object("overrides") {
		if _, e := schedule.Date(date); e != nil {
			return nil, e
		}
		m := domain.M(v.(map[string]any))
		intervals, e := Intervals(m["intervals"])
		if e != nil {
			return nil, e
		}
		if (m.String("kind") == "working") != (len(intervals) > 0) {
			return nil, domain.Validation("override")
		}
		overrides[date] = postgres.PersonalOverride{Kind: m.String("kind"), Intervals: intervals}
	}
	err = s.Store.InTx(ctx, func(db postgres.DB) error {
		_, rev, e := postgres.LockStudio(ctx, db, s.StudioID)
		if e != nil {
			return e
		}
		if rev != int64(in.Int("revision")) {
			return domain.Fail("STALE_CAPACITY_REVISION", 409, "График изменился.")
		}
		if _, e = postgres.Specialist(ctx, db, id); e != nil {
			return e
		}
		if e = postgres.SavePersonalSchedule(ctx, db, id, week, overrides); e != nil {
			return e
		}
		if e = s.Refresh(ctx, db, a); e != nil {
			return e
		}
		if e = postgres.BumpCapacity(ctx, db, s.StudioID); e != nil {
			return e
		}
		if e = postgres.Audit(ctx, db, a.AccountID, "specialist.schedule_changed", "specialist", id, nil); e != nil {
			return e
		}
		today := s.Engine.Clock.Now().UTC().Format("2006-01-02")
		d, e := postgres.Snapshot(ctx, db, s.StudioID, today, today)
		if e != nil {
			return e
		}
		out, e = Personal(d, id)
		return e
	})
	return
}

// Refresh preserves historical assignments and materializes new resource warnings.
func (s *Service) Refresh(ctx context.Context, db postgres.DB, a domain.Actor) error {
	d, e := postgres.ActiveSnapshot(ctx, db, s.StudioID, s.Engine.Clock.Now())
	if e != nil {
		return e
	}
	return s.Engine.Reconcile(ctx, db, d, a)
}
