# Business rules / source-of-truth

## Booking
Public visitor chooses active service, local date/time, enters surname/name and +7 phone and consent, submits request. The booking status starts as pending_confirmation and reserves time. The admin calls and manually confirms. Preserve service snapshot (name, duration, exact RUB integer price, currency), form name and telephone in the booking even if client/service is later edited. No anonymous confirmed booking endpoint.

Transitions: pending_confirmation → confirmed/cancelled; confirmed → completed/cancelled; cancelled → pending_confirmation only through authenticated restore after full availability check; completed terminal. Never delete historical bookings because a schedule changes. The status itself is not a schedule warning.

## Client
Telephone normalized into +7XXXXXXXXXX is unique for matching, not authentication. A new public booking with existing phone reuses client ID but never exposes or silently updates canonical client fullName. A mismatch is shown to admin using booking snapshot. Explicit merge/change phone is an audited admin-only procedure with review of associated history; no blind merge by name.

## Schedule
Two distinct independent modes: simple weekly graph with step, break, blocked dates, vacations; advanced dated published/drafts with working/off/unplanned, multiple intervals and stepOverride. Missing advanced published day is unplanned. Public never reads drafts. Vacations only affect simple mode, as the JavaScript code currently does. When switching modes, retain settings and preview conflict counts before commit. Revision on schedule_state prevents stale preview acceptance.

A publishing operation changes only selected dates, never deletes bookings, writes a change record and an audit record, and returns a fresh revision. Restore never directly republishes old versions; it copies historical before_state into draft. Publish with out-of-schedule bookings requires explicit acknowledgment of the IDs; server recalculates conflicts under lock.

## Open questions before public launch
- Set the actual IANA timezone of the cabinet (demo seed doesn't provide one).
- Confirm DST ambiguity policy (proposed: no slots at skipped/repeated minute).
- Confirm whether vacations should eventually apply to advanced mode (currently no).
- Confirm notice/cancellation windows and personal-data retention/erasure policy.
- Confirm production permissions for personal data export.

Business questions above do not block writing the architecture draft, but do block real production configuration.
