BEGIN;
DO $$ BEGIN
  IF current_setting('transaction_isolation') <> 'read committed' THEN
    RAISE EXCEPTION 'automod downgrade requires READ COMMITTED';
  END IF;
END $$;
LOCK TABLE automod_subjects,automod_terminal_subjects,automod_lifecycle_authority,
  automod_lifecycle_current IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
  IF EXISTS(SELECT 1 FROM automod_lifecycle_authority) OR EXISTS(SELECT 1 FROM automod_lifecycle_current) THEN
    RAISE EXCEPTION 'cannot discard installed lifecycle authority or current facts';
  END IF;
END $$;
DROP TRIGGER automod_lifecycle_terminal_cleanup ON automod_terminal_subjects;
DROP FUNCTION automod_lifecycle_terminal_cleanup();
DROP TRIGGER automod_lifecycle_binding_guard ON automod_subjects;
DROP FUNCTION automod_lifecycle_binding_guard();
DROP TABLE automod_lifecycle_current,automod_lifecycle_authority;
COMMIT;
