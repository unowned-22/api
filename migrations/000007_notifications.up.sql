CREATE TABLE notifications (
    id          BIGSERIAL PRIMARY KEY,
    user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    actor_id    BIGINT REFERENCES users(id) ON DELETE SET NULL,
    type        VARCHAR(64) NOT NULL,
    entity_type VARCHAR(64),
    entity_id   BIGINT,
    payload     JSONB,
    is_read     BOOLEAN NOT NULL DEFAULT FALSE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_notifications_user_unread ON notifications (user_id, is_read, created_at DESC);
