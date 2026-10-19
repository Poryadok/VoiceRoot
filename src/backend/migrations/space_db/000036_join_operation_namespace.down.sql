BEGIN;
LOCK TABLE space_join_operations IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM space_join_operations) THEN
  RAISE EXCEPTION 'cannot discard join operation history' USING ERRCODE='P0001';
 END IF;
END $$;
DROP TRIGGER space_join_operations_terminal ON spaces;
DROP FUNCTION purge_space_join_operations_terminal();
DROP TABLE space_join_operations;
COMMIT;
