-- T36: one durable Voice session owner per authenticated account, independent
-- of the selected profile. The mapping is sourced from User and immutable here.
CREATE TABLE voice_profile_account_mappings (
    profile_id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    mapped_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX voice_profile_account_mappings_account_idx
    ON voice_profile_account_mappings (account_id, profile_id);

CREATE TABLE voice_account_voice_fences (
    account_id UUID PRIMARY KEY,
    profile_id UUID NOT NULL REFERENCES voice_profile_account_mappings(profile_id) ON DELETE RESTRICT,
    room_id TEXT NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('reserving', 'active')),
    reservation_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK ((state = 'reserving' AND reservation_expires_at IS NOT NULL) OR
           (state = 'active' AND reservation_expires_at IS NULL)),
    UNIQUE (room_id, profile_id)
);

CREATE INDEX voice_account_voice_fences_room_idx
    ON voice_account_voice_fences (room_id);
