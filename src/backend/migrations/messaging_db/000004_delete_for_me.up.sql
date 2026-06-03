CREATE TABLE IF NOT EXISTS message_hides (
    message_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    hidden_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, profile_id)
);

CREATE INDEX IF NOT EXISTS message_hides_profile_id_idx
    ON message_hides (profile_id, hidden_at DESC);
