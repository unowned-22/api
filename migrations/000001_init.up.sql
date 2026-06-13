-- ========================
-- USERS
-- ========================
CREATE TABLE IF NOT EXISTS users (
    id                            BIGSERIAL PRIMARY KEY,
    email                         VARCHAR(255)             NOT NULL UNIQUE,
    password                      VARCHAR(255)             NOT NULL,
    full_name                     VARCHAR(128)             NOT NULL,
    username                      VARCHAR(64)              NOT NULL,
    phone                         VARCHAR(16)              NOT NULL,
    role_id                       BIGINT,
    email_verified_at             TIMESTAMPTZ,
    verification_token            TEXT,
    verification_token_expires_at TIMESTAMPTZ,
    created_at                    TIMESTAMPTZ              NOT NULL DEFAULT NOW()
);

ALTER TABLE users ADD CONSTRAINT users_username_key UNIQUE (username);

-- ========================
-- ROLES
-- ========================
CREATE TABLE IF NOT EXISTS roles (
    id         BIGSERIAL PRIMARY KEY,
    name       VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

INSERT INTO roles (name)
VALUES ('admin'), ('moderator'), ('user')
ON CONFLICT (name) DO NOTHING;

ALTER TABLE users
    ADD CONSTRAINT fk_users_role
    FOREIGN KEY (role_id) REFERENCES roles(id);

UPDATE users
SET role_id = (SELECT id FROM roles WHERE name = 'user')
WHERE role_id IS NULL;

ALTER TABLE users
    ALTER COLUMN role_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_users_role_id ON users(role_id);

-- ========================
-- REFRESH TOKENS
-- ========================
CREATE TABLE IF NOT EXISTS refresh_tokens (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL,
    token_hash TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    status     VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT fk_refresh_tokens_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_refresh_tokens_user_id ON refresh_tokens(user_id);

-- ========================
-- PERMISSIONS
-- ========================
CREATE TABLE IF NOT EXISTS permissions (
    id          BIGSERIAL PRIMARY KEY,
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

-- ========================
-- PASSWORD RESET TOKENS
-- ========================
CREATE TABLE IF NOT EXISTS password_reset_tokens (
    id         BIGSERIAL PRIMARY KEY,
    user_id    BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token      TEXT        NOT NULL UNIQUE,
    expires_at TIMESTAMPTZ NOT NULL,
    used_at    TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_token ON password_reset_tokens(token);
