-- Durable message content_type for list preview, shared media, and message.sent events.
ALTER TABLE messages
    ADD COLUMN IF NOT EXISTS content_type VARCHAR(32) NULL
    CHECK (content_type IS NULL OR content_type IN (
        'text', 'photo', 'video', 'document', 'voice',
        'sticker', 'gif', 'article', 'location', 'video_note', 'music'
    ));

CREATE INDEX IF NOT EXISTS messages_chat_content_type_idx
    ON messages (chat_id, content_type, id DESC)
    WHERE deleted_at IS NULL AND content_type IS NOT NULL;
