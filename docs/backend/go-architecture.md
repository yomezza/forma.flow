# Native Go architecture

Choose a modular monolith, explicit SQL and narrow dependencies.

~~~text
backend/
  cmd/server/main.go
  internal/config/
  internal/domain/
  internal/booking/
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
- booking/client/catalog/schedule: application use cases and business decisions. Do not import HTTP.
- schedule/engine: pure, independently tested algorithm taking clock, published rules and busy intervals as arguments.
- domain: typed values, transition rules and local-date validation; no persistence.
- storage/postgres: concrete pgxpool transactions and SQL queries, conversion of SQLSTATE to typed errors.
- auth: password verification, session lifecycle, authorization roles.
- cmd/server: composition root, connection pool, routing, graceful shutdown.

Only introduce interfaces where tests or multiple implementations need them. Use context.Context as the first argument of I/O operations and deadlines for SQL. Keep network calls outside transactions. Structured log/slog with request IDs; never log names, telephone numbers or session tokens.

All writes that change schedule visibility lock resource schedule_state row; all booking writes lock it to stabilize schedule policy, then enforce EXCLUDE on booking time spans. READ COMMITTED + locking + DB constraints is sufficient for the described operations; no global process mutex. Multiple running instances remain safe.

A single resources row represents today's one therapist. resource_id is present in bookings and schedule tables so supporting multiple providers does not require rewriting booking identity. Actual resource assignment UI is postponed.

Environment: DATABASE_URL, HTTP_ADDR, PUBLIC_ORIGIN, COOKIE_SECURE, DB_POOL_MAX_CONNS, request/statement timeouts, and trusted proxy settings. IANA timezone is stored on resources, not inferred from server process or client browser. Migration runner is a separate deployment step.
