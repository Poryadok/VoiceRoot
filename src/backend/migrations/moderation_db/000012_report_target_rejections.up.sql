-- BE-137 c51/c52: a rejected intake is not an identity-erasure fact.
CREATE TABLE report_target_rejections (
 report_id UUID PRIMARY KEY CHECK(report_id<>'00000000-0000-0000-0000-000000000000'::uuid),
 decision TEXT NOT NULL CHECK(decision IN ('MISSING','DELETED')),
 provenance TEXT NOT NULL CHECK(provenance IN ('OWNER_VALIDATION','LEGACY_RECORDED_DECISION')),
 schema_version INTEGER NOT NULL CHECK(schema_version=1),
 sequence BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE
);
-- No report FK: the purpose-limited receipt protects replay after raw deletion.
ALTER TABLE reports ALTER COLUMN target_id DROP NOT NULL;
-- NOT VALID permits finite repair of already recorded legacy rejections. It
-- still fences every new/updated row; validate after the inventory is repaired.
ALTER TABLE reports ADD CONSTRAINT reports_target_representation CHECK (
 (target_type='user' AND validation_state IN ('MISSING','DELETED')
  AND target_id IS NULL AND status='dismissed' AND description IS NULL AND evidence='{}'::jsonb
  AND validated_target_account_id IS NULL AND validated_target_subject_id IS NULL
  AND validated_target_subject_revision IS NULL AND validated_target_provenance='LEGACY_UNVERIFIED'
  AND validation_claim IS NULL AND validation_claim_until IS NULL
  AND resolution IS NOT DISTINCT FROM jsonb_build_object('code',CASE validation_state WHEN 'MISSING' THEN 'TARGET_NOT_FOUND' ELSE 'TARGET_DELETED' END))
 OR ((target_type<>'user' OR validation_state IN ('PENDING','VALID'))
  AND target_id IS NOT NULL AND target_id<>'00000000-0000-0000-0000-000000000000'::uuid)
) NOT VALID;
