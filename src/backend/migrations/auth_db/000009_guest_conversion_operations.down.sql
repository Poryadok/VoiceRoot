-- Never discard recoverable guest conversion work during rollback.
DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM guest_conversion_operations
        WHERE state <> 'COMPLETED'
    ) THEN
        RAISE EXCEPTION
            'cannot roll back guest_conversion_operations while non-completed work exists';
    END IF;
END
$$;

DROP TABLE guest_conversion_operations;
