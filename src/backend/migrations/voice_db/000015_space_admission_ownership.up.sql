-- Ownership starts before the first durable room-head claim, so this table
-- intentionally has no FK to an admission that may not exist yet.
CREATE TABLE voice_space_media_admission_owners (
    operation_id UUID PRIMARY KEY,
    space_id UUID NOT NULL,
    voice_room_id UUID NOT NULL,
    owner_token UUID NOT NULL,
    owner_epoch BIGINT NOT NULL CHECK (owner_epoch > 0),
    dispatch_fenced_epoch BIGINT NOT NULL DEFAULT 0 CHECK (dispatch_fenced_epoch >= 0),
    lease_until TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX voice_space_media_admission_owners_recovery
    ON voice_space_media_admission_owners (lease_until, operation_id);

-- Never assign invented ownership to historical rows. The application refuses
-- native serving while any unfinished/live row or OPEN head lacks its owner.
-- Operators must fence every old writer and explicitly reconcile such rows.
