CREATE TABLE friend_accept_outbox (
    friendship_id UUID PRIMARY KEY REFERENCES friendships(id) ON DELETE CASCADE,
    requester_profile_id UUID NOT NULL,
    target_profile_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ
);

CREATE INDEX friend_accept_outbox_pending_idx
    ON friend_accept_outbox (created_at, friendship_id)
    WHERE delivered_at IS NULL;
