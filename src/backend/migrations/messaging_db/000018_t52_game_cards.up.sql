CREATE TABLE message_game_cards (
    message_id UUID PRIMARY KEY REFERENCES messages(id) ON DELETE CASCADE,
    app_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    installation_id UUID NOT NULL,
    bot_id UUID NOT NULL,
    character_binding_id UUID,
    card_json JSONB NOT NULL CHECK (jsonb_typeof(card_json) = 'object'),
    card_sha256 CHAR(64) NOT NULL CHECK (card_sha256 ~ '^[0-9a-f]{64}$'),
    actions_enabled BOOLEAN NOT NULL DEFAULT FALSE CHECK (actions_enabled = FALSE),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX message_game_cards_app_installation_idx
    ON message_game_cards (app_id, installation_id, message_id);
