CREATE INDEX IF NOT EXISTS messages_chat_shared_attachments_idx
  ON messages (chat_id, id DESC)
  WHERE deleted_at IS NULL AND attachments <> '[]'::jsonb;

CREATE INDEX IF NOT EXISTS messages_chat_shared_links_idx
  ON messages (chat_id, id DESC)
  WHERE deleted_at IS NULL AND content ~ 'https?://';
