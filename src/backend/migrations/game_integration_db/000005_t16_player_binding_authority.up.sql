CREATE TABLE player_bindings (
    binding_id UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES applications(id),
    environment_id UUID NOT NULL REFERENCES environments(id),
    provider TEXT NOT NULL CHECK (provider ~ '^[a-z][a-z0-9_-]{0,31}$'),
    provider_subject_digest TEXT NOT NULL CHECK (provider_subject_digest ~ '^hmac-sha256-v1:[A-Za-z0-9_-]{1,32}:[0-9a-f]{64}$'),
    account_id UUID NOT NULL,
    actor_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    device_id UUID NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending', 'active', 'revoking', 'revoked')),
    authority_revision BIGINT NOT NULL CHECK (authority_revision > 0),
    permitted_character_context JSONB NOT NULL DEFAULT '[]'::jsonb,
    last_revocation_id UUID,
    last_revocation_expected_revision BIGINT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK ((last_revocation_id IS NULL) = (last_revocation_expected_revision IS NULL)),
    CHECK (last_revocation_expected_revision IS NULL OR last_revocation_expected_revision > 0)
);

CREATE INDEX player_bindings_account_idx ON player_bindings (account_id, application_id, environment_id);
CREATE INDEX player_bindings_profile_idx ON player_bindings (profile_id, application_id, environment_id);
CREATE UNIQUE INDEX player_bindings_provider_subject_idx
    ON player_bindings (application_id, environment_id, provider, provider_subject_digest);
