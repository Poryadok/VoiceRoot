DROP TABLE app_quota_windows;
ALTER TABLE registry_operations
    DROP COLUMN result_updated_at,
    DROP COLUMN result_status;
ALTER TABLE applications DROP COLUMN suspended_from_status;
DROP INDEX registry_audit_quota_denial_window_idx;
ALTER TABLE registry_audit
    DROP COLUMN denial_count,
    DROP COLUMN quota_window_start,
    DROP COLUMN installation_id,
    DROP COLUMN reason_code,
    DROP COLUMN result,
    DROP COLUMN source;
DROP TABLE installations;
