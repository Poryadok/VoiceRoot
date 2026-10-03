-- Durable Space admission fence for queue sessions and match proposals.
CREATE TABLE IF NOT EXISTS matchmaking_space_lifecycle_fence_heads (
    space_id UUID PRIMARY KEY,
    deletion_operation_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation >= 0),
    state TEXT NOT NULL CHECK (state IN ('LIVE', 'FROZEN', 'PURGE_DECIDED', 'PURGED')),
    manifest_id TEXT NOT NULL,
    manifest_sha256 BYTEA NOT NULL CHECK (octet_length(manifest_sha256) = 32),
    manifest_item_count BIGINT NOT NULL CHECK (manifest_item_count >= 0),
    applied_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    purged_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS matchmaking_space_lifecycle_fence_receipts (
    space_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    request_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    receipt_bytes BYTEA NOT NULL,
    receipt_sha256 BYTEA NOT NULL CHECK (octet_length(receipt_sha256) = 32),
    applied_at TIMESTAMPTZ NOT NULL,
    retain_until TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (space_id, generation)
);

CREATE TABLE IF NOT EXISTS matchmaking_space_lifecycle_purge_receipts (
    space_id UUID NOT NULL,
    generation BIGINT NOT NULL CHECK (generation > 0),
    request_bytes BYTEA NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    receipt_bytes BYTEA NOT NULL,
    receipt_sha256 BYTEA NOT NULL CHECK (octet_length(receipt_sha256) = 32),
    completed_at TIMESTAMPTZ NOT NULL,
    retain_until TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (space_id, generation)
);

CREATE OR REPLACE FUNCTION matchmaking_gate_space_search_session()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    fence_state TEXT;
BEGIN
    IF NEW.space_id IS NULL OR NEW.status NOT IN ('searching', 'pending_accept', 'matched') THEN
        RETURN NEW;
    END IF;

    -- The placeholder head makes first admission and first lifecycle transition
    -- serialize on the same row even before a Space has ever been fenced.
    PERFORM pg_advisory_xact_lock(hashtextextended(NEW.space_id::text, 0));
    INSERT INTO matchmaking_space_lifecycle_fence_heads (
        space_id, deletion_operation_id, generation, state,
        manifest_id, manifest_sha256, manifest_item_count
    ) VALUES (
        NEW.space_id, '00000000-0000-0000-0000-000000000000', 0, 'LIVE',
        'unfenced', decode(repeat('00', 32), 'hex'), 0
    ) ON CONFLICT (space_id) DO NOTHING;

    SELECT state INTO fence_state
    FROM matchmaking_space_lifecycle_fence_heads
    WHERE space_id = NEW.space_id
    FOR KEY SHARE;

    IF fence_state IS DISTINCT FROM 'LIVE' THEN
        RAISE EXCEPTION 'Space matchmaking lifecycle fence denies active session mutation'
            USING ERRCODE = '55000';
    END IF;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS matchmaking_space_search_session_fence ON search_sessions;
CREATE TRIGGER matchmaking_space_search_session_fence
BEFORE INSERT OR UPDATE OF status, space_id ON search_sessions
FOR EACH ROW EXECUTE FUNCTION matchmaking_gate_space_search_session();

CREATE OR REPLACE FUNCTION matchmaking_gate_space_match_result()
RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    session_row RECORD;
    fence_state TEXT;
BEGIN
    IF TG_OP = 'INSERT' OR NEW.status IS NOT DISTINCT FROM OLD.status THEN
        RETURN NEW;
    END IF;

    -- Matches do not own space_id. Their immutable participant session IDs are
    -- the authoritative link to a Space-scoped search.
    FOR session_row IN
        SELECT DISTINCT s.space_id
        FROM search_sessions s
        JOIN jsonb_array_elements(NEW.participants) p
          ON p->>'session_id' = s.id::text
        WHERE s.space_id IS NOT NULL AND s.status IN ('searching', 'pending_accept', 'matched')
        ORDER BY s.space_id
    LOOP
        PERFORM pg_advisory_xact_lock(hashtextextended(session_row.space_id::text, 0));
        SELECT state INTO fence_state
        FROM matchmaking_space_lifecycle_fence_heads
        WHERE space_id = session_row.space_id
        FOR KEY SHARE;
        IF fence_state IS DISTINCT FROM 'LIVE' THEN
            RAISE EXCEPTION 'Space matchmaking lifecycle fence denies match result mutation'
                USING ERRCODE = '55000';
        END IF;
    END LOOP;
    RETURN NEW;
END;
$$;

DROP TRIGGER IF EXISTS matchmaking_space_match_result_fence ON matches;
CREATE TRIGGER matchmaking_space_match_result_fence
BEFORE UPDATE OF status, completed_at, chat_id, voice_room_id ON matches
FOR EACH ROW EXECUTE FUNCTION matchmaking_gate_space_match_result();
