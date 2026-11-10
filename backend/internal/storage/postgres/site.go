package postgres

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/domain"
)

func Settings(ctx context.Context, db DB, studio string) (domain.M, error) {
	return Object(ctx, db, `SELECT jsonb_build_object('calendarTitle',calendar_title,'address',address,'timeZone',(SELECT time_zone FROM resources WHERE id=$1),'palette',palette,'animations',animations,'revealDuration',reveal_duration,'preloaderDuration',preloader_duration) FROM site_settings WHERE id=1`, studio)
}
func Content(ctx context.Context, db DB) (domain.M, error) {
	return Object(ctx, db, "SELECT payload FROM site_content WHERE id=1")
}
func SaveSettings(ctx context.Context, db DB, m domain.M) error {
	return Changed(ctx, db, "UPDATE site_settings SET calendar_title=$1,address=$2,palette=$3,animations=$4,reveal_duration=$5,preloader_duration=$6,updated_at=now() WHERE id=1", m.String("calendarTitle"), m.String("address"), m.Object("palette"), m.Bool("animations"), m.Int("revealDuration"), m.Int("preloaderDuration"))
}
func SaveContent(ctx context.Context, db DB, m domain.M) error {
	return Changed(ctx, db, "UPDATE site_content SET payload=$1,updated_at=now() WHERE id=1", m)
}
func DashboardCounts(ctx context.Context, db DB, studio string) (domain.M, error) {
	return Object(ctx, db, `SELECT jsonb_build_object('pendingCount',count(*) FILTER(WHERE status='pending_confirmation'),'todayCount',count(*) FILTER(WHERE local_date=(now() AT TIME ZONE (SELECT time_zone FROM resources WHERE id=$1))::date),'upcomingConfirmedCount',count(*) FILTER(WHERE status='confirmed' AND starts_at>now()),'clientsCount',(SELECT count(*) FROM clients),'activeServicesCount',(SELECT count(*) FROM services WHERE is_active)) FROM bookings WHERE resource_id=$1`, studio)
}
