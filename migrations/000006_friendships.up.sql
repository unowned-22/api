CREATE TABLE friendships (
    id            BIGSERIAL PRIMARY KEY,
    requester_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    addressee_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    status        VARCHAR(16) NOT NULL DEFAULT 'pending',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT chk_friendship_not_self CHECK (requester_id <> addressee_id)
);

CREATE UNIQUE INDEX uq_friendships_pair ON friendships (requester_id, addressee_id);

CREATE INDEX idx_friendships_addressee_pending ON friendships (addressee_id, status);
CREATE INDEX idx_friendships_requester_pending ON friendships (requester_id, status);
