-- +goose Up
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
  capacity_revision bigint NOT NULL DEFAULT 0 CHECK (capacity_revision >= 0),
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

-- One resources row represents a studio-wide schedule, not a single therapist or room.
CREATE TABLE specialists (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
  account_id uuid UNIQUE REFERENCES staff_accounts(id) ON DELETE RESTRICT,
  full_name text NOT NULL CHECK (length(trim(full_name)) BETWEEN 2 AND 120),
  state text NOT NULL DEFAULT 'active' CHECK (state IN ('active','archived')),
  archived_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(id,resource_id)
);
CREATE INDEX specialists_active ON specialists(resource_id,full_name) WHERE state='active';

CREATE TABLE rooms (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  resource_id uuid NOT NULL REFERENCES resources(id) ON DELETE RESTRICT,
  name text NOT NULL CHECK(length(trim(name)) BETWEEN 1 AND 80),
  sort_order integer NOT NULL DEFAULT 0 CHECK (sort_order BETWEEN 0 AND 9999),
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(id,resource_id)
);
CREATE INDEX rooms_active_order ON rooms(resource_id,sort_order,id) WHERE is_active;

CREATE TABLE specialist_services (
  specialist_id uuid NOT NULL REFERENCES specialists(id) ON DELETE RESTRICT,
  service_id text NOT NULL REFERENCES services(id) ON DELETE RESTRICT,
  PRIMARY KEY(specialist_id, service_id)
);

CREATE TABLE specialist_weekly_intervals (
  specialist_id uuid NOT NULL REFERENCES specialists(id) ON DELETE RESTRICT,
  weekday smallint NOT NULL CHECK (weekday BETWEEN 0 AND 6), -- 0=Sunday
  start_minute integer NOT NULL CHECK (start_minute BETWEEN 0 AND 1439),
  end_minute integer NOT NULL CHECK (end_minute BETWEEN 1 AND 1440),
  PRIMARY KEY(specialist_id,weekday,start_minute),
  CHECK (start_minute < end_minute),
  CONSTRAINT specialist_weekly_no_overlap EXCLUDE USING gist
    (specialist_id WITH =, weekday WITH =, int4range(start_minute,end_minute,'[)') WITH &&)
);

CREATE TABLE specialist_day_overrides (
  specialist_id uuid NOT NULL REFERENCES specialists(id) ON DELETE RESTRICT,
  day date NOT NULL,
  kind text NOT NULL CHECK (kind IN ('working','off')),
  PRIMARY KEY(specialist_id,day)
);
CREATE TABLE specialist_override_intervals (
  specialist_id uuid NOT NULL,
  day date NOT NULL,
  start_minute integer NOT NULL CHECK(start_minute BETWEEN 0 AND 1439),
  end_minute integer NOT NULL CHECK(end_minute BETWEEN 1 AND 1440),
  PRIMARY KEY(specialist_id,day,start_minute),
  FOREIGN KEY(specialist_id,day) REFERENCES specialist_day_overrides(specialist_id,day) ON DELETE CASCADE,
  CHECK (start_minute < end_minute),
  CONSTRAINT specialist_override_no_overlap EXCLUDE USING gist
    (specialist_id WITH =, day WITH =, int4range(start_minute,end_minute,'[)') WITH &&)
);

CREATE TABLE absence_requests (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  specialist_id uuid NOT NULL REFERENCES specialists(id) ON DELETE RESTRICT,
  kind text NOT NULL CHECK (kind IN ('vacation','day_off')),
  scope text NOT NULL CHECK (scope IN ('full_days','partial_day')),
  start_date date NOT NULL,
  end_date date NOT NULL,
  start_minute integer CHECK (start_minute BETWEEN 0 AND 1439),
  end_minute integer CHECK (end_minute BETWEEN 1 AND 1440),
  status text NOT NULL DEFAULT 'pending'
    CHECK (status IN ('pending','approved','rejected','withdrawn')),
  reason text NOT NULL DEFAULT '' CHECK (length(reason) <= 500),
  requested_by_account_id uuid REFERENCES staff_accounts(id) ON DELETE RESTRICT,
  requested_at timestamptz NOT NULL DEFAULT now(),
  reviewed_by_account_id uuid REFERENCES staff_accounts(id) ON DELETE RESTRICT,
  reviewed_at timestamptz,
  review_note text NOT NULL DEFAULT '' CHECK (length(review_note) <= 500),
  CHECK (start_date <= end_date),
  CHECK (
    (scope='full_days' AND start_minute IS NULL AND end_minute IS NULL)
    OR (scope='partial_day' AND start_date=end_date AND
        start_minute IS NOT NULL AND end_minute IS NOT NULL AND start_minute<end_minute)
  ),
  CHECK ((status IN ('approved','rejected')) = (reviewed_at IS NOT NULL))
);
CREATE INDEX absence_by_specialist_dates ON absence_requests(specialist_id,start_date,end_date)
  WHERE status='approved';
CREATE INDEX absence_pending ON absence_requests(requested_at,id) WHERE status='pending';


-- +goose Down
DROP TABLE absence_requests;
DROP TABLE specialist_override_intervals;
DROP TABLE specialist_day_overrides;
DROP TABLE specialist_weekly_intervals;
DROP TABLE specialist_services;
DROP TABLE rooms;
DROP TABLE specialists;
DROP TABLE schedule_intervals;
DROP TABLE schedule_days;
DROP TABLE schedule_state;
DROP TABLE vacations;
DROP TABLE simple_blocked_dates;
DROP TABLE simple_schedule;
