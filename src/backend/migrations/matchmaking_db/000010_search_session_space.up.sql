-- Space-scoped matchmaking sessions (docs/features/matchmaking.md §внутри спейса).

ALTER TABLE search_sessions
    ADD COLUMN IF NOT EXISTS space_id UUID;

CREATE INDEX IF NOT EXISTS search_sessions_space_searching_idx
    ON search_sessions (space_id)
    WHERE status = 'searching' AND space_id IS NOT NULL;
