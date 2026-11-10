package postgres

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/domain"
)

const ServiceJSON = `jsonb_build_object('id',id,'name',name,'description',description,'durationMinutes',duration_minutes,'priceAmount',price_amount,'currency',currency,'visualSymbol',visual_symbol,'sortOrder',sort_order,'isActive',is_active)`

func Catalog(ctx context.Context, db DB, all bool) ([]domain.M, error) {
	return ObjectsPage(ctx, db, "SELECT "+ServiceJSON+" FROM services WHERE ($1 OR is_active) ORDER BY sort_order,id", all)
}
func Service(ctx context.Context, db DB, id string) (domain.M, error) {
	return Object(ctx, db, "SELECT "+ServiceJSON+" FROM services WHERE id=$1", id)
}
func CreateService(ctx context.Context, db DB, m domain.M) (domain.M, error) {
	return Object(ctx, db, `INSERT INTO services(id,name,description,duration_minutes,price_amount,currency,visual_symbol,sort_order,is_active) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) RETURNING `+ServiceJSON, m.String("id"), m.String("name"), m.String("description"), m.Int("durationMinutes"), m.Int("priceAmount"), m.String("currency"), m.String("visualSymbol"), m.Int("sortOrder"), m.Bool("isActive"))
}
func UpdateService(ctx context.Context, db DB, id string, m domain.M) (domain.M, error) {
	return Object(ctx, db, `UPDATE services SET name=$2,description=$3,duration_minutes=$4,price_amount=$5,visual_symbol=$6,sort_order=$7,is_active=$8,updated_at=now() WHERE id=$1 RETURNING `+ServiceJSON, id, m.String("name"), m.String("description"), m.Int("durationMinutes"), m.Int("priceAmount"), m.String("visualSymbol"), m.Int("sortOrder"), m.Bool("isActive"))
}

const ClientJSON = `jsonb_build_object('id',id,'fullName',full_name,'phoneNormalized',phone_normalized,'createdAt',created_at,'updatedAt',updated_at)`

func Clients(ctx context.Context, db DB, phone, search string) ([]domain.M, error) {
	return ObjectsPage(ctx, db, "SELECT "+ClientJSON+" FROM clients WHERE ($1='' OR phone_normalized=$1) AND ($2='' OR full_name ILIKE '%'||$2||'%' OR phone_normalized LIKE '%'||$2||'%') ORDER BY full_name,id", phone, search)
}
func Client(ctx context.Context, db DB, id string) (domain.M, error) {
	return Object(ctx, db, "SELECT "+ClientJSON+" FROM clients WHERE id=$1", id)
}
func CreateClient(ctx context.Context, db DB, name, phone string) (domain.M, error) {
	return Object(ctx, db, "INSERT INTO clients(full_name,phone_normalized) VALUES($1,$2) RETURNING "+ClientJSON, name, phone)
}
func UpdateClient(ctx context.Context, db DB, id, name, phone string) (domain.M, error) {
	return Object(ctx, db, "UPDATE clients SET full_name=$2,phone_normalized=$3,updated_at=now() WHERE id=$1 RETURNING "+ClientJSON, id, name, phone)
}
