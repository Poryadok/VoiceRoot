-- Durable v2 decisions must survive ordinary Role data cleanup.
CREATE TABLE role_space_lifecycle (
    space_id UUID PRIMARY KEY,
    retired_at TIMESTAMPTZ NULL
);

CREATE TABLE ownership_transfer_v2 (
    operation_id UUID PRIMARY KEY,
    space_id UUID NOT NULL,
    protocol_version INTEGER NOT NULL CHECK (protocol_version = 2),
    old_owner_profile_id UUID NOT NULL,
    new_owner_profile_id UUID NOT NULL,
    intent_bytes BYTEA NOT NULL CHECK (octet_length(intent_bytes) > 0),
    intent_hash BYTEA NOT NULL CHECK (octet_length(intent_hash) = 32),
    state TEXT NOT NULL CHECK (state IN ('prepared', 'finalized', 'aborted')),
    prepare_request_hash BYTEA NULL CHECK (octet_length(prepare_request_hash) = 32),
    finalize_request_hash BYTEA NULL CHECK (octet_length(finalize_request_hash) = 32),
    abort_request_hash BYTEA NULL CHECK (octet_length(abort_request_hash) = 32),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (old_owner_profile_id <> new_owner_profile_id),
    CHECK (
        (state = 'prepared' AND prepare_request_hash IS NOT NULL AND finalize_request_hash IS NULL AND abort_request_hash IS NULL)
        OR (state = 'finalized' AND prepare_request_hash IS NOT NULL AND finalize_request_hash IS NOT NULL AND abort_request_hash IS NULL)
        OR (state = 'aborted' AND abort_request_hash IS NOT NULL AND finalize_request_hash IS NULL)
    )
);

CREATE UNIQUE INDEX ownership_transfer_v2_active_space_idx
    ON ownership_transfer_v2 (space_id) WHERE state = 'prepared';
