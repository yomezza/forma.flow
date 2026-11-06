# Architecture decision log — 2026-11-06

## ADR-001: Modular Go monolith
Context: one massage cabinet and tightly coupled booking/schedule transactions. Decision: net/http, Go packages, single deployable service. Alternatives: Node backend, microservices, heavyweight Go framework. Consequence: fewer deployables; module boundaries remain explicit.

## ADR-002: PostgreSQL as consistency authority
Context: concurrent clients and multiple Go instances. Decision: transactional snapshot check plus EXCLUDE USING gist on occupied time ranges, including buffer. Alternative: in-memory mutex or only checking availability. Consequence: 23P01 is handled as 409; DB storage required for integration tests.

## ADR-003: Stable service text IDs, UUID other entities
Context: demo seeds service IDs classic, relax, back. Decision: preserve text service ID and canonical UUID for other server-generated entities. Consequence: migration of legacy non-UUID client IDs must map old IDs explicitly.

## ADR-004: No destructive production demo reset
Context: DemoAPI supports import/clear/delete. Decision: demo-only tools not exposed via public production REST; deletion of service becomes archiving while referenced. Consequence: frontend admin demo panel must distinguish local-only functions from production actions.

## ADR-005: Published schedule and revisions
Context: previews can become stale. Decision: lock schedule_state and require expectedRevision on writes; keep full before/after snapshots and restore into draft. Consequence: frontend passes revision, handles 409 and refreshes preview.

## ADR-006: Time and money
Context: browser-specific timezone and RUB integer UI price. Decision: database UTC instants with cabinet-local date/time and IANA TZ; BIGINT whole-RUB priceAmount for current contract. Consequence: fractional or multi-currency price semantics require explicit API evolution.

## ADR-007: Minimal dependencies
Context: small SQL surface and need controlled transactions. Decision: pgx directly, goose for migrations, standard HTTP/log/tests. Alternatives: sqlc now, ORM, chi. Re-evaluate sqlc after queries stabilize.

## ADR-008: Client telephone lookup is not identity proof
Context: existing phone-based reuse. Decision: reuse client id by normalized phone but do not expose existing client details or overwrite existing canonical name via public POST. Consequence: name conflict goes to admin workflow.

## ADR-009: Advanced vacations mirror demo
Context: code only applies vacations to simple mode. Decision: match current semantics and flag global vacation policy as a product question. Consequence: future change requires versioned migration/contract.
