-- Add deactivated_at to users
ALTER TABLE users
    ADD COLUMN IF NOT EXISTS deactivated_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS idx_users_deactivated_at ON users(deactivated_at);
