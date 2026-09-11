BEGIN;

LOCK TABLE voice_lifecycle_operations IN ACCESS EXCLUSIVE MODE;
LOCK TABLE voice_room_memberships IN ACCESS EXCLUSIVE MODE;
LOCK TABLE voice_room_instances IN ACCESS EXCLUSIVE MODE;
LOCK TABLE voice_lifecycle_effects IN ACCESS EXCLUSIVE MODE;
LOCK TABLE voice_media_epoch_denials IN ACCESS EXCLUSIVE MODE;
LOCK TABLE voice_event_outbox IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM voice_lifecycle_operations)
       OR EXISTS (SELECT 1 FROM voice_room_memberships)
       OR EXISTS (SELECT 1 FROM voice_room_instances)
       OR EXISTS (SELECT 1 FROM voice_lifecycle_effects)
       OR EXISTS (SELECT 1 FROM voice_media_epoch_denials)
       OR EXISTS (SELECT 1 FROM voice_event_outbox) THEN
        RAISE EXCEPTION USING
            ERRCODE = '55000',
            MESSAGE = 'voice room lifecycle evidence exists; refusing DOWN migration';
    END IF;
END
$$;

DROP TRIGGER voice_room_instance_immutable_guard ON voice_room_instances;
DROP TRIGGER voice_room_membership_generation_guard ON voice_room_memberships;
DROP TRIGGER voice_lifecycle_operation_immutable_guard ON voice_lifecycle_operations;
DROP TRIGGER voice_lifecycle_operation_transition_guard ON voice_lifecycle_operations;
DROP TRIGGER voice_lifecycle_effect_immutable_guard ON voice_lifecycle_effects;
DROP TRIGGER voice_lifecycle_effect_transition_guard ON voice_lifecycle_effects;
DROP TRIGGER voice_media_epoch_denial_update_guard ON voice_media_epoch_denials;
DROP TRIGGER voice_media_epoch_denial_delete_guard ON voice_media_epoch_denials;
DROP TRIGGER voice_event_outbox_immutable_guard ON voice_event_outbox;
DROP TRIGGER voice_event_outbox_transition_guard ON voice_event_outbox;

DROP FUNCTION voice_room_instance_immutable_guard_fn();
DROP FUNCTION voice_room_membership_generation_guard_fn();
DROP FUNCTION voice_lifecycle_operation_immutable_guard_fn();
DROP FUNCTION voice_lifecycle_operation_transition_guard_fn();
DROP FUNCTION voice_lifecycle_effect_immutable_guard_fn();
DROP FUNCTION voice_lifecycle_effect_transition_guard_fn();
DROP FUNCTION voice_media_epoch_denial_update_guard_fn();
DROP FUNCTION voice_media_epoch_denial_delete_guard_fn();
DROP FUNCTION voice_event_outbox_immutable_guard_fn();
DROP FUNCTION voice_event_outbox_transition_guard_fn();

DROP TABLE voice_lifecycle_effects;
DROP TABLE voice_media_epoch_denials;
DROP TABLE voice_event_outbox;
DROP TABLE voice_room_memberships;
DROP TABLE voice_lifecycle_operations;
DROP TABLE voice_room_instances;

COMMIT;
