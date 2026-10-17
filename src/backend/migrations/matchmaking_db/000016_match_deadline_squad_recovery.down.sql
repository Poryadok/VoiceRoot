DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM matchmaking_match_recovery_effects) THEN
        RAISE EXCEPTION 'cannot remove MatchFound recovery effects or replay fences';
    END IF;
    IF EXISTS (SELECT 1 FROM search_sessions WHERE recovery_generation <> 0) THEN
        RAISE EXCEPTION 'cannot remove MatchFound session generations';
    END IF;
END;
$$;

DROP INDEX matchmaking_match_recovery_effects_pending_idx;
DROP TABLE matchmaking_match_recovery_effects;
ALTER TABLE search_sessions DROP COLUMN recovery_generation;
