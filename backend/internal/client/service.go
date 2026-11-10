package client

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"github.com/yomezza/forma.flow/backend/internal/storage/postgres"
)

type Service struct{ Store *postgres.Store }

func (s *Service) List(ctx context.Context, a domain.Actor, phone, search string) ([]domain.M, error) {
	if e := a.Admin(); e != nil {
		return nil, e
	}
	if phone != "" {
		p, e := domain.Phone(phone)
		if e != nil {
			return nil, e
		}
		phone = p
	}
	return postgres.Clients(ctx, s.Store.Pool, phone, search)
}
func (s *Service) Get(ctx context.Context, a domain.Actor, id string) (domain.M, error) {
	if e := a.Admin(); e != nil {
		return nil, e
	}
	return postgres.Client(ctx, s.Store.Pool, id)
}
func (s *Service) Save(ctx context.Context, a domain.Actor, id string, in domain.M) (out domain.M, err error) {
	if err = a.Admin(); err != nil {
		return
	}
	err = s.Store.InTx(ctx, func(db postgres.DB) error {
		m := domain.Clone(in)
		if id != "" {
			old, e := postgres.Client(ctx, db, id)
			if e != nil {
				return e
			}
			m = domain.M{"fullName": old.String("fullName"), "phone": old.String("phoneNormalized")}
			for k, v := range in {
				m[k] = v
			}
		}
		n, p, e := domain.Identity(m)
		if e != nil {
			return e
		}
		if id == "" {
			out, e = postgres.CreateClient(ctx, db, n, p)
		} else {
			out, e = postgres.UpdateClient(ctx, db, id, n, p)
		}
		if e != nil {
			return e
		}
		return postgres.Audit(ctx, db, a.AccountID, "client.saved", "client", out.String("id"), nil)
	})
	return
}
