package schedule

import (
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"time"
)

func Date(value string) (time.Time, error) {
	d, err := time.Parse("2006-01-02", value)
	if err != nil || d.Format("2006-01-02") != value {
		return d, domain.Validation("date")
	}
	return d, nil
}
func Minute(value string) (int, error) {
	t, err := time.Parse("15:04", value)
	if err != nil || t.Format("15:04") != value {
		return 0, domain.Validation("time")
	}
	return t.Hour()*60 + t.Minute(), nil
}
func Clock(minute int) string {
	return time.Date(2000, 1, 1, minute/60, minute%60, 0, 0, time.UTC).Format("15:04")
}

// Instant rejects nonexistent and ambiguous wall-clock minutes instead of letting
// time.Date silently normalize a spring gap or choose one occurrence of a fold.
func Instant(date, clock string, loc *time.Location) (time.Time, error) {
	d, err := Date(date)
	if err != nil {
		return time.Time{}, err
	}
	m, err := Minute(clock)
	if err != nil {
		return time.Time{}, err
	}
	nominal := time.Date(d.Year(), d.Month(), d.Day(), m/60, m%60, 0, 0, time.UTC)
	offsets := map[int]bool{}
	for delta := -48; delta <= 48; delta++ {
		_, off := nominal.Add(time.Duration(delta) * time.Hour).In(loc).Zone()
		offsets[off] = true
	}
	var found []time.Time
	for offset := range offsets {
		candidate := nominal.Add(-time.Duration(offset) * time.Second)
		if candidate.In(loc).Format("2006-01-02 15:04") == date+" "+clock {
			found = append(found, candidate)
		}
	}
	if len(found) != 1 {
		return time.Time{}, domain.Validation("ambiguous or nonexistent local time")
	}
	return found[0], nil
}
