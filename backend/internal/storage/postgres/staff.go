package postgres

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/domain"
)

const SpecialistJSON = `jsonb_build_object('id',s.id,'resourceId',s.resource_id,'accountId',s.account_id,'fullName',s.full_name,'state',s.state,'archivedAt',s.archived_at,'serviceIds',COALESCE((SELECT jsonb_agg(service_id ORDER BY service_id) FROM specialist_services WHERE specialist_id=s.id),'[]'::jsonb))`

func Specialists(ctx context.Context, db DB, studio string, archived bool) ([]domain.M, error) {
	return ObjectsPage(ctx, db, "SELECT "+SpecialistJSON+" FROM specialists s WHERE resource_id=$1 AND ($2 OR state='active') ORDER BY full_name,id", studio, archived)
}
func Specialist(ctx context.Context, db DB, id string) (domain.M, error) {
	return Object(ctx, db, "SELECT "+SpecialistJSON+" FROM specialists s WHERE id=$1", id)
}
func SaveSpecialist(ctx context.Context, db DB, studio, id string, in domain.M) (string, error) {
	if id == "" {
		if e := db.QueryRow(ctx, "INSERT INTO specialists(resource_id,full_name,account_id) VALUES($1,$2,NULLIF($3,'')::uuid) RETURNING id::text", studio, in.String("fullName"), in.String("accountId")).Scan(&id); e != nil {
			return "", e
		}
	} else {
		if e := Changed(ctx, db, "UPDATE specialists SET full_name=$2,updated_at=now() WHERE id=$1 AND resource_id=$3", id, in.String("fullName"), studio); e != nil {
			return "", e
		}
	}
	if _, e := db.Exec(ctx, "DELETE FROM specialist_services WHERE specialist_id=$1", id); e != nil {
		return "", e
	}
	for _, service := range in.Strings("serviceIds") {
		if _, e := db.Exec(ctx, "INSERT INTO specialist_services(specialist_id,service_id) VALUES($1,$2)", id, service); e != nil {
			return "", e
		}
	}
	return id, nil
}
func ArchiveSpecialist(ctx context.Context, db DB, id string) error {
	if e := Changed(ctx, db, "UPDATE specialists SET state='archived',archived_at=now(),updated_at=now() WHERE id=$1", id); e != nil {
		return e
	}
	_, e := db.Exec(ctx, "UPDATE staff_sessions SET revoked_at=now() WHERE user_id IN (SELECT account_id FROM specialists WHERE id=$1) AND revoked_at IS NULL", id)
	return e
}

const RoomJSON = `jsonb_build_object('id',id,'resourceId',resource_id,'name',name,'sortOrder',sort_order,'isActive',is_active)`

func Rooms(ctx context.Context, db DB, studio string, inactive bool) ([]domain.M, error) {
	return ObjectsPage(ctx, db, "SELECT "+RoomJSON+" FROM rooms WHERE resource_id=$1 AND ($2 OR is_active) ORDER BY sort_order,id", studio, inactive)
}
func Room(ctx context.Context, db DB, id string) (domain.M, error) {
	return Object(ctx, db, "SELECT "+RoomJSON+" FROM rooms WHERE id=$1", id)
}
func SaveRoom(ctx context.Context, db DB, studio, id string, m domain.M) (domain.M, error) {
	if id == "" {
		return Object(ctx, db, "INSERT INTO rooms(resource_id,name,sort_order) VALUES($1,$2,$3) RETURNING "+RoomJSON, studio, m.String("name"), m.Int("sortOrder"))
	}
	return Object(ctx, db, "UPDATE rooms SET name=$2,sort_order=$3,is_active=$4 WHERE id=$1 AND resource_id=$5 RETURNING "+RoomJSON, id, m.String("name"), m.Int("sortOrder"), m.Bool("isActive"), studio)
}
func SavePersonalSchedule(ctx context.Context, db DB, id string, week map[int][][2]int, overrides map[string]PersonalOverride) error {
	for _, q := range []string{"DELETE FROM specialist_weekly_intervals WHERE specialist_id=$1", "DELETE FROM specialist_day_overrides WHERE specialist_id=$1"} {
		if _, e := db.Exec(ctx, q, id); e != nil {
			return e
		}
	}
	for weekday, intervals := range week {
		for _, i := range intervals {
			if _, e := db.Exec(ctx, "INSERT INTO specialist_weekly_intervals(specialist_id,weekday,start_minute,end_minute) VALUES($1,$2,$3,$4)", id, weekday, i[0], i[1]); e != nil {
				return e
			}
		}
	}
	for day, o := range overrides {
		if _, e := db.Exec(ctx, "INSERT INTO specialist_day_overrides(specialist_id,day,kind) VALUES($1,$2::date,$3)", id, day, o.Kind); e != nil {
			return e
		}
		for _, i := range o.Intervals {
			if _, e := db.Exec(ctx, "INSERT INTO specialist_override_intervals(specialist_id,day,start_minute,end_minute) VALUES($1,$2::date,$3,$4)", id, day, i[0], i[1]); e != nil {
				return e
			}
		}
	}
	return nil
}

type PersonalOverride struct {
	Kind      string
	Intervals [][2]int
}

const AbsenceJSON = `jsonb_build_object('id',a.id,'specialistId',specialist_id,'kind',kind,'scope',scope,'startDate',start_date,'endDate',end_date,'startTime',CASE WHEN start_minute IS NULL THEN NULL ELSE lpad((start_minute/60)::text,2,'0')||':'||lpad((start_minute%60)::text,2,'0') END,'endTime',CASE WHEN end_minute IS NULL THEN NULL ELSE lpad((end_minute/60)::text,2,'0')||':'||lpad((end_minute%60)::text,2,'0') END,'status',status,'reason',reason,'reviewNote',review_note,'reviewedAt',reviewed_at)`

func Absences(ctx context.Context, db DB, specialist string) ([]domain.M, error) {
	return ObjectsPage(ctx, db, "SELECT "+AbsenceJSON+" FROM absence_requests a WHERE ($1='' OR specialist_id=NULLIF($1,'')::uuid) ORDER BY requested_at,id", specialist)
}
func Absence(ctx context.Context, db DB, id string, lock bool) (domain.M, error) {
	q := "SELECT " + AbsenceJSON + " FROM absence_requests a WHERE id=$1"
	if lock {
		q += " FOR UPDATE"
	}
	return Object(ctx, db, q, id)
}
func CreateAbsence(ctx context.Context, db DB, actor, specialist string, in domain.M, start, end any) (domain.M, error) {
	return Object(ctx, db, `INSERT INTO absence_requests AS a(specialist_id,requested_by_account_id,kind,scope,start_date,end_date,start_minute,end_minute,reason) VALUES($1,$2,$3,$4,$5::date,$6::date,$7,$8,$9) RETURNING `+AbsenceJSON, specialist, actor, in.String("kind"), in.String("scope"), in.String("startDate"), in.String("endDate"), start, end, in.String("reason"))
}
func DecideAbsence(ctx context.Context, db DB, id, actor, status, note string) (domain.M, error) {
	return Object(ctx, db, `UPDATE absence_requests AS a SET status=$3,reviewed_by_account_id=CASE WHEN $3='withdrawn' THEN NULL ELSE $2::uuid END,reviewed_at=CASE WHEN $3='withdrawn' THEN NULL ELSE now() END,review_note=$4 WHERE id=$1 RETURNING `+AbsenceJSON, id, actor, status, note)
}
