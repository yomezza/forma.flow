# Migration plan

Recommended: goose SQL migrations, deployed once before HTTP instances. database-schema.sql is reference PostgreSQL DDL; backend developer splits it into numbered goose files. Do not edit applied migrations in place.

~~~text
backend/migrations/
  00001_extensions_resources_admin.sql
  00002_catalog_clients_bookings.sql
  00003_schedule.sql
  00004_site_content_audit.sql
  00005_seed_primary_resource.sql
~~~

Migration dependency order: btree_gist, resources/staff_accounts/account_roles, services/clients/specialists/rooms/simple schedule, bookings/assigned-specialist-and-room EXCLUDE/idempotency, schedule days/changes, site records and audit. Seed primary resource IANA time_zone from explicit deployment configuration, never guess; seed services content optionally from audited demo snapshot. No seed admin password: first owner created through a separate secured bootstrap command.

SQL migration format: goose Up and Down sections; do not automatically downgrade destructive migrations in production. A down migration is permissible for empty test databases only. Expand/contract for schema evolution, dual-read rollout if needed, migrations executed under deployment lock, verified on PostgreSQL 16–18. On failure, prevent application startup with mismatched schema version.

Testing: apply from empty DB, reapply no-op, verify constraints, rollback in isolated test DB, forward migrate restored production-like anonymized fixtures; assert schema invariants under concurrent inserts.

Shared-phone correction (2026-11-06): the reference initial SQL uses a **non-unique** clients.phone_normalized index and nullable bookings.client_id. If a unique-phone version was deployed previously, apply a new forward migration dropping the UNIQUE constraint and relaxing the NOT NULL constraint, creating the candidate index. Never edit previously applied migrations in place. No actual backend migration has been deployed by this architect.

## Forward migration for idea 01 (if original schema already deployed)

**Never edit applied goose migrations in place.** New numbered migrations:

1. Rename `admin_users→staff_accounts`, `admin_sessions→staff_sessions` (retain PK/session hashes/foreign keys); create `account_roles`, populate roles from old `role`, add `must_change_password` without locking out existing owner accounts accidentally. Update login/permissions in same deployment, no gap in enforcement.
2. Create `specialists`, `rooms`, qualifications, weekly/day intervals, absence requests; seed one studio room and one linked specialist **without falsely attributing past sessions to them**. Explicit bootstrap owner/staff invitation.
3. Add nullable specialist_id, room_id, completion fields to bookings; add composite FK same studio, assigned-pair CHECK; remove old `bookings_no_overlap` constraint; create new assigned-specialist/room GiST EXCLUDE, staff/room indexes, assignment/completion events, capacity exceptions. **Deploy lock-and-solver code atomically with dropping global EXCLUDE**; otherwise overlapping NULL-assigned bookings could be accepted unsafely.
4. Add `schedule_state.capacity_revision` (default 0), migrate version counters, retrofit `schedule_changes.actor_id` / `audit_log.actor_id` references to staff_accounts. All previously deployed events and booking UUIDs must remain intact.
5. Rebuild statistics, verify parallel booking and assignment in two Go instances, verify rollback under failure. Apply only after reviewing real schema and testing a restore from anonymized production-like snapshot.

For a **fresh installation**, the latest reference [database-schema.sql](database-schema.sql) supplies the target table shapes; split it into ordered goose migrations respecting FKs and extension order. Do not represent this documentation as already executed SQL.


## Идея 01: источник и приоритет

Согласованные требования: [назначения и вместимость](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L15-L26), [личные графики и отсутствия](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L28-L34), [роли](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L36-L43), [завершение и аудит](https://github.com/yomezza/forma.flow/blob/ideas/01-specialists/idea.md#L45-L52). Текущий frontend не реализует эти правила; Go/PostgreSQL-проект в данном документе — целевое состояние.
