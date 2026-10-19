CREATE TABLE bot_chat_scope_observations (
    observation_id UUID PRIMARY KEY,
    bot_id UUID NOT NULL,
    chat_id UUID NOT NULL,
    space_id UUID NOT NULL,
    intent_operation_id UUID,
    origin_kind TEXT NOT NULL CHECK (origin_kind IN ('install_admission','whitelist_admission','enable_admission','cleanup_reconciliation')),
    request_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256)=32),
    response_bytes BYTEA NOT NULL,
    response_sha256 BYTEA NOT NULL CHECK (octet_length(response_sha256)=32),
    scope_incarnation UUID NOT NULL,
    authority_revision BIGINT NOT NULL CHECK (authority_revision>0),
    lifecycle_generation BIGINT NOT NULL CHECK (lifecycle_generation>=0),
    source_exists BOOLEAN NOT NULL,
    source_live BOOLEAN NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX bot_chat_scope_observations_target_idx ON bot_chat_scope_observations(bot_id,space_id,chat_id);
ALTER TABLE bot_installation_intents ADD COLUMN chat_scope_observation_ids JSONB NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(chat_scope_observation_ids)='array');
ALTER TABLE bot_chat_whitelist ADD COLUMN scope_observation_id UUID REFERENCES bot_chat_scope_observations(observation_id);
ALTER TABLE bot_chat_whitelist ADD COLUMN origin_scope_observation_id UUID REFERENCES bot_chat_scope_observations(observation_id);
-- Retained identity deliberately has no FK: deleting a local installation must
-- not erase the historical source needed by durable cleanup.
ALTER TABLE bot_chat_whitelist ADD COLUMN source_installation_id UUID;
