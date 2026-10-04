BEGIN;

CREATE EXTENSION IF NOT EXISTS pgcrypto;

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
    compaction_operation_id UUID NULL UNIQUE CHECK (compaction_operation_id IS NULL OR compaction_operation_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    teardown_aggregate_id UUID NULL CHECK (teardown_aggregate_id IS NULL OR teardown_aggregate_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    compaction_request_sha256 BYTEA NULL CHECK (compaction_request_sha256 IS NULL OR octet_length(compaction_request_sha256) = 32),
    compaction_request_bytes BYTEA NULL,
    compaction_receipt_id UUID NULL UNIQUE CHECK (compaction_receipt_id IS NULL OR compaction_receipt_id <> '00000000-0000-0000-0000-000000000000'::UUID),
    compaction_receipt_bytes BYTEA NULL,
    teardown_receipt_sha256 BYTEA NULL CHECK (teardown_receipt_sha256 IS NULL OR octet_length(teardown_receipt_sha256) = 32),
    aggregate_completed_at TIMESTAMPTZ NULL,
    compaction_authorized_at TIMESTAMPTZ NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK ((teardown_operation_id IS NULL AND teardown_request_sha256 IS NULL AND teardown_request_bytes IS NULL AND teardown_receipt_id IS NULL AND teardown_receipt_bytes IS NULL AND effects_confirmed_at IS NULL)
        OR (teardown_operation_id IS NOT NULL AND teardown_request_sha256 IS NOT NULL)),
    CHECK ((state <> 'closed') OR (teardown_operation_id IS NOT NULL AND teardown_receipt_id IS NOT NULL AND effects_confirmed_at IS NOT NULL)),
    CHECK ((create_request_bytes IS NULL AND create_receipt_bytes IS NULL) OR (create_request_bytes IS NOT NULL AND create_receipt_bytes IS NOT NULL)),
    CHECK ((compaction_operation_id IS NULL AND teardown_aggregate_id IS NULL AND compaction_request_sha256 IS NULL AND compaction_request_bytes IS NULL AND compaction_receipt_id IS NULL AND compaction_receipt_bytes IS NULL AND teardown_receipt_sha256 IS NULL AND aggregate_completed_at IS NULL AND compaction_authorized_at IS NULL)
        OR (compaction_operation_id IS NOT NULL AND teardown_aggregate_id IS NOT NULL AND compaction_request_sha256 IS NOT NULL AND octet_length(compaction_request_sha256) = 32 AND compaction_request_bytes IS NOT NULL AND compaction_request_sha256 = digest(compaction_request_bytes, 'sha256') AND compaction_receipt_id IS NOT NULL AND compaction_receipt_bytes IS NOT NULL AND teardown_receipt_sha256 IS NOT NULL AND octet_length(teardown_receipt_sha256) = 32 AND aggregate_completed_at IS NOT NULL AND compaction_authorized_at IS NOT NULL AND state = 'closed' AND effects_confirmed_at IS NOT NULL AND owner_id = match_id AND create_request_bytes IS NULL AND create_receipt_bytes IS NULL AND teardown_request_bytes IS NULL AND teardown_receipt_bytes IS NULL AND compaction_authorized_at >= aggregate_completed_at + interval '30 days'))
);

CREATE INDEX voice_match_squad_operations_compacted ON voice_match_squad_operations(compaction_authorized_at)
    WHERE compaction_operation_id IS NOT NULL;

CREATE OR REPLACE FUNCTION voice_match_squad_terminal_fence_fn() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'MatchSquad terminal and replay fences are permanent' USING ERRCODE = '55000';
    END IF;
    IF OLD.compaction_operation_id IS NOT NULL THEN
        RAISE EXCEPTION 'MatchSquad compact terminal fences are permanent' USING ERRCODE = '55000';
    END IF;
    IF OLD.state = 'closed' AND NEW.state <> 'closed' THEN
        RAISE EXCEPTION 'MatchSquad terminal fence cannot reopen' USING ERRCODE = '55000';
    END IF;
    IF (OLD.create_request_bytes IS NOT NULL AND NEW.create_request_bytes IS NOT NULL AND NEW.create_request_bytes IS DISTINCT FROM OLD.create_request_bytes)
       OR (OLD.create_receipt_bytes IS NOT NULL AND NEW.create_receipt_bytes IS NOT NULL AND NEW.create_receipt_bytes IS DISTINCT FROM OLD.create_receipt_bytes)
       OR (OLD.teardown_request_bytes IS NOT NULL AND NEW.teardown_request_bytes IS NOT NULL AND NEW.teardown_request_bytes IS DISTINCT FROM OLD.teardown_request_bytes)
       OR (OLD.teardown_receipt_bytes IS NOT NULL AND NEW.teardown_receipt_bytes IS NOT NULL AND NEW.teardown_receipt_bytes IS DISTINCT FROM OLD.teardown_receipt_bytes) THEN
        RAISE EXCEPTION 'MatchSquad operation evidence is immutable' USING ERRCODE = '55000';
    END IF;
    IF OLD.state = 'closed' AND NEW.effects_confirmed_at IS DISTINCT FROM OLD.effects_confirmed_at THEN
        RAISE EXCEPTION 'MatchSquad confirmed terminal effect time is immutable' USING ERRCODE = '55000';
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
    IF NEW.compaction_operation_id IS NOT NULL THEN
        IF OLD.state <> 'closed' OR OLD.effects_confirmed_at IS NULL OR OLD.owner_id <> OLD.match_id OR
           OLD.create_request_bytes IS NULL OR OLD.create_receipt_bytes IS NULL OR OLD.teardown_request_bytes IS NULL OR OLD.teardown_receipt_bytes IS NULL OR
           NEW.compaction_request_bytes IS NULL OR NEW.compaction_receipt_bytes IS NULL OR NEW.compaction_receipt_id IS NULL OR
           NEW.teardown_aggregate_id IS NULL OR NEW.aggregate_completed_at IS NULL OR NEW.compaction_authorized_at IS NULL OR
           NEW.compaction_authorized_at < NEW.aggregate_completed_at + interval '30 days' OR
           NEW.create_request_bytes IS NOT NULL OR NEW.create_receipt_bytes IS NOT NULL OR NEW.teardown_request_bytes IS NOT NULL OR NEW.teardown_receipt_bytes IS NOT NULL THEN
            RAISE EXCEPTION 'MatchSquad compaction command or terminal evidence is invalid' USING ERRCODE = '55000';
        END IF;
        IF NEW.teardown_receipt_sha256 IS DISTINCT FROM digest(OLD.teardown_receipt_bytes, 'sha256') THEN
            RAISE EXCEPTION 'MatchSquad teardown receipt digest is invalid' USING ERRCODE = '55000';
        END IF;
    END IF;
    IF ((OLD.create_request_bytes IS NOT NULL AND NEW.create_request_bytes IS NULL)
        OR (OLD.create_receipt_bytes IS NOT NULL AND NEW.create_receipt_bytes IS NULL)
        OR (OLD.teardown_request_bytes IS NOT NULL AND NEW.teardown_request_bytes IS NULL)
        OR (OLD.teardown_receipt_bytes IS NOT NULL AND NEW.teardown_receipt_bytes IS NULL))
       AND NOT (OLD.compaction_operation_id IS NULL AND NEW.compaction_operation_id IS NOT NULL) THEN
        RAISE EXCEPTION 'MatchSquad operation evidence requires an accepted aggregate compaction command' USING ERRCODE = '55000';
    END IF;
    NEW.updated_at := clock_timestamp();
    RETURN NEW;
END;
$$;
CREATE TRIGGER voice_match_squad_terminal_fence BEFORE UPDATE OR DELETE ON voice_match_squad_operations
    FOR EACH ROW EXECUTE FUNCTION voice_match_squad_terminal_fence_fn();

COMMIT;
