package room

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"github.com/yomezza/forma.flow/backend/internal/specialist"
	"github.com/yomezza/forma.flow/backend/internal/storage/postgres"
)

type Service struct {
	Store    *postgres.Store
	StudioID string
	Staff    *specialist.Service
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
			old, e := postgres.Room(ctx, db, id)
			if e != nil {
				return e
			}
			for k, v := range m {
				old[k] = v
			}
			m = old
		}
		if !domain.Text(m.String("name"), 1, 80) || m.Int("sortOrder") < 0 || m.Int("sortOrder") > 9999 {
			return domain.Validation("room")
		}
		var e error
		out, e = postgres.SaveRoom(ctx, db, s.StudioID, id, m)
		if e != nil {
			return e
		}
		if e = s.Staff.Refresh(ctx, db, a); e != nil {
			return e
		}
		if e = postgres.BumpCapacity(ctx, db, s.StudioID); e != nil {
			return e
		}
		return postgres.Audit(ctx, db, a.AccountID, "room.saved", "room", out.String("id"), nil)
	})
	return
}
