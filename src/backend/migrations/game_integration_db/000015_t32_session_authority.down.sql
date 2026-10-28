DROP TABLE IF EXISTS gis_session_roster_updates;
DELETE FROM gis_session_owner_receipts r USING gis_session_operations o
WHERE r.application_id=o.application_id AND r.environment_id=o.environment_id
    AND r.operation_id=o.operation_id AND o.operation_kind='roster';
DELETE FROM gis_session_operations WHERE operation_kind='roster';
DROP INDEX IF EXISTS gis_sessions_roster_lease_due;
ALTER TABLE gis_sessions
    DROP COLUMN IF EXISTS closed_at,
    DROP COLUMN IF EXISTS roster_receipt_id,
    DROP COLUMN IF EXISTS lease_expires_at,
    DROP COLUMN IF EXISTS roster_hash,
    DROP COLUMN IF EXISTS host_profile_id;
ALTER TABLE gis_session_operations
    DROP COLUMN error_outcome;
ALTER TABLE gis_session_operations
    DROP CONSTRAINT IF EXISTS gis_session_operations_operation_kind_check,
    ADD CONSTRAINT gis_session_operations_operation_kind_check
        CHECK (operation_kind IN ('create','close','failure'));
