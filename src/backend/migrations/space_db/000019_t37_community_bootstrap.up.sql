CREATE TABLE community_bootstrap_operations (
    operation_id UUID PRIMARY KEY,
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    owner_account_id UUID NOT NULL,
    owner_profile_id UUID NOT NULL,
    corporation_key TEXT NOT NULL CHECK (length(corporation_key) BETWEEN 1 AND 256),
    template_id TEXT NOT NULL CHECK (length(template_id) BETWEEN 1 AND 128),
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
    status TEXT NOT NULL CHECK (status IN ('pending', 'succeeded')),
    space_id UUID UNIQUE REFERENCES spaces(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((status = 'pending' AND space_id IS NULL) OR (status = 'succeeded' AND space_id IS NOT NULL)),
    UNIQUE (application_id, environment_id, corporation_key)
);

CREATE INDEX community_bootstrap_owner_idx ON community_bootstrap_operations(owner_profile_id, created_at DESC);
