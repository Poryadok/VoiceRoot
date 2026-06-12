-- Index for paginated match history by participant profile (docs/features/matchmaking.md)

CREATE INDEX IF NOT EXISTS matches_history_status_created_idx
    ON matches (status, created_at DESC, id DESC)
    WHERE status IN ('active', 'completed');

CREATE INDEX IF NOT EXISTS matches_participants_gin_idx
    ON matches USING gin (participants jsonb_path_ops);
