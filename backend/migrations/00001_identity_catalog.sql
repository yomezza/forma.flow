-- +goose Up
CREATE EXTENSION IF NOT EXISTS btree_gist;

CREATE TABLE resources (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  code text NOT NULL UNIQUE CHECK (code ~ '^[a-z0-9_-]{2,40}$'),
  display_name text NOT NULL CHECK (length(trim(display_name)) BETWEEN 1 AND 120),
  time_zone text NOT NULL CHECK (length(time_zone) BETWEEN 3 AND 100),
  is_active boolean NOT NULL DEFAULT true,
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE staff_accounts (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  email text NOT NULL CHECK (length(email) BETWEEN 3 AND 255),
  password_hash text NOT NULL,
  must_change_password boolean NOT NULL DEFAULT true,
  disabled_at timestamptz,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX staff_accounts_email_unique ON staff_accounts (lower(email));
CREATE TABLE account_roles (
  account_id uuid NOT NULL REFERENCES staff_accounts(id) ON DELETE RESTRICT,
  role text NOT NULL CHECK (role IN ('owner','admin','specialist')),
  PRIMARY KEY(account_id,role)
);

CREATE TABLE staff_sessions (
  id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id uuid NOT NULL REFERENCES staff_accounts(id) ON DELETE CASCADE,
  token_hash bytea NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now(),
  expires_at timestamptz NOT NULL,
  revoked_at timestamptz,
  CHECK (expires_at > created_at)
);
CREATE INDEX staff_sessions_active_by_user ON staff_sessions(user_id,expires_at) WHERE revoked_at IS NULL;

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


-- +goose Down
DROP TABLE clients;
DROP TABLE services;
DROP TABLE staff_sessions;
DROP TABLE account_roles;
DROP TABLE staff_accounts;
DROP TABLE resources;
DROP EXTENSION btree_gist;
