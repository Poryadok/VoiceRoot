DROP INDEX IF EXISTS search_sessions_expire_idx;
DROP INDEX IF EXISTS search_sessions_nudge_idx;
ALTER TABLE search_sessions DROP COLUMN IF EXISTS nudged_at;
