package schedule

import (
	"context"
	"github.com/yomezza/forma.flow/backend/internal/assignment"
	"github.com/yomezza/forma.flow/backend/internal/domain"
	"time"
)

type Engine struct {
	Clock                  domain.Clock
	MaxNodes, MaxComponent int
}

func Objects(value any) []domain.M {
	out := []domain.M{}
	switch v := value.(type) {
	case []domain.M:
		return v
	case []any:
		for _, x := range v {
			if m, ok := x.(map[string]any); ok {
				out = append(out, domain.M(m))
			}
		}
	}
	return out
}
func contains(ss []string, v string) bool {
	for _, s := range ss {
		if s == v {
			return true
		}
	}
	return false
}
func Rule(d domain.ScheduleData, date, stage string) domain.M {
	for _, r := range d.Days {
		if r.String("date") == date && r.String("stage") == stage {
			return domain.M{"kind": r["kind"], "intervals": r["intervals"], "stepOverride": r["stepOverride"]}
		}
	}
	return nil
}
func SimpleRule(d domain.ScheduleData, date string) (domain.M, string) {
	for _, v := range d.Vacations {
		if date >= v.String("startDate") && date <= v.String("endDate") {
			return closed("off"), "vacation"
		}
	}
	if contains(d.Simple.Strings("blockedDates"), date) {
		return closed("off"), "blocked"
	}
	day, _ := Date(date)
	works := false
	for _, v := range d.Simple["workDays"].([]any) {
		if int(v.(float64)) == int(day.Weekday()) {
			works = true
		}
	}
	if !works {
		return closed("off"), "off"
	}
	return domain.M{"kind": "working", "stepOverride": nil, "intervals": []domain.M{{"start": d.Simple["start"], "end": d.Simple["end"]}}}, "working"
}
func closed(kind string) domain.M {
	return domain.M{"kind": kind, "intervals": []domain.M{}, "stepOverride": nil}
}
func ActiveRule(d domain.ScheduleData, date string) (domain.M, string) {
	if d.Mode == "simple" {
		return SimpleRule(d, date)
	}
	r := Rule(d, date, "published")
	if r == nil {
		return closed("unplanned"), "unplanned"
	}
	return r, r.String("kind")
}
func Fits(rule domain.M, minute, duration int) bool {
	if rule.String("kind") != "working" {
		return false
	}
	for _, i := range Objects(rule["intervals"]) {
		start, e := Minute(i.String("start"))
		if e != nil {
			continue
		}
		end := 1440
		if i.String("end") != "24:00" {
			end, e = Minute(i.String("end"))
			if e != nil {
				continue
			}
		}
		if minute >= start && minute+duration <= end {
			return true
		}
	}
	return false
}
func Job(m domain.M) assignment.Job {
	s, _ := time.Parse(time.RFC3339Nano, m.String("startsAt"))
	end, _ := time.Parse(time.RFC3339Nano, m.String("blockedUntil"))
	j := assignment.Job{ID: m.String("id"), ServiceID: m.String("serviceId"), Start: s, OccupiedUntil: end}
	if sid := m.String("specialistId"); sid != "" {
		j.Fixed = &assignment.Pair{SpecialistID: sid, RoomID: m.String("roomId")}
	}
	return j
}
func Eligible(d domain.ScheduleData, staff, booking domain.M) bool {
	if staff.String("state") != "active" || !contains(staff.Strings("serviceIds"), booking.String("serviceId")) {
		return false
	}
	date := booking.String("date")
	day, _ := Date(date)
	start, _ := Minute(booking.String("time"))
	end := start + booking.Int("durationMinutes") + booking.Int("bufferMinutes")
	intervals := []domain.M{}
	override := false
	for _, o := range Objects(staff["overrides"]) {
		if o.String("date") == date {
			override = true
			if o.String("kind") == "working" {
				intervals = Objects(o["intervals"])
			}
			break
		}
	}
	if !override {
		for _, i := range Objects(staff["week"]) {
			if i.Int("weekday") == int(day.Weekday()) {
				intervals = append(intervals, i)
			}
		}
	}
	fits := false
	for _, i := range intervals {
		if start >= i.Int("startMinute") && end <= i.Int("endMinute") {
			fits = true
		}
	}
	if !fits {
		return false
	}
	for _, a := range d.Absences {
		if a.String("specialistId") != staff.String("id") || date < a.String("startDate") || date > a.String("endDate") {
			continue
		}
		if a.String("scope") == "full_days" || start < a.Int("endMinute") && a.Int("startMinute") < end {
			return false
		}
	}
	return true
}
func Domains(d domain.ScheduleData, booking domain.M) []assignment.Pair {
	out := []assignment.Pair{}
	for _, s := range d.Specialists {
		if !Eligible(d, s, booking) {
			continue
		}
		for _, r := range d.Rooms {
			if r.Bool("isActive") {
				out = append(out, assignment.Pair{SpecialistID: s.String("id"), RoomID: r.String("id")})
			}
		}
	}
	return out
}
func (e Engine) Check(ctx context.Context, d domain.ScheduleData, candidate domain.M, ignoreExceptions bool) (assignment.Result, []assignment.Job, error) {
	jobs := []assignment.Job{}
	records := map[string]domain.M{}
	for _, b := range d.Bookings {
		if b.String("id") == candidate.String("id") {
			continue
		}
		jobs = append(jobs, Job(b))
		records[b.String("id")] = b
	}
	jobs = append(jobs, Job(candidate))
	records[candidate.String("id")] = candidate
	component := assignment.Component(jobs, candidate.String("id"))
	unknown := assignment.Result{Outcome: assignment.UNKNOWN}
	if len(component) > e.MaxComponent {
		return unknown, component, nil
	}
	if !ignoreExceptions {
		for _, x := range d.Exceptions {
			for _, j := range component {
				if j.ID == x.String("bookingId") {
					return assignment.Result{Outcome: assignment.UNSAT}, component, nil
				}
			}
		}
	}
	domains := map[string][]assignment.Pair{}
	for _, j := range component {
		domains[j.ID] = Domains(d, records[j.ID])
	}
	result, err := assignment.Solve(ctx, component, domains, e.MaxNodes)
	return result, component, err
}
func (e Engine) Slots(ctx context.Context, d domain.ScheduleData, date string, service domain.M) ([]string, string, error) {
	if _, err := Date(date); err != nil {
		return nil, "", err
	}
	loc, err := time.LoadLocation(d.Zone)
	if err != nil {
		return nil, "", err
	}
	rule, state := ActiveRule(d, date)
	out := []string{}
	if date < e.Clock.Now().In(loc).Format("2006-01-02") {
		return out, "past", nil
	}
	if state != "working" {
		return out, state, nil
	}
	step := d.Simple.Int("step")
	if rule.Int("stepOverride") > 0 {
		step = rule.Int("stepOverride")
	}
	if step < 1 {
		return nil, "", domain.Validation("step")
	}
	for _, interval := range Objects(rule["intervals"]) {
		start, _ := Minute(interval.String("start"))
		end, _ := Minute(interval.String("end"))
		if interval.String("end") == "24:00" {
			end = 1440
		}
		for m := start; m+service.Int("durationMinutes") <= end; m += step {
			instant, err := Instant(date, Clock(m), loc)
			if err != nil || !instant.After(e.Clock.Now()) {
				continue
			}
			buffer := d.Simple.Int("breakMinutes")
			candidate := domain.M{"id": "candidate", "serviceId": service["id"], "date": date, "time": Clock(m), "durationMinutes": service["durationMinutes"], "bufferMinutes": buffer, "startsAt": instant.Format(time.RFC3339Nano), "blockedUntil": instant.Add(time.Duration(service.Int("durationMinutes")+buffer) * time.Minute).Format(time.RFC3339Nano)}
			result, _, err := e.Check(ctx, d, candidate, false)
			if err != nil {
				return nil, "", err
			}
			if result.Outcome == assignment.UNKNOWN {
				return nil, "", domain.Fail("CAPACITY_CHECK_LIMIT", 503, "Не удалось завершить расчёт вместимости.")
			}
			if result.Outcome == assignment.SAT {
				out = append(out, Clock(m))
			}
		}
	}
	if len(out) == 0 {
		return out, "full", nil
	}
	return out, "working", nil
}
