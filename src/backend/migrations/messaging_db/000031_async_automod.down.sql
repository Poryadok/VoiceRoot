BEGIN;
-- Stop new message-trigger work before locking the history-bearing tables.
-- Keep every history probe and destructive statement in this transaction.
LOCK TABLE messages, automod_producer_state, automod_source_subjects,
  automod_terminal_subjects, automod_hmac_keys IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
  IF EXISTS(SELECT 1 FROM automod_producer_state WHERE active) OR
     EXISTS(SELECT 1 FROM automod_source_subjects) OR EXISTS(SELECT 1 FROM automod_terminal_subjects) OR
     EXISTS(SELECT 1 FROM automod_hmac_keys) THEN
    RAISE EXCEPTION 'cannot discard automod source, key or terminal history';
  END IF;
END $$;
DROP TRIGGER automod_message_coverage ON messages;
DROP FUNCTION automod_message_coverage();
DROP TRIGGER automod_remove_deleted_message_coverage ON messages;
DROP FUNCTION automod_remove_deleted_message_coverage();
DROP TABLE automod_message_coverage,automod_expired_receipts,automod_evidence_outbox,automod_hmac_usage,automod_hmac_keys,automod_source_subjects,automod_terminal_subjects;
DROP FUNCTION automod_coverage_identity_guard();
DROP FUNCTION automod_evidence_immutable();
DROP FUNCTION automod_hmac_key_identity_guard();
DROP TRIGGER automod_producer_state_guard ON automod_producer_state;
DROP FUNCTION automod_producer_state_guard();
DROP TABLE automod_producer_state;
COMMIT;
