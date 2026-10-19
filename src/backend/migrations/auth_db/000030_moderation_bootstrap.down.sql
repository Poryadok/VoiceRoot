BEGIN;
DO $$ BEGIN
  IF current_setting('transaction_isolation') <> 'read committed' THEN
    RAISE EXCEPTION 'automod downgrade requires READ COMMITTED';
  END IF;
END $$;
LOCK TABLE accounts,moderation_terminal_subjects,moderation_authority_generation IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
  IF EXISTS(SELECT 1 FROM moderation_terminal_subjects) OR
     EXISTS(SELECT 1 FROM moderation_authority_generation WHERE terminal_high_water>0) THEN
    RAISE EXCEPTION 'cannot discard terminal authority generation or progress';
  END IF;
END $$;
DROP TRIGGER moderation_terminal_progress ON moderation_terminal_subjects;
DROP TRIGGER moderation_terminal_permanent ON moderation_terminal_subjects;
DROP FUNCTION moderation_terminal_progress();
DROP TABLE moderation_authority_generation;
DROP FUNCTION moderation_authority_generation_guard();
COMMIT;
