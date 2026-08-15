ALTER TABLE profiles
    ADD COLUMN IF NOT EXISTS notification_prefs_json JSONB NOT NULL DEFAULT '{}'::jsonb;
