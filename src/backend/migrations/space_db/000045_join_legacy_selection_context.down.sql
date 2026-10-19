BEGIN;
DO $$ BEGIN
 IF current_setting('transaction_isolation') <> 'read committed' THEN
  RAISE EXCEPTION 'legacy selection downgrade requires READ COMMITTED' USING ERRCODE='55000';
 END IF;
END $$;
LOCK TABLE space_join_operations IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM space_join_operations WHERE legacy_selection_context IS NOT NULL OR legacy_selection_sha256 IS NOT NULL) THEN
  RAISE EXCEPTION 'retained legacy selection prevents downgrade' USING ERRCODE='55000';
 END IF;
END $$;
DROP TRIGGER space_join_legacy_selection_immutable ON space_join_operations;
DROP FUNCTION guard_join_legacy_selection_immutable();
ALTER TABLE space_join_operations DROP CONSTRAINT space_join_legacy_selection_pair,
 DROP COLUMN legacy_selection_context, DROP COLUMN legacy_selection_sha256;
COMMIT;
