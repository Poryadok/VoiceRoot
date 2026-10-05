BEGIN;

LOCK TABLE voice_match_squad_operations IN ACCESS EXCLUSIVE MODE;
LOCK TABLE voice_match_squad_member_operations, voice_match_squad_member_effects, voice_match_squad_member_grants IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM voice_match_squad_operations) THEN
        RAISE EXCEPTION 'refusing to remove active, pending, replay, or permanent MatchSquad fences';
    END IF;
    IF EXISTS (SELECT 1 FROM voice_match_squad_member_operations)
       OR EXISTS (SELECT 1 FROM voice_match_squad_member_effects)
       OR EXISTS (SELECT 1 FROM voice_match_squad_member_grants) THEN
        RAISE EXCEPTION 'refusing to remove MatchSquad member operations, effects, grants, or replay fences';
    END IF;
END;
$$;

DROP TRIGGER voice_match_squad_terminal_fence ON voice_match_squad_operations;
DROP FUNCTION voice_match_squad_terminal_fence_fn();
DROP TABLE voice_match_squad_operations;
DROP TRIGGER voice_match_squad_member_grants_fence ON voice_match_squad_member_grants;
DROP TRIGGER voice_match_squad_member_effects_fence ON voice_match_squad_member_effects;
DROP TRIGGER voice_match_squad_member_operations_fence ON voice_match_squad_member_operations;
DROP TABLE voice_match_squad_member_grants;
DROP TABLE voice_match_squad_member_effects;
DROP TABLE voice_match_squad_member_operations;
DROP FUNCTION voice_match_squad_member_fence_fn();

COMMIT;
