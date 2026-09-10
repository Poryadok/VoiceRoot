-- Space-owned Voice room access epochs and durable invalidation snapshots.
-- A coarse per-Space epoch may invalidate extra cached decisions, never fewer.
CREATE TABLE space_voice_access_epochs (
    space_id UUID PRIMARY KEY,
    access_epoch BIGINT NOT NULL CHECK (access_epoch > 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE space_voice_access_outbox (
    event_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    space_id UUID NOT NULL,
    access_epoch BIGINT NOT NULL CHECK (access_epoch > 0),
    event_kind TEXT NOT NULL CHECK (event_kind = 'voice_room_access_invalidated'),
    voice_room_id UUID NULL,
    profile_id UUID NULL,
    payload JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (space_id, access_epoch),
    CHECK (payload = jsonb_strip_nulls(jsonb_build_object(
        'space_id', space_id::text,
        'voice_room_id', voice_room_id::text,
        'profile_id', profile_id::text,
        'access_epoch', access_epoch
    )))
);

INSERT INTO space_voice_access_epochs (space_id, access_epoch)
SELECT id, 1
FROM spaces;

CREATE FUNCTION space_voice_access_initialize() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM pg_advisory_xact_lock(hashtext(NEW.id::text));
    INSERT INTO space_voice_access_epochs (space_id, access_epoch)
    VALUES (NEW.id, 1)
    ON CONFLICT (space_id) DO NOTHING;
    RETURN NEW;
END
$$;

CREATE FUNCTION space_voice_access_bump(
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

    INSERT INTO space_voice_access_epochs (space_id, access_epoch)
    VALUES (affected_space_id, 1)
    ON CONFLICT (space_id) DO UPDATE
       SET access_epoch = space_voice_access_epochs.access_epoch + 1,
           updated_at = now()
    RETURNING access_epoch INTO next_epoch;

    snapshot := jsonb_strip_nulls(jsonb_build_object(
        'space_id', affected_space_id::text,
        'voice_room_id', affected_voice_room_id::text,
        'profile_id', affected_profile_id::text,
        'access_epoch', next_epoch
    ));

    INSERT INTO space_voice_access_outbox (
        event_id, space_id, access_epoch, event_kind,
        voice_room_id, profile_id, payload
    ) VALUES (
        gen_random_uuid(), affected_space_id, next_epoch,
        'voice_room_access_invalidated',
        affected_voice_room_id, affected_profile_id, snapshot
    );

    RETURN next_epoch;
END
$$;

CREATE FUNCTION space_voice_access_member_changed() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        PERFORM space_voice_access_bump(NEW.space_id, NULL, NEW.profile_id);
        RETURN NEW;
    END IF;
    PERFORM space_voice_access_bump(OLD.space_id, NULL, OLD.profile_id);
    RETURN OLD;
END
$$;

CREATE FUNCTION space_voice_access_room_changed() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'INSERT' THEN
        PERFORM space_voice_access_bump(NEW.space_id, NEW.id, NULL);
        RETURN NEW;
    END IF;
    PERFORM space_voice_access_bump(OLD.space_id, OLD.id, NULL);
    RETURN OLD;
END
$$;

CREATE FUNCTION space_voice_access_space_deleted() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    PERFORM space_voice_access_bump(OLD.id, NULL, NULL);
    RETURN OLD;
END
$$;

CREATE FUNCTION space_voice_access_outbox_immutable() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'space Voice access invalidation snapshots are immutable'
        USING ERRCODE = '55000';
END
$$;

CREATE TRIGGER space_voice_access_space_initialize
AFTER INSERT ON spaces
FOR EACH ROW EXECUTE FUNCTION space_voice_access_initialize();

CREATE TRIGGER space_voice_access_space_delete
BEFORE DELETE ON spaces
FOR EACH ROW EXECUTE FUNCTION space_voice_access_space_deleted();

CREATE TRIGGER space_voice_access_member_change
AFTER INSERT OR DELETE ON space_members
FOR EACH ROW EXECUTE FUNCTION space_voice_access_member_changed();

CREATE TRIGGER space_voice_access_room_change
AFTER INSERT OR DELETE ON voice_rooms
FOR EACH ROW EXECUTE FUNCTION space_voice_access_room_changed();

CREATE TRIGGER space_voice_access_outbox_no_change
BEFORE UPDATE OR DELETE ON space_voice_access_outbox
FOR EACH ROW EXECUTE FUNCTION space_voice_access_outbox_immutable();
