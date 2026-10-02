CREATE TABLE chat_space_lifecycle_fences (
    space_id UUID PRIMARY KEY,
    deletion_operation_id UUID,
    generation BIGINT NOT NULL DEFAULT 0 CHECK (generation >= 0),
    state TEXT NOT NULL CHECK (state IN ('LIVE','FROZEN','PURGE_DECIDED','PURGED')),
    schedule_generation BIGINT,
    manifest_id UUID,
    manifest_sha256 BYTEA,
    manifest_item_count BIGINT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK (
      (generation = 0 AND state = 'LIVE' AND deletion_operation_id IS NULL AND schedule_generation IS NULL AND manifest_id IS NULL AND manifest_sha256 IS NULL AND manifest_item_count IS NULL)
      OR
      (generation > 0 AND deletion_operation_id IS NOT NULL AND schedule_generation IS NOT NULL AND schedule_generation > 0 AND manifest_id IS NOT NULL AND manifest_sha256 IS NOT NULL AND octet_length(manifest_sha256) = 32 AND manifest_item_count IS NOT NULL AND manifest_item_count >= 0)
    )
);

CREATE TABLE chat_space_lifecycle_operations (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    operation_kind TEXT NOT NULL CHECK (operation_kind IN ('PREPARE','FENCE','PURGE')),
    request_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    response_bytes BYTEA NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL,
    retain_until TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (space_id, deletion_operation_id, generation, operation_kind)
);

CREATE TABLE chat_space_deletion_manifest_items (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    schedule_generation BIGINT NOT NULL CHECK (schedule_generation > 0),
    page_index BIGINT NOT NULL CHECK (page_index >= 0),
    item_index SMALLINT NOT NULL CHECK (item_index >= 0 AND item_index < 1000),
    chat_id UUID NOT NULL,
    PRIMARY KEY (space_id, deletion_operation_id, schedule_generation, page_index, item_index),
    UNIQUE (space_id, deletion_operation_id, schedule_generation, chat_id)
);

CREATE INDEX chat_space_lifecycle_operations_retention_idx
    ON chat_space_lifecycle_operations (retain_until);
