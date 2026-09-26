CREATE TABLE applications (
    id UUID PRIMARY KEY,
    owner_account_id UUID NOT NULL,
    name TEXT NOT NULL CHECK (char_length(name) BETWEEN 1 AND 128),
    game_id UUID,
    status TEXT NOT NULL CHECK (status IN ('draft', 'sandbox', 'active', 'suspended', 'retired')),
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX applications_owner_idx ON applications (owner_account_id, created_at);

CREATE TABLE environments (
    id UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES applications(id),
    kind TEXT NOT NULL CHECK (kind IN ('sandbox', 'production')),
    status TEXT NOT NULL CHECK (status IN ('pending', 'active', 'suspended', 'retired')),
    provider_policy JSONB NOT NULL DEFAULT '{}'::jsonb,
    redirect_uris JSONB NOT NULL DEFAULT '[]'::jsonb,
    allowed_origins JSONB NOT NULL DEFAULT '[]'::jsonb,
    revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (application_id, kind)
);

CREATE TABLE service_credentials (
    id UUID PRIMARY KEY,
    environment_id UUID NOT NULL REFERENCES environments(id),
    secret_digest BYTEA NOT NULL CHECK (length(secret_digest) = 32),
    scopes TEXT[] NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    UNIQUE (environment_id, generation)
);

CREATE INDEX service_credentials_environment_idx
    ON service_credentials (environment_id, expires_at)
    WHERE revoked_at IS NULL;

CREATE TABLE registry_operations (
    actor_kind TEXT NOT NULL,
    actor_id UUID NOT NULL,
    route TEXT NOT NULL,
    idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 128),
    request_hash BYTEA NOT NULL CHECK (length(request_hash) = 32),
    result_id UUID,
    result_revision BIGINT,
    status TEXT NOT NULL CHECK (status IN ('pending', 'succeeded', 'failed')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (actor_kind, actor_id, route, idempotency_key)
);

CREATE TABLE registry_audit (
    id UUID PRIMARY KEY,
    actor_kind TEXT NOT NULL,
    actor_id UUID NOT NULL,
    application_id UUID NOT NULL REFERENCES applications(id),
    environment_id UUID,
    action TEXT NOT NULL,
    previous_status TEXT,
    new_status TEXT NOT NULL,
    operation_key TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX registry_audit_application_idx ON registry_audit (application_id, created_at);
