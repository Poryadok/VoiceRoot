CREATE TABLE chat_deleted_event_outbox (
    event_id UUID PRIMARY KEY,
    chat_id UUID NOT NULL,
    space_id UUID NOT NULL,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    manifest_id UUID NOT NULL,
    manifest_sha256 BYTEA NOT NULL CHECK (octet_length(manifest_sha256) = 32),
    event_bytes BYTEA NOT NULL CHECK (octet_length(event_bytes) > 0),
    occurred_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    published_at TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    attempt_count BIGINT NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    lease_token UUID,
    lease_until TIMESTAMPTZ,
    last_error_class TEXT,
    UNIQUE (space_id, deletion_operation_id, generation, chat_id),
    CHECK ((lease_token IS NULL) = (lease_until IS NULL)),
    CHECK (published_at IS NULL OR (lease_token IS NULL AND lease_until IS NULL))
);

CREATE INDEX chat_deleted_event_outbox_pending_idx
    ON chat_deleted_event_outbox (next_attempt_at, created_at, event_id)
    WHERE published_at IS NULL;

CREATE INDEX chat_deleted_event_outbox_published_idx
    ON chat_deleted_event_outbox (published_at, event_id)
    WHERE published_at IS NOT NULL;
