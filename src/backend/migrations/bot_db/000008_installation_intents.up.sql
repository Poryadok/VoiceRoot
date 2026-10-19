CREATE TABLE bot_installation_intents (
    installation_id UUID PRIMARY KEY,
    operation_id UUID NOT NULL UNIQUE,
    bot_id UUID NOT NULL REFERENCES bots(id),
    space_id UUID NOT NULL,
    actor_profile_id UUID NOT NULL,
    installed_by_profile_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    state TEXT NOT NULL CHECK (state IN ('PREPARING','INSTALLED','CANCELING','CANCELED')),
    chat_ids JSONB NOT NULL CHECK (jsonb_typeof(chat_ids) = 'array'),
    cancel_operation_id UUID UNIQUE,
    legacy_evidence BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (bot_id, space_id, generation),
    CHECK ((state IN ('CANCELING','CANCELED')) = (cancel_operation_id IS NOT NULL))
);
CREATE UNIQUE INDEX bot_installation_intents_active_idx
ON bot_installation_intents(bot_id,space_id) WHERE state <> 'CANCELED';
