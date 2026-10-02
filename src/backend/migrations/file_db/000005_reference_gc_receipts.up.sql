CREATE TABLE file_reference_gc_receipts (
    caller_service TEXT NOT NULL,
    operation_id UUID NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    response_bytes BYTEA NOT NULL,
    response_sha256 BYTEA NOT NULL CHECK (octet_length(response_sha256) = 32),
    completed_at TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (caller_service, operation_id)
);
