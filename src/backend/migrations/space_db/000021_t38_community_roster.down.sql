DROP TABLE IF EXISTS community_roster_members;
DROP TABLE IF EXISTS community_roster_operations;
ALTER TABLE community_owner_authority
    DROP CONSTRAINT IF EXISTS community_roster_state_complete,
    DROP COLUMN IF EXISTS roster_lease_expires_at,
    DROP COLUMN IF EXISTS roster_sha256,
    DROP COLUMN IF EXISTS roster_source_revision;
