-- Existing grants/receipts have UNKNOWN credential history. Do not retrofit a
-- retirement proof from an empty table or the original Voice token lifetime.
ALTER TABLE voice_space_media_admissions ADD COLUMN lifecycle_generation BIGINT NOT NULL DEFAULT 0 CHECK(lifecycle_generation >= 0);
ALTER TABLE voice_space_media_room_heads ADD COLUMN lifecycle_generation BIGINT NOT NULL DEFAULT 0 CHECK(lifecycle_generation >= 0);
ALTER TABLE voice_space_lifecycle_fence_receipts ADD COLUMN coverage_sealed BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE voice_space_lifecycle_fence_receipts ADD COLUMN retirement_proven BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE voice_space_lifecycle_targets (
 space_id UUID NOT NULL,
 generation BIGINT NOT NULL,
 target_id TEXT NOT NULL,
 source_kind TEXT NOT NULL CHECK(source_kind IN ('native','legacy','head')),
 deletion_operation_id UUID NOT NULL,
 request_sha256 BYTEA NOT NULL CHECK(octet_length(request_sha256)=32),
 target JSONB NOT NULL,
 ejected_at TIMESTAMPTZ,
 cleanup_completed_at TIMESTAMPTZ,
 PRIMARY KEY(space_id,generation,target_id),
 FOREIGN KEY(space_id,generation) REFERENCES voice_space_lifecycle_fence_receipts(space_id,generation)
);

CREATE TABLE voice_space_media_grants (
 grant_id UUID PRIMARY KEY,
 operation_id UUID NOT NULL REFERENCES voice_space_media_admissions(operation_id),
 admission_generation TEXT NOT NULL,
 lifecycle_generation BIGINT NOT NULL CHECK(lifecycle_generation >= 0),
 claims_sha256 BYTEA NOT NULL CHECK(octet_length(claims_sha256)=32),
 claims JSONB NOT NULL,
 issued_at TIMESTAMPTZ NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,
 CHECK(expires_at > issued_at AND expires_at <= issued_at+interval '60 seconds')
);
CREATE INDEX voice_space_media_grants_admission ON voice_space_media_grants(operation_id);
