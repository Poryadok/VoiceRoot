CREATE TABLE messaging_attachment_send_intents (
    operation_id UUID PRIMARY KEY,
    message_id UUID NOT NULL UNIQUE,
    chat_id UUID NOT NULL,
    scope_space_id UUID,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256)=32),
    references_bytes BYTEA NOT NULL,
    state TEXT NOT NULL DEFAULT 'PENDING' CHECK (state IN ('PENDING','COMMITTED','RELEASED')),
    expires_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()+interval '15 minutes',
    completed_at TIMESTAMPTZ
);
CREATE INDEX messaging_attachment_send_intents_pending_idx
    ON messaging_attachment_send_intents(expires_at) WHERE state='PENDING';
CREATE INDEX messaging_attachment_send_intents_space_idx
    ON messaging_attachment_send_intents(scope_space_id) WHERE state='PENDING';
