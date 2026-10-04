BEGIN;

-- The resource row in voice_room_instances remains the current-state authority.
-- This ledger binds exact idempotent requests/receipts and retains compact
-- terminal fences after payload retention expires.
CREATE TABLE voice_match_squad_operations (
    operation_id UUID PRIMARY KEY CHECK (operation_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    match_id UUID NOT NULL UNIQUE CHECK (match_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    room_id UUID NOT NULL UNIQUE REFERENCES voice_room_instances(room_id) ON DELETE RESTRICT,
    chat_id UUID NOT NULL CHECK (chat_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    owner_id UUID NOT NULL CHECK (owner_id = match_id),
    creation_receipt_id UUID NOT NULL UNIQUE CHECK (creation_receipt_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    chat_creation_receipt_id UUID NOT NULL CHECK (chat_creation_receipt_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    participant_manifest_sha256 BYTEA NOT NULL CHECK (octet_length(participant_manifest_sha256) = 32),
    chat_creation_receipt_sha256 BYTEA NOT NULL CHECK (octet_length(chat_creation_receipt_sha256) = 32),
    create_request_sha256 BYTEA NOT NULL CHECK (octet_length(create_request_sha256) = 32),
    create_request_bytes BYTEA NULL,
    create_receipt_bytes BYTEA NULL,
    state TEXT NOT NULL CHECK (state IN ('pending', 'active', 'closing', 'closed')),
    teardown_operation_id UUID NULL UNIQUE CHECK (teardown_operation_id IS NULL OR teardown_operation_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    teardown_request_sha256 BYTEA NULL CHECK (teardown_request_sha256 IS NULL OR octet_length(teardown_request_sha256) = 32),
    teardown_request_bytes BYTEA NULL,
    teardown_receipt_id UUID NULL UNIQUE CHECK (teardown_receipt_id IS NULL OR teardown_receipt_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    teardown_receipt_bytes BYTEA NULL,
    effects_confirmed_at TIMESTAMPTZ NULL,
    retention_expires_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK ((teardown_operation_id IS NULL AND teardown_request_sha256 IS NULL AND teardown_request_bytes IS NULL AND teardown_receipt_id IS NULL AND teardown_receipt_bytes IS NULL AND effects_confirmed_at IS NULL AND retention_expires_at IS NULL)
        OR (teardown_operation_id IS NOT NULL AND teardown_request_sha256 IS NOT NULL)),
    CHECK ((state <> 'closed') OR (teardown_operation_id IS NOT NULL AND teardown_receipt_id IS NOT NULL AND effects_confirmed_at IS NOT NULL)),
    CHECK ((create_request_bytes IS NULL AND create_receipt_bytes IS NULL) OR (create_request_bytes IS NOT NULL AND create_receipt_bytes IS NOT NULL))
);

CREATE INDEX voice_match_squad_operations_retention ON voice_match_squad_operations(retention_expires_at)
    WHERE state = 'closed' AND retention_expires_at IS NOT NULL;

CREATE OR REPLACE FUNCTION voice_match_squad_terminal_fence_fn() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'MatchSquad terminal and replay fences are permanent' USING ERRCODE = '55000';
    END IF;
    IF OLD.state = 'closed' AND NEW.state <> 'closed' THEN
        RAISE EXCEPTION 'MatchSquad terminal fence cannot reopen' USING ERRCODE = '55000';
    END IF;
    IF ROW(NEW.operation_id, NEW.match_id, NEW.room_id, NEW.chat_id, NEW.owner_id,
           NEW.creation_receipt_id, NEW.chat_creation_receipt_id, NEW.participant_manifest_sha256,
           NEW.chat_creation_receipt_sha256, NEW.create_request_sha256)
       IS DISTINCT FROM
       ROW(OLD.operation_id, OLD.match_id, OLD.room_id, OLD.chat_id, OLD.owner_id,
           OLD.creation_receipt_id, OLD.chat_creation_receipt_id, OLD.participant_manifest_sha256,
           OLD.chat_creation_receipt_sha256, OLD.create_request_sha256) THEN
        RAISE EXCEPTION 'MatchSquad creation binding is immutable' USING ERRCODE = '55000';
    END IF;
    IF OLD.teardown_operation_id IS NOT NULL AND ROW(NEW.teardown_operation_id, NEW.teardown_request_sha256)
       IS DISTINCT FROM ROW(OLD.teardown_operation_id, OLD.teardown_request_sha256) THEN
        RAISE EXCEPTION 'MatchSquad teardown replay fence is immutable' USING ERRCODE = '55000';
    END IF;
    IF OLD.teardown_receipt_id IS NOT NULL AND NEW.teardown_receipt_id IS DISTINCT FROM OLD.teardown_receipt_id THEN
        RAISE EXCEPTION 'MatchSquad teardown receipt fence is immutable' USING ERRCODE = '55000';
    END IF;
    IF ((OLD.create_request_bytes IS NOT NULL AND NEW.create_request_bytes IS NULL)
        OR (OLD.create_receipt_bytes IS NOT NULL AND NEW.create_receipt_bytes IS NULL)
        OR (OLD.teardown_request_bytes IS NOT NULL AND NEW.teardown_request_bytes IS NULL)
        OR (OLD.teardown_receipt_bytes IS NOT NULL AND NEW.teardown_receipt_bytes IS NULL))
       AND NOT (OLD.state = 'closed' AND OLD.retention_expires_at IS NOT NULL AND OLD.retention_expires_at <= clock_timestamp()) THEN
        RAISE EXCEPTION 'MatchSquad operation evidence cannot be compacted before its retention fence' USING ERRCODE = '55000';
    END IF;
    NEW.updated_at := clock_timestamp();
    RETURN NEW;
END;
$$;
CREATE TRIGGER voice_match_squad_terminal_fence BEFORE UPDATE OR DELETE ON voice_match_squad_operations
    FOR EACH ROW EXECUTE FUNCTION voice_match_squad_terminal_fence_fn();

COMMIT;
