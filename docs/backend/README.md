# FORMA TELA — Backend Architecture (Go + PostgreSQL)

Architecture documentation only; not an implemented backend. Based on the actual frontend branch, commit 1bdc8c63eb0db12445e4a6dc0bf717967df0d9cf (2026-11-06).

## Reading order
1. [Actual frontend assessment](investigation.md) and [all DemoAPI methods](api-mapping.md).
2. [Go architecture](go-architecture.md), [dependencies](go-dependencies.md), [domain models](go-domain-models.md), [service contracts](go-service-contracts.md).
3. [Business rules](business-rules.md), [schedule engine](scheduling-engine.md), [booking lifecycle](booking-lifecycle.md).
4. [Data model](data-model.md), [PostgreSQL DDL](database-schema.sql), [ER diagram](er-diagram.md), [database operations](database-operations.md), [indexes](database-indexes.md).
5. [OpenAPI 3.1](openapi.yaml), [frontend integration](frontend-integration.md).
6. [Auth/security](authorization.md), [migrations](migrations.md), [implementation guide](go-implementation-guide.md), [tests](go-testing.md), [decisions](decisions.md).

Frontend: frontend branch and JavaScript DemoAPI; backend developer: main and Go implementation; architecture author: backend-architecture only. No merges authorized.

## Invariants
- Public slots derive from active mode and published schedules only.
- pending_confirmation immediately reserves time; changing a schedule never deletes bookings.
- PostgreSQL EXCLUDE/GiST plus transactions, not Go process mutex, prevent double bookings.
- Cabinet-local dates and IANA zone are distinct from stored UTC instants.
- Administrative data requires server authentication and authorization.
- Destructive DemoAPI functions are NOT production HTTP endpoints.

Sources: https://github.com/yomezza/forma.flow/blob/frontend/dist/js/demo-api.js and https://github.com/yomezza/forma.flow/blob/frontend/docs/DEMO_API.md .
