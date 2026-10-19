BEGIN;
ALTER TABLE space_join_operations
 ADD COLUMN legacy_selection_context jsonb,
 ADD COLUMN legacy_selection_sha256 bytea,
 ADD CONSTRAINT space_join_legacy_selection_pair CHECK(
  (legacy_selection_context IS NULL AND legacy_selection_sha256 IS NULL)
  OR (legacy_selection_context IS NOT NULL AND legacy_selection_sha256 IS NOT NULL
   AND octet_length(legacy_selection_sha256)=32 AND basis_version=2
   AND operation_origin IN (1,2) AND start_selection_context IS NULL
   AND request_method IN ('/voice.space.v1.SpaceService/JoinSpace','/voice.space.v1.SpaceService/JoinByInvite')));
CREATE FUNCTION guard_join_legacy_selection_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NEW.legacy_selection_context IS DISTINCT FROM OLD.legacy_selection_context
    OR NEW.legacy_selection_sha256 IS DISTINCT FROM OLD.legacy_selection_sha256
    OR (OLD.legacy_selection_context IS NOT NULL AND
      ROW(NEW.actor_profile_id,NEW.operation_id,NEW.actor_account_id,NEW.space_id,
          NEW.request_method,NEW.request_sha256,NEW.basis_version,NEW.operation_origin)
      IS DISTINCT FROM
      ROW(OLD.actor_profile_id,OLD.operation_id,OLD.actor_account_id,OLD.space_id,
          OLD.request_method,OLD.request_sha256,OLD.basis_version,OLD.operation_origin)) THEN
  RAISE EXCEPTION 'legacy selection provenance is immutable' USING ERRCODE='55000';
 END IF;
 IF OLD.legacy_selection_context IS NOT NULL AND OLD.state='COMPLETED' AND
   ROW(NEW.state,NEW.selected_attempt_id,NEW.outcome_code,NEW.outcome_proto)
   IS DISTINCT FROM ROW(OLD.state,OLD.selected_attempt_id,OLD.outcome_code,OLD.outcome_proto) THEN
  RAISE EXCEPTION 'completed legacy outcome is immutable' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER space_join_legacy_selection_immutable BEFORE UPDATE ON space_join_operations
FOR EACH ROW EXECUTE FUNCTION guard_join_legacy_selection_immutable();
COMMIT;
