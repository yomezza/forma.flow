# Go service contract examples

These are target signatures and boundaries, not a partial backend implementation.

~~~go
package booking
import "context"
type CreateRequest struct {
    ResourceID, ServiceID, Date, Time string
    FullName, PhoneNormalized, IdempotencyKey string
}
type BookingReceipt struct {
    ID, Status, ServiceID, Date, Time, Name, Currency string
    DurationMinutes int
    PriceAmount int64
}
type Service struct { /* postgres repository + schedule engine + clock */ }
func (s *Service) CreateRequest(ctx context.Context, req CreateRequest) (BookingReceipt,error) { panic("signature only") }
type AssignClientCommand struct { BookingID, AdminID string; ClientID *string; NewClient *CreateClientData }
type CreateClientData struct { FullName, PhoneNormalized string }
func (s *Service) AssignClient(ctx context.Context, cmd AssignClientCommand) error { panic("signature only") }
func (s *Service) Confirm(ctx context.Context, bookingID, adminID string) error { panic("signature only") }
func (s *Service) Restore(ctx context.Context, bookingID, adminID string) error { panic("signature only") }
~~~

~~~go
package schedule
import "context"
type AvailableQuery struct { ResourceID, ServiceID, Date string }
type Publication struct {
    ResourceID, From, To, ActorID string
    ExpectedRevision int64
    AcknowledgedBookingIDs []string
}
type Service struct {}
func (s *Service) AvailableSlots(ctx context.Context, q AvailableQuery) ([]string,error) { panic("signature only") }
func (s *Service) Publish(ctx context.Context, cmd Publication) (int64,error) { panic("signature only") }
~~~

| HTTP handler | Go service method | PostgreSQL operation |
|---|---|---|
| PublicCreateBooking | booking.CreateRequest | one transaction incl idempotency and unlinked booking (client_id NULL); no automatic client creation; EXCLUDE |
| PublicListSlots | schedule.AvailableSlots | snapshot of published rules and occupied spans |
| AdminConfirm/Cancel/Complete | booking.Transition | locked booking, update and event/audit |
| AdminRestore | booking.Restore | locked booking and schedule, exclusion re-check |
| AdminPublishSchedule | schedule.Publish | locked revision, replace selected days, change and audit |
| AdminSetMode | schedule.SetMode | locked revision, change snapshot |
| AdminUpdateClient | client.Update | validate name/phone, duplicate phone permitted, audit |
| AdminUpdateService | catalog.Update | price/duration validation and audit |

Inject clock as minimal interface or function returning time.Time and a small store/transaction boundary. Interfaces belong at the consuming service where needed. All I/O methods accept context.Context first. Domain errors should never expose SQL stack traces; transport maps them to HTTP codes and a stable error envelope.

AssignClient validates exactly one of ClientID or NewClient, locks booking, selects or creates client, associates and audits in one transaction. Confirm rejects unlinked requests with CLIENT_LINK_REQUIRED 409. client.ListByPhone(ctx,phone) returns []Client, never a singular guessed match. The public CreateRequest does not create a client.

## Идея 01: новые Go service contracts

~~~go
package booking
import "context"
type AssignmentCommand struct {
  BookingID, SpecialistID, ActorAccountID string
  RoomID *string // nil => choose available room
  ExpectedCapacityRevision int64
  AcknowledgedBookingIDs []string
}
type AssignmentPreview struct {
  Allowed, RequiresConfirmation bool
  SpecialistID, RoomID string
  CapacityRevision int64
  AffectedBookingIDs []string
  Reason string
}
func (s *Service) PreviewAssignment(ctx context.Context, cmd AssignmentCommand) (AssignmentPreview,error) { panic("contract only") }
func (s *Service) Assign(ctx context.Context, cmd AssignmentCommand) (AssignmentPreview,error) { panic("contract only") }
func (s *Service) MarkCompleted(ctx context.Context, id, specialistActorID string) error { panic("contract only") }
func (s *Service) UndoCompletion(ctx context.Context, id, specialistActorID, reason string) error { panic("contract only") }
~~~

~~~go
package absence
import "context"
type Decision struct { ID, ActorAccountID string; ExpectedCapacityRevision int64; AcknowledgedBookingIDs []string; RejectionReason string }
func (s *Service) SubmitOwn(ctx context.Context, accountID string, request any) error { panic("contract only") }
func (s *Service) Review(ctx context.Context, d Decision, approve bool) error { panic("contract only") }
~~~

Schedule.AvailableSlots now calls `assignment.Solve` across **all overlapping active bookings**, including unassigned. Repository reads are batch-based; preflight may run without lock, commit always takes `schedule_state FOR UPDATE`. Specialist service manages competence, hours and archive; room service manages active rooms; absence service owns pending/review/withdrawal; auth supports `account_roles` and staff-scoped DTO. All assignment and absence approvals recalculate candidate/warning set **inside** transaction. Specialist completion checks role, assignment identity, status and UTC ends_at; undo requires `completed_by_specialist_id` match. API handler mapping is in [api-mapping.md](api-mapping.md).


## Идея 01: источник и приоритет

Согласованные требования: [назначения и вместимость](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L26), [личные графики и отсутствия](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L28-L34), [роли](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L36-L43), [завершение и аудит](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L45-L52). Текущий frontend не реализует эти правила; Go/PostgreSQL-проект в данном документе — целевое состояние.
