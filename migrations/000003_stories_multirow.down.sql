-- WARNING: destructive rollback; this will add UNIQUE constraint back and may fail if multiple rows per user exist
ALTER TABLE stories ADD CONSTRAINT stories_user_id_key UNIQUE (user_id);
