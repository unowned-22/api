CREATE TABLE IF NOT EXISTS user_sessions (
    id               BIGSERIAL PRIMARY KEY,
    user_id          BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_token_id BIGINT      NOT NULL REFERENCES refresh_tokens(id) ON DELETE CASCADE,
    device_name      VARCHAR(255) NOT NULL,
    user_agent       TEXT        NOT NULL,
    ip_address       VARCHAR(45)  NOT NULL,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    last_used_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    revoked_at       TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_refresh_token_id ON user_sessions(refresh_token_id);
