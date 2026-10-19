-- Exact no-effect SendMessage replay is distinct from the legacy dedupe key.
-- Historical messages deliberately acquire no inferred request authority.
CREATE TABLE message_send_receipts (
  chat_id uuid NOT NULL,
  sender_profile_id uuid NOT NULL,
  client_message_id uuid NOT NULL,
  message_id uuid NOT NULL REFERENCES messages(id) ON DELETE CASCADE,
	request_rpc text NOT NULL CHECK (request_rpc IN ('/voice.messaging.v1.MessagingService/SendMessage','/voice.messaging.v1.MessagingService/SendGameEventMessage')),
  request_sha256 text NOT NULL CHECK (request_sha256 ~ '^sha256:[a-f0-9]{64}$'),
  PRIMARY KEY (chat_id, sender_profile_id, client_message_id)
);
