-- +goose Up
CREATE TABLE bookings (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
  service_id text NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
  client_id uuid REFERENCES clients(id) ON DELETE RESTRICT,
  specialist_id uuid,
  room_id uuid,
  assignment_updated_at timestamptz,
  completed_by_specialist_id uuid REFERENCES specialists(id) ON DELETE RESTRICT,
  completed_at timestamptz,
  FOREIGN KEY (specialist_id,resource_id) REFERENCES specialists(id,resource_id) ON DELETE RESTRICT,
  FOREIGN KEY (room_id,resource_id) REFERENCES rooms(id,resource_id) ON DELETE RESTRICT,
  client_name text NOT NULL CHECK (length(trim(client_name)) BETWEEN 3 AND 120),
  client_phone varchar(12) NOT NULL CHECK (client_phone ~ '^\+7[0-9]{10}$'),
  consent_version text NOT NULL CHECK (length(consent_version) BETWEEN 1 AND 80),
  consent_at timestamptz NOT NULL DEFAULT now(),
  service_name text NOT NULL,
  duration_minutes integer NOT NULL CHECK (duration_minutes BETWEEN 1 AND 1440),
  price_amount bigint NOT NULL CHECK (price_amount >= 0),
  currency char(3) NOT NULL DEFAULT 'RUB',
  local_date date NOT NULL,
  local_time time NOT NULL,
  starts_at timestamptz NOT NULL,
  ends_at timestamptz NOT NULL,
  blocked_until timestamptz NOT NULL,
  buffer_minutes integer NOT NULL DEFAULT 0 CHECK (buffer_minutes BETWEEN 0 AND 1440),
  status text NOT NULL DEFAULT 'pending_confirmation'
    CHECK (status IN ('pending_confirmation','confirmed','completed','cancelled')),
  confirmed_at timestamptz,
  cancelled_at timestamptz,
  cancel_note text CHECK (length(cancel_note) <= 500),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (starts_at < ends_at AND ends_at <= blocked_until),
  CHECK (ends_at = starts_at + make_interval(mins => duration_minutes)),
  CHECK (blocked_until = ends_at + make_interval(mins => buffer_minutes)),
  CHECK (status != 'confirmed' OR confirmed_at IS NOT NULL),
  CHECK (status NOT IN ('confirmed','completed') OR client_id IS NOT NULL),
  CHECK ((specialist_id IS NULL) = (room_id IS NULL)),
  -- Partial exclusion: a single therapist or room cannot be doubly assigned.
  -- Unassigned bookings (both NULL) are protected by serial capacity check,
  -- not a one-room global EXCLUDE constraint.
  CONSTRAINT bookings_specialist_no_overlap EXCLUDE USING gist
    (specialist_id WITH =, tstzrange(starts_at,blocked_until,'[)') WITH &&)
    WHERE (specialist_id IS NOT NULL AND status IN ('pending_confirmation','confirmed','completed')),
  CONSTRAINT bookings_room_no_overlap EXCLUDE USING gist
    (room_id WITH =, tstzrange(starts_at,blocked_until,'[)') WITH &&)
    WHERE (room_id IS NOT NULL AND status IN ('pending_confirmation','confirmed','completed'))
);
CREATE INDEX bookings_resource_start ON bookings(resource_id,starts_at DESC);
CREATE INDEX bookings_assigned_to_specialist ON bookings(specialist_id,starts_at DESC) WHERE specialist_id IS NOT NULL;
CREATE INDEX bookings_assigned_to_room ON bookings(room_id,starts_at DESC) WHERE room_id IS NOT NULL;
CREATE INDEX bookings_client_history ON bookings(client_id,starts_at DESC);
CREATE INDEX bookings_pending ON bookings(starts_at,id) WHERE status='pending_confirmation';
CREATE INDEX bookings_active_upcoming ON bookings(resource_id,starts_at) WHERE status IN ('pending_confirmation','confirmed');
CREATE INDEX bookings_unassigned_upcoming ON bookings(resource_id,starts_at) WHERE specialist_id IS NULL AND status IN ('pending_confirmation','confirmed');

