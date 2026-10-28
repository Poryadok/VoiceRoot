-- New terminal obligations must never enter the legacy recovery stream: old
-- consumers acknowledge its unsupported matched-session releases as obsolete.
-- Capture only the routing needed for exact-owner Redis cleanup. No parent FK:
-- parent purge or binary rollback must not cascade away pending cleanup.
CREATE TABLE matchmaking_terminal_search_releases (
    search_session_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    profile_id UUID NOT NULL,
    space_id UUID,
    game_id UUID NOT NULL,
    mode TEXT NOT NULL,
    region TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (search_session_id, generation)
);

CREATE INDEX matchmaking_terminal_search_releases_pending_idx
    ON matchmaking_terminal_search_releases (created_at, search_session_id, generation);
