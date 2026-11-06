# Доменные типы Go

Целевая модель: [идея 01](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L52). Типы ниже — архитектурные примеры, не готовая реализация.

~~~go
package domain

import "time"

type BookingStatus string
const (
    Pending   BookingStatus = "pending_confirmation"
    Confirmed BookingStatus = "confirmed"
    Completed BookingStatus = "completed"
    Cancelled BookingStatus = "cancelled"
)

type LocalDate string // validated YYYY-MM-DD in resource IANA timezone
type Interval struct { StartMinute, EndMinute int } // [start,end), 0..1440
type ScheduleMode string  // simple | advanced
type DayKind string       // working | off | unplanned

type Client struct {
    ID, FullName, PhoneNormalized string // Phone is NOT unique
}
type Specialist struct {
    ID, StudioID, FullName string
    AccountID *string
    State string // active | archived
    ServiceIDs []string
}
type Room struct {
    ID, StudioID, Name string
    SortOrder int
    IsActive bool
}
type StaffSchedule struct {
    SpecialistID string
    Week map[time.Weekday][]Interval
    Overrides map[LocalDate][]Interval // explicit off is empty slice, absence distinct
}
type AbsenceStatus string
const (
    AbsencePending AbsenceStatus = "pending"
    AbsenceApproved AbsenceStatus = "approved"
    AbsenceRejected AbsenceStatus = "rejected"
    AbsenceWithdrawn AbsenceStatus = "withdrawn"
)
type AbsenceRequest struct {
    ID, SpecialistID, Kind, Scope, Reason string
    StartDate, EndDate LocalDate
    StartMinute, EndMinute *int // nil for full_days
    Status AbsenceStatus
}
type Booking struct {
    ID, StudioID, ServiceID string
    ClientID *string                 // null until explicit admin linking
    SpecialistID, RoomID *string     // both null, or both assigned
    StartsAt, EndsAt, BlockedUntil time.Time
    Date LocalDate
    ClockTime string
    Status BookingStatus
    ClientName, ClientPhone, ServiceName, Currency string
    DurationMinutes int
    PriceAmount int64 // integer whole RUB
    CompletedBySpecialistID *string
    CompletedAt *time.Time
}
type Allocation struct { SpecialistID, RoomID string }
type AssignmentJob struct {
    BookingID, ServiceID string
    StartsAt, OccupiedUntil time.Time
    Fixed *Allocation // nil if unassigned
}
type SolveResult struct {
    Feasible bool
    Unknown bool
    Witness map[string]Allocation
    Reason string
    AffectedBookingIDs []string
}
~~~

## Инварианты

- Подтверждение клиента и назначение массажиста независимы. `confirmed` требует `ClientID != nil`, но не `SpecialistID`. Публичная pending заявка не связывает клиентскую карточку по телефону. При совпадающем номере админ выбирает личность по UUID.
- Конкретный специалист или кабинет не может одновременно иметь два активных закреплённых сеанса; nullable assignments не означают отсутствие резервирования студийной ёмкости.
- `Completed` для **новой** отметки устанавливает только назначенный специалист после `EndsAt`, отменяет собственную отметку в любой момент, append-only completion events. У legacy completed допустимо отсутствие автора.
- Личная неделя и исключения определяются администратором, утверждённые отсутствия вычитаются, pending requests не влияют на доступность.
- Роли `owner/admin/specialist` хранятся отдельно от аккаунтов, один аккаунт может иметь несколько ролей; привязка к `specialists.account_id` ограничивает свои booking reads.
- Старые snapshots ФИО/телефона/услуги/цены не перезаписывать при изменении клиента, специалиста и услуг.
- Проверять DST gap/fold по resource.TimeZone, не по Time.Local; `[start,end)` + stored buffer_minutes для конфликтов, шаг студийного графика для кандидатных слотов. `money` целый RUB в v1; float недопустим.

DTO HTTP: visitor receipt **без** clientPhone/clientId/specialistId/roomId; admin booking с назначениями и причиной предупреждений; specialist summary содержит `id,clientName,serviceName,date,time,durationMinutes,status,roomName`, **без** телефонных данных, client UUID и чужих записей. DTO сериализация и валидация находятся в transport.

Подробнее: [business-rules.md](business-rules.md), [scheduling-engine.md](scheduling-engine.md).
