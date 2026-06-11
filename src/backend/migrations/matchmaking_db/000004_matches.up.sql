-- matches and proposals (docs/microservices/matchmaking-service.md)

ALTER TABLE search_sessions DROP CONSTRAINT IF EXISTS search_sessions_status_check;
ALTER TABLE search_sessions ADD CONSTRAINT search_sessions_status_check
    CHECK (status IN ('searching', 'pending_accept', 'matched', 'timeout', 'cancelled'));

DROP INDEX IF EXISTS search_sessions_one_active_per_profile;
CREATE UNIQUE INDEX search_sessions_one_active_per_profile
    ON search_sessions (profile_id)
    WHERE status IN ('searching', 'pending_accept');

CREATE TABLE matches (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    game_id UUID NOT NULL REFERENCES games (id),
    mode TEXT NOT NULL,
    region TEXT NOT NULL,
    participants JSONB NOT NULL DEFAULT '[]'::jsonb,
    voice_room_id TEXT,
    chat_id TEXT,
    status VARCHAR(16) NOT NULL DEFAULT 'pending_accept'
        CHECK (status IN ('pending_accept', 'active', 'completed', 'abandoned')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    completed_at TIMESTAMPTZ
);

CREATE TABLE match_proposals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    match_id UUID NOT NULL REFERENCES matches (id) ON DELETE CASCADE,
    search_session_id UUID NOT NULL REFERENCES search_sessions (id),
    profile_id UUID NOT NULL,
    party_id UUID REFERENCES parties (id),
    response VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (response IN ('pending', 'accepted', 'declined')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (match_id, profile_id)
);

CREATE INDEX matches_status_idx ON matches (status, created_at DESC);
CREATE INDEX match_proposals_match_idx ON match_proposals (match_id);
CREATE INDEX search_sessions_match_id_idx ON search_sessions (match_id) WHERE match_id IS NOT NULL;

ALTER TABLE search_sessions
    ADD CONSTRAINT search_sessions_match_id_fkey
    FOREIGN KEY (match_id) REFERENCES matches (id);
