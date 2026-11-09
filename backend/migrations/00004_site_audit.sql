-- +goose Up
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
  actor_id uuid REFERENCES staff_accounts(id) ON DELETE SET NULL,
  action text NOT NULL,
  entity_type text NOT NULL,
  entity_id text NOT NULL,
  details jsonb NOT NULL DEFAULT '{}'::jsonb,
  created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX audit_log_recent ON audit_log(created_at DESC,id DESC);

-- +goose Down
DROP TABLE audit_log;
DROP TABLE site_content;
DROP TABLE site_settings;
