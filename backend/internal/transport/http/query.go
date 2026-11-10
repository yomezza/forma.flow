package httptransport

import (
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"github.com/yomezza/forma.flow/backend/internal/schedule"
	"net/http"
	"regexp"
)

func validateQuery(r *http.Request, rt route) error {
	q := r.URL.Query()
	for _, key := range []string{"date", "from", "to", "contains"} {
		if v := q.Get(key); v != "" {
			if _, e := schedule.Date(v); e != nil {
				return e
			}
		}
	}
	if date := r.PathValue("date"); date != "" {
		if _, e := schedule.Date(date); e != nil {
			return e
		}
	}
	for _, key := range []string{"includeArchived", "includeInactive"} {
		if v := q.Get(key); v != "" && v != "true" && v != "false" {
			return domain.Validation(key)
		}
	}
	if status := q.Get("status"); status != "" && status != "pending_confirmation" && status != "confirmed" && status != "completed" && status != "cancelled" {
		return domain.Validation("status")
	}
	if rt.Operation == "adminListScheduleDays" {
		if q.Get("stage") != "draft" && q.Get("stage") != "published" {
			return domain.Validation("stage")
		}
		if q.Get("from") == "" || q.Get("to") == "" {
			return domain.Validation("range")
		}
	}
	if rt.Operation == "getSlots" && (q.Get("date") == "" || q.Get("serviceId") == "") {
		return domain.Validation("date/serviceId")
	}
	if id := r.PathValue("id"); id != "" && rt.Operation != "getService" && rt.Operation != "adminUpdateService" && rt.Operation != "adminArchiveService" {
		if ok, _ := regexp.MatchString(`^[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}$`, id); !ok {
			return domain.Validation("id")
		}
	}
	return nil
}
