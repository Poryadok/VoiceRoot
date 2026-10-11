CREATE TABLE chat_create_requests (
    creator_profile_id UUID NOT NULL,
    request_id UUID NOT NULL,
    request_hash TEXT NOT NULL CHECK (request_hash ~ '^sha256:[0-9a-f]{64}$'),
    chat_id UUID NULL,
    result_bytes BYTEA NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (creator_profile_id, request_id),
    CHECK ((chat_id IS NULL) = (result_bytes IS NULL))
);

CREATE INDEX chat_create_requests_chat_id_idx ON chat_create_requests (chat_id);
