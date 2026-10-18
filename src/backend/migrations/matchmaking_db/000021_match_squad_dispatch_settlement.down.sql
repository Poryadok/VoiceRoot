BEGIN;
LOCK TABLE matchmaking_match_squad_operations IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM matchmaking_match_squad_operations) THEN
        RAISE EXCEPTION 'cannot remove MatchSquad dispatch or settlement evidence';
    END IF;
END $$;
ALTER TABLE matchmaking_match_squad_operations
    DROP CONSTRAINT matchmaking_provision_abandon_binding,
    DROP CONSTRAINT matchmaking_provision_chat_dispatch_binding,
    DROP CONSTRAINT matchmaking_provision_voice_dispatch_binding,
    DROP COLUMN dispatch_protocol_version,
    DROP COLUMN provision_disposition,
    DROP COLUMN provision_revision,
    DROP COLUMN provision_abandoned_at,
    DROP COLUMN provision_abandon_reason,
    DROP COLUMN provision_abandon_evidence_sha256,
    DROP COLUMN chat_dispatch_state,
    DROP COLUMN chat_first_dispatch_id,
    DROP COLUMN chat_first_dispatch_at,
    DROP COLUMN voice_dispatch_state,
    DROP COLUMN voice_first_dispatch_id,
    DROP COLUMN voice_first_dispatch_at;
COMMIT;
