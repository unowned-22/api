-- Add unique constraint on stories.user_id to support ON CONFLICT (user_id)
-- IMPORTANT: ensure there are no duplicate user_id rows before applying this migration.

ALTER TABLE stories
    ADD CONSTRAINT stories_user_id_unique UNIQUE (user_id);
