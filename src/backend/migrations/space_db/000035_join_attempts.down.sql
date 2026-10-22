BEGIN;
LOCK TABLE space_join_attempts,space_join_approval_operations IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM space_join_attempts) OR EXISTS(SELECT 1 FROM space_join_approval_operations) THEN
  RAISE EXCEPTION 'join attempts require explicit data reconciliation before downgrade';
 END IF;
END $$;
DROP TRIGGER space_join_attempts_terminal ON spaces;
DROP FUNCTION purge_space_join_attempts_terminal();
DROP TABLE space_join_approval_operations;
DROP TABLE space_join_attempts;
COMMIT;
