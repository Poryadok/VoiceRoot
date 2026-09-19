BEGIN;

-- This expansion does not activate a writer or infer legacy session identity.
ALTER TABLE voice_room_instances
    ALTER COLUMN space_id DROP NOT NULL,
    ALTER COLUMN voice_room_id DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS room_type TEXT NOT NULL DEFAULT 'voice_room',
    ADD COLUMN IF NOT EXISTS purpose TEXT NOT NULL DEFAULT 'ORDINARY',
    ADD COLUMN IF NOT EXISTS chat_id UUID NULL,
    ADD COLUMN IF NOT EXISTS owner_id UUID NULL,
    ADD COLUMN IF NOT EXISTS creation_operation_id UUID NULL,
    ADD COLUMN IF NOT EXISTS creation_manifest_hash BYTEA NULL,
    ADD COLUMN IF NOT EXISTS creation_receipt_id UUID NULL,
    ADD COLUMN IF NOT EXISTS chat_creation_receipt_id UUID NULL;

ALTER TABLE voice_room_instances
    DROP CONSTRAINT IF EXISTS voice_room_instances_kind_shape,
    DROP CONSTRAINT IF EXISTS voice_room_instances_purpose_shape,
    DROP CONSTRAINT IF EXISTS voice_room_instances_creation_ids;
ALTER TABLE voice_room_instances
    ADD CONSTRAINT voice_room_instances_kind_shape CHECK (
        (room_type = 'voice_room' AND space_id IS NOT NULL AND voice_room_id IS NOT NULL AND chat_id IS NULL)
        OR (room_type IN ('call', 'group_voice') AND space_id IS NULL AND voice_room_id IS NULL AND chat_id IS NOT NULL)
    ),
    ADD CONSTRAINT voice_room_instances_purpose_shape CHECK (
        (purpose = 'ORDINARY' AND owner_id IS NULL AND creation_operation_id IS NULL
            AND creation_manifest_hash IS NULL AND creation_receipt_id IS NULL AND chat_creation_receipt_id IS NULL)
        OR (purpose = 'MATCH_SQUAD' AND room_type = 'group_voice' AND owner_id IS NOT NULL
            AND creation_operation_id IS NOT NULL AND creation_manifest_hash IS NOT NULL
            AND octet_length(creation_manifest_hash) = 32 AND creation_receipt_id IS NOT NULL
            AND chat_creation_receipt_id IS NOT NULL)
    ),
    ADD CONSTRAINT voice_room_instances_creation_ids CHECK (
        (chat_id IS NULL OR chat_id <> '00000000-0000-0000-0000-000000000000'::UUID)
        AND (owner_id IS NULL OR owner_id <> '00000000-0000-0000-0000-000000000000'::UUID)
        AND (creation_operation_id IS NULL OR creation_operation_id <> '00000000-0000-0000-0000-000000000000'::UUID)
        AND (creation_receipt_id IS NULL OR creation_receipt_id <> '00000000-0000-0000-0000-000000000000'::UUID)
        AND (chat_creation_receipt_id IS NULL OR chat_creation_receipt_id <> '00000000-0000-0000-0000-000000000000'::UUID)
    );

CREATE UNIQUE INDEX IF NOT EXISTS voice_room_instances_match_owner ON voice_room_instances(owner_id) WHERE purpose = 'MATCH_SQUAD';
CREATE UNIQUE INDEX IF NOT EXISTS voice_room_instances_creation_operation ON voice_room_instances(creation_operation_id) WHERE creation_operation_id IS NOT NULL;
CREATE UNIQUE INDEX IF NOT EXISTS voice_room_instances_creation_receipt ON voice_room_instances(creation_receipt_id) WHERE creation_receipt_id IS NOT NULL;

CREATE OR REPLACE FUNCTION voice_room_creation_guard_fn() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.room_id, NEW.room_type, NEW.purpose, NEW.space_id, NEW.voice_room_id, NEW.chat_id,
           NEW.owner_id, NEW.creation_operation_id, NEW.creation_manifest_hash,
           NEW.creation_receipt_id, NEW.chat_creation_receipt_id)
       IS DISTINCT FROM
       ROW(OLD.room_id, OLD.room_type, OLD.purpose, OLD.space_id, OLD.voice_room_id, OLD.chat_id,
           OLD.owner_id, OLD.creation_operation_id, OLD.creation_manifest_hash,
           OLD.creation_receipt_id, OLD.chat_creation_receipt_id) THEN
        RAISE EXCEPTION 'room creation binding is immutable' USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;
DROP TRIGGER IF EXISTS voice_room_creation_guard ON voice_room_instances;
CREATE TRIGGER voice_room_creation_guard BEFORE UPDATE ON voice_room_instances
    FOR EACH ROW EXECUTE FUNCTION voice_room_creation_guard_fn();

