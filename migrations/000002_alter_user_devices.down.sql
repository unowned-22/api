ALTER TABLE user_devices
    DROP COLUMN IF EXISTS device_name,
    DROP COLUMN IF EXISTS os,
    DROP COLUMN IF EXISTS first_seen_at;

ALTER TABLE user_devices RENAME COLUMN last_seen_at TO last_seen;

ALTER TABLE user_devices
    ADD COLUMN IF NOT EXISTS platform text,
    ADD COLUMN IF NOT EXISTS country  text,
    ADD COLUMN IF NOT EXISTS city     text;
