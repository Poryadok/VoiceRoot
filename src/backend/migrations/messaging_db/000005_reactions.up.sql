CREATE TABLE IF NOT EXISTS reactions (
    message_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    emoji TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (message_id, profile_id, emoji)
);

CREATE INDEX IF NOT EXISTS reactions_message_id_emoji_idx ON reactions (message_id, emoji);