ALTER TABLE voice_room_memberships
    ALTER COLUMN space_access_epoch DROP NOT NULL,
    ALTER COLUMN role_policy_epoch DROP NOT NULL,
    ADD COLUMN IF NOT EXISTS account_id UUID NULL,
    ADD COLUMN IF NOT EXISTS session_epoch BIGINT NULL,
    ADD COLUMN IF NOT EXISTS membership_state TEXT NULL,
    ADD COLUMN IF NOT EXISTS reconnect_started_at TIMESTAMPTZ NULL,
    ADD COLUMN IF NOT EXISTS reconnect_deadline TIMESTAMPTZ NULL;
ALTER TABLE voice_room_memberships
    DROP CONSTRAINT IF EXISTS voice_room_memberships_identity_shape,
    DROP CONSTRAINT IF EXISTS voice_room_memberships_reconnect_shape;
ALTER TABLE voice_room_memberships
    ADD CONSTRAINT voice_room_memberships_identity_shape CHECK (
        (account_id IS NULL AND session_epoch IS NULL AND membership_state IS NULL)
        OR (account_id IS NOT NULL AND account_id <> '00000000-0000-0000-0000-000000000000'::UUID
            AND session_epoch IS NOT NULL AND session_epoch > 0 AND membership_state IS NOT NULL
            AND membership_state IN ('JOINING', 'JOINED', 'RECONNECTING', 'LEAVING', 'LEFT', 'EJECTED'))
    ),
    ADD CONSTRAINT voice_room_memberships_reconnect_shape CHECK (
        ((membership_state IS NULL OR membership_state <> 'RECONNECTING')
            AND reconnect_started_at IS NULL AND reconnect_deadline IS NULL)
        OR (membership_state IS NOT NULL AND membership_state = 'RECONNECTING'
            AND reconnect_started_at IS NOT NULL AND reconnect_deadline IS NOT NULL
            AND isfinite(reconnect_started_at) AND isfinite(reconnect_deadline)
            AND reconnect_deadline > reconnect_started_at
            AND reconnect_deadline <= reconnect_started_at + interval '30 seconds')
    );

CREATE INDEX IF NOT EXISTS voice_room_memberships_account_epoch ON voice_room_memberships(account_id, session_epoch) WHERE account_id IS NOT NULL;
CREATE INDEX IF NOT EXISTS voice_room_memberships_reconnect_due ON voice_room_memberships(reconnect_deadline) WHERE membership_state = 'RECONNECTING';

CREATE OR REPLACE FUNCTION voice_membership_identity_guard_fn() RETURNS TRIGGER LANGUAGE plpgsql AS $$
DECLARE
    kind TEXT;
BEGIN
    SELECT room_type INTO kind FROM voice_room_instances WHERE room_id = NEW.room_id;
    IF kind IS NULL THEN
        RAISE EXCEPTION 'membership room does not exist' USING ERRCODE = '23503';
    END IF;
    IF (kind = 'voice_room' AND (NEW.space_access_epoch IS NULL OR NEW.role_policy_epoch IS NULL))
       OR (kind <> 'voice_room' AND (NEW.space_access_epoch IS NOT NULL OR NEW.role_policy_epoch IS NOT NULL)) THEN
        RAISE EXCEPTION 'membership authority epochs do not match room kind' USING ERRCODE = '23514';
    END IF;
    IF TG_OP = 'UPDATE' THEN
        IF NEW.profile_id IS DISTINCT FROM OLD.profile_id THEN
            RAISE EXCEPTION 'membership profile is immutable' USING ERRCODE = '55000';
        END IF;
        IF OLD.account_id IS NOT NULL AND (
            NEW.account_id IS DISTINCT FROM OLD.account_id OR NEW.session_epoch IS NULL
            OR NEW.membership_state IS NULL OR NEW.session_epoch < OLD.session_epoch
            OR (NEW.media_epoch = OLD.media_epoch AND NEW.session_epoch <> OLD.session_epoch)) THEN
            RAISE EXCEPTION 'membership authenticated identity cannot regress or change within media epoch' USING ERRCODE = '55000';
        END IF;
        IF OLD.membership_state = 'RECONNECTING' AND NEW.membership_state = 'RECONNECTING'
           AND NEW.media_epoch = OLD.media_epoch
           AND ROW(NEW.reconnect_started_at, NEW.reconnect_deadline)
               IS DISTINCT FROM ROW(OLD.reconnect_started_at, OLD.reconnect_deadline) THEN
            RAISE EXCEPTION 'reconnect interval cannot be extended by retry' USING ERRCODE = '55000';
        END IF;
    END IF;
    RETURN NEW;
END;
$$;
DROP TRIGGER IF EXISTS voice_membership_identity_guard ON voice_room_memberships;
CREATE TRIGGER voice_membership_identity_guard BEFORE INSERT OR UPDATE ON voice_room_memberships
    FOR EACH ROW EXECUTE FUNCTION voice_membership_identity_guard_fn();

COMMIT;
