-- Maintenance owner evidence. Ordinary serving/executor credentials must not
-- receive DML here; explicit maintenance composition is a separate authority.
CREATE TABLE voice_native_owner_works (
 work_id UUID PRIMARY KEY,
 space_id UUID NOT NULL,
 owner_protocol TEXT NOT NULL,
 purpose TEXT NOT NULL CHECK(purpose IN ('COLD_CUT_DRAIN','P3_FROZEN_COMPLETION')),
 request_sha256 BYTEA NOT NULL CHECK(octet_length(request_sha256)=32),
 work_bytes BYTEA NOT NULL CHECK(octet_length(work_bytes)>0),
 work_sha256 BYTEA NOT NULL CHECK(octet_length(work_sha256)=32),
 state TEXT NOT NULL DEFAULT 'PREPARED' CHECK(state IN ('PREPARED','COMPLETE')),
 receipt_bytes BYTEA,
 receipt_sha256 BYTEA,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 completed_at TIMESTAMPTZ,
 CHECK ((state='PREPARED' AND receipt_bytes IS NULL AND receipt_sha256 IS NULL AND completed_at IS NULL)
     OR (state='COMPLETE' AND receipt_bytes IS NOT NULL AND receipt_sha256 IS NOT NULL AND octet_length(receipt_bytes)>0 AND octet_length(receipt_sha256)=32 AND completed_at IS NOT NULL))
);
CREATE INDEX voice_native_owner_works_space ON voice_native_owner_works(space_id,work_id);

CREATE TABLE voice_native_owner_acceptances (
 work_id UUID NOT NULL REFERENCES voice_native_owner_works(work_id),
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

CREATE TABLE voice_native_owner_targets (
 work_id UUID NOT NULL REFERENCES voice_native_owner_works(work_id),
 target_id TEXT NOT NULL,
 source_kind TEXT NOT NULL CHECK(source_kind IN ('native','head','legacy','legacy_head','grant','event','history')),
 target_bytes BYTEA NOT NULL CHECK(octet_length(target_bytes)>0),
 target_sha256 BYTEA NOT NULL CHECK(octet_length(target_sha256)=32),
 effect_kind TEXT CHECK(effect_kind='REALM_RETIRED'),
 effect_proof_id UUID,
 cleanup_completed_at TIMESTAMPTZ,
 PRIMARY KEY(work_id,target_id),
 FOREIGN KEY(work_id,effect_proof_id) REFERENCES voice_native_owner_acceptances(work_id,proof_id),
 CHECK ((effect_kind IS NULL AND effect_proof_id IS NULL) OR (effect_kind IS NOT NULL AND effect_kind='REALM_RETIRED' AND effect_proof_id IS NOT NULL))
);

ALTER TABLE voice_space_lifecycle_fence_receipts ADD COLUMN retirement_owner_work_id UUID REFERENCES voice_native_owner_works(work_id);
REVOKE ALL ON voice_native_owner_works,voice_native_owner_acceptances,voice_native_owner_targets FROM PUBLIC;

CREATE FUNCTION voice_native_owner_preserve_evidence() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN RAISE EXCEPTION 'native owner evidence is retained'; END IF;
 IF TG_TABLE_NAME='voice_native_owner_acceptances' AND NEW IS DISTINCT FROM OLD THEN
  RAISE EXCEPTION 'verified native acceptance is immutable';
 ELSIF TG_TABLE_NAME='voice_native_owner_works' THEN
  IF NEW.work_id IS DISTINCT FROM OLD.work_id OR NEW.space_id IS DISTINCT FROM OLD.space_id
    OR NEW.owner_protocol IS DISTINCT FROM OLD.owner_protocol OR NEW.purpose IS DISTINCT FROM OLD.purpose
    OR NEW.request_sha256 IS DISTINCT FROM OLD.request_sha256 OR NEW.work_bytes IS DISTINCT FROM OLD.work_bytes
    OR NEW.work_sha256 IS DISTINCT FROM OLD.work_sha256 OR NEW.created_at IS DISTINCT FROM OLD.created_at
    OR (OLD.state='COMPLETE' AND NEW IS DISTINCT FROM OLD) THEN
   RAISE EXCEPTION 'native owner work/receipt is immutable';
  END IF;
 ELSIF TG_TABLE_NAME='voice_native_owner_targets' THEN
  IF NEW.work_id IS DISTINCT FROM OLD.work_id OR NEW.target_id IS DISTINCT FROM OLD.target_id
    OR NEW.source_kind IS DISTINCT FROM OLD.source_kind OR NEW.target_bytes IS DISTINCT FROM OLD.target_bytes
    OR NEW.target_sha256 IS DISTINCT FROM OLD.target_sha256
    OR (OLD.effect_proof_id IS NOT NULL AND (NEW.effect_proof_id IS DISTINCT FROM OLD.effect_proof_id OR NEW.effect_kind IS DISTINCT FROM OLD.effect_kind))
    OR (OLD.cleanup_completed_at IS NOT NULL AND NEW.cleanup_completed_at IS DISTINCT FROM OLD.cleanup_completed_at) THEN
   RAISE EXCEPTION 'native owner target/progress is immutable';
  END IF;
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER voice_native_owner_work_evidence BEFORE UPDATE OR DELETE ON voice_native_owner_works FOR EACH ROW EXECUTE FUNCTION voice_native_owner_preserve_evidence();
CREATE TRIGGER voice_native_owner_acceptance_evidence BEFORE UPDATE OR DELETE ON voice_native_owner_acceptances FOR EACH ROW EXECUTE FUNCTION voice_native_owner_preserve_evidence();
CREATE TRIGGER voice_native_owner_target_evidence BEFORE UPDATE OR DELETE ON voice_native_owner_targets FOR EACH ROW EXECUTE FUNCTION voice_native_owner_preserve_evidence();
