CREATE TABLE IF NOT EXISTS close_friends (
    id        BIGSERIAL PRIMARY KEY,
    owner_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    friend_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_close_friends UNIQUE (owner_id, friend_id),
    CONSTRAINT chk_close_friends_not_self CHECK (owner_id <> friend_id)
);

CREATE INDEX IF NOT EXISTS idx_close_friends_owner ON close_friends (owner_id);
