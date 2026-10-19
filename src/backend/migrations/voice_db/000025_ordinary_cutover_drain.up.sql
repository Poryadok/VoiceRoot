-- Non-serving native owner evidence. Resource retirement is qualified per work;
-- it does not grant serving admission or change the live provider capability.
CREATE TABLE voice_ordinary_cutover_works (
 work_id UUID PRIMARY KEY,
 room_id UUID NOT NULL,
 incarnation UUID NOT NULL,
 cut_operation_id UUID NOT NULL,
 request_sha256 BYTEA NOT NULL CHECK(octet_length(request_sha256)=32),
 work_bytes BYTEA NOT NULL CHECK(octet_length(work_bytes)>0),
 work_sha256 BYTEA NOT NULL CHECK(octet_length(work_sha256)=32),
 state TEXT NOT NULL DEFAULT 'PREPARED' CHECK(state IN ('PREPARED','COMPLETE')),
 receipt_bytes BYTEA,
 receipt_sha256 BYTEA,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 completed_at TIMESTAMPTZ,
 UNIQUE(cut_operation_id,room_id),
 CHECK ((state='PREPARED' AND receipt_bytes IS NULL AND receipt_sha256 IS NULL AND completed_at IS NULL)
 OR (state='COMPLETE' AND receipt_bytes IS NOT NULL AND receipt_sha256 IS NOT NULL
 AND octet_length(receipt_bytes)>0 AND octet_length(receipt_sha256)=32 AND completed_at IS NOT NULL))
);
CREATE TABLE voice_ordinary_cutover_acceptances (
 work_id UUID NOT NULL REFERENCES voice_ordinary_cutover_works(work_id),
 proof_id UUID NOT NULL,
 proof_ref_bytes BYTEA NOT NULL,
 resource_receipt_sha256 BYTEA NOT NULL CHECK(octet_length(resource_receipt_sha256)=32),
 work_sha256 BYTEA NOT NULL CHECK(octet_length(work_sha256)=32),
 target_manifest_sha256 BYTEA NOT NULL CHECK(octet_length(target_manifest_sha256)=32),
 resolution_bytes BYTEA NOT NULL CHECK(octet_length(resolution_bytes)>0),
 resolution_sha256 BYTEA NOT NULL CHECK(octet_length(resolution_sha256)=32),
 accepted_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(work_id,proof_id)
);
CREATE TABLE voice_ordinary_cutover_targets (
 work_id UUID NOT NULL REFERENCES voice_ordinary_cutover_works(work_id),
 target_kind TEXT NOT NULL CHECK(target_kind IN ('room','closure','membership','fence','outbox')),
 target_key TEXT NOT NULL,
 target_bytes BYTEA NOT NULL CHECK(octet_length(target_bytes)>0),
 target_sha256 BYTEA NOT NULL CHECK(octet_length(target_sha256)=32),
 effect_kind TEXT CHECK(effect_kind='REALM_RETIRED'),
 effect_proof_id UUID,
 cleanup_completed_at TIMESTAMPTZ,
 PRIMARY KEY(work_id,target_kind,target_key),
 FOREIGN KEY(work_id,effect_proof_id) REFERENCES voice_ordinary_cutover_acceptances(work_id,proof_id),
 CHECK ((effect_kind IS NULL AND effect_proof_id IS NULL)
 OR (effect_kind IS NOT NULL AND effect_kind='REALM_RETIRED' AND effect_proof_id IS NOT NULL))
);
REVOKE ALL ON voice_ordinary_cutover_works,voice_ordinary_cutover_targets,voice_ordinary_cutover_acceptances FROM PUBLIC;

CREATE FUNCTION voice_ordinary_cutover_preserve() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'ordinary cutover evidence is retained'; END IF;
 IF TG_TABLE_NAME='voice_ordinary_cutover_acceptances' AND NEW IS DISTINCT FROM OLD THEN
  RAISE EXCEPTION 'ordinary retirement acceptance is immutable';
 ELSIF TG_TABLE_NAME='voice_ordinary_cutover_works' THEN
  IF NEW.work_id IS DISTINCT FROM OLD.work_id OR NEW.room_id IS DISTINCT FROM OLD.room_id
  OR NEW.incarnation IS DISTINCT FROM OLD.incarnation OR NEW.cut_operation_id IS DISTINCT FROM OLD.cut_operation_id
  OR NEW.request_sha256 IS DISTINCT FROM OLD.request_sha256 OR NEW.work_bytes IS DISTINCT FROM OLD.work_bytes
  OR NEW.work_sha256 IS DISTINCT FROM OLD.work_sha256 OR NEW.created_at IS DISTINCT FROM OLD.created_at
  OR (OLD.state='COMPLETE' AND NEW IS DISTINCT FROM OLD) THEN
   RAISE EXCEPTION 'ordinary cutover work and receipt are immutable';
  END IF;
 ELSIF TG_TABLE_NAME='voice_ordinary_cutover_targets' THEN
  IF NEW.work_id IS DISTINCT FROM OLD.work_id OR NEW.target_kind IS DISTINCT FROM OLD.target_kind
  OR NEW.target_key IS DISTINCT FROM OLD.target_key OR NEW.target_bytes IS DISTINCT FROM OLD.target_bytes
  OR NEW.target_sha256 IS DISTINCT FROM OLD.target_sha256
  OR (OLD.effect_proof_id IS NOT NULL AND (NEW.effect_proof_id IS DISTINCT FROM OLD.effect_proof_id OR NEW.effect_kind IS DISTINCT FROM OLD.effect_kind))
  OR (OLD.cleanup_completed_at IS NOT NULL AND NEW.cleanup_completed_at IS DISTINCT FROM OLD.cleanup_completed_at) THEN
   RAISE EXCEPTION 'ordinary cutover target and progress are immutable';
  END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER voice_ordinary_cutover_work_evidence BEFORE UPDATE OR DELETE ON voice_ordinary_cutover_works FOR EACH ROW EXECUTE FUNCTION voice_ordinary_cutover_preserve();
CREATE TRIGGER voice_ordinary_cutover_target_evidence BEFORE UPDATE OR DELETE ON voice_ordinary_cutover_targets FOR EACH ROW EXECUTE FUNCTION voice_ordinary_cutover_preserve();
CREATE TRIGGER voice_ordinary_cutover_acceptance_evidence BEFORE UPDATE OR DELETE ON voice_ordinary_cutover_acceptances FOR EACH ROW EXECUTE FUNCTION voice_ordinary_cutover_preserve();
