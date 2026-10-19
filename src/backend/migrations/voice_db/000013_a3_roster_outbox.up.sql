-- A3 party eligibility has its own version, separate from media/head generations.
ALTER TABLE voice_space_media_room_heads
    ADD COLUMN roster_version BIGINT NOT NULL DEFAULT 0 CHECK (roster_version >= 0);

-- Both native writers append here in their membership transaction. Compatibility
-- participant events keep their existing outboxes and distinct event IDs.
CREATE TABLE voice_roster_outbox (
    event_id UUID PRIMARY KEY,
    room_id UUID NOT NULL,
    roster_version BIGINT NOT NULL CHECK (roster_version > 0),
    profile_id UUID NOT NULL,
    change TEXT NOT NULL CHECK (change IN ('JOINED', 'LEFT')),
    occurred_at TIMESTAMPTZ NOT NULL,
    payload BYTEA NOT NULL,
    delivered_at TIMESTAMPTZ,
    lease_token UUID,
    lease_until TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (room_id, roster_version)
);
CREATE INDEX voice_roster_outbox_pending ON voice_roster_outbox (next_attempt_at, room_id, roster_version)
    WHERE delivered_at IS NULL;
