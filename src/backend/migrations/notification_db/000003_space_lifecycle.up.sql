CREATE TABLE notification_space_lifecycle_fences (
    space_id UUID PRIMARY KEY,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    state TEXT NOT NULL CHECK (state IN ('FROZEN', 'LIVE', 'PURGE_DECIDED', 'PURGED')),
    manifest_id TEXT NOT NULL,
    manifest_sha256 BYTEA NOT NULL CHECK (octet_length(manifest_sha256) = 32),
    manifest_item_count BIGINT NOT NULL CHECK (manifest_item_count >= 0),
    updated_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE notification_space_lifecycle_fence_receipts (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    desired_state SMALLINT NOT NULL CHECK (desired_state IN (1, 2, 3)),
    request_bytes BYTEA NOT NULL,
    receipt_bytes BYTEA NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL,
    retain_until TIMESTAMPTZ,
    PRIMARY KEY (space_id, deletion_operation_id, generation, desired_state)
);

CREATE TABLE notification_space_lifecycle_purge_receipts (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    request_bytes BYTEA NOT NULL,
    receipt_bytes BYTEA NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL,
    retain_until TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (space_id, deletion_operation_id, generation)
);
