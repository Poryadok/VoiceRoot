BEGIN;

-- Protected issuer journal. Installation provisioning grants the maintenance
-- verifier SELECT and the isolated issuer SELECT/INSERT only. Neither role may
-- own these tables/schema, grant roles, TRUNCATE, disable triggers or rewrite.
CREATE TABLE voice_party_resource_components (
 journal_ref UUID PRIMARY KEY,
 kind TEXT NOT NULL CHECK(length(kind)>0),
 protocol BIGINT NOT NULL CHECK(protocol=1),
 canonical_bytes BYTEA NOT NULL CHECK(octet_length(canonical_bytes)>0),
 canonical_bytes_sha256 BYTEA NOT NULL CHECK(octet_length(canonical_bytes_sha256)=32)
);
CREATE TABLE voice_party_resource_retirements (
 proof_id UUID PRIMARY KEY,
 protocol BIGINT NOT NULL CHECK(protocol=1),
 cutover_operation_id UUID NOT NULL REFERENCES voice_party_coverage_operations(operation_id),
 canonical_receipt_bytes BYTEA NOT NULL CHECK(octet_length(canonical_receipt_bytes)>0),
 receipt_sha256 BYTEA NOT NULL CHECK(octet_length(receipt_sha256)=32)
);
CREATE TABLE voice_party_resource_component_links (
 proof_id UUID NOT NULL REFERENCES voice_party_resource_retirements(proof_id),
 ordinal BIGINT NOT NULL CHECK(ordinal>=0),
 journal_ref UUID NOT NULL REFERENCES voice_party_resource_components(journal_ref),
 PRIMARY KEY(proof_id,ordinal),
 UNIQUE(proof_id,journal_ref)
);
REVOKE ALL ON voice_party_resource_components,voice_party_resource_retirements,voice_party_resource_component_links FROM PUBLIC;
CREATE TRIGGER voice_party_resource_components_immutable BEFORE UPDATE OR DELETE ON voice_party_resource_components FOR EACH ROW EXECUTE FUNCTION voice_party_coverage_no_rewrite();
CREATE TRIGGER voice_party_resource_retirements_immutable BEFORE UPDATE OR DELETE ON voice_party_resource_retirements FOR EACH ROW EXECUTE FUNCTION voice_party_coverage_no_rewrite();
CREATE TRIGGER voice_party_resource_component_links_immutable BEFORE UPDATE OR DELETE ON voice_party_resource_component_links FOR EACH ROW EXECUTE FUNCTION voice_party_coverage_no_rewrite();
CREATE TRIGGER voice_party_resource_components_no_truncate BEFORE TRUNCATE ON voice_party_resource_components FOR EACH STATEMENT EXECUTE FUNCTION voice_party_coverage_no_rewrite();
CREATE TRIGGER voice_party_resource_retirements_no_truncate BEFORE TRUNCATE ON voice_party_resource_retirements FOR EACH STATEMENT EXECUTE FUNCTION voice_party_coverage_no_rewrite();
CREATE TRIGGER voice_party_resource_component_links_no_truncate BEFORE TRUNCATE ON voice_party_resource_component_links FOR EACH STATEMENT EXECUTE FUNCTION voice_party_coverage_no_rewrite();
COMMIT;
