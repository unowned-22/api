-- 000009_user_settings.up.sql
-- Create user_settings table

CREATE TABLE IF NOT EXISTS user_settings (
    user_id              BIGINT      PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    storage_quota_bytes  BIGINT      NOT NULL DEFAULT 1073741824,
    storage_used_bytes   BIGINT      NOT NULL DEFAULT 0,
    bucket_name          VARCHAR(128),
    theme                JSONB       NOT NULL DEFAULT '{}',
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
