CREATE TABLE conversations (
  id              BIGSERIAL PRIMARY KEY,
  type            VARCHAR(16) NOT NULL DEFAULT 'direct',
  title           VARCHAR(128),
  description     VARCHAR(512),
  avatar_url      VARCHAR(512),
  owner_id        BIGINT REFERENCES users(id) ON DELETE SET NULL,
  created_by      BIGINT NOT NULL REFERENCES users(id),
  last_message_id BIGINT,
  last_message_at TIMESTAMPTZ,
  members_count   INT NOT NULL DEFAULT 0,
  is_archived     BOOLEAN NOT NULL DEFAULT FALSE,
  invite_link     VARCHAR(128) UNIQUE,
  disappear_after_s INT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE conversation_members (
  conversation_id      BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  user_id              BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  role                 VARCHAR(16) NOT NULL DEFAULT 'member',
  joined_at            TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  left_at              TIMESTAMPTZ,
  muted_until          TIMESTAMPTZ,
  last_read_message_id BIGINT,
  last_read_at         TIMESTAMPTZ,
  is_archived          BOOLEAN NOT NULL DEFAULT FALSE,
  PRIMARY KEY (conversation_id, user_id)
);

CREATE TABLE messages (
  id                BIGSERIAL PRIMARY KEY,
  conversation_id   BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  sender_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  type              VARCHAR(16) NOT NULL DEFAULT 'text',
  body              TEXT,
  reply_to_id       BIGINT REFERENCES messages(id) ON DELETE SET NULL,
  forwarded_from_id  BIGINT REFERENCES messages(id) ON DELETE SET NULL,
  is_deleted        BOOLEAN NOT NULL DEFAULT FALSE,
  is_edited         BOOLEAN NOT NULL DEFAULT FALSE,
  edited_at         TIMESTAMPTZ,
  pinned            BOOLEAN NOT NULL DEFAULT FALSE,
  likes_count       INT NOT NULL DEFAULT 0,
  disappears_at     TIMESTAMPTZ,
  scheduled_at      TIMESTAMPTZ,
  is_scheduled      BOOLEAN NOT NULL DEFAULT FALSE,
  delivery_status   VARCHAR(16) NOT NULL DEFAULT 'sent',
  mention_user_ids  BIGINT[] NOT NULL DEFAULT '{}',
  created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE message_attachments (
  id            BIGSERIAL PRIMARY KEY,
  message_id    BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  type          VARCHAR(16) NOT NULL,
  storage_key   TEXT NOT NULL,
  url           TEXT NOT NULL,
  mime_type     VARCHAR(64),
  size_bytes    BIGINT,
  filename      VARCHAR(255),
  duration_s    INT,
  width         INT,
  height        INT,
  thumbnail_key TEXT,
  created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE message_reactions (
  message_id  BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  user_id     BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  emoji       VARCHAR(32) NOT NULL DEFAULT '👍',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (message_id, user_id, emoji)
);

CREATE TABLE message_delivery_status (
  message_id BIGINT NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
  user_id    BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  status     VARCHAR(16) NOT NULL DEFAULT 'sent',
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (message_id, user_id)
);

CREATE TABLE message_drafts (
  conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
  user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  body            TEXT NOT NULL DEFAULT '',
  reply_to_id     BIGINT REFERENCES messages(id) ON DELETE SET NULL,
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (conversation_id, user_id)
);

CREATE TABLE messenger_privacy_settings (
  user_id         BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  who_can_message VARCHAR(32) NOT NULL DEFAULT 'everyone',
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CONSTRAINT chk_who_can_message CHECK (who_can_message IN ('everyone', 'friends', 'nobody'))
);

CREATE TABLE messenger_blocked_users (
  blocker_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  blocked_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  PRIMARY KEY (blocker_id, blocked_id),
  CONSTRAINT chk_no_self_block CHECK (blocker_id <> blocked_id)
);

CREATE TABLE user_presence (
  user_id      BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  is_online    BOOLEAN NOT NULL DEFAULT FALSE,
  last_seen_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at   TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE bot_accounts (
  user_id        BIGINT PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  owner_user_id  BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  bot_token_hash TEXT NOT NULL UNIQUE,
  username       VARCHAR(64) NOT NULL UNIQUE,
  webhook_url    VARCHAR(512),
  is_active      BOOLEAN NOT NULL DEFAULT TRUE,
  created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

ALTER TABLE conversations
    ADD CONSTRAINT fk_conversations_last_message
        FOREIGN KEY (last_message_id) REFERENCES messages(id) ON DELETE SET NULL
            DEFERRABLE INITIALLY DEFERRED;

CREATE INDEX idx_conversations_invite_link ON conversations(invite_link) WHERE invite_link IS NOT NULL;
CREATE INDEX idx_conv_members_user ON conversation_members(user_id);
CREATE INDEX idx_conv_members_conv ON conversation_members(conversation_id);
CREATE INDEX idx_messages_conv_created ON messages(conversation_id, created_at DESC);
CREATE INDEX idx_messages_sender ON messages(sender_id);
CREATE INDEX idx_messages_reply_to ON messages(reply_to_id);
CREATE INDEX idx_messages_scheduled ON messages(scheduled_at) WHERE is_scheduled = TRUE;
CREATE INDEX idx_messages_disappears ON messages(disappears_at) WHERE disappears_at IS NOT NULL;
CREATE INDEX idx_messages_body_fts ON messages USING gin((to_tsvector('russian', COALESCE(body, '')) || to_tsvector('simple', COALESCE(body, ''))));
CREATE INDEX idx_messages_mentions ON messages USING gin(mention_user_ids);
CREATE INDEX idx_msg_attachments_msg ON message_attachments(message_id);
CREATE INDEX idx_msg_reactions_message ON message_reactions(message_id);
CREATE INDEX idx_msg_reactions_user ON message_reactions(user_id);
CREATE INDEX idx_msg_delivery_message ON message_delivery_status(message_id);
CREATE INDEX idx_msg_delivery_user ON message_delivery_status(user_id);
CREATE INDEX idx_drafts_user ON message_drafts(user_id);
CREATE INDEX idx_msg_blocked_blocked_id ON messenger_blocked_users(blocked_id);
CREATE INDEX idx_bot_accounts_owner ON bot_accounts(owner_user_id);
