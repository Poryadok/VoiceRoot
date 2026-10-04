CREATE TABLE matchmaking_match_leave_operations (
    actor_profile_id UUID NOT NULL,
    operation_id UUID NOT NULL,
    match_id UUID NOT NULL REFERENCES matches(id),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (actor_profile_id, operation_id)
);

CREATE TABLE matchmaking_match_squad_operations (
    match_id UUID PRIMARY KEY REFERENCES matches(id),
    operation_id UUID NOT NULL UNIQUE,
    participant_manifest_sha256 BYTEA NOT NULL CHECK (octet_length(participant_manifest_sha256) = 32),
    participant_manifest_bytes BYTEA NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('provisioning', 'active', 'compensating', 'closed')),
    chat_operation_id UUID NOT NULL UNIQUE,
    chat_request_sha256 BYTEA NOT NULL CHECK (octet_length(chat_request_sha256) = 32),
    chat_request_bytes BYTEA NOT NULL,
    chat_receipt_id UUID,
    chat_receipt_bytes BYTEA,
    chat_id UUID,
    voice_operation_id UUID NOT NULL UNIQUE,
    voice_request_sha256 BYTEA,
    voice_request_bytes BYTEA,
    voice_receipt_id UUID,
    voice_receipt_bytes BYTEA,
    voice_room_id UUID,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK ((chat_receipt_id IS NULL) = (chat_receipt_bytes IS NULL)),
    CHECK ((chat_receipt_id IS NULL) = (chat_id IS NULL)),
    CHECK ((voice_request_sha256 IS NULL) = (voice_request_bytes IS NULL)),
    CHECK (voice_request_sha256 IS NULL OR octet_length(voice_request_sha256) = 32),
    CHECK ((voice_receipt_id IS NULL) = (voice_receipt_bytes IS NULL)),
    CHECK ((voice_receipt_id IS NULL) = (voice_room_id IS NULL)),
    CHECK (state <> 'active' OR (chat_receipt_id IS NOT NULL AND voice_receipt_id IS NOT NULL))
);

CREATE TABLE matchmaking_match_squad_teardowns (
    aggregate_id UUID PRIMARY KEY,
    match_id UUID NOT NULL UNIQUE REFERENCES matchmaking_match_squad_operations(match_id),
    state TEXT NOT NULL CHECK (state IN ('pending', 'complete')),
    chat_teardown_operation_id UUID NOT NULL UNIQUE,
    chat_teardown_request_sha256 BYTEA NOT NULL CHECK (octet_length(chat_teardown_request_sha256) = 32),
    chat_teardown_request_bytes BYTEA NOT NULL,
    chat_teardown_receipt_id UUID,
    chat_teardown_receipt_bytes BYTEA,
    voice_teardown_operation_id UUID NOT NULL UNIQUE,
    voice_teardown_request_sha256 BYTEA NOT NULL CHECK (octet_length(voice_teardown_request_sha256) = 32),
    voice_teardown_request_bytes BYTEA NOT NULL,
    voice_teardown_receipt_id UUID,
    voice_teardown_receipt_bytes BYTEA,
    aggregate_completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    CHECK ((chat_teardown_receipt_id IS NULL) = (chat_teardown_receipt_bytes IS NULL)),
    CHECK ((voice_teardown_receipt_id IS NULL) = (voice_teardown_receipt_bytes IS NULL)),
    CHECK ((state = 'complete') = (aggregate_completed_at IS NOT NULL)),
    CHECK (state <> 'complete' OR (chat_teardown_receipt_id IS NOT NULL AND voice_teardown_receipt_id IS NOT NULL))
);

CREATE TABLE matchmaking_match_squad_compaction_intents (
    aggregate_id UUID NOT NULL REFERENCES matchmaking_match_squad_teardowns(aggregate_id),
    provider TEXT NOT NULL CHECK (provider IN ('chat', 'voice')),
    operation_id UUID NOT NULL UNIQUE,
    not_before TIMESTAMPTZ NOT NULL,
    request_sha256 BYTEA,
    request_bytes BYTEA,
    receipt_id UUID,
    receipt_bytes BYTEA,
    completed_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (aggregate_id, provider),
    CHECK ((request_sha256 IS NULL) = (request_bytes IS NULL)),
    CHECK (request_sha256 IS NULL OR octet_length(request_sha256) = 32),
    CHECK ((receipt_id IS NULL) = (receipt_bytes IS NULL)),
    CHECK ((receipt_id IS NULL) = (completed_at IS NULL))
);

