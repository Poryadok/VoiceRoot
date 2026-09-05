CREATE TABLE account_deletion_operations (
    operation_id UUID PRIMARY KEY,
    account_id UUID NOT NULL REFERENCES accounts(id) ON DELETE RESTRICT,
    session_epoch BIGINT NOT NULL CHECK (session_epoch > 0),
    restore_token_hash VARCHAR(128) NOT NULL,
    state VARCHAR(32) NOT NULL CHECK (state IN ('PENDING_FLOOR', 'PENDING_EVENT', 'COMPLETED')),
    attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL,
    locked_until TIMESTAMPTZ,
    last_error_code VARCHAR(128),
    floor_recorded_at TIMESTAMPTZ,
    event_published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL,
    CONSTRAINT account_deletion_operations_account_epoch_key UNIQUE (account_id, session_epoch),
    CONSTRAINT account_deletion_operations_markers_check CHECK (
        (state = 'PENDING_FLOOR' AND floor_recorded_at IS NULL AND event_published_at IS NULL)
        OR (state = 'PENDING_EVENT' AND floor_recorded_at IS NOT NULL AND event_published_at IS NULL)
        OR (state = 'COMPLETED' AND floor_recorded_at IS NOT NULL AND event_published_at IS NOT NULL)
    )
);
