BEGIN;

DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM parties WHERE search_operation_id IS NOT NULL)
       OR EXISTS (SELECT 1 FROM matchmaking_party_search_queue_intents) THEN
        RAISE EXCEPTION 'cannot remove captured Voice parties or queue recovery intents';
    END IF;
END $$;

DROP TABLE matchmaking_party_search_queue_intents;
DROP INDEX parties_voice_source_room_active_idx;
DROP INDEX parties_voice_solo_active_idx;
DROP TRIGGER matchmaking_party_source_immutable ON parties;
DROP FUNCTION matchmaking_party_source_immutable();
ALTER TABLE parties
    DROP CONSTRAINT parties_voice_source_binding,
    DROP COLUMN search_operation_id,
    DROP COLUMN search_request_sha256,
    DROP COLUMN initiator_account_id,
    DROP COLUMN initiator_session_epoch,
    DROP COLUMN voice_snapshot_protocol_version,
    DROP COLUMN voice_source_kind,
    DROP COLUMN voice_source_room_id,
    DROP COLUMN voice_source_roster_version,
    DROP COLUMN voice_member_manifest_sha256;

COMMIT;
