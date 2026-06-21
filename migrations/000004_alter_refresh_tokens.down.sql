ALTER TABLE refresh_tokens
    DROP CONSTRAINT IF EXISTS fk_refresh_tokens_parent,
    DROP CONSTRAINT IF EXISTS fk_refresh_tokens_replaced_by,
    DROP CONSTRAINT IF EXISTS fk_refresh_tokens_session,
    DROP COLUMN IF EXISTS session_id,
    DROP COLUMN IF EXISTS parent_token_id,
    DROP COLUMN IF EXISTS replaced_by_token_id;
