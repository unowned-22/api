-- 000007_password_reset_tokens_user_id_index.up.sql
-- Add index on password_reset_tokens.user_id to speed up lookups by user

CREATE INDEX IF NOT EXISTS idx_password_reset_tokens_user_id ON password_reset_tokens(user_id);
