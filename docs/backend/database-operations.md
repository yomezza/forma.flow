# PostgreSQL operations and transaction boundaries

## Create booking: mandatory transaction

1. Normalize/validate request and idempotency key, canonicalize request payload and compute SHA-256 (fullName, normalized phone, resource/service/date/time).
2. BEGIN READ COMMITTED.
3. INSERT INTO idempotency_keys(scope,key,request_hash) VALUES ('booking',key,hash) ON CONFLICT DO NOTHING; SELECT same row FOR UPDATE. If existing row with different hash => IDEMPOTENCY_CONFLICT. If booking_id set => return existing booking without another insert.
4. SELECT schedule_state FOR SHARE WHERE resource_id=$1, serializing against schedule mode/publication writers; SELECT service active, resource TZ, simple/advanced published rules and non-cancelled bookings for bounded date. Recalculate availability under lock, including break and DST.
5. INSERT INTO clients(full_name,phone_normalized) VALUES (...) ON CONFLICT(phone_normalized) DO NOTHING; SELECT id FROM clients WHERE phone_normalized=$phone. Do not overwrite existing client's full_name.
6. INSERT INTO bookings(..., status='pending_confirmation', starts_at, ends_at, blocked_until) VALUES ... RETURNING id. PostgreSQL exclusion constraint is authoritative: SQLSTATE 23P01 => SLOT_UNAVAILABLE, rollback.
7. INSERT booking_events initial row. UPDATE idempotency_keys SET booking_id=$booking WHERE scope='booking' AND key=$key; COMMIT.
8. Return 201, or 200 and same booking for exact idempotent replay. If client supplies a different payload under same key => 409.

Important: even after the SELECT slot check, another request can insert before us. EXCLUDE guarantees only one commit. Never use sync.Mutex for distributed correctness. Use consistent lock order schedule_state -> idempotency -> clients -> bookings to avoid deadlocks: first lock schedule_state FOR SHARE, then reserve idempotency key. Multiple FOR SHARE holders can proceed together; exclusion prevents double booking. Writers of schedule_state acquire FOR UPDATE.

## Schedule publication

BEGIN; SELECT schedule_state FOR UPDATE; compare expectedRevision or 409; read requested draft rows+intervals, calculate bookings outside proposed schedule, validate acknowledged ids; replace only published rows in range, delete selected drafts, increment revision; insert schedule_changes before/after snapshots and audit_log; COMMIT. No auto-delete/auto-cancel existing bookings.

## Status changes

BEGIN; SELECT booking FOR UPDATE; validate finite-state transition; UPDATE booking.status and timestamps; INSERT booking_events; INSERT audit_log; COMMIT. A cancelled booking can be restored only after fresh schedule validation and a successful EXCLUDE check. A completed booking is terminal.

## Queries (patterns)

~~~sql
-- Get upcoming active bookings once; calculate slots in Go.
SELECT starts_at, blocked_until, status
FROM bookings
WHERE resource_id = $1 AND starts_at < $3 AND blocked_until > $2
  AND status <> 'cancelled'
ORDER BY starts_at;

-- Client lookup:
SELECT id, full_name, phone_normalized FROM clients WHERE phone_normalized = $1;

-- Published advanced days (one range scan):
SELECT day, kind, step_override FROM schedule_days
WHERE resource_id=$1 AND stage='published' AND day BETWEEN $2 AND $3
ORDER BY day;

-- Pending queue:
SELECT id, starts_at, client_name, client_phone FROM bookings
WHERE status='pending_confirmation' ORDER BY starts_at LIMIT $1;
~~~

Backups: pg_dump/PITR as required by availability goals, restore drills, encryption and access controls; transactional audit and FK integrity should not depend on backups. Timeouts, bounded date ranges, cursor pagination and statement limits are required for production.
