-- Remove deactivated_at from users
ALTER TABLE users
    DROP COLUMN IF EXISTS deactivated_at;

DROP INDEX IF EXISTS idx_users_deactivated_at;
