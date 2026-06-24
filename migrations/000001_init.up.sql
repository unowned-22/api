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

CREATE INDEX IF NOT EXISTS idx_users_role_id        ON users(role_id);
CREATE INDEX IF NOT EXISTS idx_users_deactivated_at ON users(deactivated_at);

CREATE TABLE IF NOT EXISTS refresh_tokens (
    id         BIGSERIAL   PRIMARY KEY,
    user_id    BIGINT      NOT NULL,
    session_id BIGINT,
    parent_token_id BIGINT,
    replaced_by_token_id BIGINT,
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
    refresh_token_id BIGINT       NULL REFERENCES refresh_tokens(id) ON DELETE CASCADE,
    device_id        BIGINT NULL,
    device_name      VARCHAR(255) NULL,
    status           text NOT NULL DEFAULT 'active',
    user_agent       TEXT         NULL,
    ip_address       VARCHAR(45)  NULL,
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    last_used_at     TIMESTAMPTZ  NULL DEFAULT NOW(),
    last_activity_at timestamptz NOT NULL,
    expires_at       timestamptz NOT NULL,
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
    ('default_bucket_policy',       '{"Version":"2012-10-17","Statement":[{"Effect":"Allow","Principal":{"AWS":["*"]},"Action":["s3:GetObject"],"Resource":["arn:aws:s3:::app-uploads/*"]}]}'),
    ('theme',                       '{"primary_color": "#3B82F6", "mode": "light"}')
ON CONFLICT (key) DO NOTHING;

CREATE TABLE IF NOT EXISTS user_settings (
    user_id             BIGINT      PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    storage_quota_bytes BIGINT      NOT NULL DEFAULT 1073741824,
    storage_used_bytes  BIGINT      NOT NULL DEFAULT 0,
    bucket_name         VARCHAR(128),
    theme               JSONB       NOT NULL DEFAULT '{}',
    notification_preferences JSONB  NOT NULL DEFAULT '{}',
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE user_devices (
  id BIGSERIAL PRIMARY KEY,
  user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  fingerprint TEXT NOT NULL,
  browser TEXT NOT NULL,
  ip TEXT,
  device_name TEXT,
  os TEXT,
  first_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  UNIQUE (user_id, fingerprint, browser)
);

CREATE TABLE stories (
 id                   BIGSERIAL PRIMARY KEY,
 user_id              BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 visibility           VARCHAR(16) NOT NULL DEFAULT 'everyone',
 duration_hours       SMALLINT NOT NULL,
 hidden_from_user_ids BIGINT[] NOT NULL DEFAULT '{}',
 slides               JSONB NOT NULL,
 created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
 expires_at           TIMESTAMPTZ NOT NULL
);

CREATE TABLE story_views (
     id BIGSERIAL PRIMARY KEY,
     viewer_id BIGINT NOT NULL,
     story_id BIGINT NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
     slide_index INT,
     viewed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS story_likes (
   id BIGSERIAL PRIMARY KEY,
   story_id BIGINT NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
    viewer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now(),
    UNIQUE (story_id, viewer_id)
);

CREATE TABLE IF NOT EXISTS story_replies (
     id BIGSERIAL PRIMARY KEY,
     story_id BIGINT NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
    viewer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message TEXT NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT now()
);

CREATE TABLE friendships (
     id            BIGSERIAL PRIMARY KEY,
     requester_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
     addressee_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
     status        VARCHAR(16) NOT NULL DEFAULT 'pending',
     created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
     updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
     CONSTRAINT chk_friendship_not_self CHECK (requester_id <> addressee_id)
);

CREATE TABLE notifications (
   id          BIGSERIAL PRIMARY KEY,
   user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
   actor_id    BIGINT REFERENCES users(id) ON DELETE SET NULL,
   type        VARCHAR(64) NOT NULL,
   entity_type VARCHAR(64),
   entity_id   BIGINT,
   payload     JSONB,
   is_read     BOOLEAN NOT NULL DEFAULT FALSE,
   created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS close_friends (
     id        BIGSERIAL PRIMARY KEY,
     owner_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    friend_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_close_friends UNIQUE (owner_id, friend_id),
    CONSTRAINT chk_close_friends_not_self CHECK (owner_id <> friend_id)
);

CREATE TABLE IF NOT EXISTS user_privacy_settings (
 user_id          BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    show_email       VARCHAR(16) NOT NULL DEFAULT 'nobody',
    show_phone       VARCHAR(16) NOT NULL DEFAULT 'nobody',
    show_friends     VARCHAR(16) NOT NULL DEFAULT 'everyone',
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_stories_user_id_expires_at ON stories (user_id, expires_at);
CREATE INDEX IF NOT EXISTS idx_stories_user_id_expires_at ON stories (user_id, expires_at);
CREATE UNIQUE INDEX story_views_unique_idx ON story_views(viewer_id, story_id, slide_index);
CREATE UNIQUE INDEX uq_friendships_pair ON friendships (requester_id, addressee_id);
CREATE INDEX idx_friendships_addressee_pending ON friendships (addressee_id, status);
CREATE INDEX idx_friendships_requester_pending ON friendships (requester_id, status);
CREATE INDEX idx_notifications_user_unread ON notifications (user_id, is_read, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_close_friends_owner ON close_friends (owner_id);

ALTER TABLE refresh_tokens
    ADD CONSTRAINT fk_refresh_tokens_parent
        FOREIGN KEY (parent_token_id) REFERENCES refresh_tokens(id)
            DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE refresh_tokens
    ADD CONSTRAINT fk_refresh_tokens_replaced_by
        FOREIGN KEY (replaced_by_token_id) REFERENCES refresh_tokens(id)
            DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE refresh_tokens
    ADD CONSTRAINT fk_refresh_tokens_session
        FOREIGN KEY (session_id) REFERENCES user_sessions(id)
            DEFERRABLE INITIALLY DEFERRED;

ALTER TABLE user_sessions
    ADD CONSTRAINT fk_user_sessions_device
        FOREIGN KEY (device_id) REFERENCES user_devices(id)
            DEFERRABLE INITIALLY DEFERRED;