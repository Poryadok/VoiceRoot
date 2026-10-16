ALTER TABLE search_sessions
    ADD COLUMN recovery_generation BIGINT NOT NULL DEFAULT 0 CHECK (recovery_generation >= 0);

CREATE TABLE matchmaking_match_recovery_effects (
    match_id UUID NOT NULL REFERENCES matches(id),
    search_session_id UUID NOT NULL REFERENCES search_sessions(id),
    generation BIGINT NOT NULL CHECK (generation > 0),
    action TEXT NOT NULL CHECK (action IN ('enqueue', 'release')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    applied_at TIMESTAMPTZ,
    PRIMARY KEY (search_session_id, generation)
);

CREATE INDEX matchmaking_match_recovery_effects_pending_idx
    ON matchmaking_match_recovery_effects (created_at, match_id, search_session_id)
    WHERE applied_at IS NULL;
