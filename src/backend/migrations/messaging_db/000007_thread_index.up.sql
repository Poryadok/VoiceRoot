-- Phase 10 threads — main-feed filter + GetThreadMessages
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_posted_as_chat_check;
ALTER TABLE messages DROP CONSTRAINT IF EXISTS messages_chat_type_check;
ALTER TABLE messages ADD CONSTRAINT messages_chat_type_check
    CHECK (chat_type IN ('dm', 'group', 'channel'));

CREATE INDEX IF NOT EXISTS messages_chat_thread_parent_id_desc_idx
    ON messages (chat_id, thread_parent_id, id DESC)
    WHERE thread_parent_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS messages_chat_main_feed_idx
    ON messages (chat_id, id DESC)
    WHERE thread_parent_id IS NULL AND deleted_at IS NULL;
