ALTER TABLE privacy_settings
  ADD COLUMN IF NOT EXISTS show_online VARCHAR(32),
  ADD COLUMN IF NOT EXISTS show_game_status VARCHAR(32),
  ADD COLUMN IF NOT EXISTS show_mm_rating VARCHAR(32),
  ADD COLUMN IF NOT EXISTS show_phone VARCHAR(32),
  ADD COLUMN IF NOT EXISTS show_stories VARCHAR(32),
  ADD COLUMN IF NOT EXISTS allow_dm VARCHAR(32),
  ADD COLUMN IF NOT EXISTS allow_friend_requests VARCHAR(32),
  ADD COLUMN IF NOT EXISTS show_online_include_guests BOOLEAN NOT NULL DEFAULT false;

UPDATE privacy_settings SET
  show_online = 'everyone',
  show_game_status = 'everyone',
  show_mm_rating = 'everyone',
  show_phone = 'nobody',
  show_stories = 'everyone',
  allow_dm = 'everyone',
  allow_friend_requests = 'everyone';

ALTER TABLE privacy_settings
  DROP COLUMN IF EXISTS show_online_audience,
  DROP COLUMN IF EXISTS show_game_status_audience,
  DROP COLUMN IF EXISTS show_mm_rating_audience,
  DROP COLUMN IF EXISTS show_phone_audience,
  DROP COLUMN IF EXISTS show_stories_audience,
  DROP COLUMN IF EXISTS allow_dm_audience,
  DROP COLUMN IF EXISTS allow_friend_requests_audience,
  DROP COLUMN IF EXISTS allow_phone_search_audience,
  DROP COLUMN IF EXISTS allow_calls_audience,
  DROP COLUMN IF EXISTS allow_chat_space_invites_audience,
  DROP COLUMN IF EXISTS allow_files_audience,
  DROP COLUMN IF EXISTS allow_voice_messages_audience;
