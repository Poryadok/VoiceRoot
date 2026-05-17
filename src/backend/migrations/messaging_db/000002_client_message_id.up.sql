-- Idempotent SendMessage: docs/microservices/messaging-service.md
ALTER TABLE messages ADD COLUMN IF NOT EXISTS client_message_id UUID NULL;

CREATE UNIQUE INDEX IF NOT EXISTS messages_client_dedup_idx
  ON messages (chat_id, sender_profile_id, client_message_id)
  WHERE client_message_id IS NOT NULL;
