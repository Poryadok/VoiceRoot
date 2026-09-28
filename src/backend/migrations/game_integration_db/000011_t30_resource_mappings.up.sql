CREATE UNIQUE INDEX environments_scope_id_idx ON environments (application_id, id);
CREATE UNIQUE INDEX player_bindings_scope_id_idx ON player_bindings (application_id, environment_id, binding_id);

CREATE TABLE game_resource_mappings (
    mapping_id UUID PRIMARY KEY,
    application_id UUID NOT NULL REFERENCES applications(id),
    environment_id UUID NOT NULL,
    external_key TEXT NOT NULL CHECK (length(external_key) BETWEEN 1 AND 512),
    resource_kind TEXT NOT NULL CHECK (resource_kind IN ('chat', 'voice', 'space')),
    resource_id UUID NOT NULL,
    chat_id UUID,
    chat_operation_id UUID,
    chat_request_hash TEXT,
    status TEXT NOT NULL CHECK (status IN ('active', 'retired', 'tombstoned')),
    mapping_revision BIGINT NOT NULL CHECK (mapping_revision > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (application_id, environment_id, external_key),
    FOREIGN KEY (application_id, environment_id) REFERENCES environments(application_id, id),
    CHECK ((chat_id IS NULL) = (chat_operation_id IS NULL)),
    CHECK ((chat_id IS NULL) = (chat_request_hash IS NULL)),
    CHECK ((resource_kind = 'space') = (chat_id IS NULL)),
    CHECK (chat_request_hash IS NULL OR chat_request_hash ~ '^sha256:[0-9a-f]{64}$'),
    CHECK (resource_kind <> 'chat' OR resource_id = chat_id)
);

CREATE INDEX game_resource_mappings_chat_idx
    ON game_resource_mappings (application_id, environment_id, chat_id)
    WHERE status = 'active';

CREATE TABLE game_resource_operations (
    application_id UUID NOT NULL REFERENCES applications(id),
    environment_id UUID NOT NULL,
    operation_id UUID NOT NULL,
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
    receipt JSONB NOT NULL CHECK (jsonb_typeof(receipt) = 'object'),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (application_id, environment_id, operation_id),
    FOREIGN KEY (application_id, environment_id) REFERENCES environments(application_id, id)
);

CREATE TABLE game_resource_binding_chats (
    application_id UUID NOT NULL REFERENCES applications(id),
    environment_id UUID NOT NULL,
    binding_id UUID NOT NULL,
    chat_id UUID NOT NULL,
    roster_revision BIGINT NOT NULL CHECK (roster_revision > 0),
    status TEXT NOT NULL CHECK (status IN ('active', 'revoked')),
    lease_expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (application_id, environment_id, binding_id, chat_id),
    FOREIGN KEY (application_id, environment_id) REFERENCES environments(application_id, id),
    FOREIGN KEY (application_id, environment_id, binding_id)
        REFERENCES player_bindings(application_id, environment_id, binding_id)
);

CREATE INDEX game_resource_binding_chats_live_idx
    ON game_resource_binding_chats (application_id, environment_id, binding_id, chat_id, lease_expires_at)
    WHERE status = 'active';
