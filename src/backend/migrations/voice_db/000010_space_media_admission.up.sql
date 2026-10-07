BEGIN;

ALTER TABLE voice_account_voice_fences
    ADD COLUMN admission_operation_id UUID,
    ADD COLUMN admission_generation TEXT,
    ADD CONSTRAINT voice_account_voice_fences_admission_binding_check
        CHECK ((admission_operation_id IS NULL AND admission_generation IS NULL)
            OR (admission_operation_id IS NOT NULL AND admission_generation IS NOT NULL
                AND admission_generation <> ''));

CREATE TABLE voice_space_media_admissions (
    operation_id UUID PRIMARY KEY CHECK (operation_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    generation TEXT NOT NULL CHECK (generation <> ''),
    account_id UUID NOT NULL CHECK (account_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    profile_id UUID NOT NULL CHECK (profile_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    space_id UUID NOT NULL CHECK (space_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    room_id TEXT NOT NULL CHECK (room_id <> ''),
    voice_room_id UUID NOT NULL CHECK (voice_room_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    room_generation BIGINT NOT NULL CHECK (room_generation > 0),
    livekit_identity TEXT NOT NULL CHECK (livekit_identity <> ''),
    created_room BOOLEAN NOT NULL DEFAULT false,
    call_started_at TIMESTAMPTZ NOT NULL,
    max_participants INTEGER NOT NULL CHECK (max_participants > 0),
    session_epoch BIGINT NOT NULL CHECK (session_epoch > 0),
    access_epoch BIGINT NOT NULL CHECK (access_epoch > 0),
    policy_epoch BIGINT NOT NULL CHECK (policy_epoch > 0),
    can_join BOOLEAN NOT NULL CHECK (can_join),
    can_publish_audio BOOLEAN NOT NULL,
    can_subscribe BOOLEAN NOT NULL CHECK (can_subscribe),
    state TEXT NOT NULL CHECK (state IN ('PREPARED','FENCED','COMMITTED','ABORTING')),
    participant_state TEXT NOT NULL DEFAULT 'PREPARED'
        CHECK (participant_state IN ('PREPARED','ACTIVE','REVOKING','ENDED')),
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    projection_applied_at TIMESTAMPTZ,
    cleanup_completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (voice_room_id, room_generation, profile_id, generation),
    CHECK (state <> 'COMMITTED' OR cleanup_completed_at IS NULL),
    CHECK (state <> 'ABORTING' OR projection_applied_at IS NULL)
);

-- A durable, reopenable claim for the one current Voice call incarnation of
-- a canonical Space voice_room_id. ENDED rows remain as generation history.
CREATE TABLE voice_space_media_room_heads (
    voice_room_id UUID NOT NULL CHECK (voice_room_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    room_generation BIGINT NOT NULL CHECK (room_generation > 0),
    space_id UUID NOT NULL CHECK (space_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    call_room_id TEXT NOT NULL CHECK (call_room_id <> ''),
    creator_operation_id UUID NOT NULL CHECK (creator_operation_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    state TEXT NOT NULL CHECK (state IN ('OPEN','CLOSING','ENDED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (voice_room_id, room_generation),
    UNIQUE (call_room_id),
    UNIQUE (creator_operation_id)
);

CREATE UNIQUE INDEX voice_space_media_room_heads_one_open_idx
    ON voice_space_media_room_heads (voice_room_id) WHERE state IN ('OPEN','CLOSING');
CREATE INDEX voice_space_media_room_heads_recovery_idx
    ON voice_space_media_room_heads (state, updated_at);

CREATE INDEX voice_space_media_admissions_recovery_idx
    ON voice_space_media_admissions (state, updated_at)
    WHERE cleanup_completed_at IS NULL OR state = 'COMMITTED';
CREATE INDEX voice_space_media_admissions_account_idx
    ON voice_space_media_admissions (account_id, created_at DESC);

CREATE TABLE voice_space_media_admission_outbox (
    event_id UUID PRIMARY KEY CHECK (event_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    operation_id UUID NOT NULL REFERENCES voice_space_media_admissions(operation_id) ON DELETE RESTRICT,
    voice_room_id UUID NOT NULL,
    room_generation BIGINT NOT NULL CHECK (room_generation > 0),
    subject TEXT NOT NULL CHECK (subject IN ('voice.call_started','voice.member_joined')),
    payload BYTEA NOT NULL CHECK (octet_length(payload) > 0),
    delivered_at TIMESTAMPTZ,
    lease_token UUID,
    lease_until TIMESTAMPTZ,
    attempts BIGINT NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK ((lease_token IS NULL AND lease_until IS NULL) OR (lease_token IS NOT NULL AND lease_until IS NOT NULL)),
    CHECK (delivered_at IS NULL OR (lease_token IS NULL AND lease_until IS NULL))
);

CREATE INDEX voice_space_media_admission_outbox_ready_idx
    ON voice_space_media_admission_outbox (created_at, event_id)
    WHERE delivered_at IS NULL;

CREATE UNIQUE INDEX voice_space_media_one_started_event_per_room_generation_idx
    ON voice_space_media_admission_outbox (voice_room_id, room_generation)
    WHERE subject = 'voice.call_started';

COMMIT;
