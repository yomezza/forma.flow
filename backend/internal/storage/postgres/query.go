package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/yomezza/forma.flow/backend/internal/domain"
)

func Object(ctx context.Context, db DB, query string, args ...any) (domain.M, error) {
	var raw []byte
	if err := db.QueryRow(ctx, query, args...).Scan(&raw); err != nil {
		return nil, MapError(err)
	}
	m := domain.M{}
	if err := json.Unmarshal(raw, &m); err != nil {
		return nil, err
	}
	return m, nil
}
func Objects(ctx context.Context, db DB, query string, args ...any) ([]domain.M, error) {
	rows, err := db.Query(ctx, query, args...)
	if err != nil {
		return nil, MapError(err)
	}
	defer rows.Close()
	out := []domain.M{}
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		var m domain.M
		if err = json.Unmarshal(b, &m); err != nil {
			return nil, err
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func Changed(ctx context.Context, db DB, query string, args ...any) error {
	tag, err := db.Exec(ctx, query, args...)
	if err != nil {
		return MapError(err)
	}
	if tag.RowsAffected() != 1 {
		return domain.NotFound
	}
	return nil
}
func Audit(ctx context.Context, db DB, actor, action, entity, id string, details domain.M) error {
	if details == nil {
		details = domain.M{}
	}
	_, err := db.Exec(ctx, "INSERT INTO audit_log(actor_id,action,entity_type,entity_id,details) VALUES(NULLIF($1,'')::uuid,$2,$3,$4,$5)", actor, action, entity, id, details)
	return err
}
func LockStudio(ctx context.Context, db DB, id string) (int64, int64, error) {
	var revision, capacity int64
	err := db.QueryRow(ctx, "SELECT revision,capacity_revision FROM schedule_state WHERE resource_id=$1 FOR UPDATE", id).Scan(&revision, &capacity)
	return revision, capacity, MapError(err)
}
func BumpCapacity(ctx context.Context, db DB, id string) error {
	_, err := db.Exec(ctx, "UPDATE schedule_state SET capacity_revision=capacity_revision+1,updated_at=now() WHERE resource_id=$1", id)
	return err
}
func (s *Store) Studio(ctx context.Context, tz string) (string, error) {
	var id, zone string
	err := s.Pool.QueryRow(ctx, "SELECT id::text,time_zone FROM resources WHERE is_active ORDER BY id LIMIT 1").Scan(&id, &zone)
	if err != nil {
		return "", MapError(err)
	}
	if zone != tz {
		return "", fmt.Errorf("configured studio timezone differs from database")
	}
	return id, nil
}
