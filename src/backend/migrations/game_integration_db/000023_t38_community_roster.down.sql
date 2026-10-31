DROP TABLE IF EXISTS gis_community_grants;
DROP TABLE IF EXISTS gis_community_roster_pages;
DROP TABLE IF EXISTS gis_community_roster_snapshots;
ALTER TABLE gis_community_owner_authority
    DROP CONSTRAINT IF EXISTS gis_community_roster_state_consistent,
    DROP COLUMN IF EXISTS roster_source_revision,
    DROP COLUMN IF EXISTS roster_sha256,
    DROP COLUMN IF EXISTS roster_lease_expires_at;
