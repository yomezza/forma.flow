# Index plan

| Index | Access path |
|---|---|
| clients_phone_candidates (phone_normalized,id), non-unique | list ALL possible profile matches by phone (0..N); never automatic UPSERT |
| bookings_specialist_no_overlap + bookings_room_no_overlap GiST EXCLUDE | prevent direct double allocation of *assigned* specialists/rooms, including break; do not reject all studio overlaps |
| bookings_unassigned_upcoming | efficient fetch of pending unassigned obligations for studio solver |
| bookings_assigned_to_specialist / bookings_assigned_to_room | staff/room schedule reads |
| bookings_resource_start | month/day bookings and admin upcoming |
| bookings_client_history | per-client booking history DESC |
| bookings_pending partial | dashboard pending_confirmation sorted by starts_at |
| bookings_active_upcoming partial | active reservations per resource/window and availability queries |
| services_visible_order partial | public active catalogue |
| schedule_days_by_stage_date | published/draft calendar by date range |
| schedule_intervals_no_overlap GiST | prevent intersecting working intervals per (resource,date,stage) |
| vacations_no_overlap GiST | guarantee no overlapping inclusive vacation periods per resource |
| schedule_changes_recent | latest schedule actions per resource/revision |
| staff_accounts_email_unique | case-insensitive administrator lookup |
| staff_sessions(token_hash) UNIQUE | authenticate server-side cookie session |
| booking_events_by_booking | immutable status timeline |
| audit_log_recent | admin audit listing sorted by recency |
| idempotency_keys PRIMARY KEY | competing duplicate POST serialization |

Do not prematurely index every column. If client-name search becomes slow, measure and consider pg_trgm on lower(full_name) with GIN; for MVP non-unique phone index and bounded admin client listing are adequate. Run EXPLAIN (ANALYZE, BUFFERS) against realistic fixture volumes; verify index usage and sort costs in integration/performance tests. Avoid N+1 by fetching bookings and client/service in batch; schedule availability SELECT all bookings for bounded range once rather than per slot.

One more guarantee is **not expressible as an EXCLUDE constraint**: several NULL-assigned bookings may collectively overrun staff/room capacity. All capacity writes serialize on schedule_state for the studio and run the joint matcher inside the transaction; see [transaction plan](database-operations.md). Additional indexes: specialists_active, rooms_active_order, specialist_weekly_no_overlap, specialist_override_no_overlap, absence_by_specialist_dates, absence_pending, assignment_events_booking, assignment_events_previous_staff, completion_events_booking, capacity_exceptions_open.
