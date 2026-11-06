# Business rules / source-of-truth

## Booking
Public visitor chooses active service, local date/time, enters surname/name and +7 phone and consent, submits request. The booking status starts as pending_confirmation and reserves time. The admin calls and manually confirms. Preserve service snapshot (name, duration, exact RUB integer price, currency), form name and telephone in the booking even if client/service is later edited. No anonymous confirmed booking endpoint.

Transitions: pending_confirmation → confirmed/cancelled; confirmed → completed/cancelled; cancelled → pending_confirmation only through authenticated restore after full availability check; completed terminal. Never delete historical bookings because a schedule changes. The status itself is not a schedule warning.

## Client
A phone is a contact field, not a unique identifier: relatives may share +7XXXXXXXXXX. Only client UUID identifies a person, and even (fullName, phone) is not guaranteed unique. Public POST creates pending_confirmation storing submitted fullName/phone and leaves client_id=NULL; it neither creates nor selects a client. An administrator views all candidates for that phone (0..N), explicitly links an existing profile or creates+links a new one. Confirmation requires a linked client, while cancellation before linking is permitted. Histories group by client_id and never by phone alone. Do not overwrite booking snapshots on association or edit. Client merges are reviewed/audited, never automatic. No separate contact-person entity.

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

## Verified frontend references and accepted divergences

Existing [status transitions](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L43-L52), [client match and public booking](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L303-L321), [schedule publication](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L273-L289), and [administrative confirmation button](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L129-L148) are linked for inspection. Unlike the current demo, the Go design does **not** uniquely identify a client by phone; the administrator must link the correct profile before confirmation.
