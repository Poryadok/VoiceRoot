CREATE TABLE messaging_space_lifecycle_fences (
    space_id UUID PRIMARY KEY,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    state TEXT NOT NULL CHECK (state IN ('FROZEN', 'LIVE', 'PURGE_DECIDED', 'PURGED')),
    source_schedule_generation BIGINT NOT NULL CHECK (source_schedule_generation >= 0),
    source_manifest_id UUID NOT NULL,
    source_manifest_sha256 BYTEA NOT NULL CHECK (octet_length(source_manifest_sha256) = 32),
    source_manifest_item_count BIGINT NOT NULL CHECK (source_manifest_item_count >= 0),
    request_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    receipt_bytes BYTEA NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK ((state = 'FROZEN' AND generation = source_schedule_generation) OR
           (state <> 'FROZEN' AND generation = source_schedule_generation + 1))
);

CREATE TABLE messaging_space_purge_receipts (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    purge_generation BIGINT NOT NULL CHECK (purge_generation > 0),
    source_schedule_generation BIGINT NOT NULL CHECK (source_schedule_generation >= 0),
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    receipt_bytes BYTEA NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (space_id, deletion_operation_id, purge_generation),
    CHECK (purge_generation = source_schedule_generation + 1)
);

CREATE INDEX messaging_space_purge_receipts_retention_idx
    ON messaging_space_purge_receipts (completed_at);
