CREATE INDEX CONCURRENTLY IF NOT EXISTS messages_thread_list_visible_idx
    ON messages (chat_id, thread_parent_id, created_at DESC, id DESC)
    INCLUDE (sender_profile_id, ghost_only)
    WHERE thread_parent_id IS NOT NULL AND deleted_at IS NULL;
