-- Drop unique constraint on stories.user_id

ALTER TABLE stories
    DROP CONSTRAINT IF EXISTS stories_user_id_unique;
