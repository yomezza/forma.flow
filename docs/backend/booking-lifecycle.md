# Booking lifecycle

~~~mermaid
stateDiagram-v2
  [*] --> pending_confirmation: public POST
  pending_confirmation --> confirmed: admin confirms after call
  pending_confirmation --> cancelled: admin cancels
  confirmed --> completed: admin completes
  confirmed --> cancelled: admin cancels
  cancelled --> pending_confirmation: admin restore, check availability
~~~

The tel: button is only a link; it does not confirm. New pending booking has client_id=NULL until admin explicitly chooses or creates the client profile; confirmation without profile returns 409 CLIENT_LINK_REQUIRED. Cancel without profile remains allowed. Changes to a link are audited and never rewrite original client_name/client_phone snapshots. Repeat same status changes may be treated as idempotent read/no-op; any other forbidden transition returns 409 INVALID_STATUS_TRANSITION.

Active reservation = pending_confirmation, confirmed or completed for the GiST non-overlap constraint. Completed occupies its past slot historically; cancelled releases the range. No automatic cancellation when changing working hours or vacation. Warning outOfSchedule is calculated for pending/confirmed from the currently active published mode and resource zone; it is not a new status.

Transaction: SELECT booking FOR UPDATE; validate transition and actor; for restore lock schedule_state FOR SHARE, recheck service schedule and slot and let EXCLUDE verify; UPDATE status and fields; INSERT booking_events; INSERT audit_log; COMMIT. Status update also updates confirmed_at/cancelled_at and UTC timestamps. Never delete booking_events. If a DB conflict occurs, return SLOT_UNAVAILABLE.

Public request requires a unique Idempotency-Key; same key and canonical payload return original receipt without new booking; mismatched payload is conflict. Include request hash and key in single transaction so network retry after lost response is safe. Never return full client identity or a guessable booking lookup endpoint to anonymous users.

## Existing frontend references

- [Demo status transitions](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L43-L52)
- [Demo public request creation, current automatic client lookup](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L314-L321)
- [Admin booking controls](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L129-L148)
- [Public submit and SLOT_UNAVAILABLE response](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/booking-form.js#L38-L56)

The **admin client-link prerequisite** is new architecture, not implemented by those JavaScript controls. See [integration tasks](frontend-integration.md).
