DROP TABLE IF EXISTS profile_search_key_collisions;
DROP TABLE IF EXISTS profile_search_key_backfill_checkpoints;
DROP INDEX IF EXISTS profiles_display_name_search_key_idx;
DROP INDEX IF EXISTS profiles_username_search_key_idx;
ALTER TABLE profiles
    DROP COLUMN IF EXISTS search_normalization_version,
    DROP COLUMN IF EXISTS display_name_search_key,
    DROP COLUMN IF EXISTS username_search_key;
