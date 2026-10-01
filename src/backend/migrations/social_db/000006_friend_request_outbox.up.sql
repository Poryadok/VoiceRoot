CREATE TABLE friend_request_outbox (
    friendship_id UUID PRIMARY KEY REFERENCES friendships(id) ON DELETE CASCADE,
    event_id UUID NOT NULL,
    requester_profile_id UUID NOT NULL,
    target_profile_id UUID NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    delivered_at TIMESTAMPTZ,
    cancelled_at TIMESTAMPTZ
);

CREATE INDEX friend_request_outbox_pending_idx
    ON friend_request_outbox (created_at, friendship_id)
    WHERE delivered_at IS NULL AND cancelled_at IS NULL;
