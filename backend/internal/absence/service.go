package absence

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/assignment"
	"github.com/yomezza/forma.flow/backend/internal/booking"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"github.com/yomezza/forma.flow/backend/internal/schedule"
	"github.com/yomezza/forma.flow/backend/internal/storage/postgres"
)

type Service struct {
	Store    *postgres.Store
	StudioID string
	Engine   schedule.Engine
}

func (s *Service) Submit(ctx context.Context, a domain.Actor, in domain.M) (out domain.M, err error) {
	if err = a.Specialist(); err != nil {
		return
	}
	start, e := schedule.Date(in.String("startDate"))
	if e != nil {
		return nil, e
	}
	end, e := schedule.Date(in.String("endDate"))
	if e != nil || end.Before(start) || end.Sub(start).Hours() > 24*365 {
		return nil, domain.Validation("dates")
	}
	if !domain.Text(in.String("reason"), 0, 500) {
		return nil, domain.Validation("reason")
	}
	var m0, m1 any
	if in.String("scope") == "partial_day" {
		if in.String("startDate") != in.String("endDate") {
			return nil, domain.Validation("partial_day")
		}
		x, e := schedule.Minute(in.String("startTime"))
		if e != nil {
			return nil, e
		}
		y, e := schedule.Minute(in.String("endTime"))
		if e != nil || x >= y {
			return nil, domain.Validation("endTime")
		}
		m0, m1 = x, y
	} else if in.String("startTime") != "" || in.String("endTime") != "" {
		return nil, domain.Validation("full_days")
	}
	err = s.Store.InTx(ctx, func(db postgres.DB) error {
		var e error
		out, e = postgres.CreateAbsence(ctx, db, a.AccountID, a.SpecialistID, in, m0, m1)
		if e != nil {
			return e
		}
		return postgres.Audit(ctx, db, a.AccountID, "absence.submitted", "absence", out.String("id"), nil)
	})
	return
}
func (s *Service) impacted(ctx context.Context, db postgres.DB, request domain.M) (domain.ScheduleData, []string, error) {
	d, e := postgres.Snapshot(ctx, db, s.StudioID, request.String("startDate"), request.String("endDate"))
	if e != nil {
		return d, nil, e
	}
	before := d
	absence := domain.Clone(request)
	x, _ := schedule.Minute(request.String("startTime"))
	y, _ := schedule.Minute(request.String("endTime"))
	absence["startMinute"], absence["endMinute"] = x, y
	d.Absences = append(append([]domain.M{}, d.Absences...), absence)
	var staff domain.M
	for _, candidate := range d.Specialists {
		if candidate.String("id") == request.String("specialistId") {
			staff = candidate
			break
		}
	}
	if staff == nil {
		return d, nil, domain.NotFound
	}
	affected := []string{}
	seen := map[string]bool{}
	checked := map[string]bool{}
	for _, b := range d.Bookings {
		if b.String("status") == "completed" || checked[b.String("id")] {
			continue
		}
		date := b.String("date")
		minute, _ := schedule.Minute(b.String("time"))
		direct := b.String("specialistId") == request.String("specialistId") && date >= request.String("startDate") && date <= request.String("endDate") && (request.String("scope") == "full_days" || minute < y && x < minute+b.Int("durationMinutes")+b.Int("bufferMinutes"))
		eligibilityChanged := schedule.Eligible(before, staff, b) && !schedule.Eligible(d, staff, b)
		if !direct && !eligibilityChanged {
			continue
		}
		result, jobs, e := s.Engine.Check(ctx, d, b, true)
		if e != nil {
			return d, nil, e
		}
		if result.Outcome == assignment.UNKNOWN {
			return d, nil, domain.Fail("CAPACITY_CHECK_LIMIT", 503, "Превышен бюджет расчёта.")
		}
		for _, job := range jobs {
			checked[job.ID] = true
			if result.Outcome == assignment.UNSAT && !seen[job.ID] {
				for _, record := range d.Bookings {
					if record.String("id") == job.ID && record.String("status") != "completed" {
						seen[job.ID] = true
						affected = append(affected, job.ID)
						break
					}
				}
			}
		}
	}
	return d, affected, nil
}
func (s *Service) Preview(ctx context.Context, a domain.Actor, id string) (out domain.M, err error) {
	if err = a.Admin(); err != nil {
		return
	}
	err = s.Store.ReadTx(ctx, func(db postgres.DB) error {
		r, e := postgres.Absence(ctx, db, id, false)
		if e != nil {
			return e
		}
		if r.String("status") != "pending" {
			return domain.Fail("INVALID_STATUS_TRANSITION", 409, "Заявка уже рассмотрена.")
		}
		d, ids, e := s.impacted(ctx, db, r)
		if e != nil {
			return e
		}
		items := []domain.M{}
		for _, id := range ids {
			items = append(items, domain.M{"bookingId": id, "code": "REQUIRES_REASSIGNMENT", "severity": "warning", "hint": "Утверждение отсутствия затронет запись."})
		}
		out = domain.M{"capacityRevision": d.CapacityRevision, "items": items}
		return nil
	})
	return
}
func (s *Service) Decide(ctx context.Context, a domain.Actor, id, action string, in domain.M) (out domain.M, err error) {
	if action == "withdraw" {
		err = a.Specialist()
	} else {
		err = a.Admin()
	}
	if err != nil {
		return
	}
	err = s.Store.InTx(ctx, func(db postgres.DB) error {
		_, rev, e := postgres.LockStudio(ctx, db, s.StudioID)
		if e != nil {
			return e
		}
		if action != "withdraw" && rev != int64(in.Int("expectedCapacityRevision")) {
			return domain.Fail("STALE_CAPACITY_REVISION", 409, "Вместимость изменилась.")
		}
		r, e := postgres.Absence(ctx, db, id, true)
		if e != nil {
			return e
		}
		if action == "withdraw" && r.String("specialistId") != a.SpecialistID {
			return domain.NotFound
		}
		if r.String("status") != "pending" {
			return domain.Fail("INVALID_STATUS_TRANSITION", 409, "Заявка уже рассмотрена.")
		}
		status := "withdrawn"
		if action == "approve" {
			_, ids, e := s.impacted(ctx, db, r)
			if e != nil {
				return e
			}
			if !booking.EqualIDs(ids, in.Strings("acknowledgedBookingIds")) {
				return &domain.Error{Status: 409, Code: "CONFLICT_ACK_REQUIRED", Message: "Подтвердите затронутые записи.", Details: domain.M{"bookingIds": ids}}
			}
			for _, id := range ids {
				if e = postgres.CapacityException(ctx, db, "", id, a.AccountID, "REQUIRES_REASSIGNMENT"); e != nil {
					return e
				}
			}
			status = "approved"
		} else if action == "reject" {
			status = "rejected"
			if !domain.Text(in.String("reason"), 1, 500) {
				return domain.Validation("reason")
			}
		}
		out, e = postgres.DecideAbsence(ctx, db, id, a.AccountID, status, in.String("reason"))
		if e != nil {
			return e
		}
		if action == "approve" {
			if e = postgres.BumpCapacity(ctx, db, s.StudioID); e != nil {
				return e
			}
		}
		return postgres.Audit(ctx, db, a.AccountID, "absence."+status, "absence", id, nil)
	})
	return
}
