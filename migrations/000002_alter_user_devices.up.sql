ALTER TABLE user_devices
    ADD COLUMN IF NOT EXISTS device_name text,
    ADD COLUMN IF NOT EXISTS os          text,
    ADD COLUMN IF NOT EXISTS first_seen_at timestamptz;

-- backfill first_seen_at from created_at
UPDATE user_devices SET first_seen_at = created_at WHERE first_seen_at IS NULL;

-- rename last_seen → last_seen_at
ALTER TABLE user_devices RENAME COLUMN last_seen TO last_seen_at;

-- enforce NOT NULL after backfill
ALTER TABLE user_devices ALTER COLUMN first_seen_at SET NOT NULL;

-- drop columns no longer needed
ALTER TABLE user_devices DROP COLUMN IF EXISTS platform;
ALTER TABLE user_devices DROP COLUMN IF EXISTS country;
ALTER TABLE user_devices DROP COLUMN IF EXISTS city;
