BEGIN;

DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM voice_room_instances WHERE purpose = 'GAME_SESSION') THEN
        RAISE EXCEPTION 'cannot remove game session room support while managed rooms exist';
    END IF;
END $$;
DROP TRIGGER voice_game_session_operation_immutable_guard ON voice_game_session_operations;
DROP FUNCTION voice_game_session_operation_immutable_guard_fn();
DROP TABLE voice_game_session_operations;
ALTER TABLE voice_room_instances DROP CONSTRAINT voice_room_instances_purpose_shape;
ALTER TABLE voice_room_instances ADD CONSTRAINT voice_room_instances_purpose_shape CHECK (
    (purpose = 'ORDINARY' AND owner_id IS NULL AND creation_operation_id IS NULL
        AND creation_manifest_hash IS NULL AND creation_receipt_id IS NULL AND chat_creation_receipt_id IS NULL)
    OR (purpose = 'MATCH_SQUAD' AND room_type = 'group_voice' AND owner_id IS NOT NULL
        AND creation_operation_id IS NOT NULL AND creation_manifest_hash IS NOT NULL
        AND octet_length(creation_manifest_hash) = 32 AND creation_receipt_id IS NOT NULL
        AND chat_creation_receipt_id IS NOT NULL)
);

COMMIT;
