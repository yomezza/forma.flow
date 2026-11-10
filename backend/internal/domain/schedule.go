package domain

import "time"

type ScheduleData struct {
	StudioID, Zone, Mode                                                string
	Revision, CapacityRevision                                          int64
	Simple                                                              M
	Vacations, Days, Specialists, Rooms, Absences, Bookings, Exceptions []M
}
type Clock interface{ Now() time.Time }
type RealClock struct{}

func (RealClock) Now() time.Time { return time.Now() }
