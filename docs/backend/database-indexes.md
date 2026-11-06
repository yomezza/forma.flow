# Index plan

| Index | Access path |
|---|---|
| clients.phone_normalized UNIQUE | direct lookup and UPSERT during booking |
| bookings_no_overlap GiST partial EXCLUDE | non-overlapping reservations by resource, including buffer; also enforces concurrent writes |
| bookings_resource_start | month/day bookings and admin upcoming |
| bookings_client_history | per-client booking history DESC |
| bookings_pending partial | dashboard pending_confirmation sorted by starts_at |
| bookings_active_upcoming partial | active reservations per resource/window and availability queries |
| services_visible_order partial | public active catalogue |
| schedule_days_by_stage_date | published/draft calendar by date range |
| schedule_intervals_no_overlap GiST | prevent intersecting working intervals per (resource,date,stage) |
| vacations_no_overlap GiST | guarantee no overlapping inclusive vacation periods per resource |
| schedule_changes_recent | latest schedule actions per resource/revision |
| admin_users_email_unique | case-insensitive administrator lookup |
| admin_sessions(token_hash) UNIQUE | authenticate server-side cookie session |
| booking_events_by_booking | immutable status timeline |
| audit_log_recent | admin audit listing sorted by recency |
| idempotency_keys PRIMARY KEY | competing duplicate POST serialization |

Do not prematurely index every column. If client-name search becomes slow, measure and consider pg_trgm on lower(full_name) with GIN; for MVP phone unique + bounded client listing is adequate. Run EXPLAIN (ANALYZE, BUFFERS) against realistic fixture volumes; verify index usage and sort costs in integration/performance tests. Avoid N+1 by fetching bookings and client/service in batch; schedule availability SELECT all bookings for bounded range once rather than per slot.
