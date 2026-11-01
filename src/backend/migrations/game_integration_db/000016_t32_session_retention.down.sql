DROP TABLE IF EXISTS gis_session_external_key_tombstones;
DROP TRIGGER IF EXISTS gis_session_operation_terminal_clock ON gis_session_operations;
DROP FUNCTION IF EXISTS gis_session_operation_terminal_clock();
ALTER TABLE gis_session_operations DROP COLUMN IF EXISTS terminalized_at;
DROP INDEX IF EXISTS gis_sessions_retention_due;
ALTER TABLE gis_sessions DROP COLUMN IF EXISTS history_expires_at;
ALTER TABLE gis_sessions DROP COLUMN IF EXISTS terminalized_at;
