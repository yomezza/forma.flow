package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/yomezza/forma.flow/backend/internal/domain"
)

type DB interface {
	Exec(context.Context, string, ...any) (pgconn.CommandTag, error)
	Query(context.Context, string, ...any) (pgx.Rows, error)
	QueryRow(context.Context, string, ...any) pgx.Row
}
type Store struct{ Pool *pgxpool.Pool }

func Open(ctx context.Context, dsn string, max int) (*Store, error) {
	c, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, fmt.Errorf("invalid database configuration")
	}
	c.MaxConns = int32(max)
	c.MinConns = 1
	c.MaxConnLifetime = time.Hour
	c.ConnConfig.RuntimeParams["timezone"] = "UTC"
	c.ConnConfig.RuntimeParams["statement_timeout"] = "5000"
	c.ConnConfig.RuntimeParams["lock_timeout"] = "3000"
	p, err := pgxpool.NewWithConfig(ctx, c)
	if err != nil {
		return nil, err
	}
	if err = p.Ping(ctx); err != nil {
		p.Close()
		return nil, err
	}
	return &Store{Pool: p}, nil
}
func (s *Store) InTx(ctx context.Context, fn func(DB) error) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	if err = fn(tx); err != nil {
		return MapError(err)
	}
	return MapError(tx.Commit(ctx))
}
func MapError(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.NotFound
	}
	var p *pgconn.PgError
	if errors.As(err, &p) {
		switch p.Code {
		case "23P01":
			return domain.Fail("DIRECT_RESOURCE_CONFLICT", 409, "Пересечение занятости или интервалов.")
		case "23505":
			return domain.Fail("ALREADY_EXISTS", 409, "Объект уже существует.")
		case "23503", "23514", "22P02", "22007", "22008":
			return domain.Validation("data")
		case "40001", "40P01", "55P03", "57014":
			return domain.Fail("RETRY_LATER", 503, "Повторите запрос позднее.")
		}
	}
	return err
}
func (s *Store) CheckSchema(ctx context.Context) error {
	var n int64
	if err := s.Pool.QueryRow(ctx, "SELECT version_id FROM goose_db_version WHERE is_applied ORDER BY id DESC LIMIT 1").Scan(&n); err != nil {
		return fmt.Errorf("schema is not migrated")
	}
	if n != SchemaVersion {
		return fmt.Errorf("schema version %d, required %d", n, SchemaVersion)
	}
	return nil
}

const SchemaVersion = 4
