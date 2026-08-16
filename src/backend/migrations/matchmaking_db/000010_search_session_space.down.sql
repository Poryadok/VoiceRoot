DROP INDEX IF EXISTS search_sessions_space_searching_idx;
ALTER TABLE search_sessions DROP COLUMN IF EXISTS space_id;
