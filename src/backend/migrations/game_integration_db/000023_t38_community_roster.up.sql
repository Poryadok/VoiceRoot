ALTER TABLE gis_community_owner_authority
    ADD COLUMN roster_source_revision BIGINT NOT NULL DEFAULT 0 CHECK (roster_source_revision >= 0),
    ADD COLUMN roster_sha256 BYTEA,
    ADD COLUMN roster_lease_expires_at TIMESTAMPTZ,
    ADD CONSTRAINT gis_community_roster_state_consistent CHECK (
        (roster_source_revision = 0 AND roster_sha256 IS NULL AND roster_lease_expires_at IS NULL)
        OR (roster_source_revision > 0 AND octet_length(roster_sha256) = 32 AND roster_lease_expires_at IS NOT NULL)
    );

CREATE TABLE gis_community_roster_snapshots (
    operation_id UUID PRIMARY KEY,
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    corporation_key TEXT NOT NULL,
    source_revision BIGINT NOT NULL CHECK (source_revision > 0),
    owner_generation BIGINT NOT NULL CHECK (owner_generation > 0),
    snapshot_sha256 BYTEA NOT NULL CHECK (octet_length(snapshot_sha256) = 32),
    page_count INTEGER NOT NULL CHECK (page_count BETWEEN 1 AND 1000),
    member_count INTEGER NOT NULL CHECK (member_count >= 0),
    outcome TEXT NOT NULL CHECK (outcome IN ('accepted', 'stale')),
    lease_expires_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    UNIQUE (application_id, environment_id, corporation_key, owner_generation, source_revision),
    FOREIGN KEY (application_id, environment_id, corporation_key)
        REFERENCES gis_community_owner_authority(application_id, environment_id, corporation_key),
    CHECK ((outcome = 'accepted' AND lease_expires_at IS NOT NULL)
        OR (outcome = 'stale' AND lease_expires_at IS NULL))
);

CREATE TABLE gis_community_roster_pages (
    operation_id UUID NOT NULL REFERENCES gis_community_roster_snapshots(operation_id) ON DELETE CASCADE,
    page_index INTEGER NOT NULL CHECK (page_index >= 0),
    page_sha256 BYTEA NOT NULL CHECK (octet_length(page_sha256) = 32),
    canonical_page_bytes BYTEA NOT NULL,
    PRIMARY KEY (operation_id, page_index)
);

CREATE TABLE gis_community_grants (
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    corporation_key TEXT NOT NULL,
    binding_id UUID NOT NULL,
    character_key TEXT NOT NULL CHECK (length(character_key) BETWEEN 1 AND 256),
    source_reason TEXT NOT NULL CHECK (source_reason IN ('membership', 'officer_rank')),
    source_rank TEXT NOT NULL CHECK (source_rank IN ('member', 'officer')),
    source_membership_revision BIGINT NOT NULL CHECK (source_membership_revision > 0),
    source_revision BIGINT NOT NULL CHECK (source_revision > 0),
    owner_generation BIGINT NOT NULL CHECK (owner_generation > 0),
    lease_expires_at TIMESTAMPTZ NOT NULL,
    revoked_at TIMESTAMPTZ,
    revocation_reason TEXT CHECK (revocation_reason IS NULL OR revocation_reason IN (
        'snapshot_replaced', 'source_expired', 'binding_revoked', 'owner_generation_changed')),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (application_id, environment_id, corporation_key, binding_id, character_key, source_reason, owner_generation),
    FOREIGN KEY (application_id, environment_id, corporation_key)
        REFERENCES gis_community_owner_authority(application_id, environment_id, corporation_key),
    FOREIGN KEY (application_id, environment_id, binding_id)
        REFERENCES player_bindings(application_id, environment_id, binding_id),
    CHECK ((revoked_at IS NULL) = (revocation_reason IS NULL))
);

CREATE INDEX gis_community_grants_effective_idx
    ON gis_community_grants(application_id, environment_id, corporation_key, binding_id, owner_generation)
    WHERE revoked_at IS NULL;

CREATE INDEX gis_community_grants_expiry_idx
    ON gis_community_grants(lease_expires_at, application_id, environment_id)
    WHERE revoked_at IS NULL;
