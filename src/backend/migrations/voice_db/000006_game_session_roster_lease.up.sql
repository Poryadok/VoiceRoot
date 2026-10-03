BEGIN;

ALTER TABLE voice_game_session_operations
    ADD COLUMN roster_revision BIGINT NOT NULL DEFAULT 0 CHECK (roster_revision >= 0),
    ADD COLUMN roster_profile_ids UUID[] NOT NULL DEFAULT '{}'::UUID[],
    ADD COLUMN roster_body_hash BYTEA CHECK (roster_body_hash IS NULL OR octet_length(roster_body_hash) = 32),
    ADD COLUMN lease_expires_at TIMESTAMPTZ,
    ADD COLUMN lease_fenced_at TIMESTAMPTZ;

DROP TRIGGER voice_game_session_operation_immutable_guard ON voice_game_session_operations;
DROP FUNCTION voice_game_session_operation_immutable_guard_fn();
CREATE FUNCTION voice_game_session_operation_immutable_guard_fn() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF (to_jsonb(OLD) - 'roster_revision' - 'roster_profile_ids' - 'roster_body_hash' - 'lease_expires_at' - 'lease_fenced_at')
        IS DISTINCT FROM (to_jsonb(NEW) - 'roster_revision' - 'roster_profile_ids' - 'roster_body_hash' - 'lease_expires_at' - 'lease_fenced_at') THEN
        RAISE EXCEPTION 'game session operation receipt is immutable except monotonic roster state'
            USING ERRCODE = '55000';
    END IF;
    IF NEW.roster_revision > OLD.roster_revision
        AND NEW.lease_expires_at IS NOT NULL AND NEW.roster_body_hash IS NOT NULL AND NEW.lease_fenced_at IS NULL THEN
        RETURN NEW;
    END IF;
    IF NEW.roster_revision = OLD.roster_revision
        AND NEW.roster_profile_ids IS NOT DISTINCT FROM OLD.roster_profile_ids
        AND NEW.roster_body_hash IS NOT DISTINCT FROM OLD.roster_body_hash
        AND NEW.lease_expires_at IS NOT DISTINCT FROM OLD.lease_expires_at
        AND OLD.lease_fenced_at IS NULL AND NEW.lease_fenced_at IS NOT NULL THEN
        RETURN NEW;
    END IF;
    RAISE EXCEPTION 'game session roster revision and lease fence are monotonic'
        USING ERRCODE = '55000';
END;
$$;
CREATE TRIGGER voice_game_session_operation_immutable_guard
BEFORE UPDATE ON voice_game_session_operations
FOR EACH ROW EXECUTE FUNCTION voice_game_session_operation_immutable_guard_fn();
CREATE FUNCTION voice_game_session_operation_delete_guard_fn() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'game session operation receipt is immutable' USING ERRCODE = '55000';
END;
$$;
CREATE TRIGGER voice_game_session_operation_delete_guard
BEFORE DELETE ON voice_game_session_operations
FOR EACH ROW EXECUTE FUNCTION voice_game_session_operation_delete_guard_fn();

CREATE TABLE voice_game_session_roster_receipts (
    operation_id UUID PRIMARY KEY CHECK (operation_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    room_id UUID NOT NULL REFERENCES voice_game_session_operations(room_id) ON DELETE RESTRICT,
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
    request_bytes BYTEA NOT NULL CHECK (octet_length(request_bytes) > 0),
    receipt_id UUID NOT NULL UNIQUE CHECK (receipt_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    response_bytes BYTEA NOT NULL CHECK (octet_length(response_bytes) > 0),
    accepted_revision BIGINT NOT NULL CHECK (accepted_revision >= 0),
    lease_expires_at TIMESTAMPTZ NOT NULL,
    removed_profile_ids UUID[] NOT NULL DEFAULT '{}'::UUID[],
    media_fenced_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK (media_fenced_at IS NULL OR media_fenced_at >= created_at)
);

CREATE FUNCTION voice_game_session_roster_receipt_guard_fn() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.operation_id IS DISTINCT FROM NEW.operation_id
        OR OLD.room_id IS DISTINCT FROM NEW.room_id
        OR OLD.request_hash IS DISTINCT FROM NEW.request_hash
        OR OLD.request_bytes IS DISTINCT FROM NEW.request_bytes
        OR OLD.receipt_id IS DISTINCT FROM NEW.receipt_id
        OR OLD.response_bytes IS DISTINCT FROM NEW.response_bytes
        OR OLD.accepted_revision IS DISTINCT FROM NEW.accepted_revision
        OR OLD.lease_expires_at IS DISTINCT FROM NEW.lease_expires_at
        OR OLD.removed_profile_ids IS DISTINCT FROM NEW.removed_profile_ids
        OR OLD.created_at IS DISTINCT FROM NEW.created_at
        OR OLD.media_fenced_at IS NOT NULL OR NEW.media_fenced_at IS NULL THEN
        RAISE EXCEPTION 'game session roster receipt is immutable except media fence completion'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER voice_game_session_roster_receipt_guard
BEFORE UPDATE ON voice_game_session_roster_receipts
FOR EACH ROW EXECUTE FUNCTION voice_game_session_roster_receipt_guard_fn();
CREATE FUNCTION voice_game_session_roster_receipt_delete_guard_fn() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'game session roster receipt is immutable' USING ERRCODE = '55000';
END;
$$;
CREATE TRIGGER voice_game_session_roster_receipt_delete_guard
BEFORE DELETE ON voice_game_session_roster_receipts
FOR EACH ROW EXECUTE FUNCTION voice_game_session_roster_receipt_delete_guard_fn();

COMMIT;