CREATE TABLE booking_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  booking_id uuid NOT NULL REFERENCES bookings(id) ON DELETE RESTRICT,
  actor_id uuid REFERENCES staff_accounts(id) ON DELETE SET NULL,
  previous_status text,
  new_status text NOT NULL,
  note text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (new_status IN ('pending_confirmation','confirmed','completed','cancelled'))
);
CREATE INDEX booking_events_by_booking ON booking_events(booking_id,created_at,id);

CREATE TABLE booking_assignment_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  booking_id uuid NOT NULL REFERENCES bookings(id) ON DELETE RESTRICT,
  actor_id uuid NOT NULL REFERENCES staff_accounts(id) ON DELETE RESTRICT,
  previous_specialist_id uuid REFERENCES specialists(id) ON DELETE RESTRICT,
  new_specialist_id uuid REFERENCES specialists(id) ON DELETE RESTRICT,
  previous_room_id uuid REFERENCES rooms(id) ON DELETE RESTRICT,
  new_room_id uuid REFERENCES rooms(id) ON DELETE RESTRICT,
  affected_booking_ids uuid[] NOT NULL DEFAULT '{}',
  explicit_override boolean NOT NULL DEFAULT false,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX assignment_events_booking ON booking_assignment_events(booking_id,created_at,id);
CREATE INDEX assignment_events_previous_staff ON booking_assignment_events(previous_specialist_id,created_at DESC);

CREATE TABLE booking_completion_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  booking_id uuid NOT NULL REFERENCES bookings(id) ON DELETE RESTRICT,
  specialist_id uuid NOT NULL REFERENCES specialists(id) ON DELETE RESTRICT,
  action text NOT NULL CHECK (action IN ('completed','undone')),
  note text NOT NULL DEFAULT '' CHECK (length(note) <= 500),
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX completion_events_booking ON booking_completion_events(booking_id,created_at,id);

CREATE TABLE capacity_exceptions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
  causing_booking_id uuid REFERENCES bookings(id) ON DELETE RESTRICT,
  affected_booking_id uuid NOT NULL REFERENCES bookings(id) ON DELETE RESTRICT,
  reason text NOT NULL CHECK(reason IN ('INDIRECT_ASSIGNMENT_CONFLICT','REQUIRES_REASSIGNMENT','ROOM_INACTIVE')),
  actor_id uuid REFERENCES staff_accounts(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  resolved_at timestamptz
);
CREATE INDEX capacity_exceptions_open ON capacity_exceptions(resource_id,affected_booking_id) WHERE resolved_at IS NULL;

CREATE TABLE idempotency_keys (
  scope text NOT NULL,
  key text NOT NULL CHECK (length(key) BETWEEN 16 AND 128),
  request_hash char(64) NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
  booking_id uuid REFERENCES bookings(id) ON DELETE RESTRICT,
  created_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(scope,key)
);
CREATE INDEX idempotency_created ON idempotency_keys(created_at);

CREATE TABLE schedule_changes (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
  revision bigint NOT NULL,
  actor_id uuid REFERENCES staff_accounts(id) ON DELETE SET NULL,
  action text NOT NULL,
  before_state jsonb NOT NULL,
  after_state jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(resource_id,revision)
);
CREATE INDEX schedule_changes_recent ON schedule_changes(resource_id,revision DESC);


ALTER TABLE idempotency_keys ADD COLUMN response jsonb;

-- +goose Down
DROP TABLE schedule_changes;
DROP TABLE idempotency_keys;
DROP TABLE capacity_exceptions;
DROP TABLE booking_completion_events;
DROP TABLE booking_assignment_events;
DROP TABLE booking_events;
DROP TABLE bookings;
