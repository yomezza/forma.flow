# Go domain types (examples; architecture, not implementation)

~~~go
package domain
import "time"
type BookingStatus string
const (
    Pending BookingStatus = "pending_confirmation"
    Confirmed BookingStatus = "confirmed"
    Completed BookingStatus = "completed"
    Cancelled BookingStatus = "cancelled"
)
type ScheduleMode string
const (Simple ScheduleMode = "simple"; Advanced ScheduleMode = "advanced")
type DayKind string
const (Working DayKind = "working"; Off DayKind = "off"; Unplanned DayKind = "unplanned")
type LocalDate string // validated YYYY-MM-DD in cabinet timezone
type ClockMinute uint16 // [0..1439] for start; end can be 1440
type Interval struct { Start, End int } // minutes from local midnight, [start,end)
type DayRule struct { Kind DayKind; Intervals []Interval; StepOverride *int }
type Booking struct {
    ID, ResourceID, ServiceID string
    ClientID *string // nil until admin explicitly assigns profile
    StartsAt, EndsAt, BlockedUntil time.Time
    Date LocalDate
    Time string
    Status BookingStatus
    ServiceName, Currency, ClientName, ClientPhone string
    DurationMinutes int
    PriceAmount int64
}
type Client struct { ID, FullName, PhoneNormalized string }
type Service struct {
    ID, Name, Description, Currency, VisualSymbol string
    DurationMinutes int
    PriceAmount int64
    SortOrder int
    IsActive bool
}
~~~

Date/time: time.LoadLocation(resource.TimeZone); parse local calendar date/time and verify round-trip to local components. DST nonexistent or ambiguous local minutes are rejected in MVP (VALIDATION). Store starts_at, ends_at, blocked_until as PostgreSQL timestamptz; never compare browser-local JS Date with cabinet local date.

Phone: normalize ten digits or initial 7/8 + ten digits into +7XXXXXXXXXX, matching existing frontend. This is only normalization; it neither uniquely identifies a client nor proves identity. Multiple client UUIDs may share a phone, public request does not assign one.

Money: priceAmount from DemoAPI is an integer number of RUB, preserved as PostgreSQL BIGINT (exact). Multicurrency fractional values require a deliberate new minor-unit contract; never convert implicitly through float64. Snapshots of service metadata and supplied contact name/phone remain in booking even after client/catalog edits.

Use HTTP DTO with explicit JSON camelCase tags in transport, distinct from domain structs. UUID string APIs keep existing UI IDs strings, while SQL uses UUID for booking/client/resource; service IDs stay text because frontend initial IDs are classic/relax/back.
