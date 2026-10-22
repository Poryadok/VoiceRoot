BEGIN;
DO $$ BEGIN
 IF current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'join basis downgrade requires fresh READ COMMITTED' USING ERRCODE='55000';
 END IF;
END $$;
LOCK TABLE space_join_operations IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM space_join_operations WHERE basis_version<>1 OR operation_origin<>1) THEN
  RAISE EXCEPTION 'cannot erase legacy decision provenance' USING ERRCODE='55000';
 END IF;
END $$;
DROP INDEX space_join_original_submit_basis;
CREATE UNIQUE INDEX space_join_original_submit_basis ON space_join_operations(selected_attempt_id)
 WHERE decision_basis->'context'->>'kind' IN ('1','2');
DROP INDEX space_join_original_approval_basis;
CREATE UNIQUE INDEX space_join_original_approval_basis ON space_join_operations(selected_attempt_id)
 WHERE decision_basis->'context'->>'kind'='3';
ALTER TABLE space_join_operations DROP CONSTRAINT space_join_operation_basis_family;
ALTER TABLE space_join_operations DROP CONSTRAINT space_join_operations_request_method_check;
ALTER TABLE space_join_operations ADD CONSTRAINT space_join_operations_request_method_check CHECK(request_method IN (
 '/voice.space.v1.SpaceService/StartJoinAttempt',
 '/voice.space.v1.SpaceService/SubmitJoinAttempt',
 '/voice.space.v1.SpaceService/ApproveJoinRequest'));
ALTER TABLE space_join_operations DROP COLUMN basis_version,DROP COLUMN operation_origin;
COMMIT;
