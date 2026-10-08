DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM gis_session_operations o
        JOIN gis_sessions s ON s.id=o.session_id
        WHERE o.application_id<>s.application_id OR o.environment_id<>s.environment_id
    ) THEN
        RAISE EXCEPTION 'T31 operation/session scope mismatch; scoped-key migration aborted';
    END IF;
    IF EXISTS (
        SELECT 1
        FROM gis_session_owner_receipts r
        LEFT JOIN gis_session_operations o ON o.operation_id=r.operation_id
        WHERE o.operation_id IS NULL
    ) THEN
        RAISE EXCEPTION 'T31 owner receipt has no authoritative operation parent; scoped-key migration aborted';
    END IF;
END $$;

ALTER TABLE gis_sessions
    ADD CONSTRAINT gis_sessions_id_app_environment_key UNIQUE (id,application_id,environment_id);

ALTER TABLE gis_session_owner_receipts
    ADD COLUMN application_id uuid,
    ADD COLUMN environment_id uuid;

-- The old global operation_id primary key and FK make this join one-to-one.
-- Copy only the namespace stored on that authoritative parent operation.
UPDATE gis_session_owner_receipts r
SET application_id=o.application_id,
    environment_id=o.environment_id
FROM gis_session_operations o
WHERE o.operation_id=r.operation_id;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM gis_session_owner_receipts
        WHERE application_id IS NULL OR environment_id IS NULL
    ) THEN
        RAISE EXCEPTION 'T31 owner receipt namespace backfill incomplete; scoped-key migration aborted';
    END IF;
END $$;

ALTER TABLE gis_session_owner_receipts
    ALTER COLUMN application_id SET NOT NULL,
    ALTER COLUMN environment_id SET NOT NULL;

ALTER TABLE gis_session_owner_receipts
    DROP CONSTRAINT gis_session_owner_receipts_operation_id_fkey,
    DROP CONSTRAINT gis_session_owner_receipts_pkey;

ALTER TABLE gis_session_operations
    DROP CONSTRAINT gis_session_operations_session_id_fkey,
    DROP CONSTRAINT gis_session_operations_pkey;

ALTER TABLE gis_session_operations
    ADD CONSTRAINT gis_session_operations_pkey PRIMARY KEY (application_id,environment_id,operation_id),
    ADD CONSTRAINT gis_session_operations_session_scope_fkey
        FOREIGN KEY (session_id,application_id,environment_id)
        REFERENCES gis_sessions(id,application_id,environment_id);

ALTER TABLE gis_session_owner_receipts
    ADD CONSTRAINT gis_session_owner_receipts_pkey PRIMARY KEY (application_id,environment_id,operation_id,stage),
    ADD CONSTRAINT gis_session_owner_receipts_operation_scope_fkey
        FOREIGN KEY (application_id,environment_id,operation_id)
        REFERENCES gis_session_operations(application_id,environment_id,operation_id);
