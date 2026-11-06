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
    SpecialistID, RoomID *string // assigned as a pair, may be nil even after confirmation
    CompletedBySpecialistID *string
    CompletedAt *time.Time
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

## Typed models for specialists and room assignment

~~~go
package domain
import "time"
type StaffRole string
const ( RoleOwner StaffRole="owner"; RoleAdmin StaffRole="admin"; RoleSpecialist StaffRole="specialist" )
type SpecialistState string
const ( SpecialistActive SpecialistState="active"; SpecialistArchived SpecialistState="archived" )
type Specialist struct { ID, ResourceID string; AccountID *string; FullName string; State SpecialistState; ServiceIDs []string }
type Room struct { ID, ResourceID, Name string; SortOrder int; IsActive bool }
type StaffAccount struct { ID, Email string; Roles []StaffRole; MustChangePassword bool }
type AbsenceStatus string
const ( AbsencePending AbsenceStatus="pending"; AbsenceApproved AbsenceStatus="approved"; AbsenceRejected AbsenceStatus="rejected"; AbsenceWithdrawn AbsenceStatus="withdrawn" )
type Absence struct {
  ID, SpecialistID string
  Kind, Scope string // day_off|vacation; full_days|partial_day
  StartDate, EndDate LocalDate
  StartMinute, EndMinute *int // partial_day only
  Status AbsenceStatus
  ReviewedAt *time.Time
}
type AssignmentEvent struct {
  BookingID, ActorAccountID string
  BeforeSpecialistID, BeforeRoomID *string
  AfterSpecialistID, AfterRoomID *string
  AffectedBookingIDs []string
  ExplicitOverride bool
}
~~~

`resource_id` означает **студию**. Важное правило БД: `specialist_id` и `room_id` обе NULL либо обе NOT NULL. `client_id` — независимо nullable до ручного выбора карточки. Статус брони не кодирует назначение. Личные интервалы `ClockMinute` отсчитываются в IANA зоне студии; период отсутствия может покрывать часть дня. Не допускать выдачи `ClientPhone` через DTO специалиста.


## Идея 01: источник и приоритет

Согласованные требования: [назначения и вместимость](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L26), [личные графики и отсутствия](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L28-L34), [роли](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L36-L43), [завершение и аудит](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L45-L52). Текущий frontend не реализует эти правила; Go/PostgreSQL-проект в данном документе — целевое состояние.
