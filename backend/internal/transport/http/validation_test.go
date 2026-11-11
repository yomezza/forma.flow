package httptransport

import (
	"errors"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestStrictBookingJSON(t *testing.T) {
	valid := `{"serviceId":"A","date":"2027-01-02","time":"10:00","fullName":"Иван Петров","phone":"+79991234567","consentAccepted":true,"consentVersion":"v1"}`
	for _, tt := range []struct {
		name, raw string
		status    int
	}{
		{"valid", valid, 0},
		{"duplicate field", strings.Replace(valid, `"serviceId":"A"`, `"serviceId":"B","serviceId":"A"`, 1), 400},
		{"two documents", valid + ` {}`, 400},
		{"bad date", strings.Replace(valid, "2027-01-02", "2027-02-30", 1), 422},
		{"forbidden assignment", strings.Replace(valid, `"serviceId":"A"`, `"specialistId":"someone","serviceId":"A"`, 1), 422},
		{"missing consent", strings.Replace(valid, `"consentAccepted":true`, `"consentAccepted":false`, 1), 422},
		{"body limit", strings.Replace(valid, `"v1"`, `"`+strings.Repeat("x", 33<<10)+`"`, 1), 400},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest("POST", "/api/v1/booking-requests", strings.NewReader(tt.raw))
			r.Header.Set("Content-Type", "application/json")
			_, err := readBody(httptest.NewRecorder(), r, "BookingRequest")
			if tt.status == 0 {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			var de *domain.Error
			if !errors.As(err, &de) || de.Status != tt.status {
				t.Fatalf("got %v want HTTP %d", err, tt.status)
			}
		})
	}
	if err := uniqueKeys([]byte(`{"newClient":{"phone":"one","phone":"two"}}`)); err == nil {
		t.Fatal("nested duplicate accepted")
	}
}