CREATE INDEX matchmaking_match_squad_compaction_due_idx
    ON matchmaking_match_squad_compaction_intents (not_before, aggregate_id, provider)
    WHERE completed_at IS NULL;

CREATE TABLE matchmaking_match_squad_completion_events (
    aggregate_id UUID PRIMARY KEY REFERENCES matchmaking_match_squad_teardowns(aggregate_id),
    event_id UUID NOT NULL UNIQUE,
    occurred_at TIMESTAMPTZ NOT NULL,
    duration_seconds BIGINT NOT NULL CHECK (duration_seconds >= 0),
    profile_ids JSONB NOT NULL CHECK (jsonb_typeof(profile_ids) = 'array'),
    published_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX matchmaking_match_squad_completion_events_pending_idx
    ON matchmaking_match_squad_completion_events (created_at, aggregate_id)
    WHERE published_at IS NULL;

CREATE FUNCTION guard_matchmaking_match_squad_completion_event() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'cannot delete MatchSquad completion event evidence';
    END IF;
    IF ROW(NEW.aggregate_id, NEW.event_id, NEW.occurred_at, NEW.duration_seconds, NEW.profile_ids)
       IS DISTINCT FROM ROW(OLD.aggregate_id, OLD.event_id, OLD.occurred_at, OLD.duration_seconds, OLD.profile_ids) THEN
        RAISE EXCEPTION 'cannot rewrite MatchSquad completion event identity or payload';
    END IF;
    IF OLD.published_at IS NOT NULL AND NEW.published_at IS DISTINCT FROM OLD.published_at THEN
        RAISE EXCEPTION 'cannot rewrite published MatchSquad completion event';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER matchmaking_match_squad_completion_event_fence
    BEFORE UPDATE OR DELETE ON matchmaking_match_squad_completion_events
    FOR EACH ROW EXECUTE FUNCTION guard_matchmaking_match_squad_completion_event();

CREATE FUNCTION guard_matchmaking_match_squad_operation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'cannot delete MatchSquad provisioning operation or permanent ownership fence';
    END IF;
    IF ROW(NEW.match_id, NEW.operation_id, NEW.participant_manifest_sha256, NEW.participant_manifest_bytes,
           NEW.chat_operation_id, NEW.chat_request_sha256, NEW.chat_request_bytes, NEW.voice_operation_id)
       IS DISTINCT FROM
       ROW(OLD.match_id, OLD.operation_id, OLD.participant_manifest_sha256, OLD.participant_manifest_bytes,
           OLD.chat_operation_id, OLD.chat_request_sha256, OLD.chat_request_bytes, OLD.voice_operation_id) THEN
        RAISE EXCEPTION 'cannot rewrite MatchSquad provisioning identity or original request';
    END IF;
    IF OLD.chat_receipt_id IS NOT NULL AND ROW(NEW.chat_receipt_id, NEW.chat_receipt_bytes, NEW.chat_id)
       IS DISTINCT FROM ROW(OLD.chat_receipt_id, OLD.chat_receipt_bytes, OLD.chat_id) THEN
        RAISE EXCEPTION 'cannot rewrite persisted Chat receipt';
    END IF;
    IF OLD.voice_request_bytes IS NOT NULL AND ROW(NEW.voice_request_sha256, NEW.voice_request_bytes)
       IS DISTINCT FROM ROW(OLD.voice_request_sha256, OLD.voice_request_bytes) THEN
        RAISE EXCEPTION 'cannot rewrite persisted Voice request';
    END IF;
    IF OLD.voice_receipt_id IS NOT NULL AND ROW(NEW.voice_receipt_id, NEW.voice_receipt_bytes, NEW.voice_room_id)
       IS DISTINCT FROM ROW(OLD.voice_receipt_id, OLD.voice_receipt_bytes, OLD.voice_room_id) THEN
        RAISE EXCEPTION 'cannot rewrite persisted Voice receipt';
    END IF;
    IF OLD.state <> NEW.state AND NOT (
        (OLD.state = 'provisioning' AND NEW.state IN ('active', 'compensating', 'closed')) OR
        (OLD.state = 'compensating' AND NEW.state = 'closed') OR
        (OLD.state = 'active' AND NEW.state = 'closed')
    ) THEN
        RAISE EXCEPTION 'invalid MatchSquad provisioning state transition';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER matchmaking_match_squad_operation_fence
    BEFORE UPDATE OR DELETE ON matchmaking_match_squad_operations
    FOR EACH ROW EXECUTE FUNCTION guard_matchmaking_match_squad_operation();

CREATE FUNCTION guard_matchmaking_match_squad_lifecycle() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'cannot delete MatchSquad lifecycle evidence';
    END IF;
    IF ROW(NEW.aggregate_id, NEW.match_id, NEW.chat_teardown_operation_id,
           NEW.chat_teardown_request_sha256, NEW.chat_teardown_request_bytes,
           NEW.voice_teardown_operation_id, NEW.voice_teardown_request_sha256,
           NEW.voice_teardown_request_bytes)
       IS DISTINCT FROM
       ROW(OLD.aggregate_id, OLD.match_id, OLD.chat_teardown_operation_id,
           OLD.chat_teardown_request_sha256, OLD.chat_teardown_request_bytes,
           OLD.voice_teardown_operation_id, OLD.voice_teardown_request_sha256,
           OLD.voice_teardown_request_bytes) THEN
        RAISE EXCEPTION 'cannot rewrite MatchSquad teardown identity or original requests';
    END IF;
    IF OLD.chat_teardown_receipt_id IS NOT NULL AND ROW(NEW.chat_teardown_receipt_id, NEW.chat_teardown_receipt_bytes)
       IS DISTINCT FROM ROW(OLD.chat_teardown_receipt_id, OLD.chat_teardown_receipt_bytes) THEN
        RAISE EXCEPTION 'cannot rewrite persisted Chat MatchSquad teardown receipt';
    END IF;
    IF OLD.voice_teardown_receipt_id IS NOT NULL AND ROW(NEW.voice_teardown_receipt_id, NEW.voice_teardown_receipt_bytes)
       IS DISTINCT FROM ROW(OLD.voice_teardown_receipt_id, OLD.voice_teardown_receipt_bytes) THEN
        RAISE EXCEPTION 'cannot rewrite persisted Voice MatchSquad teardown receipt';
    END IF;
    IF OLD.state = 'complete' AND (NEW.state <> OLD.state OR NEW.aggregate_completed_at IS DISTINCT FROM OLD.aggregate_completed_at) THEN
        RAISE EXCEPTION 'cannot rewrite completed MatchSquad teardown aggregate';
    END IF;
    RETURN NEW;
END;
$$;

CREATE FUNCTION guard_matchmaking_match_squad_compaction() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP = 'DELETE' THEN
        RAISE EXCEPTION 'cannot delete MatchSquad compaction evidence';
    END IF;
    IF ROW(NEW.aggregate_id, NEW.provider, NEW.operation_id, NEW.not_before)
       IS DISTINCT FROM ROW(OLD.aggregate_id, OLD.provider, OLD.operation_id, OLD.not_before) THEN
        RAISE EXCEPTION 'cannot rewrite MatchSquad compaction identity or due time';
    END IF;
    IF OLD.request_bytes IS NOT NULL AND NEW.request_bytes IS DISTINCT FROM OLD.request_bytes THEN
        RAISE EXCEPTION 'cannot rewrite persisted MatchSquad compaction request';
    END IF;
    IF OLD.receipt_bytes IS NOT NULL AND NEW.receipt_bytes IS DISTINCT FROM OLD.receipt_bytes THEN
        RAISE EXCEPTION 'cannot rewrite persisted MatchSquad compaction receipt';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER matchmaking_match_squad_teardown_fence
    BEFORE UPDATE OR DELETE ON matchmaking_match_squad_teardowns
    FOR EACH ROW EXECUTE FUNCTION guard_matchmaking_match_squad_lifecycle();

CREATE TRIGGER matchmaking_match_squad_compaction_fence
    BEFORE UPDATE OR DELETE ON matchmaking_match_squad_compaction_intents
    FOR EACH ROW EXECUTE FUNCTION guard_matchmaking_match_squad_compaction();

CREATE FUNCTION require_matchmaking_match_squad_compaction_due() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    aggregate_state TEXT;
    completed_at TIMESTAMPTZ;
BEGIN
    SELECT state, aggregate_completed_at INTO aggregate_state, completed_at
    FROM matchmaking_match_squad_teardowns
    WHERE aggregate_id = NEW.aggregate_id;
    IF aggregate_state IS DISTINCT FROM 'complete' OR completed_at IS NULL OR
       NEW.not_before IS DISTINCT FROM completed_at + INTERVAL '30 days' THEN
        RAISE EXCEPTION 'MatchSquad compaction intent must follow aggregate completion by exactly 30 days';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER matchmaking_match_squad_compaction_due_guard
    BEFORE INSERT OR UPDATE OF aggregate_id, not_before ON matchmaking_match_squad_compaction_intents
    FOR EACH ROW EXECUTE FUNCTION require_matchmaking_match_squad_compaction_due();
