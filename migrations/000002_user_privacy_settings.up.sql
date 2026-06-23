CREATE TABLE IF NOT EXISTS user_privacy_settings (
    user_id          BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    show_email       VARCHAR(16) NOT NULL DEFAULT 'nobody',
    show_phone       VARCHAR(16) NOT NULL DEFAULT 'nobody',
    show_friends     VARCHAR(16) NOT NULL DEFAULT 'everyone',
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
