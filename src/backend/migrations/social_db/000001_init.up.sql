-- social_db v1 — docs/microservices/social-service.md
CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE TABLE friendships (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    requester_profile_id UUID NOT NULL,
    target_profile_id UUID NOT NULL,
    status VARCHAR(16) NOT NULL CHECK (status IN ('pending', 'accepted', 'declined')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX friendships_pair_uq ON friendships (requester_profile_id, target_profile_id);
CREATE INDEX friendships_target_status_idx ON friendships (target_profile_id, status, created_at DESC);
CREATE INDEX friendships_requester_status_idx ON friendships (requester_profile_id, status, created_at DESC);

CREATE TABLE blocks (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    blocker_account_id UUID NOT NULL,
    blocked_account_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE UNIQUE INDEX blocks_pair_uq ON blocks (blocker_account_id, blocked_account_id);
CREATE INDEX blocks_blocked_account_idx ON blocks (blocked_account_id);
