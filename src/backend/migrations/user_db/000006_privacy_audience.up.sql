-- Phase 11: multiselect PrivacyAudience (privacy.md)
ALTER TABLE privacy_settings
  ADD COLUMN IF NOT EXISTS show_online_audience JSONB,
  ADD COLUMN IF NOT EXISTS show_game_status_audience JSONB,
  ADD COLUMN IF NOT EXISTS show_mm_rating_audience JSONB,
  ADD COLUMN IF NOT EXISTS show_phone_audience JSONB,
  ADD COLUMN IF NOT EXISTS show_stories_audience JSONB,
  ADD COLUMN IF NOT EXISTS allow_dm_audience JSONB,
  ADD COLUMN IF NOT EXISTS allow_friend_requests_audience JSONB,
  ADD COLUMN IF NOT EXISTS allow_phone_search_audience JSONB,
  ADD COLUMN IF NOT EXISTS allow_calls_audience JSONB,
  ADD COLUMN IF NOT EXISTS allow_chat_space_invites_audience JSONB,
  ADD COLUMN IF NOT EXISTS allow_files_audience JSONB,
  ADD COLUMN IF NOT EXISTS allow_voice_messages_audience JSONB;

-- Migrate legacy VARCHAR values to JSONB audiences.
UPDATE privacy_settings SET
  show_online_audience = CASE show_online
    WHEN 'everyone' THEN '{"friends":true,"friends_of_friends":true,"space_members":true,"include_guests":true}'::jsonb
    WHEN 'friends' THEN '{"friends":true}'::jsonb
    WHEN 'friends_of_friends' THEN '{"friends":true,"friends_of_friends":true}'::jsonb
    WHEN 'nobody' THEN '{"friends":false,"friends_of_friends":false,"space_members":false,"include_guests":false}'::jsonb
    ELSE '{"friends":true,"friends_of_friends":true,"space_members":true,"include_guests":true}'::jsonb
  END,
  show_game_status_audience = CASE show_game_status
    WHEN 'everyone' THEN '{"friends":true,"friends_of_friends":true,"space_members":true,"include_guests":true}'::jsonb
    WHEN 'friends' THEN '{"friends":true}'::jsonb
    WHEN 'friends_of_friends' THEN '{"friends":true,"friends_of_friends":true}'::jsonb
    WHEN 'nobody' THEN '{"friends":false,"friends_of_friends":false,"space_members":false,"include_guests":false}'::jsonb
    ELSE '{"friends":true,"friends_of_friends":true,"space_members":true,"include_guests":true}'::jsonb
  END,
  show_mm_rating_audience = CASE show_mm_rating
    WHEN 'everyone' THEN '{"friends":true,"friends_of_friends":true,"space_members":true,"include_guests":true}'::jsonb
    WHEN 'friends' THEN '{"friends":true}'::jsonb
    WHEN 'friends_of_friends' THEN '{"friends":true,"friends_of_friends":true}'::jsonb
    WHEN 'nobody' THEN '{"friends":false,"friends_of_friends":false,"space_members":false,"include_guests":false}'::jsonb
    ELSE '{"friends":true,"friends_of_friends":true,"space_members":true,"include_guests":true}'::jsonb
  END,
  show_phone_audience = CASE show_phone
    WHEN 'friends' THEN '{"friends":true}'::jsonb
    WHEN 'friends_of_friends' THEN '{"friends":true,"friends_of_friends":true}'::jsonb
    WHEN 'nobody' THEN '{"friends":false,"friends_of_friends":false,"space_members":false,"include_guests":false}'::jsonb
    ELSE '{"friends":false,"friends_of_friends":false,"space_members":false,"include_guests":false}'::jsonb
  END,
  show_stories_audience = CASE show_stories
    WHEN 'everyone' THEN '{"friends":true,"friends_of_friends":true,"space_members":true,"include_guests":true}'::jsonb
    WHEN 'friends' THEN '{"friends":true}'::jsonb
    WHEN 'friends_of_friends' THEN '{"friends":true,"friends_of_friends":true}'::jsonb
    WHEN 'nobody' THEN '{"friends":false,"friends_of_friends":false,"space_members":false,"include_guests":false}'::jsonb
    ELSE '{"friends":true,"friends_of_friends":true,"space_members":true,"include_guests":true}'::jsonb
  END,
  allow_dm_audience = CASE allow_dm
    WHEN 'everyone' THEN '{"friends":true,"friends_of_friends":true,"space_members":true,"include_guests":true}'::jsonb
    WHEN 'friends' THEN '{"friends":true}'::jsonb
    WHEN 'friends_of_friends' THEN '{"friends":true,"friends_of_friends":true}'::jsonb
    WHEN 'nobody' THEN '{"friends":false,"friends_of_friends":false,"space_members":false,"include_guests":false}'::jsonb
    ELSE '{"friends":true,"friends_of_friends":true,"space_members":true,"include_guests":true}'::jsonb
  END,
  allow_friend_requests_audience = CASE allow_friend_requests
    WHEN 'everyone' THEN '{"friends":true,"friends_of_friends":true,"space_members":true,"include_guests":true}'::jsonb
    WHEN 'friends_of_friends' THEN '{"friends":true,"friends_of_friends":true}'::jsonb
    WHEN 'nobody' THEN '{"friends":false,"friends_of_friends":false,"space_members":false,"include_guests":false}'::jsonb
    ELSE '{"friends":true,"friends_of_friends":true,"space_members":true,"include_guests":true}'::jsonb
  END;

