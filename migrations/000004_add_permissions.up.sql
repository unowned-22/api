CREATE TABLE IF NOT EXISTS permissions (
    id BIGSERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL UNIQUE,
    description TEXT,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS role_permissions (
    role_id BIGINT NOT NULL,
    permission_id BIGINT NOT NULL,
    PRIMARY KEY(role_id, permission_id)
);

INSERT INTO roles (name)
VALUES ('moderator')
ON CONFLICT (name) DO NOTHING;

ALTER TABLE role_permissions
    ADD CONSTRAINT fk_role_permissions_role
    FOREIGN KEY(role_id) REFERENCES roles(id);

ALTER TABLE role_permissions
    ADD CONSTRAINT fk_role_permissions_permission
    FOREIGN KEY(permission_id) REFERENCES permissions(id);

INSERT INTO permissions (name, description)
VALUES
    ('users.read', 'Read user data'),
    ('users.create', 'Create users'),
    ('users.update', 'Update user data'),
    ('users.delete', 'Delete users'),
    ('roles.read', 'Read roles'),
    ('roles.update', 'Update roles'),
    ('admin.access', 'Access administration endpoints')
ON CONFLICT (name) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
CROSS JOIN permissions p
WHERE r.name = 'admin'
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.name IN ('users.read', 'users.update')
WHERE r.name = 'moderator'
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO role_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON p.name = 'users.read'
WHERE r.name = 'user'
ON CONFLICT (role_id, permission_id) DO NOTHING;

CREATE INDEX IF NOT EXISTS idx_role_permissions_permission_id ON role_permissions(permission_id);
