DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'stories_user_id_key' AND conrelid = 'stories'::regclass
    ) THEN
        ALTER TABLE stories DROP CONSTRAINT stories_user_id_key;
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'stories_user_id_unique' AND conrelid = 'stories'::regclass
    ) THEN
        ALTER TABLE stories DROP CONSTRAINT stories_user_id_unique;
    END IF;
END
$$;

CREATE INDEX IF NOT EXISTS idx_stories_user_id_expires_at ON stories (user_id, expires_at);
