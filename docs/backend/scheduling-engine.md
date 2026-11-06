# Scheduling engine (pure Go)

Inputs: service duration and active state; local date and now clock; resource IANA timezone; mode and published day rules; simple weekly start/end/day/step/break, blocked dates/vacations; existing non-cancelled booking spans. Result: sorted HH:mm slots and date state (past/working/full/off/unplanned/vacation/blocked).

1. Load only one active mode. If simple: block date on vacation or blockedDates, otherwise check weekday 0=Sunday. If advanced: absent published day returns unplanned; drafts are never read.
2. Validate working intervals [start,end) with positive step; a session must fit entirely within one interval.
3. Generate candidate start minutes from interval start plus n*step while start+service.durationMinutes <= interval.end.
4. Interpret local date/time in resource time_zone and verify round-trip with requested components. Reject nonexistent and ambiguous DST minutes (no arbitrary preference for fold instance). Exclude starts at/before now and apply configurable notice window when approved.
5. Calculate start/end timestamps and buffer based occupiedUntil = end + breakMinutes (elapsed duration). Candidate conflicts when candidate.start < busy.blockedUntil AND busy.start < candidate.blockedUntil; equality does not overlap.
6. Return HH:mm sorted and unique. In POST booking, recalculate under schedule_state lock and rely on PostgreSQL EXCLUDE to reject a race after the check.

~~~go
package schedule
import "time"
type Busy struct { Start, OccupiedUntil time.Time }
func Overlaps(aStart, aOccupiedUntil, bStart, bOccupiedUntil time.Time) bool {
    return aStart.Before(bOccupiedUntil) && bStart.Before(aOccupiedUntil)
}
~~~

Large range queries: fetch schedule days + intervals and busy rows once, calculate calendar in memory; avoid DB query per day or per candidate slot. Current app has no multi-provider selector; one resource is configured, with future resource_id routing reserved.

Out-of-schedule: reuse same eligibility function but ignore a booking's own collision for that decision; evaluate only fit to published work intervals/mode/step/blocked/vacation and optionally mark separate overlap anomalies. Do not update booking status. Preview and publish recalculate on same DB revision.

## Evidence from the actual `frontend` branch

- [`simple()`, `effective()`, `fits()`](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L122-L138) — current weekly/published rule and booking fit logic.
- [`available()`](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L226-L239) — existing slot generator and browser-clock dependence.
- [Public calendar calls](https://github.com/yomezza/forma.flow/blob/frontend/dist/app.js#L70-L103) and [admin publish preview](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/planner.js#L153-L157) — current consumers.

These are **reference implementations to analyze**, not authoritative rules for Go. In particular, time zones and schedule revision checks differ in the target Go architecture. See [cross-branch source index](frontend-source-links.md).
