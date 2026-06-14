CREATE TABLE IF NOT EXISTS roles (
    id         BIGSERIAL PRIMARY KEY,
    name       VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

INSERT INTO roles (name)
VALUES ('admin'), ('moderator'), ('user')
ON CONFLICT (name) DO NOTHING;

CREATE TABLE IF NOT EXISTS users (
    id                            BIGSERIAL    PRIMARY KEY,
    email                         VARCHAR(255) NOT NULL UNIQUE,
    password                      VARCHAR(255) NOT NULL,
    full_name                     VARCHAR(128) NOT NULL,
    username                      VARCHAR(64)  NOT NULL,
    phone                         VARCHAR(16)  NULL,
    role_id                       BIGINT,
    email_verified_at             TIMESTAMPTZ,
    verification_token            TEXT,
    verification_token_expires_at TIMESTAMPTZ,
    token_version                 INTEGER      NOT NULL DEFAULT 1,
    deactivated_at                TIMESTAMPTZ,
    avatar_url                    VARCHAR(512),
    cover_url                     VARCHAR(512),
    created_at                    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at                    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

ALTER TABLE users ADD CONSTRAINT users_username_key UNIQUE (username);

ALTER TABLE users
    ADD CONSTRAINT fk_users_role
    FOREIGN KEY (role_id) REFERENCES roles(id);

UPDATE users
SET role_id = (SELECT id FROM roles WHERE name = 'user')
WHERE role_id IS NULL;

ALTER TABLE users
    ALTER COLUMN role_id SET NOT NULL;

-- ALTER TABLE users
--     ADD CONSTRAINT chk_users_phone
--         CHECK (phone IS NULL OR phone = '' OR phone ~ '^\\+[1-9][0-9]{6,14}$');

CREATE INDEX IF NOT EXISTS idx_users_role_id        ON users(role_id);
CREATE INDEX IF NOT EXISTS idx_users_deactivated_at ON users(deactivated_at);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id         BIGSERIAL   PRIMARY KEY,
    user_id    BIGINT      NOT NULL,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    status     VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_refresh_tokens_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);

CREATE TABLE IF NOT EXISTS permissions (
    id          BIGSERIAL    PRIMARY KEY,
    name        VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    created_at  TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id       BIGINT NOT NULL,
    permission_id BIGINT NOT NULL,
    PRIMARY KEY (role_id, permission_id),
    CONSTRAINT fk_role_permissions_role
        FOREIGN KEY (role_id) REFERENCES roles(id),
    CONSTRAINT fk_role_permissions_permission
        FOREIGN KEY (permission_id) REFERENCES permissions(id)
);

CREATE INDEX IF NOT EXISTS idx_role_permissions_permission_id ON role_permissions(permission_id);

INSERT INTO permissions (name, description)
VALUES
    ('users.read',   'Read user data'),
    ('users.create', 'Create users'),
    ('users.update', 'Update user data'),
    ('users.delete', 'Delete users'),
    ('roles.read',   'Read roles'),
    ('roles.update', 'Update roles'),
    ('admin.access', 'Access administration endpoints')
ON CONFLICT (name) DO NOTHING;

-- admin gets all permissions
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'admin'
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- moderator gets read + update users
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.name IN ('users.read', 'users.update')
WHERE r.name = 'moderator'
ON CONFLICT (role_id, permission_id) DO NOTHING;

-- user gets read users
INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.name = 'users.read'
WHERE r.name = 'user'
ON CONFLICT (role_id, permission_id) DO NOTHING;

CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id         BIGSERIAL   PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token   ON password_reset_tokens(token);
CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);  -- 000007

CREATE TABLE IF NOT EXISTS user_sessions (
    id               BIGSERIAL    PRIMARY KEY,
    user_id          BIGINT       NOT NULL REFERENCES users(id)          ON DELETE CASCADE,
    refresh_token_id BIGINT       NOT NULL REFERENCES refresh_tokens(id) ON DELETE CASCADE,
    device_name      VARCHAR(255) NOT NULL,
    user_agent       TEXT         NOT NULL,
    ip_address       VARCHAR(45)  NOT NULL,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    last_used_at     TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    revoked_at       TIMESTAMPTZ
);

CREATE INDEX IF NOT EXISTS idx_user_sessions_user_id          ON user_sessions(user_id);
CREATE INDEX IF NOT EXISTS idx_user_sessions_refresh_token_id ON user_sessions(refresh_token_id);

CREATE TABLE IF NOT EXISTS audit_logs (
    id         BIGSERIAL   PRIMARY KEY,
    user_id    BIGINT,
    event_type TEXT        NOT NULL,
    ip_address TEXT,
    user_agent TEXT,
    metadata   JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_audit_logs_user_id    ON audit_logs(user_id);
CREATE INDEX IF NOT EXISTS idx_audit_logs_event_type ON audit_logs(event_type);

CREATE TABLE IF NOT EXISTS outbox_events (
    id           UUID        PRIMARY KEY,
    event_type   TEXT        NOT NULL,
    payload      JSONB       NOT NULL,
    status       TEXT        NOT NULL,
    created_at   TIMESTAMPTZ NOT NULL,
    processed_at TIMESTAMPTZ,
    retry_count  INTEGER     NOT NULL DEFAULT 0
);

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

CREATE TABLE IF NOT EXISTS user_settings (
    user_id             BIGINT      PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    storage_quota_bytes BIGINT      NOT NULL DEFAULT 1073741824,
    storage_used_bytes  BIGINT      NOT NULL DEFAULT 0,
    bucket_name         VARCHAR(128),
    theme               JSONB       NOT NULL DEFAULT '{}',
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

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