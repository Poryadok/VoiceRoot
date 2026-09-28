CREATE TABLE game_session_grant_sessions (
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    session_id UUID NOT NULL,
    voice_room_id UUID,
    roster_revision BIGINT NOT NULL DEFAULT 0 CHECK (roster_revision >= 0),
    profile_set_sha256 BYTEA NOT NULL CHECK (octet_length(profile_set_sha256) = 32),
    status TEXT NOT NULL CHECK (status IN ('active', 'revoked')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (application_id, environment_id, session_id),
    CHECK (status <> 'active' OR voice_room_id IS NOT NULL)
);

CREATE TABLE game_session_grants (
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    session_id UUID NOT NULL,
    voice_room_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    permission TEXT NOT NULL DEFAULT 'VOICE_JOIN' CHECK (permission = 'VOICE_JOIN'),
    roster_revision BIGINT NOT NULL CHECK (roster_revision > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (application_id, environment_id, session_id, profile_id),
    FOREIGN KEY (application_id, environment_id, session_id)
        REFERENCES game_session_grant_sessions(application_id, environment_id, session_id)
        ON DELETE RESTRICT
);

CREATE INDEX game_session_grants_voice_lookup_idx
    ON game_session_grants(application_id, environment_id, session_id, voice_room_id, profile_id);

CREATE TABLE game_session_grant_operations (
    operation_id UUID PRIMARY KEY,
    operation_kind TEXT NOT NULL CHECK (operation_kind IN ('apply', 'revoke')),
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    session_id UUID NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    receipt_id UUID NOT NULL UNIQUE,
    roster_revision BIGINT NOT NULL CHECK (roster_revision >= 0),
    applied_profile_set_sha256 BYTEA NOT NULL CHECK (octet_length(applied_profile_set_sha256) = 32),
    outcome TEXT NOT NULL CHECK (outcome IN ('APPLIED', 'REPLAYED', 'STALE', 'REVOKED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX game_session_grant_operations_session_idx
    ON game_session_grant_operations(application_id, environment_id, session_id, created_at);
