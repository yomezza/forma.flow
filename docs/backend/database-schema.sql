-- FORMA TELA PostgreSQL 16–18 reference DDL. Apply through goose versioned migration.
-- No production data or initial passwords are inserted by this script.
BEGIN;
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE resources (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code text NOT NULL UNIQUE CHECK (code ~ '^[a-z0-9_-]{2,40}$'),
  display_name text NOT NULL CHECK (length(trim(display_name)) BETWEEN 1 AND 120),
  time_zone text NOT NULL CHECK (length(time_zone) BETWEEN 3 AND 100),
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE admin_users (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL CHECK (length(email) BETWEEN 3 AND 255),
  password_hash text NOT NULL,
  role text NOT NULL DEFAULT 'admin' CHECK (role IN ('admin','owner')),
  disabled_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX admin_users_email_unique ON admin_users (lower(email));

CREATE TABLE admin_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES admin_users(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  CHECK (expires_at > created_at)
);
CREATE INDEX admin_sessions_active_by_user ON admin_sessions(user_id,expires_at) WHERE revoked_at IS NULL;

CREATE TABLE services (
  id text PRIMARY KEY CHECK (id ~ '^[A-Za-z0-9_-]{1,100}$'),
  name text NOT NULL CHECK (length(trim(name)) BETWEEN 1 AND 120),
  description text NOT NULL DEFAULT '',
  duration_minutes integer NOT NULL CHECK (duration_minutes BETWEEN 1 AND 1440),
  price_amount bigint NOT NULL CHECK (price_amount BETWEEN 0 AND 100000000),
  currency char(3) NOT NULL DEFAULT 'RUB' CHECK (currency ~ '^[A-Z]{3}$'),
  visual_symbol text NOT NULL DEFAULT 'logo-01' CHECK (visual_symbol ~ '^logo-0[1-6]$'),
  sort_order integer NOT NULL DEFAULT 0 CHECK (sort_order BETWEEN 0 AND 9999),
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX services_visible_order ON services(sort_order,id) WHERE is_active;

CREATE TABLE clients (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  full_name text NOT NULL CHECK (length(trim(full_name)) BETWEEN 3 AND 120),
  phone_normalized varchar(12) NOT NULL CHECK (phone_normalized ~ '^\+7[0-9]{10}$'),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX clients_phone_candidates ON clients(phone_normalized,id);

CREATE TABLE simple_schedule (
  resource_id uuid PRIMARY KEY REFERENCES resources(id) ON DELETE RESTRICT,
  work_days smallint[] NOT NULL DEFAULT '{0,1,2,3,4,5,6}',
  day_start time NOT NULL DEFAULT '09:00',
  day_end time NOT NULL DEFAULT '21:00',
  step_minutes integer NOT NULL DEFAULT 120 CHECK (step_minutes BETWEEN 1 AND 1440),
  break_minutes integer NOT NULL DEFAULT 0 CHECK (break_minutes BETWEEN 0 AND 1440),
  CHECK (day_start < day_end)
);

CREATE TABLE simple_blocked_dates (
  resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE CASCADE,
  day date NOT NULL,
  PRIMARY KEY(resource_id,day)
);

CREATE TABLE vacations (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
  start_date date NOT NULL,
  end_date date NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (end_date >= start_date),
  CONSTRAINT vacations_no_overlap EXCLUDE USING gist
    (resource_id WITH =, daterange(start_date,end_date + 1,'[)') WITH &&)
);

CREATE TABLE schedule_state (
  resource_id uuid PRIMARY KEY REFERENCES resources(id) ON DELETE RESTRICT,
  active_mode text NOT NULL DEFAULT 'simple' CHECK (active_mode IN ('simple','advanced')),
  revision bigint NOT NULL DEFAULT 0 CHECK (revision >= 0),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE schedule_days (
  resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
  day date NOT NULL,
  stage text NOT NULL CHECK (stage IN ('draft','published')),
  kind text NOT NULL CHECK (kind IN ('working','off','unplanned')),
  step_override integer CHECK (step_override BETWEEN 1 AND 1440),
  updated_at timestamptz NOT NULL DEFAULT now(),
  PRIMARY KEY(resource_id,day,stage),
  CHECK (kind = 'working' OR step_override IS NULL),
  CHECK (stage = 'draft' OR kind != 'unplanned')
);
CREATE INDEX schedule_days_by_stage_date ON schedule_days(resource_id,stage,day);

CREATE TABLE schedule_intervals (
  resource_id uuid NOT NULL,
  day date NOT NULL,
  stage text NOT NULL,
  start_minute integer NOT NULL CHECK (start_minute BETWEEN 0 AND 1439),
  end_minute integer NOT NULL CHECK (end_minute BETWEEN 1 AND 1440),
  PRIMARY KEY(resource_id,day,stage,start_minute),
  FOREIGN KEY(resource_id,day,stage) REFERENCES schedule_days(resource_id,day,stage) ON DELETE CASCADE,
  CHECK (start_minute < end_minute),
  CONSTRAINT schedule_intervals_no_overlap EXCLUDE USING gist
    (resource_id WITH =, day WITH =, stage WITH =,
     int4range(start_minute,end_minute,'[)') WITH &&)
);

CREATE TABLE bookings (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
  service_id text NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
  client_id uuid REFERENCES clients(id) ON DELETE RESTRICT,
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
  CONSTRAINT bookings_no_overlap EXCLUDE USING gist
    (resource_id WITH =, tstzrange(starts_at,blocked_until,'[)') WITH &&)
    WHERE (status IN ('pending_confirmation','confirmed','completed'))
);
CREATE INDEX bookings_resource_start ON bookings(resource_id,starts_at DESC);
CREATE INDEX bookings_client_history ON bookings(client_id,starts_at DESC);
CREATE INDEX bookings_pending ON bookings(starts_at,id) WHERE status='pending_confirmation';
CREATE INDEX bookings_active_upcoming ON bookings(resource_id,starts_at) WHERE status IN ('pending_confirmation','confirmed');

CREATE TABLE booking_events (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  booking_id uuid NOT NULL REFERENCES bookings(id) ON DELETE RESTRICT,
  actor_id uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  previous_status text,
  new_status text NOT NULL,
  note text NOT NULL DEFAULT '',
  created_at timestamptz NOT NULL DEFAULT now(),
  CHECK (new_status IN ('pending_confirmation','confirmed','completed','cancelled'))
);
CREATE INDEX booking_events_by_booking ON booking_events(booking_id,created_at,id);

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
  actor_id uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  action text NOT NULL,
  before_state jsonb NOT NULL,
  after_state jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(resource_id,revision)
);
CREATE INDEX schedule_changes_recent ON schedule_changes(resource_id,revision DESC);

CREATE TABLE site_settings (
  id smallint PRIMARY KEY DEFAULT 1 CHECK (id=1),
  calendar_title text NOT NULL DEFAULT 'FORMA TELA' CHECK (length(calendar_title)<=120),
  address text NOT NULL DEFAULT '' CHECK (length(address)<=500),
  palette jsonb NOT NULL DEFAULT '{"green":"#284B35","sand":"#D8C6A6","paper":"#F8F5EE","gold":"#BE9D52"}'::jsonb,
  animations boolean NOT NULL DEFAULT true,
  reveal_duration integer NOT NULL DEFAULT 700 CHECK (reveal_duration BETWEEN 0 AND 3000),
  preloader_duration integer NOT NULL DEFAULT 1200 CHECK (preloader_duration BETWEEN 0 AND 15000),
  updated_at timestamptz NOT NULL DEFAULT now(),
  CHECK (jsonb_typeof(palette)='object')
);
CREATE TABLE site_content (
  id smallint PRIMARY KEY DEFAULT 1 CHECK(id=1),
  payload jsonb NOT NULL DEFAULT '{}'::jsonb CHECK(jsonb_typeof(payload)='object'),
  updated_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE audit_log (
  id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
  actor_id uuid REFERENCES admin_users(id) ON DELETE SET NULL,
  action text NOT NULL,
  entity_type text NOT NULL,
  entity_id text NOT NULL,
  details jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_recent ON audit_log(created_at DESC,id DESC);
COMMIT;
