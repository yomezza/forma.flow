package postgres

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"strings"
	"time"
)

// Snapshot is loaded in bounded batches inside a transaction by callers. The
// recursive query closes overlap chains, including jobs beyond the first window.
func Snapshot(ctx context.Context, db DB, id, from, to string) (domain.ScheduleData, error) {
	d := domain.ScheduleData{StudioID: id}
	e := db.QueryRow(ctx, `SELECT r.time_zone,s.active_mode,s.revision,s.capacity_revision FROM resources r JOIN schedule_state s ON s.resource_id=r.id WHERE r.id=$1`, id).Scan(&d.Zone, &d.Mode, &d.Revision, &d.CapacityRevision)
	if e != nil {
		return d, MapError(e)
	}
	d.Simple, e = Object(ctx, db, `SELECT jsonb_build_object('workDays',work_days,'start',to_char(day_start,'HH24:MI'),'end',to_char(day_end,'HH24:MI'),'step',step_minutes,'breakMinutes',break_minutes,'blockedDates',COALESCE((SELECT jsonb_agg(day::text ORDER BY day) FROM simple_blocked_dates WHERE resource_id=$1),'[]'::jsonb)) FROM simple_schedule WHERE resource_id=$1`, id)
	if e != nil {
		return d, e
	}
	queries := []struct {
		dest *[]domain.M
		q    string
		args []any
	}{
		{&d.Vacations, `SELECT jsonb_build_object('id',id,'startDate',start_date,'endDate',end_date) FROM vacations WHERE resource_id=$1`, []any{id}},
		{&d.Days, `SELECT jsonb_build_object('date',d.day,'stage',d.stage,'kind',d.kind,'stepOverride',d.step_override,'intervals',COALESCE((SELECT jsonb_agg(jsonb_build_object('start',lpad((i.start_minute/60)::text,2,'0')||':'||lpad((i.start_minute%60)::text,2,'0'),'end',lpad((i.end_minute/60)::text,2,'0')||':'||lpad((i.end_minute%60)::text,2,'0')) ORDER BY i.start_minute) FROM schedule_intervals i WHERE i.resource_id=d.resource_id AND i.day=d.day AND i.stage=d.stage),'[]'::jsonb)) FROM schedule_days d WHERE d.resource_id=$1`, []any{id}},
		{&d.Specialists, `SELECT jsonb_build_object('id',s.id,'state',s.state,'fullName',s.full_name,'serviceIds',COALESCE((SELECT jsonb_agg(service_id ORDER BY service_id) FROM specialist_services WHERE specialist_id=s.id),'[]'::jsonb),'week',COALESCE((SELECT jsonb_agg(jsonb_build_object('weekday',weekday,'startMinute',start_minute,'endMinute',end_minute) ORDER BY weekday,start_minute) FROM specialist_weekly_intervals WHERE specialist_id=s.id),'[]'::jsonb),'overrides',COALESCE((SELECT jsonb_agg(jsonb_build_object('date',o.day,'kind',o.kind,'intervals',COALESCE((SELECT jsonb_agg(jsonb_build_object('startMinute',start_minute,'endMinute',end_minute) ORDER BY start_minute) FROM specialist_override_intervals WHERE specialist_id=o.specialist_id AND day=o.day),'[]'::jsonb))) FROM specialist_day_overrides o WHERE o.specialist_id=s.id),'[]'::jsonb)) FROM specialists s WHERE resource_id=$1 ORDER BY id`, []any{id}},
		{&d.Rooms, `SELECT jsonb_build_object('id',id,'name',name,'isActive',is_active) FROM rooms WHERE resource_id=$1 ORDER BY sort_order,id`, []any{id}},
		{&d.Absences, `SELECT jsonb_build_object('id',a.id,'specialistId',a.specialist_id,'startDate',a.start_date,'endDate',a.end_date,'scope',a.scope,'startMinute',a.start_minute,'endMinute',a.end_minute) FROM absence_requests a JOIN specialists s ON s.id=a.specialist_id WHERE s.resource_id=$1 AND a.status='approved'`, []any{id}},
		{&d.Bookings, `WITH RECURSIVE component AS (SELECT b.id,b.starts_at,b.blocked_until FROM bookings b WHERE b.resource_id=$1 AND b.status IN ('pending_confirmation','confirmed','completed') AND b.starts_at<($3::date+2)::timestamp AT TIME ZONE $4 AND b.blocked_until>($2::date-2)::timestamp AT TIME ZONE $4 UNION SELECT b.id,b.starts_at,b.blocked_until FROM bookings b JOIN component c ON b.starts_at<c.blocked_until AND b.blocked_until>c.starts_at WHERE b.resource_id=$1 AND b.status IN ('pending_confirmation','confirmed','completed')) SELECT ` + BookingJSON + ` FROM bookings b WHERE b.id IN (SELECT id FROM component) ORDER BY b.starts_at,b.id LIMIT 5001`, []any{id, from, to, d.Zone}},
		{&d.Exceptions, `SELECT jsonb_build_object('bookingId',affected_booking_id,'reason',reason) FROM capacity_exceptions WHERE resource_id=$1 AND resolved_at IS NULL`, []any{id}},
	}
	for _, q := range queries {
		query := q.q
		if !strings.Contains(query, "LIMIT 5001") {
			query += " LIMIT 5001"
		}
		*q.dest, e = Objects(ctx, db, query, q.args...)
		if e != nil {
			return d, e
		}
		if len(*q.dest) > 5000 {
			return d, domain.Fail("CAPACITY_CHECK_LIMIT", 503, "Превышен бюджет расчёта.")
		}
	}
	return d, nil
}

const BookingJSON = `jsonb_build_object('id',b.id,'serviceId',b.service_id,'name',b.service_name,'durationMinutes',b.duration_minutes,'priceAmount',b.price_amount,'currency',b.currency,'date',b.local_date,'time',to_char(b.local_time,'HH24:MI'),'status',b.status,'clientId',b.client_id,'clientName',b.client_name,'clientPhone',b.client_phone,'specialistId',b.specialist_id,'roomId',b.room_id,'createdAt',b.created_at,'updatedAt',b.updated_at,'confirmedAt',b.confirmed_at,'cancelledAt',b.cancelled_at,'cancelNote',b.cancel_note,'assignmentUpdatedAt',b.assignment_updated_at,'completedAt',b.completed_at,'completedBySpecialistId',b.completed_by_specialist_id,'startsAt',b.starts_at,'endsAt',b.ends_at,'blockedUntil',b.blocked_until,'bufferMinutes',b.buffer_minutes)`

func Booking(ctx context.Context, db DB, id string, lock bool) (domain.M, error) {
	q := "SELECT " + BookingJSON + " FROM bookings b WHERE b.id=$1"
	if lock {
		q += " FOR UPDATE"
	}
	return Object(ctx, db, q, id)
}

func ActiveSnapshot(ctx context.Context, db DB, id string, now time.Time) (domain.ScheduleData, error) {
	fallback := now.UTC().Format("2006-01-02")
	var from, to string
	err := db.QueryRow(ctx, `SELECT COALESCE(min(local_date)::text,$2),COALESCE(max(local_date)::text,$2) FROM bookings WHERE resource_id=$1 AND status IN ('pending_confirmation','confirmed') AND blocked_until>$3`, id, fallback, now).Scan(&from, &to)
	if err != nil {
		return domain.ScheduleData{}, err
	}
	return Snapshot(ctx, db, id, from, to)
}
