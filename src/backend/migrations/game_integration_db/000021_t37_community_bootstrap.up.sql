CREATE TABLE gis_community_bootstrap_operations (
    operation_id UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES applications(id),
    environment_id UUID NOT NULL REFERENCES environments(id),
    owner_account_id UUID NOT NULL,
    owner_profile_id UUID NOT NULL,
    corporation_key TEXT NOT NULL CHECK (length(corporation_key) BETWEEN 1 AND 256),
    template_id TEXT NOT NULL CHECK (length(template_id) BETWEEN 1 AND 128),
    owner_generation BIGINT NOT NULL DEFAULT 1 CHECK (owner_generation > 0),
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
    status TEXT NOT NULL CHECK (status IN ('pending', 'approved', 'succeeded')),
    operator_account_id UUID,
    approved_at TIMESTAMPTZ,
    approval_expires_at TIMESTAMPTZ,
    space_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((status = 'pending' AND operator_account_id IS NULL AND approved_at IS NULL AND approval_expires_at IS NULL AND space_id IS NULL)
        OR (status = 'approved' AND operator_account_id IS NOT NULL AND approved_at IS NOT NULL AND approval_expires_at IS NOT NULL AND space_id IS NULL)
        OR (status = 'succeeded' AND operator_account_id IS NOT NULL AND approved_at IS NOT NULL AND approval_expires_at IS NOT NULL AND space_id IS NOT NULL)),
    UNIQUE (application_id, environment_id, corporation_key)
);

CREATE INDEX gis_community_bootstrap_owner_idx
    ON gis_community_bootstrap_operations(owner_account_id, created_at DESC);

CREATE INDEX gis_community_bootstrap_approval_expiry_idx
    ON gis_community_bootstrap_operations(approval_expires_at)
    WHERE status = 'approved';
