DO $$
BEGIN
    -- Serialize the empty check with every first writer. Keep these locks until
    -- the whole migration transaction completes, including the DROP statements.
    -- Supported writers read operation receipts before touching session/grants.
    PERFORM set_config('lock_timeout', '2s', true);
    LOCK TABLE game_session_grant_operations,
               game_session_grant_sessions,
               game_session_grants IN ACCESS EXCLUSIVE MODE;

    IF EXISTS (SELECT 1 FROM game_session_grant_operations)
       OR EXISTS (SELECT 1 FROM game_session_grants)
       OR EXISTS (SELECT 1 FROM game_session_grant_sessions) THEN
        RAISE EXCEPTION '000013 down would destroy Role game-session grant or receipt evidence';
    END IF;
END $$;

DROP TABLE game_session_grant_operations;
DROP TABLE game_session_grants;
DROP TABLE game_session_grant_sessions;
