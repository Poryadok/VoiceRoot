-- Protected Auth inputs only. None of these rows qualifies paid/CURRENT state.
CREATE TABLE subscription_auth_account_observations (
    account_id UUID PRIMARY KEY,
    generation UUID NOT NULL,
    auth_revision BIGINT NOT NULL CHECK (auth_revision>=0),
    auth_security_revision BIGINT NOT NULL CHECK (auth_security_revision>0),
    auth_session_epoch BIGINT NOT NULL CHECK (auth_session_epoch>0),
    capture_coverage TEXT NOT NULL CHECK (capture_coverage IN ('UNRECORDED','CAPTURED')),
    payload BYTEA NOT NULL,
    payload_hash BYTEA NOT NULL CHECK (octet_length(payload_hash)=32),
    state_hash BYTEA NOT NULL CHECK (octet_length(state_hash)=32),
    observed_at TIMESTAMPTZ NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    state TEXT NOT NULL DEFAULT 'OBSERVED' CHECK (state='OBSERVED'),
    CHECK ((auth_revision=0)=(capture_coverage='UNRECORDED'))
);
CREATE TABLE subscription_auth_lifecycle_facts (
    account_id UUID NOT NULL,
    source_revision BIGINT NOT NULL CHECK (source_revision>0),
    event_id UUID NOT NULL UNIQUE,
    first_generation UUID NOT NULL,
    payload BYTEA NOT NULL,
    payload_hash BYTEA NOT NULL CHECK (octet_length(payload_hash)=32),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (account_id,source_revision)
);
CREATE TABLE subscription_auth_observation_receipts (
    request_id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    generation UUID NOT NULL,
    minimum_revision BIGINT NOT NULL CHECK (minimum_revision>=0),
    auth_revision BIGINT NOT NULL CHECK (auth_revision>=0),
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash)=32),
    snapshot_hash BYTEA NOT NULL CHECK (octet_length(snapshot_hash)=32),
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
REVOKE ALL ON subscription_auth_account_observations,subscription_auth_lifecycle_facts,subscription_auth_observation_receipts FROM PUBLIC;
