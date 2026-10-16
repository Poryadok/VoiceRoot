DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM message_event_outbox)
     OR EXISTS (SELECT 1 FROM managed_chat_purge_events)
     OR EXISTS (SELECT 1 FROM managed_chat_purge_operations WHERE event_count <> 0 OR event_set_sha256 IS NOT NULL) THEN
    RAISE EXCEPTION 'refusing to drop durable message event or purge evidence';
  END IF;
END $$;

DROP TABLE managed_chat_purge_events;
ALTER TABLE managed_chat_purge_operations
  DROP CONSTRAINT managed_chat_purge_event_set_hash_len,
  DROP COLUMN event_set_sha256,
  DROP COLUMN event_count;
DROP INDEX message_event_outbox_message_idx;
DROP INDEX message_event_outbox_pending_idx;
DROP TABLE message_event_outbox;
