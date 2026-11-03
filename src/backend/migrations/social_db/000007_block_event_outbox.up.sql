-- Retain revocation independently of block deletion and caller lifetime.
CREATE TABLE block_event_outbox (
    event_id UUID PRIMARY KEY,
    payload BYTEA NOT NULL CHECK (octet_length(payload) > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ NULL
);

CREATE INDEX block_event_outbox_pending_idx
    ON block_event_outbox (created_at, event_id)
    WHERE delivered_at IS NULL;
