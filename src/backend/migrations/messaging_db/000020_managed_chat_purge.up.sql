CREATE TABLE managed_chat_purge_operations (
    operation_id UUID PRIMARY KEY,
    chat_id UUID NOT NULL,
    purge_after TIMESTAMPTZ NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    state TEXT NOT NULL CHECK (state IN ('PENDING', 'COMPLETED')),
    message_count BIGINT NOT NULL DEFAULT 0 CHECK (message_count >= 0),
    file_reference_count BIGINT NOT NULL DEFAULT 0 CHECK (file_reference_count >= 0),
    file_receipt_sha256 BYTEA NULL CHECK (file_receipt_sha256 IS NULL OR octet_length(file_receipt_sha256) = 32),
    search_receipt_sha256 BYTEA NULL CHECK (search_receipt_sha256 IS NULL OR octet_length(search_receipt_sha256) = 32),
    completed_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK ((state = 'PENDING' AND completed_at IS NULL) OR (state = 'COMPLETED' AND completed_at IS NOT NULL))
);

CREATE INDEX managed_chat_purge_pending_idx
    ON managed_chat_purge_operations (created_at, operation_id)
    WHERE state = 'PENDING';

CREATE TABLE managed_chat_purge_messages (
    operation_id UUID NOT NULL REFERENCES managed_chat_purge_operations(operation_id) ON DELETE CASCADE,
    message_id UUID NOT NULL,
    attachments JSONB NOT NULL CHECK (jsonb_typeof(attachments) = 'array'),
    PRIMARY KEY (operation_id, message_id)
);
