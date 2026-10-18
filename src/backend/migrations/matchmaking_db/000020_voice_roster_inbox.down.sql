BEGIN;

LOCK TABLE matchmaking_voice_roster_inbox,
    matchmaking_voice_roster_contract_conflicts,
    matchmaking_voice_roster_cancellation_outbox IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM matchmaking_voice_roster_inbox)
       OR EXISTS (SELECT 1 FROM matchmaking_voice_roster_contract_conflicts)
       OR EXISTS (SELECT 1 FROM matchmaking_voice_roster_cancellation_outbox) THEN
        RAISE EXCEPTION 'cannot remove Voice roster inbox or cancellation evidence';
    END IF;
END $$;

DROP TABLE matchmaking_voice_roster_cancellation_outbox;
DROP TABLE matchmaking_voice_roster_contract_conflicts;
DROP TABLE matchmaking_voice_roster_inbox;

COMMIT;
