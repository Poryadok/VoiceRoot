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
    compacted_at TIMESTAMPTZ,
    CHECK (
      (teardown_operation_id IS NULL AND teardown_request_sha256 IS NULL AND teardown_request_bytes IS NULL
        AND teardown_receipt_id IS NULL AND teardown_receipt_bytes IS NULL AND teardown_completed_at IS NULL)
      OR
      (teardown_operation_id IS NOT NULL AND teardown_request_sha256 IS NOT NULL AND teardown_receipt_id IS NOT NULL
        AND teardown_completed_at IS NOT NULL AND
        ((compacted_at IS NULL AND teardown_request_bytes IS NOT NULL AND teardown_receipt_bytes IS NOT NULL)
          OR (compacted_at IS NOT NULL AND teardown_request_bytes IS NULL AND teardown_receipt_bytes IS NULL)))
    ),
    CHECK ((creation_request_bytes IS NULL) = (compacted_at IS NOT NULL)),
    CHECK ((creation_receipt_bytes IS NULL) = (compacted_at IS NOT NULL)),
    CHECK (compacted_at IS NULL OR teardown_completed_at IS NOT NULL)
);

CREATE INDEX chat_match_squad_compaction_idx
    ON chat_match_squad_operations (teardown_completed_at)
    WHERE teardown_completed_at IS NOT NULL AND compacted_at IS NULL;

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
