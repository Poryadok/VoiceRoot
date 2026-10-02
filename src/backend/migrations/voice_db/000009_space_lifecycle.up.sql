BEGIN;

CREATE TABLE voice_space_lifecycle_fence_heads (
    space_id UUID PRIMARY KEY CHECK (space_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    generation BIGINT NOT NULL CHECK (generation > 0),
    deletion_operation_id UUID NOT NULL CHECK (deletion_operation_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    state TEXT NOT NULL CHECK (state IN ('FROZEN', 'LIVE', 'PURGE_DECIDED')),
    manifest_id UUID NOT NULL CHECK (manifest_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    manifest_sha256 BYTEA NOT NULL CHECK (octet_length(manifest_sha256) = 32),
    manifest_item_count NUMERIC(20,0) NOT NULL CHECK (manifest_item_count >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE voice_space_lifecycle_fence_receipts (
    space_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    deletion_operation_id UUID NOT NULL,
    request_bytes BYTEA NOT NULL CHECK (octet_length(request_bytes) > 0),
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    receipt_id UUID NOT NULL UNIQUE CHECK (receipt_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    applied_state TEXT NOT NULL CHECK (applied_state IN ('FROZEN', 'LIVE', 'PURGE_DECIDED')),
    receipt_bytes BYTEA,
    receipt_sha256 BYTEA,
    applied_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (space_id, generation),
    CHECK ((receipt_bytes IS NULL AND receipt_sha256 IS NULL AND applied_at IS NULL)
        OR (receipt_bytes IS NOT NULL AND octet_length(receipt_bytes) > 0
            AND octet_length(receipt_sha256) = 32 AND applied_at IS NOT NULL))
);

CREATE TABLE voice_space_lifecycle_purge_receipts (
    space_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    deletion_operation_id UUID NOT NULL,
    request_bytes BYTEA NOT NULL CHECK (octet_length(request_bytes) > 0),
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    receipt_id UUID NOT NULL UNIQUE CHECK (receipt_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    receipt_bytes BYTEA,
    receipt_sha256 BYTEA,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (space_id, generation),
    CHECK ((receipt_bytes IS NULL AND receipt_sha256 IS NULL AND completed_at IS NULL)
        OR (receipt_bytes IS NOT NULL AND octet_length(receipt_bytes) > 0
            AND octet_length(receipt_sha256) = 32 AND completed_at IS NOT NULL))
);

CREATE INDEX voice_space_lifecycle_fence_receipts_retention_idx
    ON voice_space_lifecycle_fence_receipts (applied_at) WHERE applied_at IS NOT NULL;
CREATE INDEX voice_space_lifecycle_purge_receipts_retention_idx
    ON voice_space_lifecycle_purge_receipts (completed_at) WHERE completed_at IS NOT NULL;

COMMIT;
