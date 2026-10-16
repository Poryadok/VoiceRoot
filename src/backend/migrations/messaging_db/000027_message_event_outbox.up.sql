-- Durable message events are committed with their domain mutation. Payloads
-- remain only while delivery is uncertain; positive PubAck can prune bytes.
CREATE TABLE message_event_outbox (
  event_id UUID PRIMARY KEY,
  subject TEXT NOT NULL CHECK (subject IN (
    'message.sent', 'message.edited', 'message.deleted', 'message.read',
    'message.read_receipt_revoked', 'message.reaction_added',
    'message.reaction_removed', 'message.mention_added', 'message.pinned',
    'message.unpinned', 'message.forwarded'
  )),
  message_id UUID NOT NULL,
  chat_id UUID NOT NULL,
  payload_bytes BYTEA,
  payload_sha256 BYTEA NOT NULL CHECK (octet_length(payload_sha256) = 32),
  headers JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (jsonb_typeof(headers) = 'object'),
  created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
  attempt_count BIGINT NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
  lease_token UUID,
  lease_expires_at TIMESTAMPTZ,
  pubacked_at TIMESTAMPTZ,
  puback_sequence BIGINT,
  payload_pruned_at TIMESTAMPTZ,
  CHECK ((lease_token IS NULL) = (lease_expires_at IS NULL)),
  CHECK ((pubacked_at IS NULL) = (puback_sequence IS NULL)),
  CHECK (pubacked_at IS NULL OR payload_bytes IS NULL),
  CHECK ((payload_bytes IS NULL) = (payload_pruned_at IS NOT NULL))
);

CREATE INDEX message_event_outbox_pending_idx
  ON message_event_outbox(next_attempt_at, created_at, event_id)
  WHERE pubacked_at IS NULL;
CREATE INDEX message_event_outbox_message_idx
  ON message_event_outbox(chat_id, message_id, event_id);

ALTER TABLE managed_chat_purge_operations
  ADD COLUMN event_count BIGINT NOT NULL DEFAULT 0 CHECK (event_count >= 0),
  ADD COLUMN event_set_sha256 BYTEA;
ALTER TABLE managed_chat_purge_operations
  ADD CONSTRAINT managed_chat_purge_event_set_hash_len
  CHECK (event_set_sha256 IS NULL OR octet_length(event_set_sha256) = 32);

-- Immutable event membership/digests bind the event set to the frozen message
-- work set. Delivery/PubAck state remains only in message_event_outbox.
CREATE TABLE managed_chat_purge_events (
  operation_id UUID NOT NULL REFERENCES managed_chat_purge_operations(operation_id) ON DELETE CASCADE,
  event_id UUID NOT NULL,
  message_id UUID NOT NULL,
  event_sha256 BYTEA NOT NULL CHECK (octet_length(event_sha256) = 32),
  PRIMARY KEY (operation_id, event_id),
  UNIQUE (operation_id, message_id, event_id)
);
CREATE INDEX managed_chat_purge_events_message_idx
  ON managed_chat_purge_events(message_id, event_id);
