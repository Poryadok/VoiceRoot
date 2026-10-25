-- Never discard recoverable guest conversion work during rollback.
DO $$
BEGIN
    -- A table lock cannot refresh an inherited REPEATABLE READ snapshot.
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION
            'guest_conversion_operations rollback requires READ COMMITTED isolation';
    END IF;

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
