CREATE TABLE game_event_nonce_replays (
    credential_id UUID NOT NULL REFERENCES service_credentials(id),
    nonce UUID NOT NULL,
    event_id UUID NOT NULL,
    payload_hash BYTEA NOT NULL CHECK (octet_length(payload_hash) = 32),
    retain_until TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (credential_id, nonce)
);

CREATE INDEX game_event_nonce_replay_expiry_idx ON game_event_nonce_replays (retain_until);

CREATE TABLE game_event_operations (
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    event_id UUID NOT NULL,
    operation_id UUID NOT NULL UNIQUE,
    application_owner_account_id UUID NOT NULL,
    installation_id UUID NOT NULL REFERENCES installations(id),
    bot_id UUID NOT NULL,
    chat_id UUID NOT NULL,
    client_message_id UUID NOT NULL,
    payload_hash BYTEA NOT NULL CHECK (octet_length(payload_hash) = 32),
    canonical_payload BYTEA NOT NULL CHECK (octet_length(canonical_payload) BETWEEN 1 AND 16384),
    authority_revision BIGINT NOT NULL CHECK (authority_revision > 0),
    status TEXT NOT NULL CHECK (status IN ('queued','processing','succeeded','rejected','expired','cancelled','reconciliation_required')),
    message_id UUID,
    reason_code TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (application_id, environment_id, event_id),
    FOREIGN KEY (application_id, environment_id) REFERENCES environments(application_id, id),
    CHECK ((status = 'succeeded') = (message_id IS NOT NULL))
);

CREATE INDEX game_event_operations_scope_operation_idx
    ON game_event_operations (application_id, environment_id, operation_id);

CREATE TABLE game_event_publication_outbox (
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    event_id UUID NOT NULL,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    lease_token UUID,
    lease_expires_at TIMESTAMPTZ,
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (application_id, environment_id, event_id),
    FOREIGN KEY (application_id, environment_id, event_id)
        REFERENCES game_event_operations(application_id, environment_id, event_id),
    CHECK ((lease_token IS NULL) = (lease_expires_at IS NULL))
);

CREATE INDEX game_event_publication_claim_idx
    ON game_event_publication_outbox (next_attempt_at, created_at)
    WHERE published_at IS NULL;
