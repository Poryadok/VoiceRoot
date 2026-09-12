-- `show_last_seen` is independent from live `show_online` (privacy.md).
ALTER TABLE privacy_settings
  ADD COLUMN IF NOT EXISTS show_last_seen_audience JSONB;

UPDATE privacy_settings
SET show_last_seen_audience = CASE preset
  WHEN 'personal' THEN '{"friends":true}'::jsonb
  WHEN 'work' THEN '{"space_members":true}'::jsonb
  ELSE '{"friends":true,"friends_of_friends":true,"space_members":true,"include_guests":true}'::jsonb
END
WHERE show_last_seen_audience IS NULL;

ALTER TABLE privacy_settings
  ALTER COLUMN show_last_seen_audience SET NOT NULL;
