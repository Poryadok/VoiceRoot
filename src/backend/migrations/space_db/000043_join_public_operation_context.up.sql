BEGIN;
-- START preselection is saved at reservation INSERT, before owner I/O. Existing
-- reservations without provenance are not backfilled from today's policy.
ALTER TABLE space_join_operations
 ADD COLUMN start_selection_context jsonb,
 ADD COLUMN start_selection_sha256 bytea,
 ADD CONSTRAINT space_join_start_selection_pair CHECK(
  (start_selection_context IS NULL AND start_selection_sha256 IS NULL)
  OR (start_selection_context IS NOT NULL AND start_selection_sha256 IS NOT NULL
   AND octet_length(start_selection_sha256)=32 AND basis_version=1 AND operation_origin=1
   AND request_method='/voice.space.v1.SpaceService/StartJoinAttempt'));
ALTER TABLE space_join_operations DROP CONSTRAINT space_join_operations_request_method_check;
ALTER TABLE space_join_operations ADD CONSTRAINT space_join_operations_request_method_check CHECK(
 (basis_version=1 AND operation_origin=1 AND request_method IN (
 '/voice.space.v1.SpaceService/StartJoinAttempt',
 '/voice.space.v1.SpaceService/SubmitJoinAttempt',
 '/voice.space.v1.SpaceService/ApproveJoinRequest',
 '/voice.space.v1.SpaceService/SetEntryPolicy'))
 OR (basis_version=2 AND (
 request_method IN ('/voice.space.v1.SpaceService/JoinSpace','/voice.space.v1.SpaceService/JoinByInvite')
 OR (request_method='/voice.space.v1.SpaceService/ApproveJoinRequest' AND operation_origin=1)))
);
CREATE FUNCTION guard_join_start_selection_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.start_selection_context IS DISTINCT FROM OLD.start_selection_context
    OR NEW.start_selection_sha256 IS DISTINCT FROM OLD.start_selection_sha256
    OR (OLD.start_selection_context IS NOT NULL AND
      ROW(NEW.actor_profile_id,NEW.operation_id,NEW.actor_account_id,NEW.space_id,
          NEW.request_method,NEW.request_sha256,NEW.basis_version,NEW.operation_origin)
      IS DISTINCT FROM
      ROW(OLD.actor_profile_id,OLD.operation_id,OLD.actor_account_id,OLD.space_id,
          OLD.request_method,OLD.request_sha256,OLD.basis_version,OLD.operation_origin)) THEN
  RAISE EXCEPTION 'START selection provenance is immutable' USING ERRCODE='55000';
 END IF;
 IF OLD.start_selection_context IS NOT NULL AND OLD.state='COMPLETED' AND
   ROW(NEW.state,NEW.selected_attempt_id,NEW.outcome_code,NEW.outcome_proto)
   IS DISTINCT FROM ROW(OLD.state,OLD.selected_attempt_id,OLD.outcome_code,OLD.outcome_proto) THEN
  RAISE EXCEPTION 'completed START outcome is immutable' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER space_join_start_selection_immutable BEFORE UPDATE ON space_join_operations
FOR EACH ROW EXECUTE FUNCTION guard_join_start_selection_immutable();
COMMIT;