-- Guest flag from legacy column.
UPDATE privacy_settings SET
  show_online_audience = jsonb_set(
    COALESCE(show_online_audience, '{"friends":true}'::jsonb),
    '{include_guests}',
    to_jsonb(COALESCE(show_online_include_guests, false))
  )
WHERE show_online = 'everyone' OR show_online_include_guests = true;

-- Default new action fields from gaming preset until user updates.
UPDATE privacy_settings SET
  allow_phone_search_audience = COALESCE(allow_phone_search_audience, '{"friends":true}'::jsonb),
  allow_calls_audience = COALESCE(allow_calls_audience, '{"friends":true,"friends_of_friends":true}'::jsonb),
  allow_chat_space_invites_audience = COALESCE(allow_chat_space_invites_audience, '{"friends":true,"friends_of_friends":true}'::jsonb),
  allow_files_audience = COALESCE(allow_files_audience, '{"friends":true,"friends_of_friends":true}'::jsonb),
  allow_voice_messages_audience = COALESCE(allow_voice_messages_audience, '{"friends":true,"friends_of_friends":true}'::jsonb);

ALTER TABLE privacy_settings
  ALTER COLUMN show_online_audience SET NOT NULL,
  ALTER COLUMN show_game_status_audience SET NOT NULL,
  ALTER COLUMN show_mm_rating_audience SET NOT NULL,
  ALTER COLUMN show_phone_audience SET NOT NULL,
  ALTER COLUMN show_stories_audience SET NOT NULL,
  ALTER COLUMN allow_dm_audience SET NOT NULL,
  ALTER COLUMN allow_friend_requests_audience SET NOT NULL,
  ALTER COLUMN allow_phone_search_audience SET NOT NULL,
  ALTER COLUMN allow_calls_audience SET NOT NULL,
  ALTER COLUMN allow_chat_space_invites_audience SET NOT NULL,
  ALTER COLUMN allow_files_audience SET NOT NULL,
  ALTER COLUMN allow_voice_messages_audience SET NOT NULL;

ALTER TABLE privacy_settings DROP COLUMN IF EXISTS show_online;
ALTER TABLE privacy_settings DROP COLUMN IF EXISTS show_game_status;
ALTER TABLE privacy_settings DROP COLUMN IF EXISTS show_mm_rating;
ALTER TABLE privacy_settings DROP COLUMN IF EXISTS show_phone;
ALTER TABLE privacy_settings DROP COLUMN IF EXISTS show_stories;
ALTER TABLE privacy_settings DROP COLUMN IF EXISTS allow_dm;
ALTER TABLE privacy_settings DROP COLUMN IF EXISTS allow_friend_requests;
ALTER TABLE privacy_settings DROP COLUMN IF EXISTS show_online_include_guests;
