CREATE TABLE chat_match_squad_operations (
    operation_id UUID PRIMARY KEY,
    match_id UUID NOT NULL UNIQUE,
    chat_id UUID NOT NULL UNIQUE,
    creation_receipt_id UUID NOT NULL UNIQUE,
    participant_manifest_sha256 BYTEA NOT NULL CHECK (octet_length(participant_manifest_sha256) = 32),
    creation_request_sha256 BYTEA NOT NULL CHECK (octet_length(creation_request_sha256) = 32),
    creation_request_bytes BYTEA,
    creation_receipt_bytes BYTEA,
    created_at TIMESTAMPTZ NOT NULL,
    teardown_operation_id UUID UNIQUE,
    teardown_request_sha256 BYTEA CHECK (teardown_request_sha256 IS NULL OR octet_length(teardown_request_sha256) = 32),
    teardown_request_bytes BYTEA,
    teardown_receipt_id UUID UNIQUE,
    teardown_receipt_bytes BYTEA,
    teardown_completed_at TIMESTAMPTZ,
    compaction_operation_id UUID UNIQUE,
    teardown_aggregate_id UUID UNIQUE,
    compaction_request_sha256 BYTEA CHECK (compaction_request_sha256 IS NULL OR octet_length(compaction_request_sha256) = 32),
    compaction_request_bytes BYTEA,
    compaction_receipt_id UUID UNIQUE,
    compaction_receipt_bytes BYTEA,
    aggregate_completed_at TIMESTAMPTZ,
    compaction_authorized_at TIMESTAMPTZ,
    compacted_at TIMESTAMPTZ,
    CHECK (
      (compacted_at IS NULL AND creation_request_bytes IS NOT NULL AND creation_receipt_bytes IS NOT NULL AND
        ((teardown_operation_id IS NULL AND teardown_request_sha256 IS NULL AND teardown_request_bytes IS NULL
          AND teardown_receipt_id IS NULL AND teardown_receipt_bytes IS NULL AND teardown_completed_at IS NULL
          AND compaction_operation_id IS NULL AND teardown_aggregate_id IS NULL AND compaction_request_sha256 IS NULL
          AND compaction_request_bytes IS NULL AND compaction_receipt_id IS NULL AND compaction_receipt_bytes IS NULL
          AND aggregate_completed_at IS NULL AND compaction_authorized_at IS NULL)
        OR
        (teardown_operation_id IS NOT NULL AND teardown_request_sha256 IS NOT NULL AND teardown_request_bytes IS NOT NULL
          AND teardown_receipt_id IS NOT NULL AND teardown_receipt_bytes IS NOT NULL AND teardown_completed_at IS NOT NULL
          AND compaction_operation_id IS NULL AND teardown_aggregate_id IS NULL AND compaction_request_sha256 IS NULL
          AND compaction_request_bytes IS NULL AND compaction_receipt_id IS NULL AND compaction_receipt_bytes IS NULL
          AND aggregate_completed_at IS NULL AND compaction_authorized_at IS NULL)))
      OR
      (compacted_at IS NOT NULL AND creation_request_bytes IS NULL AND creation_receipt_bytes IS NULL
        AND teardown_operation_id IS NOT NULL AND teardown_request_sha256 IS NOT NULL AND teardown_request_bytes IS NULL
        AND teardown_receipt_id IS NOT NULL AND teardown_receipt_bytes IS NULL AND teardown_completed_at IS NOT NULL
        AND compaction_operation_id IS NOT NULL AND teardown_aggregate_id IS NOT NULL
        AND compaction_request_sha256 IS NOT NULL AND compaction_request_bytes IS NOT NULL
        AND compaction_receipt_id IS NOT NULL AND compaction_receipt_bytes IS NOT NULL
        AND aggregate_completed_at IS NOT NULL AND compaction_authorized_at IS NOT NULL
        AND compaction_authorized_at >= aggregate_completed_at + INTERVAL '30 days')
    ),
    CHECK (compacted_at IS NULL OR teardown_completed_at IS NOT NULL)
);

