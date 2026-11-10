package postgres

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/domain"
)

func PutDay(ctx context.Context, db DB, studio, date, stage string, rule domain.M, intervals [][2]int) error {
	if _, e := db.Exec(ctx, "DELETE FROM schedule_days WHERE resource_id=$1 AND day=$2::date AND stage=$3", studio, date, stage); e != nil {
		return e
	}
	if stage == "published" && rule.String("kind") == "unplanned" {
		return nil
	}
	if _, e := db.Exec(ctx, "INSERT INTO schedule_days(resource_id,day,stage,kind,step_override) VALUES($1,$2::date,$3,$4,$5)", studio, date, stage, rule.String("kind"), rule["stepOverride"]); e != nil {
		return e
	}
	for _, i := range intervals {
		if _, e := db.Exec(ctx, "INSERT INTO schedule_intervals(resource_id,day,stage,start_minute,end_minute) VALUES($1,$2::date,$3,$4,$5)", studio, date, stage, i[0], i[1]); e != nil {
			return e
		}
	}
	return nil
}
func PutSimple(ctx context.Context, db DB, studio string, m domain.M) error {
	days := []int16{}
	for _, v := range m["workDays"].([]any) {
		days = append(days, int16(v.(float64)))
	}
	if e := Changed(ctx, db, "UPDATE simple_schedule SET work_days=$2,day_start=$3::time,day_end=$4::time,step_minutes=$5,break_minutes=$6 WHERE resource_id=$1", studio, days, m.String("start"), m.String("end"), m.Int("step"), m.Int("breakMinutes")); e != nil {
		return e
	}
	if _, e := db.Exec(ctx, "DELETE FROM simple_blocked_dates WHERE resource_id=$1", studio); e != nil {
		return e
	}
	for _, date := range m.Strings("blockedDates") {
		if _, e := db.Exec(ctx, "INSERT INTO simple_blocked_dates(resource_id,day) VALUES($1,$2::date)", studio, date); e != nil {
			return e
		}
	}
	return nil
}
func SetMode(ctx context.Context, db DB, studio, mode string) error {
	return Changed(ctx, db, "UPDATE schedule_state SET active_mode=$2 WHERE resource_id=$1", studio, mode)
}
func RecordSchedule(ctx context.Context, db DB, studio, actor, action string, before, after domain.M, capacity bool) error {
	var rev int64
	if e := db.QueryRow(ctx, "UPDATE schedule_state SET revision=revision+1,capacity_revision=capacity_revision+CASE WHEN $2 THEN 1 ELSE 0 END,updated_at=now() WHERE resource_id=$1 RETURNING revision", studio, capacity).Scan(&rev); e != nil {
		return e
	}
	if _, e := db.Exec(ctx, "INSERT INTO schedule_changes(resource_id,revision,actor_id,action,before_state,after_state) VALUES($1,$2,$3,$4,$5,$6)", studio, rev, actor, action, before, after); e != nil {
		return e
	}
	return Audit(ctx, db, actor, "schedule."+action, "resource", studio, domain.M{"revision": rev})
}

const HistoryJSON = `jsonb_build_object('id',id,'revision',revision,'action',action,'at',created_at,'before',before_state,'after',after_state)`

func History(ctx context.Context, db DB, studio string) ([]domain.M, error) {
	return ObjectsPage(ctx, db, "SELECT "+HistoryJSON+" FROM schedule_changes WHERE resource_id=$1 ORDER BY revision DESC", studio)
}
func HistoryEntry(ctx context.Context, db DB, id string) (domain.M, error) {
	return Object(ctx, db, "SELECT "+HistoryJSON+" FROM schedule_changes WHERE id=$1", id)
}
func Vacations(ctx context.Context, db DB, studio, contains string) ([]domain.M, error) {
	return ObjectsPage(ctx, db, "SELECT jsonb_build_object('id',id,'startDate',start_date,'endDate',end_date) FROM vacations WHERE resource_id=$1 AND ($2='' OR NULLIF($2,'')::date BETWEEN start_date AND end_date) ORDER BY start_date,id", studio, contains)
}
func SaveVacation(ctx context.Context, db DB, studio, id string, in domain.M) (domain.M, error) {
	if id == "" {
		return Object(ctx, db, "INSERT INTO vacations(resource_id,start_date,end_date) VALUES($1,$2::date,$3::date) RETURNING jsonb_build_object('id',id,'startDate',start_date,'endDate',end_date)", studio, in.String("startDate"), in.String("endDate"))
	}
	return Object(ctx, db, "UPDATE vacations SET start_date=$3::date,end_date=$4::date WHERE id=$1 AND resource_id=$2 RETURNING jsonb_build_object('id',id,'startDate',start_date,'endDate',end_date)", id, studio, in.String("startDate"), in.String("endDate"))
}
func DeleteVacation(ctx context.Context, db DB, studio, id string) error {
	return Changed(ctx, db, "DELETE FROM vacations WHERE id=$1 AND resource_id=$2", id, studio)
}
