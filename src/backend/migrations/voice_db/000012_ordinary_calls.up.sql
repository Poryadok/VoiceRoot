BEGIN;

-- NULL preserves unknown provenance of older pending conversion receipts.
-- Never backfill an empty source or rewrite an existing operation's snapshot.
ALTER TABLE voice_sdk_conversion_fence_receipts ADD COLUMN ordinary_source_snapshot JSONB;

-- This is the ordinary DM/group authority, not activation of the R22/Space
-- coordinator. Immutable IDs and compact terminal rows are never reused.
CREATE TABLE voice_ordinary_rooms (
    room_id UUID PRIMARY KEY CHECK (room_id <> '00000000-0000-0000-0000-000000000000'),
    operation_id UUID NOT NULL UNIQUE,
    chat_id UUID NOT NULL,
    is_group BOOLEAN NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('PREPARING','RINGING','ACTIVE','ABORTING','CLOSING','CLOSED')),
    version BIGINT NOT NULL CHECK (version > 0),
    roster_version BIGINT NOT NULL DEFAULT 0 CHECK (roster_version >= 0),
    document JSONB NOT NULL CHECK (jsonb_typeof(document) = 'object'),
    projection_version BIGINT NOT NULL DEFAULT 0 CHECK (projection_version >= 0 AND projection_version <= version),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    lease_token UUID,
    lease_until TIMESTAMPTZ,
    recovery_attempts BIGINT NOT NULL DEFAULT 0 CHECK (recovery_attempts >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK ((lease_token IS NULL) = (lease_until IS NULL)),
    CHECK ((document->'intent'->>'room_id') IS NOT DISTINCT FROM room_id::text),
    CHECK ((document->'intent'->>'operation_id') IS NOT DISTINCT FROM operation_id::text),
    CHECK ((document->'intent'->>'chat_id') IS NOT DISTINCT FROM chat_id::text),
    CHECK ((document->'intent'->>'group')::boolean IS NOT DISTINCT FROM is_group),
    CHECK ((document->>'stage') IS NOT DISTINCT FROM state),
    CHECK ((document->>'version')::bigint IS NOT DISTINCT FROM version),
    CHECK ((document->>'roster_version')::bigint IS NOT DISTINCT FROM roster_version)
);

CREATE UNIQUE INDEX voice_ordinary_one_live_group_chat
    ON voice_ordinary_rooms(chat_id) WHERE is_group AND state <> 'CLOSED';
CREATE INDEX voice_ordinary_recovery_idx ON voice_ordinary_rooms(next_attempt_at,room_id);

-- Same-transaction lookup index over the native document. Account exclusivity
-- remains solely in voice_account_voice_fences; this table grants no ownership.
CREATE TABLE voice_ordinary_memberships (
    room_id UUID NOT NULL REFERENCES voice_ordinary_rooms(room_id) ON DELETE RESTRICT,
    profile_id UUID NOT NULL,
    account_id UUID NOT NULL,
    membership_generation UUID NOT NULL,
    fence_operation_id UUID NOT NULL,
    media_identity TEXT NOT NULL,
    session_epoch BIGINT NOT NULL DEFAULT 0 CHECK (session_epoch >= 0),
    state TEXT NOT NULL CHECK (state IN ('PENDING','RESERVED','JOINED','LEAVING','LEFT','ABORTED')),
    PRIMARY KEY(room_id,profile_id)
);
CREATE INDEX voice_ordinary_membership_account_idx ON voice_ordinary_memberships(account_id)
    WHERE state IN ('PENDING','RESERVED','JOINED','LEAVING');
CREATE INDEX voice_ordinary_membership_profile_idx ON voice_ordinary_memberships(profile_id)
    WHERE state IN ('PENDING','RESERVED','JOINED','LEAVING');

CREATE TABLE voice_ordinary_outbox (
    event_id UUID PRIMARY KEY,
    room_id UUID NOT NULL REFERENCES voice_ordinary_rooms(room_id) ON DELETE RESTRICT,
    room_version BIGINT NOT NULL CHECK (room_version > 0),
    roster_version BIGINT CHECK (roster_version > 0),
    ordinal BIGINT NOT NULL CHECK (ordinal > 0),
    subject TEXT NOT NULL CHECK (subject IN (
      'voice.call_incoming','voice.call_accepted','voice.call_declined','voice.call_missed',
      'voice.call_ended','voice.call_started','voice.participant_joined','voice.participant_left')),
    payload BYTEA NOT NULL CHECK (octet_length(payload) > 0),
    delivered_at TIMESTAMPTZ,
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    lease_token UUID,
    lease_until TIMESTAMPTZ,
    attempts BIGINT NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE(room_id,ordinal),
    CHECK ((lease_token IS NULL) = (lease_until IS NULL)),
    CHECK (delivered_at IS NULL OR lease_token IS NULL)
);
CREATE INDEX voice_ordinary_outbox_ready_idx
    ON voice_ordinary_outbox(next_attempt_at,room_id,ordinal) WHERE delivered_at IS NULL;
CREATE UNIQUE INDEX voice_ordinary_outbox_roster_version_idx
    ON voice_ordinary_outbox(room_id,roster_version) WHERE roster_version IS NOT NULL;

COMMIT;
