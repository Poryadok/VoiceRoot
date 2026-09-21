DROP TABLE IF EXISTS user_profile_search_outbox;
DROP TABLE IF EXISTS user_profile_search_journal;
ALTER TABLE profiles DROP COLUMN IF EXISTS search_projection_revision;
