-- A machine sanction must be revoked/exported explicitly before downgrade.
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM sanctions WHERE issuer_service IS NOT NULL) THEN
    RAISE EXCEPTION 'machine sanctions prevent destructive automod downgrade';
  END IF;
END $$;
DROP TABLE automod_restriction_sync, automod_event_outbox, automod_actions, automod_pattern_aliases, automod_patterns, automod_inbox, automod_subjects, automod_terminal_subjects;
ALTER TABLE sanctions DROP CONSTRAINT sanctions_issuer_kind;
ALTER TABLE sanctions DROP COLUMN issuer_service;
ALTER TABLE sanctions DROP COLUMN machine_case_id;
ALTER TABLE sanctions ALTER COLUMN issued_by SET NOT NULL;
