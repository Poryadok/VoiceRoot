-- No cycle is inferred for old DELETED bindings. Private recovery must obtain
-- their current exact Auth tuple before retained identity can be consumed.
ALTER TABLE automod_subjects ADD COLUMN deletion_cycle_id uuid;
ALTER TABLE automod_subjects ADD CONSTRAINT automod_subject_cycle CHECK (
  (deletion_cycle_id IS NULL OR deletion_cycle_id <> '00000000-0000-0000-0000-000000000000'::uuid)
  AND (lifecycle_state <> 'LIVE' OR deletion_cycle_id IS NULL)
);

CREATE FUNCTION automod_guard_cycle_deadline() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  IF OLD.lifecycle_state='DELETED' AND NEW.lifecycle_state='DELETED'
     AND OLD.deletion_cycle_id IS NOT NULL AND OLD.deletion_cycle_id=NEW.deletion_cycle_id
     AND OLD.purge_at IS DISTINCT FROM NEW.purge_at THEN
    RAISE EXCEPTION 'automod_cycle_deadline_immutable';
  END IF;
  RETURN NEW;
END $$;
CREATE TRIGGER automod_cycle_deadline_guard BEFORE UPDATE ON automod_subjects
FOR EACH ROW EXECUTE FUNCTION automod_guard_cycle_deadline();
