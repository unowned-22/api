-- Create photo_visibility type and albums/photos tables (no FK for cover_photo_id yet)
CREATE TYPE photo_visibility AS ENUM ('everyone', 'friends', 'nobody');

CREATE TABLE albums (
    id            BIGSERIAL PRIMARY KEY,
    user_id       BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    title         VARCHAR(128) NOT NULL,
    description   VARCHAR(512) NOT NULL DEFAULT '',
    visibility    photo_visibility NOT NULL DEFAULT 'everyone',
    hidden_from   BIGINT[] NOT NULL DEFAULT '{}',
    cover_photo_id BIGINT,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_albums_user_id ON albums(user_id);

CREATE TABLE photos (
    id              BIGSERIAL PRIMARY KEY,
    user_id         BIGINT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    album_id        BIGINT REFERENCES albums(id) ON DELETE SET NULL,
    display_name    VARCHAR(255) NOT NULL,
    storage_key     TEXT NOT NULL UNIQUE,
    url             TEXT NOT NULL,
    size_bytes      BIGINT NOT NULL,
    width           INT,
    height          INT,
    mime_type       VARCHAR(64) NOT NULL,
    visibility      photo_visibility NOT NULL DEFAULT 'everyone',
    hidden_from     BIGINT[] NOT NULL DEFAULT '{}',
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_photos_user_id   ON photos(user_id);
CREATE INDEX idx_photos_album_id  ON photos(album_id);
