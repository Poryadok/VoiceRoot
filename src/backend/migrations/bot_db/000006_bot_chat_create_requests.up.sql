CREATE TABLE bot_chat_create_requests (
    bot_id UUID NOT NULL REFERENCES bots(id) ON DELETE CASCADE,
    request_id UUID NOT NULL,
    request_hash TEXT NOT NULL CHECK (request_hash ~ '^sha256:[0-9a-f]{64}$'),
    day DATE NOT NULL,
    active_attempts INTEGER NOT NULL DEFAULT 1 CHECK (active_attempts >= 0),
    retain_reservation BOOLEAN NOT NULL DEFAULT false,
    quota_reserved BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (bot_id, request_id),
    CHECK (NOT retain_reservation OR quota_reserved),
    CHECK (quota_reserved OR (active_attempts = 0 AND NOT retain_reservation))
);
