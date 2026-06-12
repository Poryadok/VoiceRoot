-- search nudge tracking (docs/features/matchmaking.md flow step 5)

ALTER TABLE search_sessions ADD COLUMN IF NOT EXISTS nudged_at TIMESTAMPTZ;

CREATE INDEX IF NOT EXISTS search_sessions_nudge_idx
    ON search_sessions (created_at)
    WHERE status = 'searching' AND nudged_at IS NULL;

CREATE INDEX IF NOT EXISTS search_sessions_expire_idx
    ON search_sessions (timeout_at)
    WHERE status = 'searching';
