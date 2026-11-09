# Go: сервисные контракты

Сигнатуры — проект, не компилируемый код backend. HTTP handlers используют `context.Context` и не принимают бизнес-решения самостоятельно.

~~~go
package booking
import "context"

type CreateRequest struct {
    StudioID, ServiceID, Date, Time string
    FullName, Phone, ConsentVersion, IdempotencyKey string
}
type AssignClient struct { BookingID, ActorID string; ExistingClientID *string; NewClient *ClientInput }
type ClientInput struct { FullName, Phone string }
type AssignSpecialist struct {
    BookingID, SpecialistID, ActorAccountID string
    RoomID *string // nil means auto-select a compatible room
    ExpectedCapacityRevision int64
    AcknowledgedBookingIDs []string
}
type AssignmentPreview struct {
    Allowed, RequiresConfirmation bool
    CapacityRevision int64
    SpecialistID, RoomID, Reason string
    AffectedBookingIDs []string
}
type Service struct { /* pgx unit of work, schedule provider, solver, clock */ }
func (s *Service) CreateRequest(ctx context.Context, in CreateRequest) (Receipt,error) { panic("signature only") }
func (s *Service) PreviewAssignment(ctx context.Context, cmd AssignSpecialist) (AssignmentPreview,error) { panic("signature only") }
func (s *Service) AssignSpecialist(ctx context.Context, cmd AssignSpecialist) (Receipt,error) { panic("signature only") }
func (s *Service) LinkClient(ctx context.Context, cmd AssignClient) error { panic("signature only") }
func (s *Service) Confirm(ctx context.Context, id, actorID string) error { panic("signature only") }
func (s *Service) Cancel(ctx context.Context, id, actorID, note string) error { panic("signature only") }
func (s *Service) Restore(ctx context.Context, id, actorID string) error { panic("signature only") }
func (s *Service) CompleteMyBooking(ctx context.Context, id, specialistID string) error { panic("signature only") }
func (s *Service) UndoMyCompletion(ctx context.Context, id, specialistID, note string) error { panic("signature only") }
~~~

~~~go
package assignment
import ("context"; "time")
type Allocation struct { SpecialistID, RoomID string }
type Job struct { ID, ServiceID string; StartsAt, BlockedUntil time.Time; Fixed *Allocation }
type Result struct { SAT, Unknown bool; Witness map[string]Allocation; Reason string }
func Solve(ctx context.Context, jobs []Job, domains map[string][]Allocation, limit int) (Result,error) { panic("signature only") }
~~~

~~~go
package absence
import "context"
type Command struct {
    RequestID, ActorAccountID string
    ExpectedCapacityRevision int64
    AcknowledgedBookingIDs []string
    RejectionReason string
}
type Service struct{}
func (s *Service) SubmitMine(ctx context.Context, specialistID string, in RequestInput) (Request,error) { panic("signature only") }
func (s *Service) WithdrawMine(ctx context.Context, specialistID, requestID string) error { panic("signature only") }
func (s *Service) PreviewApproval(ctx context.Context, requestID string) (Preview,error) { panic("signature only") }
func (s *Service) Approve(ctx context.Context, cmd Command) error { panic("signature only") }
func (s *Service) Reject(ctx context.Context, cmd Command) error { panic("signature only") }
~~~

Выше `Receipt`, `RequestInput`, `Request`, `Preview` — заполняемые доменные структуры, а **не** контракты JSON; точные JSON схемы — [openapi.yaml](openapi.yaml).

## Handler → service → PostgreSQL

| HTTP | Сервис | Операции |
|---|---|---|
| PublicCreateBooking | booking.CreateRequest | idempotency tx, studio schedule_state FOR UPDATE, capacity solver, bookings INSERT nullable assignment, events |
| PublicListSlots/DateStates | schedule.AvailableSlots / DateStates | published studio + staff calendars/absences + rooms + bookings; pure solver |
| AdminPreviewAssignment | booking.PreviewAssignment | read snapshot + global solver, no writes |
| AdminAssignSpecialist | booking.AssignSpecialist | studio-lock, booking FOR UPDATE, direct GiST, witness/conflicts, assignment events, exceptions, audit |
| AdminSetSpecialistServices/Week | specialist.UpdateServices/UpdateSchedule | studio-lock, staff rules update, warnings and revision |
| AdminArchiveSpecialist | specialist.Archive | studio-lock, historical assignments preserved |
| AdminUpdateRoom | room.Update | studio-lock, room state, warnings |
| SpecialistSubmit/WithdrawAbsence | absence.SubmitMine / WithdrawMine | status events, no public availability change until approval |
| AdminApprove/RejectAbsence | absence.Approve / Reject | studio-lock for approved, conflicts/acknowledgement, absence request UPDATE, audit |
| SpecialistComplete/Undo | booking.CompleteMyBooking / UndoMyCompletion | authenticated own profile, booking lock, ends_at check, completion event + booking event |
| AdminConfirm/Cancel/Restore | booking.Confirm/Cancel/Restore | linked-client validation, status transition, studio-lock for cancel/restore |
| AdminPublishSchedule | schedule.Publish | studio-lock, schedule revision/ capacity revision, published days/history |
| AdminLinkClient | booking.LinkClient | booking lock, explicit client select or insert, audit |

**Ошибки:** `VALIDATION` 422, `CLIENT_LINK_REQUIRED` 409, `SPECIALIST_INELIGIBLE` 409, `ROOM_UNAVAILABLE` 409, `GLOBAL_CAPACITY_CONFLICT` 409, `CAPACITY_UNRESOLVED` 409, `CAPACITY_CHECK_LIMIT` 503, `VERSION_CONFLICT` 409, `CONFLICT_ACK_REQUIRED` 409, `NOT_ENDED_YET` 409, `FORBIDDEN` 403, `SLOT_UNAVAILABLE` 409. SQLSTATE 23P01 → direct assignment conflict, not public personal disclosure.

Интерфейсы определять в пакетах-потребителях (включая Clock), не создавать repository на каждый DTO. Use case atomicity — `storage/postgres` с транзакционным UnitOfWork, solver — чистый unit-testable пакет. [Транзакции](database-operations.md), [RBAC](authorization.md).

## Staff authentication services (idea 01)

~~~go
package auth
import "context"
type StaffInvitation struct { Email string; Roles []string; SpecialistID *string; ActorAccountID string }
type InvitationReceipt struct { AccountID, Email, TemporaryPassword string; Roles []string }
type ChangePassword struct { AccountID, CurrentPassword, NewPassword string }
func (s *Service) InviteStaff(ctx context.Context, in StaffInvitation) (InvitationReceipt,error) { panic("signature only") }
func (s *Service) CreateStaffSession(ctx context.Context, email, password string) (Session,error) { panic("signature only") }
func (s *Service) ChangeStaffPassword(ctx context.Context, in ChangePassword) (Session,error) { panic("signature only") }
func (s *Service) GetCurrentSession(ctx context.Context, accountID string) (Session,error) { panic("signature only") }
func (s *Service) Logout(ctx context.Context, sessionID string) error { panic("signature only") }
~~~

`Session` is a DTO mapped to `accountId,roles,activeRole,mustChangePassword,expiresAt,csrfToken` in [OpenAPI](openapi.yaml). `InviteStaff` is an admin-only use case; generate a cryptographically secure temporary secret, hash Argon2id, save account/roles/profile relation atomically, return the one-time secret exactly once with `no-store`. Password change requires current password, revokes other sessions, rotates the current session and releases the `must_change_password` gate. Never store plaintext in audit details.
