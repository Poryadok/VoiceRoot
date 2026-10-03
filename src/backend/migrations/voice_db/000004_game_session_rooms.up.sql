BEGIN;

ALTER TABLE voice_room_instances DROP CONSTRAINT voice_room_instances_purpose_shape;
ALTER TABLE voice_room_instances ADD CONSTRAINT voice_room_instances_purpose_shape CHECK (
    (purpose = 'ORDINARY' AND owner_id IS NULL AND creation_operation_id IS NULL
        AND creation_manifest_hash IS NULL AND creation_receipt_id IS NULL AND chat_creation_receipt_id IS NULL)
    OR (purpose = 'MATCH_SQUAD' AND room_type = 'group_voice' AND owner_id IS NOT NULL
        AND creation_operation_id IS NOT NULL AND creation_manifest_hash IS NOT NULL
        AND octet_length(creation_manifest_hash) = 32 AND creation_receipt_id IS NOT NULL
        AND chat_creation_receipt_id IS NOT NULL)
    OR (purpose = 'GAME_SESSION' AND room_type = 'group_voice' AND owner_id IS NULL
        AND creation_operation_id IS NOT NULL AND creation_manifest_hash IS NOT NULL
        AND octet_length(creation_manifest_hash) = 32 AND creation_receipt_id IS NOT NULL
        AND chat_creation_receipt_id IS NOT NULL)
);

CREATE TABLE voice_game_session_operations (
    operation_id UUID PRIMARY KEY CHECK (operation_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    application_id UUID NOT NULL CHECK (application_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    environment_id UUID NOT NULL CHECK (environment_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    resource_kind SMALLINT NOT NULL CHECK (resource_kind IN (1, 2, 3)),
    external_resource_key TEXT NOT NULL CHECK (btrim(external_resource_key) <> '' AND octet_length(external_resource_key) <= 512),
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
    chat_id UUID NOT NULL CHECK (chat_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    chat_creation_operation_id UUID NOT NULL CHECK (chat_creation_operation_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    room_id UUID NOT NULL UNIQUE REFERENCES voice_room_instances(room_id) ON DELETE RESTRICT,
    voice_creation_receipt_id UUID NOT NULL UNIQUE CHECK (voice_creation_receipt_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    response_bytes BYTEA NOT NULL CHECK (octet_length(response_bytes) > 0),
    created_at TIMESTAMPTZ NOT NULL,
    UNIQUE (application_id, environment_id, resource_kind, external_resource_key)
);

CREATE FUNCTION voice_game_session_operation_immutable_guard_fn() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'game session operation receipt is immutable' USING ERRCODE = '55000';
END;
$$;
CREATE TRIGGER voice_game_session_operation_immutable_guard
BEFORE UPDATE OR DELETE ON voice_game_session_operations
FOR EACH ROW EXECUTE FUNCTION voice_game_session_operation_immutable_guard_fn();

COMMIT;
