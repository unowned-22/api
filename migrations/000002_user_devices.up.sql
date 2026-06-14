-- Add table to track known user devices for login notifications
CREATE TABLE user_devices (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  fingerprint TEXT NOT NULL,
  browser TEXT NOT NULL,
  platform TEXT,
  country TEXT,
  city TEXT,
  ip TEXT,
  last_seen TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, fingerprint, browser, country)
);
