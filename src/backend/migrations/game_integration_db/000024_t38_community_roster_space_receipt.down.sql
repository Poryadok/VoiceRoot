DROP INDEX IF EXISTS gis_community_roster_projection_pending_idx;
ALTER TABLE gis_community_roster_snapshots
    DROP CONSTRAINT IF EXISTS gis_community_roster_space_receipt_consistent,
    DROP COLUMN IF EXISTS space_request_hash,
    DROP COLUMN IF EXISTS space_receipt_id,
    DROP COLUMN IF EXISTS space_projection_status;
