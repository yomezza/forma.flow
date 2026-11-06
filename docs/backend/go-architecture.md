# Native Go architecture

Choose a modular monolith, explicit SQL and narrow dependencies.

~~~text
backend/
  cmd/server/main.go
  internal/config/
  internal/domain/
  internal/booking/
  internal/assignment/       # pure joint eligibility solver
  internal/specialist/       # profiles, competence, personal hours
  internal/room/             # studio rooms
  internal/absence/          # absence requests/review
  internal/client/
  internal/catalog/
  internal/schedule/
  internal/auth/
  internal/admin/
  internal/storage/postgres/
  internal/transport/http/
  migrations/
  api/openapi.yaml
  tests/integration/
  go.mod
~~~

- transport/http: decode, validation at boundary, auth middleware, HTTP status and error envelopes, serialization.
- booking/client/catalog/schedule/specialist/room/absence: application use cases and business decisions. Do not import HTTP.
- schedule/engine + assignment solver: pure, independently tested algorithms taking clock, published rules and busy intervals as arguments.
- domain: typed values, transition rules and local-date validation; no persistence.
- storage/postgres: concrete pgxpool transactions and SQL queries, conversion of SQLSTATE to typed errors.
- auth: password verification, session lifecycle, authorization roles.
- cmd/server: composition root, connection pool, routing, graceful shutdown.

Only introduce interfaces where tests or multiple implementations need them. Use context.Context as the first argument of I/O operations and deadlines for SQL. Keep network calls outside transactions. Structured log/slog with request IDs; never log names, telephone numbers or session tokens.

**All capacity-affecting writes** (booking creation/cancel/restore, assignments, staff schedules, absences, rooms, studio publication) lock one studio `schedule_state` row **FOR UPDATE**, recalculate the global matching problem, write and increment `capacity_revision`. READ COMMITTED with that serialized protocol plus two GiST EXCLUDE constraints for assigned staff/room preserves capacity across processes. FOR SHARE is **not sufficient** for unassigned bookings. No Go process mutex.

One resources row represents the **studio**, not a therapist or physical room. There is one seeded room now and multiple may be added later. Specialist and room UUIDs are nullable in bookings until explicit admin assignment; each specialist has an account, allowed services, hours and approved absences. Account roles can combine admin and specialist.

Environment: DATABASE_URL, HTTP_ADDR, PUBLIC_ORIGIN, COOKIE_SECURE, DB_POOL_MAX_CONNS, request/statement timeouts, and trusted proxy settings. IANA timezone is stored on resources, not inferred from server process or client browser. Migration runner is a separate deployment step.


## Идея 01: источник и приоритет

Согласованные требования: [назначения и вместимость](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L26), [личные графики и отсутствия](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L28-L34), [роли](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L36-L43), [завершение и аудит](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L45-L52). Текущий frontend не реализует эти правила; Go/PostgreSQL-проект в данном документе — целевое состояние.
