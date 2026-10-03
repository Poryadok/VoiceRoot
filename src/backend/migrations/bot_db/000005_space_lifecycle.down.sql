DROP INDEX IF EXISTS bot_event_log_space_pending_idx;
ALTER TABLE bot_event_log DROP COLUMN IF EXISTS space_id;
DROP TABLE IF EXISTS bot_space_lifecycle_purge_receipts;
DROP TABLE IF EXISTS bot_space_lifecycle_fence_receipts;
DROP TABLE IF EXISTS bot_space_lifecycle_heads;
