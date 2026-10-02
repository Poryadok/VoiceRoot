CREATE TABLE community_owner_authority (
    space_id UUID PRIMARY KEY REFERENCES spaces(id) ON DELETE CASCADE,
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    corporation_key TEXT NOT NULL CHECK (length(corporation_key) BETWEEN 1 AND 256),
    owner_account_id UUID NOT NULL,
    owner_profile_id UUID NOT NULL,
    owner_generation BIGINT NOT NULL CHECK (owner_generation > 0),
    status TEXT NOT NULL CHECK (status IN ('active','recovery_pending')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE(application_id,environment_id,corporation_key)
);

CREATE TABLE community_owner_recovery_operations (
    operation_id UUID PRIMARY KEY,
    space_id UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    corporation_key TEXT NOT NULL CHECK (length(corporation_key) BETWEEN 1 AND 256),
    previous_owner_account_id UUID NOT NULL,
    previous_owner_profile_id UUID NOT NULL,
    replacement_account_id UUID NOT NULL,
    replacement_profile_id UUID NOT NULL,
    expected_generation BIGINT NOT NULL CHECK (expected_generation > 0),
    reason_code TEXT NOT NULL CHECK (reason_code IN ('owner_lost','corporation_dissolved')),
    evidence_sha256 BYTEA NOT NULL CHECK (octet_length(evidence_sha256)=32),
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash)=32),
    status TEXT NOT NULL CHECK (status IN ('pending','succeeded')),
    role_receipt BYTEA,
    result_generation BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((status='pending' AND role_receipt IS NULL AND result_generation IS NULL) OR
           (status='succeeded' AND role_receipt IS NOT NULL AND result_generation=expected_generation+1))
);

CREATE UNIQUE INDEX community_owner_recovery_one_pending
    ON community_owner_recovery_operations(space_id) WHERE status='pending';
