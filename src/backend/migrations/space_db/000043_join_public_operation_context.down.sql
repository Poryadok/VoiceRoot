BEGIN;
DO $$ BEGIN
 IF current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'public context downgrade requires fresh READ COMMITTED' USING ERRCODE='55000';
 END IF;
END $$;
LOCK TABLE space_join_operations IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM space_join_operations WHERE start_selection_context IS NOT NULL
  OR request_method='/voice.space.v1.SpaceService/SetEntryPolicy') THEN
  RAISE EXCEPTION 'cannot erase public operation provenance' USING ERRCODE='55000';
 END IF;
END $$;
DROP TRIGGER space_join_start_selection_immutable ON space_join_operations;
DROP FUNCTION guard_join_start_selection_immutable();
ALTER TABLE space_join_operations DROP CONSTRAINT space_join_start_selection_pair,
 DROP COLUMN start_selection_context,DROP COLUMN start_selection_sha256;
ALTER TABLE space_join_operations DROP CONSTRAINT space_join_operations_request_method_check;
ALTER TABLE space_join_operations ADD CONSTRAINT space_join_operations_request_method_check CHECK(
 (basis_version=1 AND operation_origin=1 AND request_method IN (
 '/voice.space.v1.SpaceService/StartJoinAttempt',
 '/voice.space.v1.SpaceService/SubmitJoinAttempt',
 '/voice.space.v1.SpaceService/ApproveJoinRequest'))
 OR (basis_version=2 AND (
 request_method IN ('/voice.space.v1.SpaceService/JoinSpace','/voice.space.v1.SpaceService/JoinByInvite')
 OR (request_method='/voice.space.v1.SpaceService/ApproveJoinRequest' AND operation_origin=1)))
);
COMMIT;
