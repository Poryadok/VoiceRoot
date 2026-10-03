ALTER TABLE community_owner_authority
    ADD COLUMN roster_source_revision BIGINT NOT NULL DEFAULT 0 CHECK (roster_source_revision >= 0),
    ADD COLUMN roster_sha256 BYTEA,
    ADD COLUMN roster_lease_expires_at TIMESTAMPTZ,
    ADD CONSTRAINT community_roster_state_complete CHECK (
        (roster_source_revision = 0 AND roster_sha256 IS NULL AND roster_lease_expires_at IS NULL) OR
        (roster_source_revision > 0 AND octet_length(roster_sha256) = 32 AND roster_lease_expires_at IS NOT NULL)
    );

CREATE TABLE community_roster_operations (
    operation_id UUID PRIMARY KEY,
    space_id UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    corporation_key TEXT NOT NULL CHECK (length(corporation_key) BETWEEN 1 AND 256),
    owner_generation BIGINT NOT NULL CHECK (owner_generation > 0),
    source_revision BIGINT NOT NULL CHECK (source_revision > 0),
    snapshot_sha256 BYTEA NOT NULL CHECK (octet_length(snapshot_sha256) = 32),
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
    receipt_id UUID NOT NULL UNIQUE,
    member_count INTEGER NOT NULL CHECK (member_count >= 0),
    profile_ids UUID[] NOT NULL,
    removed_profile_ids UUID[] NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE community_roster_members (
    space_id UUID NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    profile_id UUID NOT NULL,
    source_rank TEXT NOT NULL CHECK (source_rank IN ('member','officer')),
    source_reasons TEXT[] NOT NULL CHECK (source_reasons = ARRAY['membership']::TEXT[] OR source_reasons = ARRAY['membership','officer_rank']::TEXT[]),
    source_revision BIGINT NOT NULL CHECK (source_revision > 0),
    owner_generation BIGINT NOT NULL CHECK (owner_generation > 0),
    lease_expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    PRIMARY KEY (space_id, profile_id)
);

CREATE INDEX community_roster_members_profile_idx ON community_roster_members(profile_id, space_id);
CREATE INDEX community_roster_members_expiry_idx ON community_roster_members(lease_expires_at, space_id);
