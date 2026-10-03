-- Immutable signed revision and retry receipts for game-authored messages.
CREATE TABLE game_message_revisions (
    chat_id UUID NOT NULL,
    message_id UUID NOT NULL,
    revision BIGINT NOT NULL CHECK (revision > 0),
    operation TEXT NOT NULL CHECK (operation IN ('create', 'edit', 'delete', 'moderator_delete')),
    operation_id UUID NOT NULL,
    previous_revision_hash CHAR(64),
    compact_jws TEXT NOT NULL,
    content_sha256 CHAR(64) NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (chat_id, message_id, revision),
    UNIQUE (operation_id),
    CHECK ((revision = 1 AND operation = 'create' AND previous_revision_hash IS NULL)
        OR (revision > 1 AND operation IN ('edit', 'delete', 'moderator_delete') AND previous_revision_hash IS NOT NULL)),
    CHECK ((operation IN ('delete', 'moderator_delete')) OR content_sha256 ~ '^[0-9a-f]{64}$')
);

CREATE TABLE game_message_operation_receipts (
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    operation_id UUID NOT NULL,
    compact_jws TEXT NOT NULL,
    chat_id UUID NOT NULL,
    message_id UUID NOT NULL,
    result_content TEXT NOT NULL,
    result_deleted BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (application_id, environment_id, operation_id)
);

CREATE INDEX game_message_revisions_message_idx
    ON game_message_revisions (chat_id, message_id, revision DESC);

CREATE TABLE game_message_device_authorities (
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    device_id UUID NOT NULL,
    authority_revision BIGINT NOT NULL CHECK (authority_revision > 0),
    key_id UUID NOT NULL,
    status TEXT NOT NULL CHECK (status = 'active'),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (application_id, environment_id, device_id)
);

-- Internal moderation actions have their own immutable idempotency record;
-- they do not reuse a user device's operation receipt or actor identity.
CREATE TABLE game_message_tombstone_actions (
    action_id UUID PRIMARY KEY,
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    chat_id UUID NOT NULL,
    message_id UUID NOT NULL,
    reason_class TEXT NOT NULL CHECK (reason_class IN ('moderation', 'system_retention')),
    revision BIGINT NOT NULL CHECK (revision > 1),
    compact_jws TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
