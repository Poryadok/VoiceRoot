-- Never discard recoverable guest conversion work during rollback.
DO $$
BEGIN
    -- Serialize with already admitted writers before observing their state.
    -- Keep LOCK, probe and DROP in one statement even for statement-splitting runners.
    LOCK TABLE guest_conversion_operations IN ACCESS EXCLUSIVE MODE;

    IF EXISTS (
        SELECT 1
        FROM guest_conversion_operations
        WHERE state <> 'COMPLETED'
    ) THEN
        RAISE EXCEPTION
            'cannot roll back guest_conversion_operations while non-completed work exists';
    END IF;

    DROP TABLE guest_conversion_operations;
END
$$;
