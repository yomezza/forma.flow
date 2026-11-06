# Relational model and invariants

## Entity responsibilities

- resources: owner of schedule and booking exclusion; one primary resource in MVP, multiple later; IANA time_zone.
- services: persistent text IDs compatible with classic/relax/back; BIGINT price_amount in whole RUB; is_active archives without deleting booking FK.
- clients: distinct UUID identity; normalized +7 telephone is NOT UNIQUE (relatives can share it), phone lookup returns multiple possible profiles; booking snapshots keep submitted name/phone.
- bookings: resource/service FKs and nullable client_id; a public pending request is deliberately unlinked until admin assigns an existing profile or creates+assigns a new profile. Confirmed/completed bookings require linked client_id. Store submitted name/phone, service snapshots, local and UTC timestamps, consent_version and consent_at.
- simple_schedule, simple_blocked_dates, vacations: weekly rules, date exceptions, inclusive vacation ranges; exclusively applied in simple mode in MVP.
- schedule_state: active mode and monotonically increasing revision; locked with FOR UPDATE for all schedule writes and booking policy reads.
- schedule_days (draft/published) and schedule_intervals: dated advanced rules, several intervals; absence of published rule => unplanned.
- schedule_changes: durable snapshots before/after each schedule mutation; restore copies historical before_state into drafts only.
- booking_events: append-only status timeline; audit_log: admin action trail.
- idempotency_keys: exactly-once effect for same scope/key+payload, transaction-local reservation.
- admin_users/admin_sessions: server auth, never frontend-only. site_settings/site_content: singleton content and appearance configuration.

## Key decisions

All IDs for clients, bookings, vacation, admin and resource are UUID; service ID is a stable text slug because the existing frontend uses symbolic IDs. ON DELETE RESTRICT protects references for bookings; admin removal archives services; PII deletion needs explicit separate anonymization policy. No cascading deletes of bookings or client history.

PostgreSQL GiST EXCLUDE on (resource_id, tstzrange(starts_at, blocked_until)) for non-cancelled bookings ensures no overlap even with multiple Go instances. Completed remains in the exclusion to preserve historical truth; cancelled frees interval. Buffer after each session is stored as blocked_until and does not shrink retroactively when settings change.

EXCLUDE on overlapping vacations and schedule_intervals; app service must additionally verify working days have at least one interval and that off/unplanned days have zero intervals. Constraints cannot express cross-row count simply without triggers; avoid unnecessary triggers.

Local date/time are snapshots and should equal starts_at expressed in the resource timezone at creation. The service verifies this, but resource timezone updates after bookings exist must be blocked or treated as explicit migration.

Working days 0=Sunday..6=Saturday matching JS getDay. Working hours do not cross midnight in MVP. Public API never returns clients/booking details or admin schedule drafts.

Full PostgreSQL DDL: database-schema.sql; index rationale: database-indexes.md.

SQL adds CHECK constraints tying ends_at to duration_minutes and blocked_until to buffer_minutes; together with the partial EXCLUDE this prevents inconsistent stored time spans from bypassing the collision rule. This is enforced in the database, not only in Go.

## Shared-phone client rule

A client is identified by UUID, not phone and not (name, phone). Several clients may share identical phone and name. Admin chooses existing profile or creates a new one explicitly when processing an unlinked booking. Public POST leaves client_id NULL and does not create clients. Cancellation before linking is allowed; confirming requires a link. Client history uses bookings.client_id exclusively. Admin reassignment is logged and never overwrites booking client_name/client_phone snapshots. No separate contact-person entity.

## Frontend source references

- [Default catalogue, service fields and simple schedule](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-data.js#L1-L18).
- [Client uniqueness in current demo](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L25-L43) and [booking-time automatic association](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L303-L321), both **superseded** for Go by the shared-phone rule.
- [Published/draft date rule representation](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js#L99-L118).
- [Client/card history rendered by frontend](https://github.com/yomezza/forma.flow/blob/frontend/dist/js/admin.js#L153-L176).
