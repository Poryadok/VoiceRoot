CREATE TABLE matchmaking_voice_roster_inbox (
    event_id UUID PRIMARY KEY,
    payload_sha256 BYTEA NOT NULL CHECK (octet_length(payload_sha256)=32),
    room_id UUID NOT NULL,
    roster_version BIGINT NOT NULL CHECK (roster_version>0),
    profile_id UUID NOT NULL,
    change TEXT NOT NULL CHECK (change IN ('JOINED','LEFT')),
    received_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE(room_id,roster_version)
);
CREATE TABLE matchmaking_voice_roster_contract_conflicts (
    payload_sha256 BYTEA PRIMARY KEY CHECK (octet_length(payload_sha256)=32),
    payload BYTEA NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE matchmaking_voice_roster_cancellation_outbox (
    event_id UUID PRIMARY KEY,
    search_session_id UUID NOT NULL UNIQUE REFERENCES search_sessions(id),
    profile_id UUID NOT NULL,
    occurred_at TIMESTAMPTZ NOT NULL,
    payload BYTEA NOT NULL,
    delivered_at TIMESTAMPTZ,
    lease_token UUID,
    lease_until TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX matchmaking_voice_roster_cancellation_pending ON matchmaking_voice_roster_cancellation_outbox(next_attempt_at)
    WHERE delivered_at IS NULL;
