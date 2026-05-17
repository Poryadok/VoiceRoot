DROP INDEX IF EXISTS messages_client_dedup_idx;
ALTER TABLE messages DROP COLUMN IF EXISTS client_message_id;
