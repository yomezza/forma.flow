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
  STAFF_ACCOUNTS ||--o{ STAFF_SESSIONS : signs_in
  STAFF_ACCOUNTS ||--o{ ACCOUNT_ROLES : has
  STAFF_ACCOUNTS o|--o| SPECIALISTS : personal_profile
  STAFF_ACCOUNTS ||--o{ BOOKING_EVENTS : records
  STAFF_ACCOUNTS ||--o{ SCHEDULE_CHANGES : publishes
  STAFF_ACCOUNTS ||--o{ AUDIT_LOG : acts
  RESOURCES ||--o{ SPECIALISTS : employs
  RESOURCES ||--o{ ROOMS : offers
  SPECIALISTS ||--o{ SPECIALIST_SERVICES : provides
  SERVICES ||--o{ SPECIALIST_SERVICES : qualified_for
  SPECIALISTS ||--o{ SPECIALIST_WEEKLY_INTERVALS : weekly
  SPECIALISTS ||--o{ SPECIALIST_DAY_OVERRIDES : overrides
  SPECIALIST_DAY_OVERRIDES ||--o{ SPECIALIST_OVERRIDE_INTERVALS : contains
  SPECIALISTS ||--o{ ABSENCE_REQUESTS : requests
  SPECIALISTS o|--o{ BOOKINGS : assigned
  ROOMS o|--o{ BOOKINGS : assigned
  BOOKINGS ||--o{ BOOKING_ASSIGNMENT_EVENTS : reassignments
  BOOKINGS ||--o{ BOOKING_COMPLETION_EVENTS : completions
  BOOKINGS ||--o{ CAPACITY_EXCEPTIONS : problems
~~~

Actual columns/constraints: database-schema.sql. schedule_days composite primary key is (resource_id, day, stage); schedule_intervals composite FK uses all three columns. IDENTITY keys, session hashes and immutable booking snapshots are included in SQL even where diagram does not show field detail. Singleton site_settings and site_content stand alone without an artificial relation.

Specialist/room assignment is **nullable as a pair** until admin assignment. Old global booking exclusion was removed; direct staff and room exclusions remain. See [SQL schema](database-schema.sql) and [transaction plan](database-operations.md).
