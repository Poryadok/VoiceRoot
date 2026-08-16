-- LFP story → party (docs/features/matchmaking.md Social Discovery; roadmap П.3)

CREATE TABLE lfp_listings (
    story_id UUID PRIMARY KEY,
    author_profile_id UUID NOT NULL,
    criteria_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    inactive_at TIMESTAMPTZ
);

CREATE INDEX lfp_listings_author_idx ON lfp_listings (author_profile_id)
    WHERE inactive_at IS NULL;

CREATE TABLE lfp_requests (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    story_id UUID NOT NULL REFERENCES lfp_listings (story_id),
    author_profile_id UUID NOT NULL,
    responder_profile_id UUID NOT NULL,
    response_type VARCHAR(16) NOT NULL
        CHECK (response_type IN ('JOIN', 'INVITE')),
    status VARCHAR(16) NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'accepted', 'declined', 'expired')),
    party_id UUID REFERENCES parties (id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at TIMESTAMPTZ,
    UNIQUE (story_id, responder_profile_id, response_type)
);

CREATE INDEX lfp_requests_author_pending_idx ON lfp_requests (author_profile_id, status)
    WHERE status = 'pending';
