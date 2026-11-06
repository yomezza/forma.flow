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
