package schedule

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"reflect"
	"testing"
	"time"
)

type fixedClock struct{ at time.Time }

func (c fixedClock) Now() time.Time { return c.at }
func engineFixture() domain.ScheduleData {
	return domain.ScheduleData{StudioID: "studio", Zone: "Etc/UTC", Mode: "simple", Simple: domain.M{"workDays": []any{float64(0), float64(1), float64(2), float64(3), float64(4), float64(5), float64(6)}, "start": "09:00", "end": "12:00", "step": 30, "breakMinutes": 0, "blockedDates": []any{}}, Specialists: []domain.M{{"id": "S1", "state": "active", "serviceIds": []string{"A"}, "week": []domain.M{{"weekday": 6, "startMinute": 540, "endMinute": 780}}}}, Rooms: []domain.M{{"id": "R1", "isActive": true}}}
}
func TestPublishedSlotRules(t *testing.T) {
	for _, tt := range []struct {
		name     string
		change   func(*domain.ScheduleData)
		duration int
		want     []string
		state    string
	}{
		{"simple", func(d *domain.ScheduleData) {}, 60, []string{"09:00", "09:30", "10:00", "10:30", "11:00"}, "working"},
		{"occupied", func(d *domain.ScheduleData) {
			d.Bookings = []domain.M{{"id": "old", "serviceId": "A", "date": "2027-01-02", "time": "10:00", "durationMinutes": 60, "startsAt": "2027-01-02T10:00:00Z", "blockedUntil": "2027-01-02T11:00:00Z"}}
		}, 60, []string{"09:00", "11:00"}, "working"},
		{"buffers", func(d *domain.ScheduleData) {
			d.Simple["breakMinutes"] = 15
			d.Bookings = []domain.M{{"id": "old", "serviceId": "A", "date": "2027-01-02", "time": "10:00", "durationMinutes": 60, "bufferMinutes": 15, "startsAt": "2027-01-02T10:00:00Z", "blockedUntil": "2027-01-02T11:15:00Z"}}
		}, 60, []string{}, "full"},
		{"unpublished", func(d *domain.ScheduleData) { d.Mode = "advanced" }, 60, []string{}, "unplanned"},
		{"draft", func(d *domain.ScheduleData) {
			d.Mode = "advanced"
			d.Days = []domain.M{{"date": "2027-01-02", "stage": "draft", "kind": "working", "stepOverride": nil, "intervals": []domain.M{{"start": "09:00", "end": "12:00"}}}}
		}, 60, []string{}, "unplanned"},
		{"split intervals", func(d *domain.ScheduleData) {
			d.Mode = "advanced"
			d.Days = []domain.M{{"date": "2027-01-02", "stage": "published", "kind": "working", "stepOverride": nil, "intervals": []domain.M{{"start": "09:00", "end": "10:00"}, {"start": "11:00", "end": "12:00"}}}}
		}, 90, []string{}, "full"},
		{"blocked", func(d *domain.ScheduleData) { d.Simple["blockedDates"] = []string{"2027-01-02"} }, 60, []string{}, "blocked"},
		{"vacation", func(d *domain.ScheduleData) {
			d.Vacations = []domain.M{{"startDate": "2027-01-02", "endDate": "2027-01-02"}}
		}, 60, []string{}, "vacation"},
		{"approved leave", func(d *domain.ScheduleData) {
			d.Absences = []domain.M{{"specialistId": "S1", "startDate": "2027-01-02", "endDate": "2027-01-02", "scope": "full_days"}}
		}, 60, []string{}, "full"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			d := engineFixture()
			tt.change(&d)
			e := Engine{Clock: fixedClock{time.Date(2027, 1, 1, 0, 0, 0, 0, time.UTC)}, MaxNodes: 10000, MaxComponent: 100}
			slots, state, err := e.Slots(context.Background(), d, "2027-01-02", domain.M{"id": "A", "durationMinutes": tt.duration})
			if err != nil || state != tt.state || !reflect.DeepEqual(slots, tt.want) {
				t.Fatalf("slots %v state %s error %v", slots, state, err)
			}
		})
	}
}
