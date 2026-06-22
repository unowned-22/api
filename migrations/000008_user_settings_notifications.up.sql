ALTER TABLE user_settings
  ADD COLUMN notification_preferences JSONB NOT NULL DEFAULT '{}'::jsonb;
