package schedule

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/assignment"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"github.com/yomezza/forma.flow/backend/internal/storage/postgres"
)

// Reconcile never clears exceptions outside the loaded overlap components.
func (e Engine) Reconcile(ctx context.Context, db postgres.DB, d domain.ScheduleData, a domain.Actor) error {
	visited := map[string]bool{}
	records := map[string]domain.M{}
	for _, b := range d.Bookings {
		records[b.String("id")] = b
	}
	for _, b := range d.Bookings {
		if visited[b.String("id")] || b.String("status") == "completed" {
			continue
		}
		r, jobs, err := e.Check(ctx, d, b, true)
		if err != nil {
			return err
		}
		if r.Outcome == assignment.UNKNOWN {
			return domain.Fail("CAPACITY_CHECK_LIMIT", 503, "Превышен бюджет расчёта.")
		}
		for _, j := range jobs {
			visited[j.ID] = true
			if records[j.ID].String("status") == "completed" {
				continue
			}
			if r.Outcome == assignment.SAT {
				if err = postgres.ResolveException(ctx, db, j.ID); err != nil {
					return err
				}
			} else {
				if err = postgres.CapacityException(ctx, db, "", j.ID, a.AccountID, "REQUIRES_REASSIGNMENT"); err != nil {
					return err
				}
			}
		}
	}
	return nil
}
