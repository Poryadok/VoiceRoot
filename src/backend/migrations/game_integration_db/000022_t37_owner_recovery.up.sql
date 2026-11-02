CREATE TABLE gis_community_owner_authority (
    application_id UUID NOT NULL REFERENCES applications(id),
    environment_id UUID NOT NULL REFERENCES environments(id),
    corporation_key TEXT NOT NULL CHECK (length(corporation_key) BETWEEN 1 AND 256),
    owner_account_id UUID NOT NULL,
    owner_profile_id UUID NOT NULL,
    owner_generation BIGINT NOT NULL CHECK (owner_generation > 0),
    status TEXT NOT NULL CHECK (status IN ('pending', 'active', 'recovery_pending')),
    space_id UUID,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (application_id, environment_id, corporation_key),
    CHECK ((status = 'pending' AND space_id IS NULL) OR (status IN ('active','recovery_pending') AND space_id IS NOT NULL))
);

CREATE TABLE gis_community_owner_recovery_operations (
    operation_id UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES applications(id),
    environment_id UUID NOT NULL REFERENCES environments(id),
    corporation_key TEXT NOT NULL CHECK (length(corporation_key) BETWEEN 1 AND 256),
    previous_owner_account_id UUID NOT NULL,
    previous_owner_profile_id UUID NOT NULL,
    replacement_account_id UUID NOT NULL,
    replacement_profile_id UUID NOT NULL,
    expected_generation BIGINT NOT NULL CHECK (expected_generation > 0),
    reason_code TEXT NOT NULL CHECK (reason_code IN ('owner_lost', 'corporation_dissolved')),
    evidence_sha256 BYTEA NOT NULL CHECK (octet_length(evidence_sha256) = 32),
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'succeeded', 'cancelled')),
    operator_account_id UUID,
    approved_at TIMESTAMPTZ,
    approval_expires_at TIMESTAMPTZ,
    cancelled_by_account_id UUID,
    cancelled_at TIMESTAMPTZ,
    space_id UUID,
    result_generation BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((status = 'pending' AND operator_account_id IS NULL AND approved_at IS NULL AND approval_expires_at IS NULL AND space_id IS NULL AND result_generation IS NULL)
        OR (status = 'approved' AND operator_account_id IS NOT NULL AND approved_at IS NOT NULL AND approval_expires_at IS NOT NULL AND space_id IS NULL AND result_generation IS NULL)
        OR (status = 'succeeded' AND operator_account_id IS NOT NULL AND approved_at IS NOT NULL AND approval_expires_at IS NOT NULL AND cancelled_by_account_id IS NULL AND cancelled_at IS NULL AND space_id IS NOT NULL AND result_generation = expected_generation + 1)
        OR (status = 'cancelled' AND cancelled_by_account_id IS NOT NULL AND cancelled_at IS NOT NULL AND space_id IS NULL AND result_generation IS NULL))
);

CREATE INDEX gis_community_owner_recovery_current
    ON gis_community_owner_recovery_operations(application_id,environment_id,corporation_key,expected_generation,status);

CREATE UNIQUE INDEX gis_community_owner_recovery_one_open
    ON gis_community_owner_recovery_operations(application_id,environment_id,corporation_key)
    WHERE status IN ('pending','approved');
