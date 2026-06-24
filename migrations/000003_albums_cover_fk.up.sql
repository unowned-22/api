-- Add FK constraint for albums.cover_photo_id -> photos.id
ALTER TABLE albums
    ADD CONSTRAINT fk_albums_cover_photo
    FOREIGN KEY (cover_photo_id) REFERENCES photos(id) ON DELETE SET NULL;
