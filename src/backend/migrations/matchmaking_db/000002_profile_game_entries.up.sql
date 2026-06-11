CREATE TABLE profile_game_entries (
    profile_id UUID NOT NULL,
    game_id UUID NOT NULL REFERENCES games(id),
    region TEXT NOT NULL,
    role TEXT,
    rank TEXT,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (profile_id, game_id)
);

CREATE INDEX profile_game_entries_profile_updated_idx
    ON profile_game_entries (profile_id, updated_at DESC);
