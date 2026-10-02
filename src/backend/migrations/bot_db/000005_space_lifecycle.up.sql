CREATE TABLE bot_space_lifecycle_heads (
    space_id UUID PRIMARY KEY,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    desired_state TEXT NOT NULL CHECK (desired_state IN ('FROZEN', 'LIVE', 'PURGE_DECIDED', 'PURGED')),
    manifest_id UUID NOT NULL,
    manifest_sha256 BYTEA NOT NULL CHECK (octet_length(manifest_sha256) = 32),
    manifest_item_count BIGINT NOT NULL CHECK (manifest_item_count >= 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE bot_space_lifecycle_fence_receipts (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    request_bytes BYTEA NOT NULL,
    receipt_bytes BYTEA NOT NULL,
    applied_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (space_id, generation)
);

CREATE TABLE bot_space_lifecycle_purge_receipts (
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    request_bytes BYTEA NOT NULL,
    receipt_bytes BYTEA NOT NULL,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (space_id, generation)
);

ALTER TABLE bot_event_log ADD COLUMN space_id UUID;
UPDATE bot_event_log e SET space_id = w.space_id
FROM bot_chat_whitelist w
WHERE w.bot_id=e.bot_id AND w.chat_id::text=e.payload->>'chat_id';
CREATE INDEX bot_event_log_space_pending_idx ON bot_event_log(space_id,created_at)
    WHERE space_id IS NOT NULL AND delivery_status IN ('pending','deferred');