CREATE FUNCTION reject_match_squad_chat_delete() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF EXISTS (SELECT 1 FROM chat_match_squad_operations WHERE chat_id = OLD.id) THEN
        RAISE EXCEPTION 'MatchSquad Chat resource is protected by its permanent ownership fence';
    END IF;
    RETURN OLD;
END
$$;

CREATE TRIGGER chat_match_squad_delete_fence
    BEFORE DELETE ON chats FOR EACH ROW EXECUTE FUNCTION reject_match_squad_chat_delete();

CREATE FUNCTION guard_match_squad_operation_update() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.teardown_operation_id IS NULL THEN
        IF NEW.teardown_operation_id IS NULL OR
           ROW(NEW.operation_id, NEW.match_id, NEW.chat_id, NEW.creation_receipt_id,
               NEW.participant_manifest_sha256, NEW.creation_request_sha256, NEW.creation_request_bytes,
               NEW.creation_receipt_bytes, NEW.created_at, NEW.compaction_operation_id,
               NEW.teardown_aggregate_id, NEW.compaction_request_sha256, NEW.compaction_request_bytes,
               NEW.compaction_receipt_id, NEW.compaction_receipt_bytes, NEW.aggregate_completed_at,
               NEW.compaction_authorized_at, NEW.compacted_at)
           IS DISTINCT FROM
           ROW(OLD.operation_id, OLD.match_id, OLD.chat_id, OLD.creation_receipt_id,
               OLD.participant_manifest_sha256, OLD.creation_request_sha256, OLD.creation_request_bytes,
               OLD.creation_receipt_bytes, OLD.created_at, OLD.compaction_operation_id,
               OLD.teardown_aggregate_id, OLD.compaction_request_sha256, OLD.compaction_request_bytes,
               OLD.compaction_receipt_id, OLD.compaction_receipt_bytes, OLD.aggregate_completed_at,
               OLD.compaction_authorized_at, OLD.compacted_at) THEN
            RAISE EXCEPTION 'MatchSquad creation evidence is immutable';
        END IF;
    ELSIF OLD.compaction_operation_id IS NULL THEN
        IF NEW.compaction_operation_id IS NULL OR NEW.compacted_at IS NULL
           OR NEW.creation_request_bytes IS NOT NULL OR NEW.creation_receipt_bytes IS NOT NULL
           OR NEW.teardown_request_bytes IS NOT NULL OR NEW.teardown_receipt_bytes IS NOT NULL
           OR ROW(NEW.operation_id, NEW.match_id, NEW.chat_id, NEW.creation_receipt_id,
                  NEW.participant_manifest_sha256, NEW.creation_request_sha256, NEW.created_at,
                  NEW.teardown_operation_id, NEW.teardown_request_sha256, NEW.teardown_receipt_id,
                  NEW.teardown_completed_at)
              IS DISTINCT FROM
              ROW(OLD.operation_id, OLD.match_id, OLD.chat_id, OLD.creation_receipt_id,
                  OLD.participant_manifest_sha256, OLD.creation_request_sha256, OLD.created_at,
                  OLD.teardown_operation_id, OLD.teardown_request_sha256, OLD.teardown_receipt_id,
                  OLD.teardown_completed_at) THEN
            RAISE EXCEPTION 'MatchSquad compaction must be one authorized terminal transition';
        END IF;
    ELSE
        RAISE EXCEPTION 'MatchSquad terminal evidence is immutable';
    END IF;
    RETURN NEW;
END
$$;

CREATE TRIGGER chat_match_squad_operation_update_guard
    BEFORE UPDATE ON chat_match_squad_operations FOR EACH ROW EXECUTE FUNCTION guard_match_squad_operation_update();

CREATE FUNCTION reject_match_squad_operation_delete() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'MatchSquad operation and terminal fences are permanent';
    RETURN OLD;
END
$$;

CREATE TRIGGER chat_match_squad_operation_delete_fence
    BEFORE DELETE ON chat_match_squad_operations FOR EACH ROW EXECUTE FUNCTION reject_match_squad_operation_delete();
