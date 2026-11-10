package catalog

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"github.com/yomezza/forma.flow/backend/internal/storage/postgres"
	"regexp"
)

type Service struct {
	Store    *postgres.Store
	StudioID string
}

var slug = regexp.MustCompile(`^[A-Za-z0-9_-]{1,100}$`)
var symbol = regexp.MustCompile(`^logo-0[1-6]$`)

func valid(m domain.M) error {
	if !slug.MatchString(m.String("id")) || !domain.Text(m.String("name"), 1, 120) || !domain.Text(m.String("description"), 0, 3000) || m.Int("durationMinutes") < 1 || m.Int("durationMinutes") > 1440 || m.Int("priceAmount") < 0 || m.Int("priceAmount") > 100000000 || m.String("currency") != "RUB" || m.Int("sortOrder") < 0 || m.Int("sortOrder") > 9999 || !symbol.MatchString(m.String("visualSymbol")) {
		return domain.Validation("service")
	}
	return nil
}
func (s *Service) List(ctx context.Context, a domain.Actor, admin bool) ([]domain.M, error) {
	if admin {
		if e := a.Admin(); e != nil {
			return nil, e
		}
	}
	return postgres.Catalog(ctx, s.Store.Pool, admin)
}
func (s *Service) Get(ctx context.Context, id string) (domain.M, error) {
	m, e := postgres.Service(ctx, s.Store.Pool, id)
	if e == nil && !m.Bool("isActive") {
		return nil, domain.NotFound
	}
	return m, e
}
func (s *Service) Save(ctx context.Context, a domain.Actor, id string, in domain.M) (out domain.M, err error) {
	if err = a.Admin(); err != nil {
		return
	}
	err = s.Store.InTx(ctx, func(db postgres.DB) error {
		if _, _, e := postgres.LockStudio(ctx, db, s.StudioID); e != nil {
			return e
		}
		m := domain.Clone(in)
		if id != "" {
			old, e := postgres.Service(ctx, db, id)
			if e != nil {
				return e
			}
			for k, v := range in {
				old[k] = v
			}
			m = old
		} else if m.String("id") == "" {
			m["id"] = domain.ID()
		}
		if e := valid(m); e != nil {
			return e
		}
		var e error
		if id == "" {
			out, e = postgres.CreateService(ctx, db, m)
		} else {
			out, e = postgres.UpdateService(ctx, db, id, m)
		}
		if e != nil {
			return e
		}
		if e = postgres.BumpCapacity(ctx, db, s.StudioID); e != nil {
			return e
		}
		return postgres.Audit(ctx, db, a.AccountID, "service.saved", "service", out.String("id"), nil)
	})
	return
}
func (s *Service) Archive(ctx context.Context, a domain.Actor, id string) error {
	_, err := s.Save(ctx, a, id, domain.M{"isActive": false})
	return err
}
