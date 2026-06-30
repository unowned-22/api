CREATE TABLE IF NOT EXISTS communities (
    id                 BIGSERIAL PRIMARY KEY,
    owner_id           BIGINT        NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    type               VARCHAR(32)   NOT NULL DEFAULT 'general',
    visibility         VARCHAR(16)   NOT NULL DEFAULT 'public' CHECK (visibility IN ('public', 'private')),
    name               VARCHAR(128)  NOT NULL,
    slug               VARCHAR(128)  NOT NULL UNIQUE,
    description        TEXT          NOT NULL DEFAULT '',
    avatar_key         TEXT,
    banner_key         TEXT,
    members_count      BIGINT        NOT NULL DEFAULT 0,
    posts_count        BIGINT        NOT NULL DEFAULT 0,
    subscribers_count  BIGINT        NOT NULL DEFAULT 0,
    videos_count       BIGINT        NOT NULL DEFAULT 0,
    created_at         TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ   NOT NULL DEFAULT NOW(),
    deleted_at         TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS community_members (
    community_id  BIGINT      NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
    user_id       BIGINT      NOT NULL REFERENCES users(id)       ON DELETE CASCADE,
    role          VARCHAR(32) NOT NULL DEFAULT 'member'
    CHECK (role IN ('owner', 'admin', 'member', 'subscriber')),
    joined_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (community_id, user_id)
);

CREATE TABLE conversations (
    id              BIGSERIAL PRIMARY KEY,
    community_id    BIGINT NULL REFERENCES communities(id) ON DELETE SET NULL,
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

CREATE TABLE stories (
    id                   BIGSERIAL PRIMARY KEY,
    user_id              BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    community_id         BIGINT NULL REFERENCES communities(id) ON DELETE CASCADE,
    author_type  VARCHAR(16) NOT NULL DEFAULT 'user' CHECK (author_type IN ('user', 'community')),
    visibility           VARCHAR(16) NOT NULL DEFAULT 'everyone',
    duration_hours       SMALLINT NOT NULL,
    hidden_from_user_ids BIGINT[] NOT NULL DEFAULT '{}',
    slides               JSONB NOT NULL,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at           TIMESTAMPTZ NOT NULL
);

CREATE TABLE story_views (
    id BIGSERIAL PRIMARY KEY,
    viewer_id BIGINT NOT NULL,
    story_id BIGINT NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
    slide_index INT,
    viewed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS story_likes (
    id BIGSERIAL PRIMARY KEY,
    story_id BIGINT NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
    viewer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (story_id, viewer_id)
);

CREATE TABLE IF NOT EXISTS story_replies (
    id BIGSERIAL PRIMARY KEY,
    story_id BIGINT NOT NULL REFERENCES stories(id) ON DELETE CASCADE,
    viewer_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    message TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);


CREATE TABLE IF NOT EXISTS videos (
    id BIGSERIAL PRIMARY KEY,
    community_id BIGINT NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(255) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    category VARCHAR(64) NOT NULL DEFAULT 'other',
    visibility VARCHAR(16) NOT NULL DEFAULT 'public' CHECK (visibility IN ('public','unlisted','private')),
    status VARCHAR(16) NOT NULL DEFAULT 'pending' CHECK (status IN ('pending','processing','ready','failed')),
    processing_stage VARCHAR(32),
    processing_progress SMALLINT NOT NULL DEFAULT 0 CHECK (processing_progress BETWEEN 0 AND 100),
    processing_started_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    boosted_until TIMESTAMPTZ,
    publish_targets  TEXT[] NOT NULL DEFAULT '{}',
    raw_key TEXT,
    hls_key TEXT,
    mp4_360_key TEXT,
    mp4_720_key TEXT,
    thumbnail_key TEXT,
    duration_sec FLOAT8,
    width INT,
    height INT,
    size_bytes BIGINT,
    video_codec VARCHAR(32),
    audio_codec VARCHAR(32),
    views_count BIGINT NOT NULL DEFAULT 0,
    likes_count BIGINT NOT NULL DEFAULT 0,
    comments_count BIGINT NOT NULL DEFAULT 0,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS video_tags (
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    tag VARCHAR(64) NOT NULL,
    PRIMARY KEY (video_id, tag)
);

CREATE TABLE IF NOT EXISTS video_views (
    id BIGSERIAL PRIMARY KEY,
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    user_id BIGINT REFERENCES users(id) ON DELETE SET NULL,
    ip_hash TEXT,
    viewed_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS video_likes (
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (video_id, user_id)
);

CREATE TABLE IF NOT EXISTS video_comments (
    id BIGSERIAL PRIMARY KEY,
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    parent_id BIGINT REFERENCES video_comments(id) ON DELETE CASCADE,
    body TEXT NOT NULL,
    likes_count BIGINT NOT NULL DEFAULT 0,
    is_deleted BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS video_comment_likes (
    comment_id BIGINT NOT NULL REFERENCES video_comments(id) ON DELETE CASCADE,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (comment_id, user_id)
);

CREATE TABLE IF NOT EXISTS video_playlists (
    id BIGSERIAL PRIMARY KEY,
    user_id BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title VARCHAR(128) NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    visibility VARCHAR(16) NOT NULL DEFAULT 'public' CHECK (visibility IN ('public','private')),
    items_count BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE TABLE IF NOT EXISTS video_playlist_items (
    id BIGSERIAL PRIMARY KEY,
    playlist_id BIGINT NOT NULL REFERENCES video_playlists(id) ON DELETE CASCADE,
    video_id BIGINT NOT NULL REFERENCES videos(id) ON DELETE CASCADE,
    position INT NOT NULL DEFAULT 0,
    added_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE (playlist_id, video_id)
);

CREATE TABLE IF NOT EXISTS user_posts (
    id             BIGSERIAL PRIMARY KEY,
    user_id        BIGINT      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    text           TEXT,
    media          JSONB       NOT NULL DEFAULT '[]',
    visibility     VARCHAR(16) NOT NULL DEFAULT 'everyone'
    CHECK (visibility IN ('everyone','friends','private')),
    likes_count    BIGINT      NOT NULL DEFAULT 0,
    comments_count BIGINT      NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS community_posts (
    id             BIGSERIAL PRIMARY KEY,
    community_id   BIGINT      NOT NULL REFERENCES communities(id) ON DELETE CASCADE,
    author_user_id BIGINT      NOT NULL REFERENCES users(id),
    text           TEXT,
    media          JSONB       NOT NULL DEFAULT '[]',
    video_id       BIGINT      REFERENCES videos(id) ON DELETE SET NULL,
    pinned         BOOLEAN     NOT NULL DEFAULT false,
    likes_count    BIGINT      NOT NULL DEFAULT 0,
    comments_count BIGINT      NOT NULL DEFAULT 0,
    created_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    deleted_at     TIMESTAMPTZ
);

CREATE OR REPLACE VIEW feed_items AS
SELECT
    'user'::text       AS source_type,
    up.id,
    up.user_id         AS owner_id,
    NULL::bigint        AS community_id,
    up.text,
    up.media,
    up.likes_count,
    up.comments_count,
    up.created_at
FROM user_posts up
WHERE up.deleted_at IS NULL
UNION ALL
SELECT
    'community'::text  AS source_type,
    cp.id,
    cp.author_user_id  AS owner_id,
    cp.community_id,
    cp.text,
    cp.media,
    cp.likes_count,
    cp.comments_count,
    cp.created_at
FROM community_posts cp
WHERE cp.deleted_at IS NULL;

ALTER TABLE conversations
    ADD CONSTRAINT fk_conversations_last_message
        FOREIGN KEY (last_message_id) REFERENCES messages(id) ON DELETE SET NULL
            DEFERRABLE INITIALLY DEFERRED;

CREATE INDEX IF NOT EXISTS idx_conversations_community_id ON conversations(community_id) WHERE community_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS uq_conversations_community_id ON conversations(community_id) WHERE community_id IS NOT NULL;
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

CREATE INDEX IF NOT EXISTS idx_communities_owner_id      ON communities(owner_id);
CREATE INDEX IF NOT EXISTS idx_communities_type          ON communities(type);
CREATE INDEX IF NOT EXISTS idx_communities_slug          ON communities(slug);
CREATE INDEX IF NOT EXISTS idx_communities_deleted_at    ON communities(deleted_at) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_communities_name_fts      ON communities USING gin(to_tsvector('simple', name));
CREATE INDEX IF NOT EXISTS idx_community_members_user_id ON community_members(user_id);
CREATE INDEX IF NOT EXISTS idx_community_members_role    ON community_members(community_id, role);
CREATE INDEX IF NOT EXISTS idx_videos_community_id       ON videos(community_id);
CREATE INDEX IF NOT EXISTS idx_videos_user_id            ON videos(user_id);
CREATE INDEX IF NOT EXISTS idx_videos_status             ON videos(status);
CREATE INDEX IF NOT EXISTS idx_videos_visibility ON videos(visibility);
CREATE INDEX IF NOT EXISTS idx_videos_created_at ON videos(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_videos_draft ON videos(community_id, published_at) WHERE published_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_video_tags_tag ON video_tags(tag);
CREATE INDEX IF NOT EXISTS idx_video_views_video_id ON video_views(video_id);
CREATE UNIQUE INDEX IF NOT EXISTS uidx_video_views_user_day ON video_views(video_id, user_id, ((viewed_at AT TIME ZONE 'UTC')::date)) WHERE user_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_video_comments_video_id ON video_comments(video_id);
CREATE INDEX IF NOT EXISTS idx_video_comments_parent_id ON video_comments(parent_id);
CREATE INDEX IF NOT EXISTS idx_video_comments_user_id ON video_comments(user_id);
CREATE INDEX IF NOT EXISTS idx_video_playlists_user_id ON video_playlists(user_id);
CREATE INDEX IF NOT EXISTS idx_video_playlist_items_playlist ON video_playlist_items(playlist_id, position);

CREATE INDEX IF NOT EXISTS idx_user_posts_user_id ON user_posts(user_id);
CREATE INDEX IF NOT EXISTS idx_user_posts_created_at ON user_posts(created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_community_posts_community_id ON community_posts(community_id);
CREATE INDEX IF NOT EXISTS idx_community_posts_created_at ON community_posts(created_at DESC) WHERE deleted_at IS NULL;
CREATE INDEX IF NOT EXISTS idx_community_posts_video_id ON community_posts(video_id) WHERE video_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_stories_community_id           ON stories(community_id) WHERE community_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS idx_stories_community_active       ON stories(community_id, expires_at) WHERE community_id IS NOT NULL;
CREATE INDEX idx_stories_user_id_expires_at                   ON stories (user_id, expires_at);
CREATE INDEX IF NOT EXISTS idx_stories_user_id_expires_at     ON stories (user_id, expires_at);
CREATE UNIQUE INDEX story_views_unique_idx                    ON story_views(viewer_id, story_id, slide_index);