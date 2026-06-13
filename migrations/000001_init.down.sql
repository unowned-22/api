DROP TABLE IF EXISTS password_reset_tokens;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
DROP TABLE IF EXISTS refresh_tokens;
ALTER TABLE users DROP CONSTRAINT IF EXISTS fk_users_role;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS roles;
