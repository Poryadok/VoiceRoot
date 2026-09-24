DROP INDEX IF EXISTS bot_event_log_message_once_idx;
ALTER TABLE bot_event_log DROP COLUMN IF EXISTS source_message_id;
DROP TABLE IF EXISTS bot_message_deliveries;
