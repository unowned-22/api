-- add new columns nullable first
ALTER TABLE refresh_tokens
    ADD COLUMN IF NOT EXISTS session_id           bigint,
    ADD COLUMN IF NOT EXISTS parent_token_id      bigint,
    ADD COLUMN IF NOT EXISTS replaced_by_token_id bigint;

-- self-referential FKs (deferrable)
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
