CREATE INDEX CONCURRENTLY IF NOT EXISTS messages_thread_list_visible_idx
    ON messages (chat_id, created_at DESC, thread_parent_id DESC)
    WHERE thread_parent_id IS NOT NULL AND deleted_at IS NULL;
