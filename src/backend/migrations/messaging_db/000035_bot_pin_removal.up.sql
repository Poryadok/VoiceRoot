CREATE TABLE bot_pin_removal_decisions (
    decision_id UUID PRIMARY KEY,
    decision_sha256 BYTEA NOT NULL CHECK (octet_length(decision_sha256)=32),
    purpose TEXT NOT NULL CHECK (purpose IN ('global_delete','space_uninstall')),
    bot_id UUID NOT NULL,
    actor_profile_id UUID NOT NULL,
    scope_space_id UUID,
    CHECK ((purpose='space_uninstall')=(scope_space_id IS NOT NULL))
);

CREATE TABLE bot_pin_actor_bindings (
    actor_profile_id UUID PRIMARY KEY,
    bot_id UUID NOT NULL
);

CREATE TABLE bot_pin_removal_revisions (
    chat_id UUID NOT NULL,
    actor_profile_id UUID NOT NULL,
    revision BIGINT NOT NULL CHECK (revision>0),
    PRIMARY KEY (chat_id,actor_profile_id)
);

CREATE TABLE bot_pin_removal_receipts (
    operation_id UUID PRIMARY KEY,
    decision_id UUID NOT NULL REFERENCES bot_pin_removal_decisions(decision_id),
    bot_id UUID NOT NULL,
    actor_profile_id UUID NOT NULL,
    space_id UUID NOT NULL,
    chat_id UUID NOT NULL,
    request_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256)=32),
    response_bytes BYTEA NOT NULL,
    response_sha256 BYTEA NOT NULL CHECK (octet_length(response_sha256)=32),
    receipt_id UUID NOT NULL UNIQUE,
    completed_at TIMESTAMPTZ NOT NULL,
    cleanup_revision BIGINT NOT NULL CHECK (cleanup_revision>0)
);
