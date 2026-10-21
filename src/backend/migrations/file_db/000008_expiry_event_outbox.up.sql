CREATE TABLE file_expiry_event_outbox (
    event_id UUID PRIMARY KEY,
    file_id UUID NOT NULL UNIQUE,
    payload BYTEA NOT NULL CHECK (octet_length(payload) BETWEEN 1 AND 8192),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    published_at TIMESTAMPTZ NULL
);
CREATE INDEX file_expiry_event_pending ON file_expiry_event_outbox(created_at, event_id)
    WHERE published_at IS NULL;
