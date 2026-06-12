-- Create roles lookup table
CREATE TABLE IF NOT EXISTS roles (
    id         BIGSERIAL PRIMARY KEY,
    name       VARCHAR(100) NOT NULL UNIQUE,
    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

-- Seed default roles
INSERT INTO roles (name)
VALUES ('admin'), ('moderator'), ('user')
ON CONFLICT (name) DO NOTHING;

-- Add role_id to users and assign the default "user" role to pre-existing rows
ALTER TABLE users ADD COLUMN role_id BIGINT;

ALTER TABLE users
    ADD CONSTRAINT fk_users_role
    FOREIGN KEY (role_id) REFERENCES roles(id);

UPDATE users
SET role_id = (SELECT id FROM roles WHERE name = 'user')
WHERE role_id IS NULL;

ALTER TABLE users
    ALTER COLUMN role_id SET NOT NULL;

CREATE INDEX IF NOT EXISTS idx_users_role_id ON users(role_id);
