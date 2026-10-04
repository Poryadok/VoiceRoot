BEGIN;

LOCK TABLE voice_match_squad_operations IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM voice_match_squad_operations) THEN
        RAISE EXCEPTION 'refusing to remove active, pending, replay, or permanent MatchSquad fences';
    END IF;
END;
$$;

DROP TRIGGER voice_match_squad_terminal_fence ON voice_match_squad_operations;
DROP FUNCTION voice_match_squad_terminal_fence_fn();
DROP TABLE voice_match_squad_operations;

COMMIT;
