CREATE TABLE message_game_action_results (
    message_id UUID NOT NULL REFERENCES message_game_cards(message_id) ON DELETE CASCADE,
    action_id TEXT NOT NULL,
    operation_id UUID NOT NULL UNIQUE,
    result_id UUID NOT NULL UNIQUE,
    state_version TEXT NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('succeeded', 'failed')),
    safe_summary TEXT NOT NULL,
    recorded_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (message_id, action_id),
    CHECK (length(action_id) BETWEEN 1 AND 128),
    CHECK (length(state_version) BETWEEN 1 AND 256),
    CHECK (length(safe_summary) <= 512)
);

CREATE INDEX message_game_action_results_operation_idx
    ON message_game_action_results (operation_id);
