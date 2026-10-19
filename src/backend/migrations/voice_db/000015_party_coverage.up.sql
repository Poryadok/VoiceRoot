BEGIN;

-- Separate capability issuer owns writes. Application credentials receive only
-- SELECT through explicit installation provisioning, never table ownership.
CREATE TABLE voice_party_coverage_operations (
 operation_id UUID PRIMARY KEY,
 environment_id UUID NOT NULL,
 authority_incarnation UUID NOT NULL,
 coverage_epoch BIGINT NOT NULL CHECK(coverage_epoch>0),
 plan_bytes BYTEA NOT NULL CHECK(octet_length(plan_bytes)>0),
 plan_sha256 BYTEA NOT NULL CHECK(octet_length(plan_sha256)=32),
 UNIQUE(environment_id,authority_incarnation,coverage_epoch)
);
CREATE TABLE voice_party_coverage_receipts (
 coverage_id UUID PRIMARY KEY,
 cutover_operation_id UUID NOT NULL UNIQUE REFERENCES voice_party_coverage_operations(operation_id),
 environment_id UUID NOT NULL,
 authority_incarnation UUID NOT NULL,
 coverage_epoch BIGINT NOT NULL CHECK(coverage_epoch>0),
 voice_database_identity BYTEA NOT NULL CHECK(octet_length(voice_database_identity)=32),
 source_catalog_sha256 BYTEA NOT NULL CHECK(octet_length(source_catalog_sha256)=32),
 schema_contract_sha256 BYTEA NOT NULL CHECK(octet_length(schema_contract_sha256)=32),
 writer_manifest_sha256 BYTEA NOT NULL CHECK(octet_length(writer_manifest_sha256)=32),
 external_fence_receipt_sha256 BYTEA NOT NULL CHECK(octet_length(external_fence_receipt_sha256)=32),
 drain_manifest_sha256 BYTEA NOT NULL CHECK(octet_length(drain_manifest_sha256)=32),
 native_validation_sha256 BYTEA NOT NULL CHECK(octet_length(native_validation_sha256)=32),
 canonical_receipt_bytes BYTEA NOT NULL CHECK(octet_length(canonical_receipt_bytes)>0),
 receipt_sha256 BYTEA NOT NULL CHECK(octet_length(receipt_sha256)=32),
 UNIQUE(environment_id,authority_incarnation,coverage_epoch)
);
CREATE TABLE voice_party_coverage_head (
 singleton BOOLEAN PRIMARY KEY DEFAULT true CHECK(singleton),
 environment_id UUID NOT NULL,
 authority_incarnation UUID NOT NULL,
 coverage_epoch BIGINT NOT NULL CHECK(coverage_epoch>0),
 phase TEXT NOT NULL CHECK(phase IN ('PREPARING','FROZEN','DRAINED','ACTIVE','SUSPENDED')),
 coverage_id UUID REFERENCES voice_party_coverage_receipts(coverage_id),
 current_operation_id UUID NOT NULL REFERENCES voice_party_coverage_operations(operation_id),
 revision BIGINT NOT NULL CHECK(revision>0),
 CHECK((phase='ACTIVE')=(coverage_id IS NOT NULL))
);
CREATE TABLE voice_party_coverage_steps (
 operation_id UUID NOT NULL REFERENCES voice_party_coverage_operations(operation_id),
 phase TEXT NOT NULL CHECK(phase IN ('FROZEN','DRAINED','VALIDATED','ACTIVE','SUSPENDED')),
 proof_bytes BYTEA NOT NULL CHECK(octet_length(proof_bytes)>0),
 proof_sha256 BYTEA NOT NULL CHECK(octet_length(proof_sha256)=32),
 PRIMARY KEY(operation_id,phase)
);
REVOKE ALL ON voice_party_coverage_operations,voice_party_coverage_receipts,voice_party_coverage_head,voice_party_coverage_steps FROM PUBLIC;

-- Even the issuer can only append immutable operation/proof/receipt material.
CREATE FUNCTION voice_party_coverage_no_rewrite() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN RAISE EXCEPTION 'party coverage proof is append-only'; END $$;
CREATE TRIGGER voice_party_coverage_operations_immutable BEFORE UPDATE OR DELETE ON voice_party_coverage_operations FOR EACH ROW EXECUTE FUNCTION voice_party_coverage_no_rewrite();
CREATE TRIGGER voice_party_coverage_receipts_immutable BEFORE UPDATE OR DELETE ON voice_party_coverage_receipts FOR EACH ROW EXECUTE FUNCTION voice_party_coverage_no_rewrite();
CREATE TRIGGER voice_party_coverage_steps_immutable BEFORE UPDATE OR DELETE ON voice_party_coverage_steps FOR EACH ROW EXECUTE FUNCTION voice_party_coverage_no_rewrite();
COMMIT;
