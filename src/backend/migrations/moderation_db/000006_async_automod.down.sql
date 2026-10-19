-- Downgrade may remove only unused automod state. Lock before observing history
-- so a first concurrent write cannot be committed between the check and DROP.
BEGIN;
DO $$ BEGIN
  IF current_setting('transaction_isolation') <> 'read committed' THEN
    RAISE EXCEPTION 'automod downgrade requires READ COMMITTED';
  END IF;
END $$;
LOCK TABLE automod_subjects, automod_terminal_subjects, automod_inbox,
  automod_patterns, automod_pattern_aliases, automod_actions,
  automod_event_outbox, automod_restriction_sync, sanctions
  IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM automod_subjects)
     OR EXISTS (SELECT 1 FROM automod_terminal_subjects)
     OR EXISTS (SELECT 1 FROM automod_inbox)
     OR EXISTS (SELECT 1 FROM automod_patterns)
     OR EXISTS (SELECT 1 FROM automod_pattern_aliases)
     OR EXISTS (SELECT 1 FROM automod_actions)
     OR EXISTS (SELECT 1 FROM automod_event_outbox)
     OR EXISTS (SELECT 1 FROM automod_restriction_sync)
     OR EXISTS (SELECT 1 FROM sanctions WHERE issuer_service IS NOT NULL) THEN
    RAISE EXCEPTION 'automod history prevents destructive downgrade';
  END IF;
END $$;
DROP TABLE automod_restriction_sync, automod_event_outbox, automod_actions, automod_pattern_aliases, automod_patterns, automod_inbox, automod_subjects, automod_terminal_subjects;
ALTER TABLE sanctions DROP CONSTRAINT sanctions_issuer_kind;
ALTER TABLE sanctions DROP COLUMN issuer_service;
ALTER TABLE sanctions DROP COLUMN machine_case_id;
ALTER TABLE sanctions ALTER COLUMN issued_by SET NOT NULL;
COMMIT;
