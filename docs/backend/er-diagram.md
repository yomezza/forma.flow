# Entity relationship diagram

~~~mermaid
erDiagram
  RESOURCES ||--|| SIMPLE_SCHEDULE : configured
  RESOURCES ||--|| SCHEDULE_STATE : active_mode
  RESOURCES ||--o{ SIMPLE_BLOCKED_DATES : blocks
  RESOURCES ||--o{ VACATIONS : takes
  RESOURCES ||--o{ SCHEDULE_DAYS : publishes
  SCHEDULE_DAYS ||--o{ SCHEDULE_INTERVALS : contains
  RESOURCES ||--o{ BOOKINGS : handles
  SERVICES ||--o{ BOOKINGS : snapshots
  CLIENTS o|--o{ BOOKINGS : assigned_by_admin
  BOOKINGS ||--o{ BOOKING_EVENTS : transitions
  BOOKINGS ||--o{ IDEMPOTENCY_KEYS : returns
  RESOURCES ||--o{ SCHEDULE_CHANGES : versions
  ADMIN_USERS ||--o{ ADMIN_SESSIONS : signs_in
  ADMIN_USERS ||--o{ BOOKING_EVENTS : records
  ADMIN_USERS ||--o{ SCHEDULE_CHANGES : publishes
  ADMIN_USERS ||--o{ AUDIT_LOG : acts
~~~

Actual columns/constraints: database-schema.sql. schedule_days composite primary key is (resource_id, day, stage); schedule_intervals composite FK uses all three columns. IDENTITY keys, session hashes and immutable booking snapshots are included in SQL even where diagram does not show field detail. Singleton site_settings and site_content stand alone without an artificial relation.
