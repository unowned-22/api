-- Add device, geolocation, exif, counters; create likes/comments tables
BEGIN;

ALTER TABLE photos
    ADD COLUMN device_name   VARCHAR(255),
    ADD COLUMN device_os     VARCHAR(128),
    ADD COLUMN device_type   VARCHAR(32),
    ADD COLUMN latitude      DOUBLE PRECISION,
    ADD COLUMN longitude     DOUBLE PRECISION,
    ADD COLUMN location_name VARCHAR(255),
    ADD COLUMN exif_data     JSONB,
    ADD COLUMN likes_count    INT NOT NULL DEFAULT 0,
    ADD COLUMN comments_count INT NOT NULL DEFAULT 0;

CREATE TABLE photo_likes (
    photo_id    BIGINT NOT NULL REFERENCES photos(id) ON DELETE CASCADE,
    user_id     BIGINT NOT NULL REFERENCES users(id)  ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (photo_id, user_id)
);
CREATE INDEX idx_photo_likes_photo_id ON photo_likes(photo_id);

CREATE TABLE photo_comments (
    id          BIGSERIAL PRIMARY KEY,
    photo_id    BIGINT  NOT NULL REFERENCES photos(id)         ON DELETE CASCADE,
    author_id   BIGINT  NOT NULL REFERENCES users(id)          ON DELETE CASCADE,
    parent_id   BIGINT  REFERENCES photo_comments(id)          ON DELETE CASCADE,
    body        TEXT    NOT NULL CHECK (char_length(body) BETWEEN 1 AND 2000),
    is_deleted  BOOLEAN NOT NULL DEFAULT FALSE,
    likes_count INT NOT NULL DEFAULT 0,
    replies_count INT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
CREATE INDEX idx_photo_comments_photo_id  ON photo_comments(photo_id);
CREATE INDEX idx_photo_comments_parent_id ON photo_comments(parent_id);
CREATE INDEX idx_photo_comments_author_id ON photo_comments(author_id);

CREATE TABLE photo_comment_likes (
    comment_id  BIGINT NOT NULL REFERENCES photo_comments(id) ON DELETE CASCADE,
    user_id     BIGINT NOT NULL REFERENCES users(id)           ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    PRIMARY KEY (comment_id, user_id)
);
CREATE INDEX idx_photo_comment_likes_comment_id ON photo_comment_likes(comment_id);

COMMIT;
