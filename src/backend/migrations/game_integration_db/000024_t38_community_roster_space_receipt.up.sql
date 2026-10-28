ALTER TABLE gis_community_roster_snapshots
    ADD COLUMN space_projection_status TEXT NOT NULL DEFAULT 'pending'
        CHECK (space_projection_status IN ('pending','applied')),
    ADD COLUMN space_receipt_id UUID,
    ADD COLUMN space_request_hash BYTEA,
    ADD CONSTRAINT gis_community_roster_space_receipt_consistent CHECK (
        (space_projection_status='pending' AND space_receipt_id IS NULL AND space_request_hash IS NULL) OR
        (space_projection_status='applied' AND space_receipt_id IS NOT NULL AND octet_length(space_request_hash)=32)
    );

CREATE INDEX gis_community_roster_projection_pending_idx
    ON gis_community_roster_snapshots(created_at,application_id,environment_id,corporation_key)
    WHERE outcome='accepted' AND space_projection_status='pending';
