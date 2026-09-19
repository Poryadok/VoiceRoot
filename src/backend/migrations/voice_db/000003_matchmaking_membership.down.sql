BEGIN;

-- Lock before examining data, including writers that committed while DOWN waited.
LOCK TABLE voice_room_instances, voice_room_memberships IN ACCESS EXCLUSIVE MODE;
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM voice_room_instances WHERE room_type <> 'voice_room' OR purpose <> 'ORDINARY')
       OR EXISTS (SELECT 1 FROM voice_room_memberships WHERE account_id IS NOT NULL OR session_epoch IS NOT NULL
                  OR membership_state IS NOT NULL OR reconnect_started_at IS NOT NULL OR reconnect_deadline IS NOT NULL
                  OR space_access_epoch IS NULL OR role_policy_epoch IS NULL) THEN
        RAISE EXCEPTION 'cannot discard expanded Voice room or membership evidence' USING ERRCODE = '55000';
    END IF;
END;
$$;

DROP TRIGGER voice_membership_identity_guard ON voice_room_memberships;
DROP FUNCTION voice_membership_identity_guard_fn();
DROP INDEX voice_room_memberships_account_epoch;
DROP INDEX voice_room_memberships_reconnect_due;
ALTER TABLE voice_room_memberships
    DROP CONSTRAINT voice_room_memberships_identity_shape,
    DROP CONSTRAINT voice_room_memberships_reconnect_shape,
    DROP COLUMN account_id,
    DROP COLUMN session_epoch,
    DROP COLUMN membership_state,
    DROP COLUMN reconnect_started_at,
    DROP COLUMN reconnect_deadline,
    ALTER COLUMN space_access_epoch SET NOT NULL,
    ALTER COLUMN role_policy_epoch SET NOT NULL;

DROP TRIGGER voice_room_creation_guard ON voice_room_instances;
DROP FUNCTION voice_room_creation_guard_fn();
DROP INDEX voice_room_instances_match_owner;
DROP INDEX voice_room_instances_creation_operation;
DROP INDEX voice_room_instances_creation_receipt;
ALTER TABLE voice_room_instances
    DROP CONSTRAINT voice_room_instances_kind_shape,
    DROP CONSTRAINT voice_room_instances_purpose_shape,
    DROP CONSTRAINT voice_room_instances_creation_ids,
    DROP COLUMN room_type,
    DROP COLUMN purpose,
    DROP COLUMN chat_id,
    DROP COLUMN owner_id,
    DROP COLUMN creation_operation_id,
    DROP COLUMN creation_manifest_hash,
    DROP COLUMN creation_receipt_id,
    DROP COLUMN chat_creation_receipt_id,
    ALTER COLUMN space_id SET NOT NULL,
    ALTER COLUMN voice_room_id SET NOT NULL;

COMMIT;
