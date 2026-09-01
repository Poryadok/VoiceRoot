DROP INDEX IF EXISTS messages_chat_content_type_idx;
ALTER TABLE messages DROP COLUMN IF EXISTS content_type;
