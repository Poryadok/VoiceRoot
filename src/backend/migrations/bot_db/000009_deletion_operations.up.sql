CREATE TABLE bot_deletion_operations (
    operation_id UUID PRIMARY KEY,
    bot_id UUID NOT NULL UNIQUE REFERENCES bots(id),
    owner_account_id UUID NOT NULL,
    actor_profile_id UUID NOT NULL,
    decision_bytes BYTEA NOT NULL,
    decision_sha256 BYTEA NOT NULL CHECK (octet_length(decision_sha256)=32),
    state TEXT NOT NULL CHECK (state IN ('PENDING','RUNNING','RETRY_WAIT','BLOCKED','COMPLETE')),
    claim_generation BIGINT NOT NULL DEFAULT 0 CHECK (claim_generation>=0),
    claimed_until TIMESTAMPTZ,
    attempts BIGINT NOT NULL DEFAULT 0 CHECK (attempts>=0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_error TEXT NOT NULL DEFAULT '' CHECK (octet_length(last_error)<=512),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ,
    CHECK ((state='COMPLETE')=(completed_at IS NOT NULL))
);

CREATE TABLE bot_deletion_installations (
    target_id UUID PRIMARY KEY,
    operation_id UUID NOT NULL REFERENCES bot_deletion_operations(operation_id),
    bot_id UUID NOT NULL REFERENCES bots(id),
    space_id UUID NOT NULL,
    actor_profile_id UUID NOT NULL,
    installation_id UUID,
    install_operation_id UUID,
    bot_generation BIGINT NOT NULL CHECK (bot_generation>=0),
    cancel_operation_id UUID NOT NULL UNIQUE,
    chat_ids JSONB NOT NULL CHECK (jsonb_typeof(chat_ids)='array'),
    legacy_evidence BOOLEAN NOT NULL,
    scope_bytes BYTEA NOT NULL,
    scope_sha256 BYTEA NOT NULL CHECK (octet_length(scope_sha256)=32),
    UNIQUE (operation_id,space_id),
    CHECK (legacy_evidence OR (installation_id IS NOT NULL AND install_operation_id IS NOT NULL AND bot_generation>0))
);

CREATE TABLE bot_deletion_effects (
    effect_id UUID PRIMARY KEY,
    target_id UUID NOT NULL REFERENCES bot_deletion_installations(target_id),
    kind TEXT NOT NULL CHECK (kind IN ('SPACE_CLEANUP','ROLE_REMOVE','MESSAGING_UNPIN')),
    chat_id UUID,
    request_bytes BYTEA,
    request_sha256 BYTEA CHECK (octet_length(request_sha256)=32),
    receipt_bytes BYTEA,
    receipt_sha256 BYTEA CHECK (octet_length(receipt_sha256)=32),
    state TEXT NOT NULL DEFAULT 'PENDING' CHECK (state IN ('PENDING','COMPLETE')),
    CHECK ((request_bytes IS NULL)=(request_sha256 IS NULL)),
    CHECK ((receipt_bytes IS NULL)=(receipt_sha256 IS NULL)),
    CHECK (state<>'COMPLETE' OR (request_bytes IS NOT NULL AND receipt_bytes IS NOT NULL)),
    CHECK ((kind='MESSAGING_UNPIN')=(chat_id IS NOT NULL))
);
CREATE UNIQUE INDEX bot_deletion_effect_target_kind_idx ON bot_deletion_effects(target_id,kind) WHERE chat_id IS NULL;
CREATE UNIQUE INDEX bot_deletion_effect_target_chat_idx ON bot_deletion_effects(target_id,kind,chat_id) WHERE chat_id IS NOT NULL;
CREATE INDEX bot_deletion_operations_due_idx ON bot_deletion_operations(next_attempt_at) WHERE state<>'COMPLETE';
