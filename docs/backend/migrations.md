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

Migration dependency order: btree_gist, resources/admin_users, services/clients/simple schedule, bookings/EXCLUDE/idempotency, schedule days/changes, site records and audit. Seed primary resource IANA time_zone from explicit deployment configuration, never guess; seed services content optionally from audited demo snapshot. No seed admin password: first owner created through a separate secured bootstrap command.

SQL migration format: goose Up and Down sections; do not automatically downgrade destructive migrations in production. A down migration is permissible for empty test databases only. Expand/contract for schema evolution, dual-read rollout if needed, migrations executed under deployment lock, verified on PostgreSQL 16–18. On failure, prevent application startup with mismatched schema version.

Testing: apply from empty DB, reapply no-op, verify constraints, rollback in isolated test DB, forward migrate restored production-like anonymized fixtures; assert schema invariants under concurrent inserts.

Shared-phone correction (2026-11-06): the reference initial SQL uses a **non-unique** clients.phone_normalized index and nullable bookings.client_id. If a unique-phone version was deployed previously, apply a new forward migration dropping the UNIQUE constraint and relaxing the NOT NULL constraint, creating the candidate index. Never edit previously applied migrations in place. No actual backend migration has been deployed by this architect.
