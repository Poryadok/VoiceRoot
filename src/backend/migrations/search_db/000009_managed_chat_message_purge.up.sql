CREATE TABLE search_managed_chat_purge_operations (
    operation_id UUID PRIMARY KEY,
    chat_id UUID NOT NULL,
    message_ids_sha256 BYTEA NOT NULL CHECK (octet_length(message_ids_sha256) = 32),
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    deleted_count BIGINT NOT NULL CHECK (deleted_count >= 0),
    receipt_id UUID NOT NULL UNIQUE,
    completed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE TABLE search_managed_chat_message_purge_fences (
    message_id UUID PRIMARY KEY,
    chat_id UUID NOT NULL,
    operation_id UUID NOT NULL REFERENCES search_managed_chat_purge_operations(operation_id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX search_managed_chat_message_purge_operation_idx
    ON search_managed_chat_message_purge_fences(operation_id, message_id);
