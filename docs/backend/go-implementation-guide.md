# Go implementation roadmap — bounded, testable tasks

Each task specifies tables, HTTP operations and a concrete acceptance test. Implement in main only. Architecture docs remain in backend-architecture until authorized merge.

## Phase 0 — baseline
Create backend module as in go-architecture.md, go.mod pinned for Go 1.27, net/http, pgxpool, goose, slog. Add /healthz readiness check, config validation (DATABASE_URL, primary resource IANA zone), graceful shutdown. No bookings before timezone configured. CI gofmt, go vet, go test, dependency audit. Acceptance: start/stop locally, /healthz returns 200 only after database connectivity.

## Phase 1 — persistence
Split database-schema.sql into goose migrations and seed one resources row, schedule_state, simple_schedule, site_settings/site_content and initial catalog. Tables: all DDL; endpoints none yet. Acceptance: migrate up fresh PostgreSQL 16/18, check FK/EXCLUDE/unique constraints, repeat idempotently, test clean schema in integration.

## Phase 2 — public content and catalog
Build GET services/{id}, GET services, GET site/content and settings; catalog/site services. Tables: services, resources, site_settings, site_content. Acceptance: outputs match existing DemoAPI fields, including symbolic IDs and RUB integer price; inactive hidden on public list.

## Phase 3 — pure schedule engine
Implement LocalDate/ClockMinute parsing, basic/advanced rule selection, DST handling, steps, break buffers, date-state generation. Endpoints GET availability/dates, GET availability/slots. Tables: simple_schedule, simple_blocked_dates, vacations, schedule_days, schedule_intervals, bookings. Acceptance: table-driven test cases including DST and cross-boundary; no DB query per candidate slot.

## Phase 4 — atomic booking
Implement POST booking-requests, idempotency_keys, snapshot insertion with client_id=NULL (no client UPSERT), SQL EXCLUDE error mapping. Tables: bookings, booking_events, idempotency_keys (client creation is deliberately deferred until admin assignment). Acceptance: 20 simultaneous requests for same resource/time yield exactly 1 distinct non-cancelled booking; retries with same key receive original receipt; other keys conflict 409; DB remains consistent on errors.

## Phase 5 — admin authentication, clients, bookings
Implement Argon2id admin bootstrap, POST /admin/sessions, session cookie + CSRF, logout, role middleware, booking status actions, GET listings/dashboard, client CRUD, list-based phone candidate search, explicit PUT /admin/bookings/{id}/client (existing or new profile), and histories by client_id. Tables: admin_users, admin_sessions, clients, bookings, booking_events, audit_log. Acceptance: all admin routes reject unauthenticated access; each transition and prohibited transition tested; unlinked pending cannot be confirmed; duplicate phones on distinct profiles are supported; assignment/reassignment audited; cancelled frees slot; deleted booking forbidden; audit carries actor and action.

## Phase 6 — schedule mutations
Implement simple schedule PUT, vacation CRUD, advanced day drafts, preview, range publication, mode switch, history/version restore. Tables: schedule_state, schedule_days, schedule_intervals, simple_schedule, simple_blocked_dates, vacations, schedule_changes, audit_log. Acceptance: changed published revision invalidates stale preview; selected dates publish atomically; restore makes drafts only; existing bookings survive and get out-of-schedule warnings.

## Phase 7 — site administration and integration
Admin service edits/archive, site settings/content, owner export with no-store, frontend adapter contract tests. Tables: services, site_settings, site_content, audit_log. Acceptance: stale requests show useful errors; full UI service booking flow works across separate devices; calendar .ics and QR omit PII.

## Phase 8 — production reliability
Load testing, backup/PITR and restore exercises, migration deployment, rate limits, TLS, retention/legal policy review and monitoring. Acceptance: concurrent bookings across two Go instances stay unique; no PII in HTTP logs; production DB credentials least-privileged; rollback/restore plan tested.

Avoid premature Redis, jobs or distributed systems; measure first. A phase is complete only with CI and acceptance tests passing, not merely when handlers compile.
