DROP TABLE IF EXISTS user_profile_search_outbox;
DROP TABLE IF EXISTS user_profile_search_journal;
DROP TABLE IF EXISTS user_profile_search_offset;
ALTER TABLE profiles DROP COLUMN IF EXISTS search_projection_revision;
