-- Rollback is only safe before any v2 operation or retirement is accepted.
DO $$
BEGIN
    -- Wait for in-flight decisions before observing the guard. Keep both locks
    -- through the DROP statements in this same atomic statement.
    LOCK TABLE ownership_transfer_v2, role_space_lifecycle IN ACCESS EXCLUSIVE MODE;
    IF EXISTS (SELECT 1 FROM ownership_transfer_v2)
        OR EXISTS (SELECT 1 FROM role_space_lifecycle WHERE retired_at IS NOT NULL) THEN
        RAISE EXCEPTION 'cannot discard durable ownership decisions or retirement fences';
    END IF;
    DROP TABLE ownership_transfer_v2;
    DROP TABLE role_space_lifecycle;
END $$;
