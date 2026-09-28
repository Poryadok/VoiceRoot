BEGIN;

ALTER TABLE voice_room_instances DROP CONSTRAINT voice_room_instances_state_check;
ALTER TABLE voice_room_instances ADD CONSTRAINT voice_room_instances_state_check
    CHECK (state IN ('active', 'closing', 'closed'));
ALTER TABLE voice_room_instances DROP CONSTRAINT voice_room_instances_closed_at_check;
ALTER TABLE voice_room_instances ADD CONSTRAINT voice_room_instances_closed_at_check CHECK (
    (state IN ('active', 'closing') AND closed_at IS NULL)
    OR (state = 'closed' AND closed_at IS NOT NULL)
);

ALTER TABLE voice_game_session_operations
    ADD COLUMN session_id UUID NULL
        CHECK (session_id IS NULL OR session_id <> '00000000-0000-0000-0000-000000000000'::UUID);

CREATE TABLE voice_game_session_closures (
    operation_id UUID PRIMARY KEY CHECK (operation_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    room_id UUID NOT NULL UNIQUE REFERENCES voice_game_session_operations(room_id) ON DELETE RESTRICT,
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
    request_bytes BYTEA NOT NULL CHECK (octet_length(request_bytes) > 0),
    status TEXT NOT NULL CHECK (status IN ('CLOSING', 'CLOSED')),
    close_receipt_id UUID NOT NULL UNIQUE CHECK (close_receipt_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    closing_at TIMESTAMPTZ NOT NULL,
    media_fenced_at TIMESTAMPTZ,
    closed_at TIMESTAMPTZ,
    response_bytes BYTEA,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK (
        (status = 'CLOSING' AND media_fenced_at IS NULL AND closed_at IS NULL AND response_bytes IS NULL)
        OR (status = 'CLOSED' AND media_fenced_at IS NOT NULL AND closed_at IS NOT NULL
            AND media_fenced_at >= closing_at AND closed_at >= media_fenced_at
            AND response_bytes IS NOT NULL AND octet_length(response_bytes) > 0)
    )
);
CREATE UNIQUE INDEX voice_game_session_closure_one_operation_per_room_idx
    ON voice_game_session_closures(room_id);

CREATE FUNCTION voice_game_session_closure_transition_guard_fn() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.operation_id IS DISTINCT FROM NEW.operation_id
        OR OLD.room_id IS DISTINCT FROM NEW.room_id
        OR OLD.request_hash IS DISTINCT FROM NEW.request_hash
        OR OLD.request_bytes IS DISTINCT FROM NEW.request_bytes
        OR OLD.close_receipt_id IS DISTINCT FROM NEW.close_receipt_id
        OR OLD.closing_at IS DISTINCT FROM NEW.closing_at
        OR OLD.created_at IS DISTINCT FROM NEW.created_at
        OR OLD.status <> 'CLOSING' OR NEW.status <> 'CLOSED'
        OR OLD.media_fenced_at IS NOT NULL OR OLD.closed_at IS NOT NULL OR OLD.response_bytes IS NOT NULL THEN
        RAISE EXCEPTION 'game session close receipt is immutable except CLOSING to CLOSED'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER voice_game_session_closure_transition_guard
BEFORE UPDATE ON voice_game_session_closures
FOR EACH ROW EXECUTE FUNCTION voice_game_session_closure_transition_guard_fn();
CREATE FUNCTION voice_game_session_closure_delete_guard_fn() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'game session close receipt is immutable' USING ERRCODE = '55000';
END;
$$;
CREATE TRIGGER voice_game_session_closure_delete_guard
BEFORE DELETE ON voice_game_session_closures
FOR EACH ROW EXECUTE FUNCTION voice_game_session_closure_delete_guard_fn();

COMMIT;
