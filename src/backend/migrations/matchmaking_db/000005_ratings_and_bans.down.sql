DROP TABLE IF EXISTS mm_bans;
DROP TABLE IF EXISTS player_ratings;
DROP TABLE IF EXISTS match_ratings;
ALTER TABLE matches DROP COLUMN IF EXISTS left_profile_ids;
