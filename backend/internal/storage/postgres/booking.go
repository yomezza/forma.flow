package postgres

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"time"
)

func ReserveKey(ctx context.Context, db DB, key, hash string) (string, domain.M, error) {
	if _, e := db.Exec(ctx, "INSERT INTO idempotency_keys(scope,key,request_hash) VALUES('public_booking',$1,$2) ON CONFLICT DO NOTHING", key, hash); e != nil {
		return "", nil, e
	}
	var saved string
	var response []byte
	e := db.QueryRow(ctx, "SELECT request_hash,response FROM idempotency_keys WHERE scope='public_booking' AND key=$1 FOR UPDATE", key).Scan(&saved, &response)
	if e != nil {
		return "", nil, e
	}
	var m domain.M
	if response != nil {
		if e = decode(response, &m); e != nil {
			return "", nil, e
		}
	}
	return saved, m, nil
}
func SaveKey(ctx context.Context, db DB, key, id string, receipt domain.M) error {
	return Changed(ctx, db, "UPDATE idempotency_keys SET booking_id=$2,response=$3 WHERE scope='public_booking' AND key=$1", key, id, receipt)
}
func CreateBooking(ctx context.Context, db DB, studio string, m domain.M) (domain.M, error) {
	var id string
	e := db.QueryRow(ctx, `INSERT INTO bookings(resource_id,service_id,client_name,client_phone,consent_version,service_name,duration_minutes,price_amount,currency,local_date,local_time,starts_at,ends_at,blocked_until,buffer_minutes) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10::date,$11::time,$12::timestamptz,$13::timestamptz,$14::timestamptz,$15) RETURNING id::text`, studio, m.String("serviceId"), m.String("clientName"), m.String("clientPhone"), m.String("consentVersion"), m.String("name"), m.Int("durationMinutes"), m.Int("priceAmount"), m.String("currency"), m.String("date"), m.String("time"), m.String("startsAt"), m.String("endsAt"), m.String("blockedUntil"), m.Int("bufferMinutes")).Scan(&id)
	if e != nil {
		return nil, e
	}
	return Booking(ctx, db, id, false)
}
func BookingEvent(ctx context.Context, db DB, id, actor, previous, status, note string) error {
	_, e := db.Exec(ctx, `INSERT INTO booking_events(booking_id,actor_id,previous_status,new_status,note) VALUES($1,NULLIF($2,'')::uuid,NULLIF($3,''),$4,$5)`, id, actor, previous, status, note)
	return e
}
func LinkClient(ctx context.Context, db DB, id, clientID string) error {
	return Changed(ctx, db, "UPDATE bookings SET client_id=$2,updated_at=now() WHERE id=$1", id, clientID)
}
func Status(ctx context.Context, db DB, id, status, note, specialist string) error {
	return Changed(ctx, db, `UPDATE bookings SET status=$2,updated_at=now(),confirmed_at=CASE WHEN $2='confirmed' THEN COALESCE(confirmed_at,now()) WHEN $2='pending_confirmation' THEN NULL ELSE confirmed_at END,cancelled_at=CASE WHEN $2='cancelled' THEN now() WHEN $2='pending_confirmation' THEN NULL ELSE cancelled_at END,cancel_note=CASE WHEN $2='cancelled' THEN $3 ELSE cancel_note END,completed_at=CASE WHEN $2='completed' THEN now() ELSE NULL END,completed_by_specialist_id=CASE WHEN $2='completed' THEN NULLIF($4,'')::uuid ELSE NULL END WHERE id=$1`, id, status, note, specialist)
}
func CompletionEvent(ctx context.Context, db DB, id, specialist, action, note string) error {
	_, e := db.Exec(ctx, "INSERT INTO booking_completion_events(booking_id,specialist_id,action,note) VALUES($1,$2,$3,$4)", id, specialist, action, note)
	return e
}
func Assign(ctx context.Context, db DB, id, actor, specialist, room string, affected []string, before domain.M) error {
	if e := Changed(ctx, db, "UPDATE bookings SET specialist_id=$2,room_id=$3,assignment_updated_at=now(),updated_at=now() WHERE id=$1", id, specialist, room); e != nil {
		return e
	}
	if _, e := db.Exec(ctx, `INSERT INTO booking_assignment_events(booking_id,actor_id,previous_specialist_id,new_specialist_id,previous_room_id,new_room_id,affected_booking_ids,explicit_override) VALUES($1,$2,NULLIF($3,'')::uuid,$4,NULLIF($5,'')::uuid,$6,$7,$8)`, id, actor, before.String("specialistId"), specialist, before.String("roomId"), room, affected, len(affected) > 0); e != nil {
		return e
	}
	for _, affectedID := range affected {
		if e := CapacityException(ctx, db, id, affectedID, actor, "INDIRECT_ASSIGNMENT_CONFLICT"); e != nil {
			return e
		}
	}
	return nil
}
func CapacityException(ctx context.Context, db DB, cause, affected, actor, reason string) error {
	_, e := db.Exec(ctx, `INSERT INTO capacity_exceptions(resource_id,causing_booking_id,affected_booking_id,actor_id,reason) SELECT resource_id,NULLIF($1,'')::uuid,id,NULLIF($3,'')::uuid,$4 FROM bookings WHERE id=$2 AND NOT EXISTS(SELECT 1 FROM capacity_exceptions WHERE affected_booking_id=$2 AND reason=$4 AND resolved_at IS NULL)`, cause, affected, actor, reason)
	return e
}
func ResolveException(ctx context.Context, db DB, id string) error {
	_, e := db.Exec(ctx, "UPDATE capacity_exceptions SET resolved_at=now() WHERE affected_booking_id=$1 AND resolved_at IS NULL", id)
	return e
}
func BookingList(ctx context.Context, db DB, studio, client, date, status, specialist string) ([]domain.M, error) {
	return ObjectsPage(ctx, db, "SELECT "+BookingJSON+` FROM bookings b WHERE b.resource_id=$1 AND ($2='' OR b.client_id=NULLIF($2,'')::uuid) AND ($3='' OR b.local_date=NULLIF($3,'')::date) AND ($4='' OR b.status=$4) AND ($5='' OR (b.specialist_id=NULLIF($5,'')::uuid AND b.status IN ('pending_confirmation','confirmed','completed'))) ORDER BY b.starts_at,b.id`, studio, client, date, status, specialist)
}
func AssignmentEvents(ctx context.Context, db DB, id, specialist string) ([]domain.M, error) {
	return ObjectsPage(ctx, db, `SELECT jsonb_build_object('id',id,'bookingId',booking_id,'fromSpecialistId',previous_specialist_id,'toSpecialistId',new_specialist_id,'fromRoomId',previous_room_id,'toRoomId',new_room_id,'at',created_at) FROM booking_assignment_events WHERE ($1='' OR booking_id=NULLIF($1,'')::uuid) AND ($2='' OR previous_specialist_id=NULLIF($2,'')::uuid OR new_specialist_id=NULLIF($2,'')::uuid) ORDER BY created_at,id`, id, specialist)
}
func UpcomingBookings(ctx context.Context, db DB, studio string, now time.Time) ([]domain.M, error) {
	return Objects(ctx, db, "SELECT "+BookingJSON+" FROM bookings b WHERE b.resource_id=$1 AND b.starts_at>$2 AND b.status IN ('pending_confirmation','confirmed') ORDER BY b.starts_at,b.id LIMIT 10", studio, now)
}
