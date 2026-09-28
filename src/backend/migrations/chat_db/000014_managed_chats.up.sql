-- T31: managed application chats remain Chat-owned and have no human owner.
ALTER TABLE chats
    ALTER COLUMN creator_profile_id DROP NOT NULL,
    ADD COLUMN managed_by_application_id UUID NULL,
    ADD COLUMN managed_environment_id UUID NULL,
    ADD COLUMN external_chat_key TEXT NULL,
    ADD CONSTRAINT chats_managed_ownership_check CHECK (
        (managed_by_application_id IS NULL AND managed_environment_id IS NULL AND external_chat_key IS NULL AND creator_profile_id IS NOT NULL)
        OR
        (managed_by_application_id IS NOT NULL AND managed_environment_id IS NOT NULL AND external_chat_key IS NOT NULL AND creator_profile_id IS NULL AND type = 'group' AND space_id IS NULL)
    );

CREATE UNIQUE INDEX chats_managed_resource_key_uq
    ON chats (managed_by_application_id, managed_environment_id, external_chat_key)
    WHERE managed_by_application_id IS NOT NULL;

CREATE TABLE managed_chat_operations (
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    operation_id UUID NOT NULL,
    method TEXT NOT NULL CHECK (method IN ('create', 'sync_members')),
    request_hash TEXT NOT NULL CHECK (request_hash ~ '^sha256:[0-9a-f]{64}$'),
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE RESTRICT,
    receipt JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (application_id, environment_id, operation_id)
);

CREATE INDEX managed_chat_operations_chat_idx ON managed_chat_operations (chat_id, created_at DESC);
