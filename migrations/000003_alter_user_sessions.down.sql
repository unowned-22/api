ALTER TABLE user_sessions
DROP CONSTRAINT IF EXISTS fk_user_sessions_device,
    DROP COLUMN IF EXISTS device_id,
    DROP COLUMN IF EXISTS status,
    DROP COLUMN IF EXISTS last_activity_at,
    DROP COLUMN IF EXISTS expires_at;

ALTER TABLE user_sessions
    ALTER COLUMN refresh_token_id SET NOT NULL,
ALTER COLUMN device_name      SET NOT NULL,
    ALTER COLUMN user_agent       SET NOT NULL,
    ALTER COLUMN ip_address       SET NOT NULL,
    ALTER COLUMN last_used_at     SET NOT NULL;