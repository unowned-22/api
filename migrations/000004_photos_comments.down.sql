BEGIN;

DROP INDEX IF EXISTS idx_photo_comment_likes_comment_id;
DROP TABLE IF EXISTS photo_comment_likes;

DROP INDEX IF EXISTS idx_photo_comments_author_id;
DROP INDEX IF EXISTS idx_photo_comments_parent_id;
DROP INDEX IF EXISTS idx_photo_comments_photo_id;
DROP TABLE IF EXISTS photo_comments;

DROP INDEX IF EXISTS idx_photo_likes_photo_id;
DROP TABLE IF EXISTS photo_likes;

ALTER TABLE photos
    DROP COLUMN IF EXISTS likes_count,
    DROP COLUMN IF EXISTS comments_count,
    DROP COLUMN IF EXISTS device_name,
    DROP COLUMN IF EXISTS device_os,
    DROP COLUMN IF EXISTS device_type,
    DROP COLUMN IF EXISTS latitude,
    DROP COLUMN IF EXISTS longitude,
    DROP COLUMN IF EXISTS location_name,
    DROP COLUMN IF EXISTS exif_data;

COMMIT;
