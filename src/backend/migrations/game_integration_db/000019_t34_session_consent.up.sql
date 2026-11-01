CREATE TABLE gis_session_consent_receipts (
    operation_id UUID PRIMARY KEY,
    receipt_id UUID NOT NULL UNIQUE,
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash)=32),
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    session_id UUID NOT NULL REFERENCES gis_sessions(id),
    account_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    roster_revision BIGINT NOT NULL CHECK (roster_revision > 0),
    destination_key TEXT NOT NULL CHECK (length(destination_key) BETWEEN 1 AND 256),
    policy_revision BIGINT NOT NULL CHECK (policy_revision > 0),
    receipt_bytes BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (session_id, roster_revision, profile_id)
);

CREATE INDEX gis_session_consent_current
    ON gis_session_consent_receipts(session_id, roster_revision, profile_id);
