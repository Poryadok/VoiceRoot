-- Role-owned Voice policy epochs and durable invalidation snapshots.
-- The epoch is deliberately coarse per Space: extra invalidation is safe,
-- while a missed effective-policy mutation is not.
CREATE TABLE role_voice_policy_epochs (
    space_id UUID PRIMARY KEY,
    policy_epoch BIGINT NOT NULL CHECK (policy_epoch > 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE role_voice_policy_outbox (
    event_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL,
    policy_epoch BIGINT NOT NULL CHECK (policy_epoch > 0),
    event_kind TEXT NOT NULL CHECK (event_kind = 'voice_room_policy_invalidated'),
    voice_room_id UUID NULL,
    profile_id UUID NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (space_id, policy_epoch),
    CHECK (payload = jsonb_strip_nulls(jsonb_build_object(
        'space_id', space_id::text,
        'voice_room_id', voice_room_id::text,
        'profile_id', profile_id::text,
        'policy_epoch', policy_epoch
    )))
);

INSERT INTO role_voice_policy_epochs (space_id, policy_epoch)
SELECT DISTINCT space_id, 1
FROM roles;

CREATE FUNCTION role_voice_policy_bump(
    affected_space_id UUID,
    affected_voice_room_id UUID,
    affected_profile_id UUID
) RETURNS BIGINT
LANGUAGE plpgsql AS $$
DECLARE
    next_epoch BIGINT;
    snapshot JSONB;
BEGIN
    PERFORM pg_advisory_xact_lock(hashtext(affected_space_id::text));

    INSERT INTO role_voice_policy_epochs (space_id, policy_epoch)
    VALUES (affected_space_id, 1)
    ON CONFLICT (space_id) DO UPDATE
       SET policy_epoch = role_voice_policy_epochs.policy_epoch + 1,
           updated_at = now()
    RETURNING policy_epoch INTO next_epoch;

    snapshot := jsonb_strip_nulls(jsonb_build_object(
        'space_id', affected_space_id::text,
        'voice_room_id', affected_voice_room_id::text,
        'profile_id', affected_profile_id::text,
        'policy_epoch', next_epoch
    ));

    INSERT INTO role_voice_policy_outbox (
        event_id, space_id, policy_epoch, event_kind,
        voice_room_id, profile_id, payload
    ) VALUES (
        gen_random_uuid(), affected_space_id, next_epoch,
        'voice_room_policy_invalidated',
        affected_voice_room_id, affected_profile_id, snapshot
    );

    RETURN next_epoch;
END
$$;

CREATE FUNCTION role_voice_policy_role_changed() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM role_voice_policy_bump(OLD.space_id, NULL, NULL);
        RETURN OLD;
    END IF;
    PERFORM role_voice_policy_bump(NEW.space_id, NULL, NULL);
    RETURN NEW;
END
$$;

CREATE FUNCTION role_voice_policy_member_changed() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    changed_space_id UUID;
    changed_profile_id UUID;
    changed_role_id UUID;
BEGIN
    IF TG_OP = 'DELETE' THEN
        changed_space_id := OLD.space_id;
        changed_profile_id := OLD.profile_id;
        changed_role_id := OLD.role_id;
    ELSE
        changed_space_id := NEW.space_id;
        changed_profile_id := NEW.profile_id;
        changed_role_id := NEW.role_id;
    END IF;

    -- A cascading role deletion has its own Space-scoped invalidation.
    IF TG_OP = 'DELETE' AND NOT EXISTS (SELECT 1 FROM roles WHERE id = changed_role_id) THEN
        RETURN OLD;
    END IF;

    PERFORM role_voice_policy_bump(changed_space_id, NULL, changed_profile_id);
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END
$$;

CREATE FUNCTION role_voice_policy_voice_override_changed() RETURNS trigger
LANGUAGE plpgsql AS $$
DECLARE
    changed_voice_room_id UUID;
    changed_role_id UUID;
    changed_space_id UUID;
BEGIN
    IF TG_OP = 'DELETE' THEN
        changed_voice_room_id := OLD.voice_room_id;
        changed_role_id := OLD.role_id;
    ELSE
        changed_voice_room_id := NEW.voice_room_id;
        changed_role_id := NEW.role_id;
    END IF;

    SELECT space_id INTO changed_space_id FROM roles WHERE id = changed_role_id;
    -- The parent role deletion emits the authoritative Space wildcard.
    IF changed_space_id IS NULL THEN
        IF TG_OP = 'DELETE' THEN
            RETURN OLD;
        END IF;
        RETURN NEW;
    END IF;

    PERFORM role_voice_policy_bump(changed_space_id, changed_voice_room_id, NULL);
    IF TG_OP = 'DELETE' THEN
        RETURN OLD;
    END IF;
    RETURN NEW;
END
$$;

CREATE FUNCTION role_voice_policy_ownership_changed() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM role_voice_policy_bump(OLD.space_id, NULL, NULL);
        RETURN OLD;
    END IF;
    PERFORM role_voice_policy_bump(NEW.space_id, NULL, NULL);
    RETURN NEW;
END
$$;

CREATE FUNCTION role_voice_policy_lifecycle_changed() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        PERFORM role_voice_policy_bump(OLD.space_id, NULL, NULL);
        RETURN OLD;
    END IF;
    PERFORM role_voice_policy_bump(NEW.space_id, NULL, NULL);
    RETURN NEW;
END
$$;

CREATE FUNCTION role_voice_policy_outbox_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'Role Voice policy invalidation snapshots are immutable'
        USING ERRCODE = '55000';
END
$$;

CREATE TRIGGER role_voice_policy_role_change
AFTER INSERT OR UPDATE OR DELETE ON roles
FOR EACH ROW EXECUTE FUNCTION role_voice_policy_role_changed();

CREATE TRIGGER role_voice_policy_member_change
AFTER INSERT OR UPDATE OR DELETE ON member_roles
FOR EACH ROW EXECUTE FUNCTION role_voice_policy_member_changed();

CREATE TRIGGER role_voice_policy_voice_override_change
AFTER INSERT OR UPDATE OR DELETE ON voice_room_overrides
FOR EACH ROW EXECUTE FUNCTION role_voice_policy_voice_override_changed();

CREATE TRIGGER role_voice_policy_ownership_change
AFTER INSERT OR UPDATE OR DELETE ON ownership_transfer_v2
FOR EACH ROW EXECUTE FUNCTION role_voice_policy_ownership_changed();

CREATE TRIGGER role_voice_policy_lifecycle_change
AFTER INSERT OR UPDATE OR DELETE ON role_space_lifecycle
FOR EACH ROW EXECUTE FUNCTION role_voice_policy_lifecycle_changed();

CREATE TRIGGER role_voice_policy_outbox_no_change
BEFORE UPDATE OR DELETE ON role_voice_policy_outbox
FOR EACH ROW EXECUTE FUNCTION role_voice_policy_outbox_immutable();

