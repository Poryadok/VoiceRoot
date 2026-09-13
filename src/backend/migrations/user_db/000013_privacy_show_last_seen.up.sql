-- `show_last_seen` is independent from live `show_online` (privacy.md).
-- Keep the additive field nullable during expand: deployed legacy User writers do
-- not include it in INSERT/UPSERT. New readers derive the documented audience
-- from `preset` only for NULL legacy rows; new writers persist explicit JSON.
ALTER TABLE privacy_settings
  ADD COLUMN IF NOT EXISTS show_last_seen_audience JSONB;

UPDATE privacy_settings
SET show_last_seen_audience = CASE preset
  WHEN 'personal' THEN '{"friends":true}'::jsonb
  WHEN 'work' THEN '{"space_members":true}'::jsonb
  ELSE '{"friends":true,"friends_of_friends":true,"space_members":true,"include_guests":true}'::jsonb
END
WHERE show_last_seen_audience IS NULL;
