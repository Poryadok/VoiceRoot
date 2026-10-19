BEGIN;
ALTER TABLE space_join_operations
 ADD COLUMN basis_version smallint NOT NULL DEFAULT 1 CHECK(basis_version IN (1,2)),
 ADD COLUMN operation_origin smallint NOT NULL DEFAULT 1 CHECK(operation_origin IN (1,2));
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
ALTER TABLE space_join_operations ADD CONSTRAINT space_join_operation_basis_family CHECK(
 decision_basis IS NULL OR COALESCE(
 (basis_version=1 AND decision_basis->'context'->>'kind' IN ('1','2','3'))
 OR (basis_version=2 AND decision_basis->'context'->>'kind' IN ('4','5','6')
 AND decision_basis->'context'->>'operation_origin'=operation_origin::text),false)
);
-- One original A submission and one original approval across both provenance
-- families. A legacy call can never wrap an existing Submit winner anew.
DROP INDEX space_join_original_submit_basis;
CREATE UNIQUE INDEX space_join_original_submit_basis ON space_join_operations(selected_attempt_id)
 WHERE decision_basis->'context'->>'kind' IN ('1','2','4','5');
DROP INDEX space_join_original_approval_basis;
CREATE UNIQUE INDEX space_join_original_approval_basis ON space_join_operations(selected_attempt_id)
 WHERE decision_basis->'context'->>'kind' IN ('3','6');
COMMIT;
