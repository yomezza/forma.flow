# PostgreSQL operations and transaction boundaries

## Public POST booking-requests: one transaction, no automatic client match

Normalize/validate form name, +7 telephone, service, cabinet-local date/time, consent and Idempotency-Key. Public POST identifies a **pending booking request**, not a verified client. One telephone can legitimately belong to several client UUIDs.

1. BEGIN READ COMMITTED.
2. INSERT idempotency_keys(scope,key,request_hash) ON CONFLICT DO NOTHING; SELECT key row FOR UPDATE. If hash differs => IDEMPOTENCY_CONFLICT 409; if booking_id already exists with same hash => 200 original receipt.
3. SELECT schedule_state WHERE resource_id=$resource FOR SHARE, serialize against schedule writers; load published schedule and service, recalculate slot from cabinet TZ and occupied bookings.
4. INSERT bookings with client_id=NULL, submitted client_name/client_phone, consent_version, pending_confirmation, service snapshot and timestamps. **Do not create or lookup client.** PostgreSQL GiST EXCLUDE is authoritative; SQLSTATE 23P01 => SLOT_UNAVAILABLE 409.
5. INSERT initial booking_events, UPDATE idempotency_keys.booking_id, COMMIT. New request => 201; identical retry => 200.

The lock order is idempotency key → schedule_state FOR SHARE → bookings. EXCLUDE prevents double booking across Go processes. No Go sync.Mutex required. A public response never reveals other clients or profile IDs.

## Admin finds possible profiles

~~~sql
SELECT id,full_name,phone_normalized,created_at,updated_at
FROM clients WHERE phone_normalized=$1
ORDER BY full_name,id LIMIT $2;
~~~

This returns **zero, one, or many** matches. Never automatically pick first. The old JS findClientByPhone returns one/null and must be adapted to findClientsByPhone returning a list.

## Admin explicitly assigns client to booking

PUT /admin/bookings/{id}/client accepts exactly one of {"clientId":"UUID"} OR {"newClient":{"fullName":"...","phone":"+7..."}}.
1. BEGIN; SELECT booking FOR UPDATE; validate admin rights.
2. Existing: SELECT clients WHERE id=$clientId. New: INSERT clients(full_name,phone_normalized) VALUES (...) RETURNING id; duplicate phone and even identical full name are allowed. Creating and linking happen atomically.
3. UPDATE bookings SET client_id=$id, updated_at=now() WHERE id=$bookingId; keep original booking.client_name/client_phone snapshots.
4. INSERT audit_log for association (old/new UUID and admin actor), COMMIT. Assigning the same UUID twice is idempotent. Changing a link is an explicit audited correction.

Admin confirms only after a client profile has been linked: pending_confirmation → confirmed requires client_id NOT NULL (otherwise CLIENT_LINK_REQUIRED, 409). An unlinked pending request may be cancelled. Restore of a cancelled booking goes to pending_confirmation and must pass schedule + EXCLUDE again; it may still have client_id NULL.

## Client records

INSERT clients permits repeated normalized phones. Only internal UUID is unique. Client phone/name edits do not rewrite booking snapshots. Bookings linked to client A are never associated with client B just because they share phone. No automatic merge or contact-person entity.

## Schedule publication

BEGIN; SELECT schedule_state FOR UPDATE, verify expectedRevision or 409; recompute conflicts; update only selected published days/drafts, increment revision, INSERT schedule_changes + audit_log, COMMIT. Existing bookings remain.

## Booking status transition

BEGIN; SELECT booking FOR UPDATE; check finite-state transition and for confirmation enforce non-NULL client_id. UPDATE status/timestamps, INSERT booking_events + audit_log; COMMIT. Cancelled frees time, completed remains historically occupied; restore additionally validates schedule and EXCLUDE.

## Representative SQL

~~~sql
SELECT starts_at,blocked_until,status FROM bookings
WHERE resource_id=$1 AND starts_at<$3 AND blocked_until>$2 AND status<>'cancelled' ORDER BY starts_at;
SELECT id,full_name,phone_normalized FROM clients WHERE phone_normalized=$1 ORDER BY full_name,id;
SELECT id,starts_at,status FROM bookings WHERE client_id=$1 ORDER BY starts_at DESC LIMIT $2;
SELECT id,starts_at,client_name,client_phone,client_id FROM bookings
WHERE status='pending_confirmation' ORDER BY starts_at LIMIT $1;
~~~

Apply pgx transactions, bounded queries, statement timeouts and actual PostgreSQL concurrency integration tests.
