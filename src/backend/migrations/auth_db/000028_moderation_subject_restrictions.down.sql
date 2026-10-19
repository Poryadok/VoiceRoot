-- Downgrade would remove the durable ban/deletion authority. Do not silently
-- reactivate accounts or discard terminal fences during a rollback.
BEGIN;
DO $$ BEGIN
  IF current_setting('transaction_isolation') <> 'read committed' THEN
    RAISE EXCEPTION 'automod downgrade requires READ COMMITTED';
  END IF;
END $$;
LOCK TABLE accounts,moderation_account_restrictions,moderation_terminal_subjects IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
  IF EXISTS(SELECT 1 FROM moderation_account_restrictions)
      OR EXISTS(SELECT 1 FROM accounts WHERE moderation_subject_id IS NOT NULL)
      OR EXISTS(SELECT 1 FROM moderation_terminal_subjects) THEN
    RAISE EXCEPTION 'moderation authority requires an explicit preserved-state downgrade';
  END IF;
END $$;
DROP TRIGGER auth_moderation_account_created ON accounts;
DROP TRIGGER auth_moderation_account_guard ON accounts;
DROP FUNCTION auth_moderation_account_created();
DROP FUNCTION auth_moderation_account_guard();
DROP FUNCTION auth_reconcile_moderation(uuid,uuid,bigint,boolean);
DROP TABLE moderation_account_restrictions,moderation_terminal_subjects;
DROP INDEX accounts_moderation_subject_idx;
ALTER TABLE accounts DROP COLUMN moderation_subject_id,DROP COLUMN moderation_lifecycle_revision,
  DROP COLUMN moderation_deletion_cycle_id,DROP COLUMN moderation_purge_at;
COMMIT;
