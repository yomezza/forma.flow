package admin

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/assignment"
	"github.com/yomezza/forma.flow/backend/internal/booking"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"github.com/yomezza/forma.flow/backend/internal/schedule"
	"github.com/yomezza/forma.flow/backend/internal/storage/postgres"
	"regexp"
	"time"
)

type Service struct {
	Store    *postgres.Store
	StudioID string
	Engine   schedule.Engine
}

func (s *Service) Settings(ctx context.Context) (domain.M, error) {
	return postgres.Settings(ctx, s.Store.Pool, s.StudioID)
}
func (s *Service) Content(ctx context.Context) (domain.M, error) {
	return postgres.Content(ctx, s.Store.Pool)
}
func (s *Service) SaveSite(ctx context.Context, a domain.Actor, in domain.M, settings bool) (out domain.M, err error) {
	if err = a.Admin(); err != nil {
		return
	}
	err = s.Store.InTx(ctx, func(db postgres.DB) error {
		if settings {
			old, e := postgres.Settings(ctx, db, s.StudioID)
			if e != nil {
				return e
			}
			if in.String("timeZone") != old.String("timeZone") {
				return domain.Fail("TIME_ZONE_CHANGE_REQUIRES_MIGRATION", 409, "Часовой пояс изменяется отдельной миграцией.")
			}
			if !domain.Text(in.String("calendarTitle"), 0, 120) || !domain.Text(in.String("address"), 0, 500) || in.Int("revealDuration") < 0 || in.Int("revealDuration") > 3000 || in.Int("preloaderDuration") < 0 || in.Int("preloaderDuration") > 15000 {
				return domain.Validation("settings")
			}
			for _, key := range []string{"green", "sand", "paper", "gold"} {
				if ok, _ := regexp.MatchString(`^#[0-9A-Fa-f]{6}$`, in.Object("palette").String(key)); !ok {
					return domain.Validation("palette")
				}
			}
			if e = postgres.SaveSettings(ctx, db, in); e != nil {
				return e
			}
		} else {
			for _, v := range in {
				if str, ok := v.(string); !ok || !domain.Text(str, 0, 10000) {
					return domain.Validation("content")
				}
			}
			if e := postgres.SaveContent(ctx, db, in); e != nil {
				return e
			}
		}
		out = in
		return postgres.Audit(ctx, db, a.AccountID, "site.updated", "site", "1", nil)
	})
	return
}
func (s *Service) Dashboard(ctx context.Context, a domain.Actor) (out domain.M, err error) {
	if err = a.Admin(); err != nil {
		return
	}
	err = s.Store.ReadTx(ctx, func(db postgres.DB) error {
		var e error
		out, e = postgres.DashboardCounts(ctx, db, s.StudioID)
		if e != nil {
			return e
		}
		list, e := postgres.UpcomingBookings(ctx, db, s.StudioID, s.Engine.Clock.Now())
		if e != nil {
			return e
		}
		upcoming := []domain.M{}
		for _, b := range list {
			start, _ := time.Parse(time.RFC3339Nano, b.String("startsAt"))
			if start.After(s.Engine.Clock.Now()) && (b.String("status") == "confirmed" || b.String("status") == "pending_confirmation") {
				upcoming = append(upcoming, booking.PublicBooking(b))
				if len(upcoming) == 10 {
					break
				}
			}
		}
		out["upcoming"] = upcoming
		return nil
	})
	return
}
func (s *Service) Attention(ctx context.Context, a domain.Actor) (out domain.M, err error) {
	if err = a.Admin(); err != nil {
		return
	}
	err = s.Store.ReadTx(ctx, func(db postgres.DB) error {
		d, e := postgres.ActiveSnapshot(ctx, db, s.StudioID, s.Engine.Clock.Now())
		if e != nil {
			return e
		}
		list := d.Bookings
		items := []domain.M{}
		rev := d.CapacityRevision
		for _, b := range list {
			if b.String("status") != "pending_confirmation" && b.String("status") != "confirmed" {
				continue
			}
			start, _ := time.Parse(time.RFC3339Nano, b.String("startsAt"))
			if start.Before(s.Engine.Clock.Now()) {
				continue
			}
			add := func(code, hint string) {
				items = append(items, domain.M{"bookingId": b["id"], "code": code, "severity": "warning", "hint": hint})
			}
			if b.String("specialistId") == "" {
				add("ASSIGNMENT_REQUIRED", "Назначьте специалиста и кабинет.")
			}
			rule, _ := schedule.ActiveRule(d, b.String("date"))
			m, _ := schedule.Minute(b.String("time"))
			if !schedule.Fits(rule, m, b.Int("durationMinutes")) {
				add("OUTSIDE_SCHEDULE", "Запись вне опубликованного графика.")
			}
			r, _, e := s.Engine.Check(ctx, d, b, true)
			if e != nil {
				return e
			}
			if r.Outcome == assignment.UNSAT {
				add("REQUIRES_REASSIGNMENT", "Требуется ручное решение о назначении.")
			} else if r.Outcome == assignment.UNKNOWN {
				add("CAPACITY_CHECK_LIMIT", "Проверка вместимости не завершена.")
			}
		}
		absences, e := postgres.Absences(ctx, db, "")
		if e != nil {
			return e
		}
		for _, r := range absences {
			if r.String("status") == "pending" {
				items = append(items, domain.M{"code": "ABSENCE_PENDING", "severity": "info", "hint": "Заявка на отсутствие ожидает рассмотрения."})
			}
		}
		if rev == 0 {
			if e = db.QueryRow(ctx, "SELECT capacity_revision FROM schedule_state WHERE resource_id=$1", s.StudioID).Scan(&rev); e != nil {
				return e
			}
		}
		out = domain.M{"capacityRevision": rev, "items": items}
		return nil
	})
	return
}
func (s *Service) Export(ctx context.Context, a domain.Actor) (out domain.M, err error) {
	if !a.Has("owner") {
		return nil, domain.Forbidden
	}
	err = s.Store.InTx(ctx, func(db postgres.DB) error {
		clients, e := postgres.Clients(ctx, db, "", "")
		if e != nil {
			return e
		}
		bookings, e := postgres.BookingList(ctx, db, s.StudioID, "", "", "", "")
		if e != nil {
			return e
		}
		services, e := postgres.Catalog(ctx, db, true)
		if e != nil {
			return e
		}
		out = domain.M{"clients": clients, "bookings": bookings, "services": services, "exportedAt": s.Engine.Clock.Now().UTC().Format(time.RFC3339)}
		return postgres.Audit(ctx, db, a.AccountID, "pii.exported", "studio", s.StudioID, nil)
	})
	return
}
