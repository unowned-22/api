-- Drop FK constraint for albums.cover_photo_id
ALTER TABLE albums
    DROP CONSTRAINT IF EXISTS fk_albums_cover_photo;
