-- search queue sessions (docs/microservices/matchmaking-service.md)

CREATE TABLE parties (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    leader_profile_id UUID NOT NULL,
    member_profile_ids JSONB NOT NULL DEFAULT '[]'::jsonb,
    game_id UUID NOT NULL REFERENCES games (id),
    mode TEXT NOT NULL,
    criteria JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    disbanded_at TIMESTAMPTZ
);

CREATE TABLE search_sessions (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    profile_id UUID NOT NULL,
    party_id UUID REFERENCES parties (id),
    game_id UUID NOT NULL REFERENCES games (id),
    mode TEXT NOT NULL,
    criteria JSONB NOT NULL,
    status VARCHAR(16) NOT NULL DEFAULT 'searching'
        CHECK (status IN ('searching', 'matched', 'timeout', 'cancelled')),
    timeout_at TIMESTAMPTZ,
    matched_at TIMESTAMPTZ,
    match_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX search_sessions_profile_status_idx ON search_sessions (profile_id, status);
CREATE INDEX search_sessions_queue_idx ON search_sessions (game_id, mode, status, created_at);
CREATE UNIQUE INDEX search_sessions_one_active_per_profile
    ON search_sessions (profile_id)
    WHERE status = 'searching';
