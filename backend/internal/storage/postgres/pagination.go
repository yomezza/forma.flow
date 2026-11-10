package postgres

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"strconv"
)

type PageSpec struct{ Limit, Offset int }
type pageKey struct{}

func WithPage(ctx context.Context, p PageSpec) context.Context {
	return context.WithValue(ctx, pageKey{}, p)
}
func Page(ctx context.Context) (PageSpec, bool) {
	p, ok := ctx.Value(pageKey{}).(PageSpec)
	return p, ok
}
func ObjectsPage(ctx context.Context, db DB, q string, args ...any) ([]domain.M, error) {
	if p, ok := Page(ctx); ok {
		q += " LIMIT " + strconv.Itoa(p.Limit+1) + " OFFSET " + strconv.Itoa(p.Offset)
	}
	return Objects(ctx, db, q, args...)
}
