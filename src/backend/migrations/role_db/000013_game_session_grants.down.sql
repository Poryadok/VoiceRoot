DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM game_session_grant_operations)
       OR EXISTS (SELECT 1 FROM game_session_grants)
       OR EXISTS (SELECT 1 FROM game_session_grant_sessions) THEN
        RAISE EXCEPTION '000013 down would destroy Role game-session grant or receipt evidence';
    END IF;
END $$;

DROP TABLE game_session_grant_operations;
DROP TABLE game_session_grants;
DROP TABLE game_session_grant_sessions;
