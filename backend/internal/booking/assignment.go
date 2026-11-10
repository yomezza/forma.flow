package booking

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/assignment"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"github.com/yomezza/forma.flow/backend/internal/schedule"
	"github.com/yomezza/forma.flow/backend/internal/storage/postgres"
	"sort"
)

func (s *Service) plan(ctx context.Context, d domain.ScheduleData, b domain.M, in domain.M) (domain.M, error) {
	p := domain.M{"allowed": false, "requiresConfirmation": false, "capacityRevision": d.CapacityRevision, "affectedBookingIds": []string{}, "specialistId": in.String("specialistId"), "roomId": nil, "reason": nil}
	if !active(b) {
		p["reason"] = "INVALID_STATUS_TRANSITION"
		return p, nil
	}
	staffID := in.String("specialistId")
	roomID := in.String("roomId")
	eligible := false
	for _, staff := range d.Specialists {
		if staff.String("id") == staffID && schedule.Eligible(d, staff, b) {
			eligible = true
		}
	}
	if !eligible {
		p["reason"] = "DIRECT_RESOURCE_CONFLICT"
		return p, nil
	}
	target := schedule.Job(b)
	var fallback domain.M
	for _, room := range d.Rooms {
		rid := room.String("id")
		if !room.Bool("isActive") || (roomID != "" && roomID != rid) {
			continue
		}
		direct := false
		for _, old := range d.Bookings {
			if old.String("id") == b.String("id") {
				continue
			}
			j := schedule.Job(old)
			if j.Fixed != nil && assignment.Overlaps(target, j) && (j.Fixed.SpecialistID == staffID || j.Fixed.RoomID == rid) {
				direct = true
				break
			}
		}
		if direct {
			continue
		}
		candidate := domain.Clone(b)
		candidate["specialistId"] = staffID
		candidate["roomId"] = rid
		result, component, err := s.Engine.Check(ctx, d, candidate, true)
		if err != nil {
			return nil, err
		}
		if result.Outcome == assignment.UNKNOWN {
			return nil, domain.Fail("CAPACITY_CHECK_LIMIT", 503, "Превышен бюджет расчёта.")
		}
		plan := domain.Clone(p)
		plan["roomId"] = rid
		if result.Outcome == assignment.SAT {
			plan["allowed"] = true
			return plan, nil
		}
		domains := map[string][]assignment.Pair{}
		records := map[string]domain.M{}
		for _, r := range d.Bookings {
			records[r.String("id")] = r
		}
		records[b.String("id")] = candidate
		removable := []string{}
		for _, j := range component {
			domains[j.ID] = schedule.Domains(d, records[j.ID])
			if j.Fixed == nil && j.ID != b.String("id") {
				removable = append(removable, j.ID)
			}
		}
		sort.Strings(removable)
		// Search minimal cardinality displaced set, with a shared node budget.
		budget := s.Engine.MaxNodes
		var affected []string
		var search func(int, int, []string) (bool, error)
		search = func(start, need int, drop []string) (bool, error) {
			if ctx.Err() != nil || budget <= 0 {
				return false, domain.Fail("CAPACITY_CHECK_LIMIT", 503, "Превышен бюджет предпросмотра.")
			}
			budget--
			if need == 0 {
				keep := []assignment.Job{}
				for _, j := range component {
					if !contains(drop, j.ID) {
						keep = append(keep, j)
					}
				}
				r, e := assignment.Solve(ctx, keep, domains, budget)
				budget -= r.Nodes
				if e != nil {
					return false, e
				}
				if r.Outcome == assignment.UNKNOWN {
					return false, domain.Fail("CAPACITY_CHECK_LIMIT", 503, "Превышен бюджет расчёта.")
				}
				if r.Outcome == assignment.SAT {
					affected = append([]string{}, drop...)
					return true, nil
				}
				return false, nil
			}
			for i := start; i <= len(removable)-need; i++ {
				ok, e := search(i+1, need-1, append(drop, removable[i]))
				if ok || e != nil {
					return ok, e
				}
			}
			return false, nil
		}
		for n := 1; n <= len(removable); n++ {
			found, e := search(0, n, nil)
			if e != nil {
				return nil, e
			}
			if found {
				plan["allowed"] = true
				plan["requiresConfirmation"] = true
				plan["affectedBookingIds"] = affected
				plan["reason"] = "INDIRECT_ASSIGNMENT_CONFLICT"
				if fallback == nil {
					fallback = plan
				}
				break
			}
		}
	}
	if fallback != nil {
		return fallback, nil
	}
	p["reason"] = "DIRECT_RESOURCE_CONFLICT"
	return p, nil
}
func (s *Service) Preview(ctx context.Context, a domain.Actor, id string, in domain.M) (out domain.M, err error) {
	if err = a.Admin(); err != nil {
		return
	}
	err = s.Store.ReadTx(ctx, func(db postgres.DB) error {
		b, e := postgres.Booking(ctx, db, id, false)
		if e != nil {
			return e
		}
		d, e := postgres.Snapshot(ctx, db, s.StudioID, b.String("date"), b.String("date"))
		if e != nil {
			return e
		}
		out, e = s.plan(ctx, d, b, in)
		return e
	})
	return
}
func (s *Service) Candidates(ctx context.Context, a domain.Actor, id string) (out domain.M, err error) {
	if err = a.Admin(); err != nil {
		return
	}
	err = s.Store.ReadTx(ctx, func(db postgres.DB) error {
		b, e := postgres.Booking(ctx, db, id, false)
		if e != nil {
			return e
		}
		d, e := postgres.Snapshot(ctx, db, s.StudioID, b.String("date"), b.String("date"))
		if e != nil {
			return e
		}
		plans := []domain.M{}
		for _, staff := range d.Specialists {
			p, e := s.plan(ctx, d, b, domain.M{"specialistId": staff.String("id")})
			if e != nil {
				return e
			}
			if p.Bool("allowed") {
				plans = append(plans, p)
			}
		}
		out = domain.M{"capacityRevision": d.CapacityRevision, "candidates": plans}
		return nil
	})
	return
}
func (s *Service) Assign(ctx context.Context, a domain.Actor, id string, in domain.M) (out domain.M, err error) {
	if err = a.Admin(); err != nil {
		return
	}
	err = s.Store.InTx(ctx, func(db postgres.DB) error {
		_, rev, e := postgres.LockStudio(ctx, db, s.StudioID)
		if e != nil {
			return e
		}
		if int64(in.Int("expectedCapacityRevision")) != rev {
			return domain.Fail("STALE_CAPACITY_REVISION", 409, "Вместимость изменилась. Повторите предпросмотр.")
		}
		b, e := postgres.Booking(ctx, db, id, true)
		if e != nil {
			return e
		}
		d, e := postgres.Snapshot(ctx, db, s.StudioID, b.String("date"), b.String("date"))
		if e != nil {
			return e
		}
		p, e := s.plan(ctx, d, b, in)
		if e != nil {
			return e
		}
		if !p.Bool("allowed") {
			return domain.Fail(p.String("reason"), 409, "Недопустимое назначение.")
		}
		affected := p.Strings("affectedBookingIds")
		if !EqualIDs(affected, in.Strings("acknowledgedBookingIds")) {
			return &domain.Error{Status: 409, Code: "INDIRECT_ASSIGNMENT_CONFLICT", Message: "Подтвердите точный список затронутых заявок.", Details: domain.M{"bookingIds": affected}}
		}
		if e = postgres.Assign(ctx, db, id, a.AccountID, p.String("specialistId"), p.String("roomId"), affected, b); e != nil {
			return e
		}
		if e = postgres.BumpCapacity(ctx, db, s.StudioID); e != nil {
			return e
		}
		if e = postgres.Audit(ctx, db, a.AccountID, "booking.assignment_changed", "booking", id, domain.M{"previousSpecialistId": b["specialistId"], "specialistId": p["specialistId"], "previousRoomId": b["roomId"], "roomId": p["roomId"], "affectedBookingIds": affected}); e != nil {
			return e
		}
		out, e = postgres.Booking(ctx, db, id, false)
		if e != nil {
			return e
		}
		d, e = postgres.Snapshot(ctx, db, s.StudioID, b.String("date"), b.String("date"))
		if e != nil {
			return e
		}
		return s.Resolve(ctx, db, d)
	})
	return
}
func (s *Service) Resolve(ctx context.Context, db postgres.DB, d domain.ScheduleData) error {
	return s.Engine.Reconcile(ctx, db, d, domain.Actor{})
}
