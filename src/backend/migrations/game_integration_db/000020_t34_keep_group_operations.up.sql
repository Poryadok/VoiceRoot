CREATE TABLE gis_session_keep_group_operations (
    operation_id UUID PRIMARY KEY,
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    session_id UUID NOT NULL REFERENCES gis_sessions(id),
    account_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    roster_revision BIGINT NOT NULL CHECK (roster_revision > 0),
    destination_key TEXT NOT NULL,
    policy_revision BIGINT NOT NULL CHECK (policy_revision > 0),
    target_kind TEXT NOT NULL CHECK (target_kind IN ('new_chat', 'party_chat')),
    chat_id UUID,
    display_name TEXT NOT NULL,
    member_profiles UUID[] NOT NULL,
    create_operation_id UUID NOT NULL,
    create_receipt_id UUID,
    create_request_hash TEXT,
    member_operation_id UUID NOT NULL,
    member_receipt_id UUID,
    member_request_hash TEXT,
    status TEXT NOT NULL DEFAULT 'pending' CHECK (status IN ('pending', 'completed')),
    receipt_bytes JSONB,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK ((target_kind = 'new_chat') OR chat_id IS NOT NULL),
    CHECK ((status = 'pending' AND receipt_bytes IS NULL) OR (status = 'completed' AND receipt_bytes IS NOT NULL))
);

CREATE INDEX gis_session_keep_group_session_idx
    ON gis_session_keep_group_operations (session_id, created_at DESC);
