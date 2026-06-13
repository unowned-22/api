-- 000007_password_reset_tokens_user_id_index.down.sql
-- Remove index on password_reset_tokens.user_id

DROP INDEX IF EXISTS idx_password_reset_tokens_user_id;
