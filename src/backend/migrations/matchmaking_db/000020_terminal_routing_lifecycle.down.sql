-- Binary rollback retains schema and permanent local lifecycle outcomes.
DO $$
BEGIN
    PERFORM set_config('lock_timeout','2s',true);
    LOCK TABLE matchmaking_terminal_routing_heads, matchmaking_terminal_routing_cycles,
        matchmaking_terminal_search_releases IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF EXISTS(SELECT 1 FROM matchmaking_terminal_routing_heads)
       OR EXISTS(SELECT 1 FROM matchmaking_terminal_routing_cycles)
       OR EXISTS(SELECT 1 FROM matchmaking_terminal_search_releases WHERE routing_fence_id IS NOT NULL) THEN
        RAISE EXCEPTION 'cannot remove terminal routing lifecycle evidence';
    END IF;
    ALTER TABLE matchmaking_terminal_search_releases
        DROP CONSTRAINT terminal_routing_binding_complete,
        DROP COLUMN routing_fence_id, DROP COLUMN routing_cycle_id, DROP COLUMN routing_purge_at,
        DROP COLUMN projection_claim, DROP COLUMN projection_claim_until;
    DROP TABLE matchmaking_terminal_routing_cycles, matchmaking_terminal_routing_heads;
END $$;
