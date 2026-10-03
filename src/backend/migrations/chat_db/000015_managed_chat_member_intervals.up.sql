CREATE TABLE managed_chat_member_intervals (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    chat_id UUID NOT NULL REFERENCES chats(id) ON DELETE CASCADE,
    profile_id UUID NOT NULL,
    joined_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    CHECK (revoked_at IS NULL OR revoked_at > joined_at)
);

CREATE UNIQUE INDEX managed_chat_member_intervals_active_uq
    ON managed_chat_member_intervals (chat_id, profile_id)
    WHERE revoked_at IS NULL;
CREATE INDEX managed_chat_member_intervals_history_idx
    ON managed_chat_member_intervals (chat_id, profile_id, joined_at DESC, revoked_at);
