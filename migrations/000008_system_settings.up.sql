-- 000008_system_settings.up.sql
-- Create system_settings table and seed default settings

CREATE TABLE IF NOT EXISTS system_settings (
    key        VARCHAR(100) PRIMARY KEY,
    value      JSONB        NOT NULL,
    updated_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

INSERT INTO system_settings (key, value) VALUES
    ('default_storage_quota_bytes', '1073741824'),
    ('default_bucket_policy',       '{"versioning": false}'),
    ('theme',                       '{"primary_color": "#3B82F6", "mode": "light"}')
ON CONFLICT (key) DO NOTHING;
