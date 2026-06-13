DROP INDEX IF EXISTS messages_chat_main_feed_idx;
DROP INDEX IF EXISTS messages_chat_thread_parent_id_desc_idx;

ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_chat_type_check;
ALTER TABLE messages ADD CONSTRAINT messages_chat_type_check CHECK (chat_type = 'dm');
