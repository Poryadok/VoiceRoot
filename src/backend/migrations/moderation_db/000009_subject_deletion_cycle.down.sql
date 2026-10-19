DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM automod_subjects WHERE deletion_cycle_id IS NOT NULL) THEN
    RAISE EXCEPTION 'cannot discard current deletion cycle authority';
  END IF;
END $$;
DROP TRIGGER automod_cycle_deadline_guard ON automod_subjects;
DROP FUNCTION automod_guard_cycle_deadline();
ALTER TABLE automod_subjects DROP CONSTRAINT automod_subject_cycle;
ALTER TABLE automod_subjects DROP COLUMN deletion_cycle_id;
