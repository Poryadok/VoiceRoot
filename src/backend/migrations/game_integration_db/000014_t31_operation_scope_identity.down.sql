DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM gis_session_operations
        GROUP BY operation_id
        HAVING count(*)>1
    ) THEN
        RAISE EXCEPTION 'cannot restore global T31 operation_id uniqueness while scoped duplicates exist';
    END IF;
END $$;

ALTER TABLE gis_session_owner_receipts
    DROP CONSTRAINT gis_session_owner_receipts_operation_scope_fkey,
    DROP CONSTRAINT gis_session_owner_receipts_pkey;

ALTER TABLE gis_session_operations
    DROP CONSTRAINT gis_session_operations_session_scope_fkey,
    DROP CONSTRAINT gis_session_operations_pkey,
    ADD CONSTRAINT gis_session_operations_pkey PRIMARY KEY (operation_id),
    ADD CONSTRAINT gis_session_operations_session_id_fkey
        FOREIGN KEY (session_id) REFERENCES gis_sessions(id);

ALTER TABLE gis_session_owner_receipts
    ADD CONSTRAINT gis_session_owner_receipts_pkey PRIMARY KEY (operation_id,stage),
    ADD CONSTRAINT gis_session_owner_receipts_operation_id_fkey
        FOREIGN KEY (operation_id) REFERENCES gis_session_operations(operation_id),
    DROP COLUMN application_id,
    DROP COLUMN environment_id;

ALTER TABLE gis_sessions
    DROP CONSTRAINT gis_sessions_id_app_environment_key;
