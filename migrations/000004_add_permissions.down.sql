DROP INDEX IF EXISTS idx_role_permissions_permission_id;
ALTER TABLE role_permissions DROP CONSTRAINT IF EXISTS fk_role_permissions_permission;
ALTER TABLE role_permissions DROP CONSTRAINT IF EXISTS fk_role_permissions_role;
DROP TABLE IF EXISTS role_permissions;
DROP TABLE IF EXISTS permissions;
