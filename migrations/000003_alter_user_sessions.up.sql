-- Drop NOT NULL from legacy columns that are no longer populated by the new
-- session model. The columns themselves are kept so existing data is preserved
-- and the down migration can restore the constraints if needed.
ALTER TABLE user_sessions
    ALTER COLUMN refresh_token_id DROP NOT NULL,
ALTER COLUMN device_name      DROP NOT NULL,
    ALTER COLUMN user_agent       DROP NOT NULL,
    ALTER COLUMN ip_address       DROP NOT NULL,
    ALTER COLUMN last_used_at     DROP NOT NULL;

-- Add new columns (nullable first so existing rows don't break).
ALTER TABLE user_sessions
    ADD COLUMN IF NOT EXISTS device_id        bigint,
    ADD COLUMN IF NOT EXISTS status           text NOT NULL DEFAULT 'active',
    ADD COLUMN IF NOT EXISTS last_activity_at timestamptz,
    ADD COLUMN IF NOT EXISTS expires_at       timestamptz;

-- Backfill last_activity_at from last_used_at.
UPDATE user_sessions SET last_activity_at = last_used_at WHERE last_activity_at IS NULL;

-- Backfill status from revoked_at.
UPDATE user_sessions SET status = 'revoked' WHERE revoked_at IS NOT NULL;

-- Backfill expires_at (30 days from created_at as a safe default).
UPDATE user_sessions SET expires_at = created_at + interval '30 days' WHERE expires_at IS NULL;

-- Enforce NOT NULL now that backfill is complete.
ALTER TABLE user_sessions ALTER COLUMN last_activity_at SET NOT NULL;
ALTER TABLE user_sessions ALTER COLUMN expires_at        SET NOT NULL;

-- Add FK to user_devices (deferrable so backfill can follow later).
ALTER TABLE user_sessions
    ADD CONSTRAINT fk_user_sessions_device
        FOREIGN KEY (device_id) REFERENCES user_devices(id)
            DEFERRABLE INITIALLY DEFERRED;