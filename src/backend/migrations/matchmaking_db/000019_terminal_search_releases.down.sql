-- Serialize the guard with concurrent first writers; binary rollback retains
-- this additive schema and its pending obligations instead of running DOWN.
DO $$
BEGIN
	PERFORM set_config('lock_timeout', '2s', true);
	LOCK TABLE matchmaking_terminal_search_releases IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS (SELECT 1 FROM matchmaking_terminal_search_releases) THEN
        RAISE EXCEPTION 'cannot remove pending terminal search release obligations';
    END IF;
	DROP TABLE matchmaking_terminal_search_releases;
END;
$$;
