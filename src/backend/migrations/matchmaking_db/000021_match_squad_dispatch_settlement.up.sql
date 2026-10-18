BEGIN;

ALTER TABLE matchmaking_match_squad_operations
    ADD COLUMN dispatch_protocol_version SMALLINT NOT NULL DEFAULT 0 CHECK (dispatch_protocol_version IN (0,1)),
    ADD COLUMN provision_disposition TEXT NOT NULL DEFAULT 'LIVE' CHECK (provision_disposition IN ('LIVE','ABANDONED')),
    ADD COLUMN provision_revision BIGINT NOT NULL DEFAULT 1 CHECK (provision_revision > 0),
    ADD COLUMN provision_abandoned_at TIMESTAMPTZ,
    ADD COLUMN provision_abandon_reason TEXT,
    ADD COLUMN provision_abandon_evidence_sha256 BYTEA,
    ADD COLUMN chat_dispatch_state TEXT NOT NULL DEFAULT 'NOT_ISSUED' CHECK (chat_dispatch_state IN ('NOT_ISSUED','MAY_HAVE_CREATED','RECEIPT_KNOWN')),
    ADD COLUMN chat_first_dispatch_id UUID,
    ADD COLUMN chat_first_dispatch_at TIMESTAMPTZ,
    ADD COLUMN voice_dispatch_state TEXT NOT NULL DEFAULT 'NOT_ISSUED' CHECK (voice_dispatch_state IN ('NOT_ISSUED','MAY_HAVE_CREATED','RECEIPT_KNOWN')),
    ADD COLUMN voice_first_dispatch_id UUID,
    ADD COLUMN voice_first_dispatch_at TIMESTAMPTZ;

-- Historical frozen requests may already be in flight. No receipt is not a
-- no-create proof. A legacy stored Voice request also counts as possibly issued.
UPDATE matchmaking_match_squad_operations SET
    chat_dispatch_state=CASE WHEN chat_receipt_id IS NULL THEN 'MAY_HAVE_CREATED' ELSE 'RECEIPT_KNOWN' END,
    chat_first_dispatch_id=gen_random_uuid(),chat_first_dispatch_at=created_at,
    voice_dispatch_state=CASE WHEN voice_receipt_id IS NOT NULL THEN 'RECEIPT_KNOWN'
        WHEN voice_request_bytes IS NOT NULL THEN 'MAY_HAVE_CREATED' ELSE 'NOT_ISSUED' END,
    voice_first_dispatch_id=CASE WHEN voice_request_bytes IS NOT NULL THEN gen_random_uuid() END,
    voice_first_dispatch_at=CASE WHEN voice_request_bytes IS NOT NULL THEN created_at END;

ALTER TABLE matchmaking_match_squad_operations
    ADD CONSTRAINT matchmaking_provision_abandon_binding CHECK (
      (provision_disposition='LIVE' AND provision_abandoned_at IS NULL AND provision_abandon_reason IS NULL AND provision_abandon_evidence_sha256 IS NULL) OR
      (provision_disposition='ABANDONED' AND provision_abandoned_at IS NOT NULL AND provision_abandon_reason IS NOT NULL AND provision_abandon_evidence_sha256 IS NOT NULL AND octet_length(provision_abandon_evidence_sha256)=32)
    ),
    ADD CONSTRAINT matchmaking_provision_chat_dispatch_binding CHECK (
      (chat_dispatch_state='NOT_ISSUED' AND chat_first_dispatch_id IS NULL AND chat_first_dispatch_at IS NULL AND chat_receipt_id IS NULL) OR
      (chat_dispatch_state='MAY_HAVE_CREATED' AND chat_first_dispatch_id IS NOT NULL AND chat_first_dispatch_at IS NOT NULL AND chat_receipt_id IS NULL) OR
      (chat_dispatch_state='RECEIPT_KNOWN' AND chat_first_dispatch_id IS NOT NULL AND chat_first_dispatch_at IS NOT NULL AND chat_receipt_id IS NOT NULL)
    ),
    ADD CONSTRAINT matchmaking_provision_voice_dispatch_binding CHECK (
      (voice_dispatch_state='NOT_ISSUED' AND voice_first_dispatch_id IS NULL AND voice_first_dispatch_at IS NULL AND voice_receipt_id IS NULL) OR
      (voice_dispatch_state='MAY_HAVE_CREATED' AND voice_first_dispatch_id IS NOT NULL AND voice_first_dispatch_at IS NOT NULL AND voice_receipt_id IS NULL AND voice_request_bytes IS NOT NULL) OR
      (voice_dispatch_state='RECEIPT_KNOWN' AND voice_first_dispatch_id IS NOT NULL AND voice_first_dispatch_at IS NOT NULL AND voice_receipt_id IS NOT NULL)
    ),
    ADD CONSTRAINT matchmaking_provision_voice_dispatch_prerequisite CHECK (
      voice_dispatch_state='NOT_ISSUED' OR chat_dispatch_state='RECEIPT_KNOWN'
    );

-- A possibly dispatched create has a cleanup obligation even before its
-- creation receipt identifies the resource. These slots are not runnable
-- teardown requests and cannot satisfy aggregate completion.
ALTER TABLE matchmaking_match_squad_teardown_participants
    DROP CONSTRAINT matchmaking_match_squad_teardown_participants_state_check,
    ALTER COLUMN request_sha256 DROP NOT NULL,
    ALTER COLUMN request_bytes DROP NOT NULL,
    ADD CONSTRAINT matchmaking_match_squad_teardown_participants_state_check CHECK
      (state IN ('WAITING_FOR_CREATION','NOT_STARTED','IN_FLIGHT','COMPLETE','RETRYABLE_FAILURE','CONTRACT_MISMATCH')),
    ADD CONSTRAINT matchmaking_match_squad_participant_request_binding CHECK (
      (state='WAITING_FOR_CREATION' AND request_sha256 IS NULL AND request_bytes IS NULL AND receipt_id IS NULL) OR
      (state<>'WAITING_FOR_CREATION' AND request_sha256 IS NOT NULL AND request_bytes IS NOT NULL)
    );

ALTER TABLE matchmaking_match_squad_teardowns
    DROP CONSTRAINT matchmaking_match_squad_teardown_required_set_check,
    DROP CONSTRAINT matchmaking_match_squad_teardown_chat_binding_check,
    DROP CONSTRAINT matchmaking_match_squad_teardown_voice_binding_check,
    DROP CONSTRAINT matchmaking_match_squad_teardown_completion_check,
    ADD CONSTRAINT matchmaking_match_squad_teardown_required_set_check CHECK (
      (purpose='FINAL_LEAVE' AND required_providers=ARRAY['chat','voice']::TEXT[]) OR
      (purpose='PROVISION_COMPENSATION' AND required_providers IN
        (ARRAY['chat']::TEXT[],ARRAY['chat','voice']::TEXT[]))
    ),
    ADD CONSTRAINT matchmaking_match_squad_teardown_chat_binding_check CHECK (
      ('chat'=ANY(required_providers) AND chat_teardown_operation_id IS NOT NULL AND
        ((chat_teardown_request_sha256 IS NOT NULL AND chat_teardown_request_bytes IS NOT NULL) OR
         (purpose='PROVISION_COMPENSATION' AND state='pending' AND chat_teardown_request_sha256 IS NULL AND chat_teardown_request_bytes IS NULL AND chat_teardown_receipt_id IS NULL))) OR
      (NOT ('chat'=ANY(required_providers)) AND chat_teardown_operation_id IS NULL AND chat_teardown_request_sha256 IS NULL AND chat_teardown_request_bytes IS NULL AND chat_teardown_receipt_id IS NULL)
    ),
    ADD CONSTRAINT matchmaking_match_squad_teardown_voice_binding_check CHECK (
      ('voice'=ANY(required_providers) AND voice_teardown_operation_id IS NOT NULL AND
        ((voice_teardown_request_sha256 IS NOT NULL AND voice_teardown_request_bytes IS NOT NULL) OR
         (purpose='PROVISION_COMPENSATION' AND state='pending' AND voice_teardown_request_sha256 IS NULL AND voice_teardown_request_bytes IS NULL AND voice_teardown_receipt_id IS NULL))) OR
      (NOT ('voice'=ANY(required_providers)) AND voice_teardown_operation_id IS NULL AND voice_teardown_request_sha256 IS NULL AND voice_teardown_request_bytes IS NULL AND voice_teardown_receipt_id IS NULL)
    ),
    ADD CONSTRAINT matchmaking_match_squad_teardown_completion_check CHECK (
      state<>'complete' OR
      ((NOT ('chat'=ANY(required_providers)) OR chat_teardown_receipt_id IS NOT NULL) AND
       (NOT ('voice'=ANY(required_providers)) OR voice_teardown_receipt_id IS NOT NULL))
    );

CREATE FUNCTION guard_matchmaking_match_squad_dispatch() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF OLD.dispatch_protocol_version <> NEW.dispatch_protocol_version THEN
        RAISE EXCEPTION 'cannot rewrite MatchSquad dispatch protocol';
    END IF;
    IF OLD.provision_disposition='ABANDONED' AND
       ROW(NEW.provision_disposition,NEW.provision_abandoned_at,NEW.provision_abandon_reason,NEW.provision_abandon_evidence_sha256)
       IS DISTINCT FROM ROW(OLD.provision_disposition,OLD.provision_abandoned_at,OLD.provision_abandon_reason,OLD.provision_abandon_evidence_sha256) THEN
        RAISE EXCEPTION 'cannot reopen or rewrite abandoned MatchSquad provisioning';
    END IF;
    IF (OLD.chat_dispatch_state<>'NOT_ISSUED' AND
        (NEW.chat_dispatch_state='NOT_ISSUED' OR ROW(NEW.chat_first_dispatch_id,NEW.chat_first_dispatch_at)
         IS DISTINCT FROM ROW(OLD.chat_first_dispatch_id,OLD.chat_first_dispatch_at))) OR
       (OLD.voice_dispatch_state<>'NOT_ISSUED' AND
        (NEW.voice_dispatch_state='NOT_ISSUED' OR ROW(NEW.voice_first_dispatch_id,NEW.voice_first_dispatch_at)
         IS DISTINCT FROM ROW(OLD.voice_first_dispatch_id,OLD.voice_first_dispatch_at))) OR
       (OLD.chat_dispatch_state='RECEIPT_KNOWN' AND NEW.chat_dispatch_state<>'RECEIPT_KNOWN') OR
       (OLD.voice_dispatch_state='RECEIPT_KNOWN' AND NEW.voice_dispatch_state<>'RECEIPT_KNOWN') THEN
        RAISE EXCEPTION 'cannot forget MatchSquad possible dispatch or creation receipt';
    END IF;
    IF OLD.provision_disposition='ABANDONED' AND
       ((OLD.chat_dispatch_state='NOT_ISSUED' AND NEW.chat_dispatch_state<>'NOT_ISSUED') OR
        (OLD.voice_dispatch_state='NOT_ISSUED' AND NEW.voice_dispatch_state<>'NOT_ISSUED')) THEN
        RAISE EXCEPTION 'cannot dispatch an unissued provider after abandonment';
    END IF;
    IF NEW.state='active' AND NEW.provision_disposition<>'LIVE' THEN
        RAISE EXCEPTION 'cannot activate abandoned MatchSquad provisioning';
    END IF;
    IF NEW.provision_revision<OLD.provision_revision OR
       (ROW(NEW.provision_disposition,NEW.chat_dispatch_state,NEW.voice_dispatch_state)
        IS DISTINCT FROM ROW(OLD.provision_disposition,OLD.chat_dispatch_state,OLD.voice_dispatch_state)
        AND NEW.provision_revision<=OLD.provision_revision) THEN
        RAISE EXCEPTION 'MatchSquad dispatch revision must advance';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER matchmaking_match_squad_dispatch_fence
    BEFORE UPDATE ON matchmaking_match_squad_operations
    FOR EACH ROW EXECUTE FUNCTION guard_matchmaking_match_squad_dispatch();

CREATE FUNCTION require_matchmaking_match_squad_abandoned_obligation() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    disposition TEXT;
    chat_dispatch TEXT;
    voice_dispatch TEXT;
    expected TEXT[] := ARRAY[]::TEXT[];
BEGIN
    IF NEW.purpose<>'PROVISION_COMPENSATION' THEN
        RETURN NEW;
    END IF;
    SELECT provision_disposition,chat_dispatch_state,voice_dispatch_state
      INTO disposition,chat_dispatch,voice_dispatch
      FROM matchmaking_match_squad_operations WHERE match_id=NEW.match_id;
    IF disposition IS DISTINCT FROM 'ABANDONED' THEN
        RAISE EXCEPTION 'compensation requires abandoned dispatch evidence';
    END IF;
    IF chat_dispatch<>'NOT_ISSUED' THEN expected := array_append(expected,'chat'); END IF;
    IF voice_dispatch<>'NOT_ISSUED' THEN expected := array_append(expected,'voice'); END IF;
    IF cardinality(expected)=0 OR NEW.required_providers IS DISTINCT FROM expected THEN
        RAISE EXCEPTION 'compensation must preserve exact issued provider set';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER matchmaking_match_squad_abandoned_obligation_guard
    BEFORE INSERT ON matchmaking_match_squad_teardowns
    FOR EACH ROW EXECUTE FUNCTION require_matchmaking_match_squad_abandoned_obligation();

-- Preserve predecessor trigger functions verbatim for a guarded empty DOWN.
-- The sole permitted request transition is unresolved NULL -> exact frozen
-- bytes after the required provider's creation binding has become known.
ALTER FUNCTION guard_matchmaking_match_squad_lifecycle() RENAME TO guard_matchmaking_match_squad_lifecycle_pre021;
DROP TRIGGER matchmaking_match_squad_teardown_fence ON matchmaking_match_squad_teardowns;
CREATE FUNCTION guard_matchmaking_match_squad_lifecycle() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN RAISE EXCEPTION 'cannot delete MatchSquad lifecycle evidence'; END IF;
    IF ROW(NEW.aggregate_id,NEW.match_id,NEW.chat_teardown_operation_id,NEW.voice_teardown_operation_id)
       IS DISTINCT FROM ROW(OLD.aggregate_id,OLD.match_id,OLD.chat_teardown_operation_id,OLD.voice_teardown_operation_id) THEN
        RAISE EXCEPTION 'cannot rewrite MatchSquad teardown identity';
    END IF;
    IF ROW(NEW.chat_teardown_request_sha256,NEW.chat_teardown_request_bytes)
       IS DISTINCT FROM ROW(OLD.chat_teardown_request_sha256,OLD.chat_teardown_request_bytes) AND NOT (
       OLD.chat_teardown_request_sha256 IS NULL AND OLD.chat_teardown_request_bytes IS NULL AND
       NEW.chat_teardown_request_sha256 IS NOT NULL AND NEW.chat_teardown_request_bytes IS NOT NULL AND
       OLD.purpose='PROVISION_COMPENSATION' AND OLD.state='pending' AND 'chat'=ANY(OLD.required_providers) AND
       EXISTS (SELECT 1 FROM matchmaking_match_squad_operations o WHERE o.match_id=OLD.match_id
         AND o.provision_disposition='ABANDONED' AND o.chat_dispatch_state='RECEIPT_KNOWN' AND o.chat_receipt_id IS NOT NULL)) THEN
        RAISE EXCEPTION 'cannot rewrite MatchSquad Chat teardown request';
    END IF;
    IF ROW(NEW.voice_teardown_request_sha256,NEW.voice_teardown_request_bytes)
       IS DISTINCT FROM ROW(OLD.voice_teardown_request_sha256,OLD.voice_teardown_request_bytes) AND NOT (
       OLD.voice_teardown_request_sha256 IS NULL AND OLD.voice_teardown_request_bytes IS NULL AND
       NEW.voice_teardown_request_sha256 IS NOT NULL AND NEW.voice_teardown_request_bytes IS NOT NULL AND
       OLD.purpose='PROVISION_COMPENSATION' AND OLD.state='pending' AND 'voice'=ANY(OLD.required_providers) AND
       EXISTS (SELECT 1 FROM matchmaking_match_squad_operations o WHERE o.match_id=OLD.match_id
         AND o.provision_disposition='ABANDONED' AND o.voice_dispatch_state='RECEIPT_KNOWN' AND o.voice_receipt_id IS NOT NULL)) THEN
        RAISE EXCEPTION 'cannot rewrite MatchSquad Voice teardown request';
    END IF;
    IF OLD.chat_teardown_receipt_id IS NOT NULL AND ROW(NEW.chat_teardown_receipt_id,NEW.chat_teardown_receipt_bytes)
       IS DISTINCT FROM ROW(OLD.chat_teardown_receipt_id,OLD.chat_teardown_receipt_bytes) THEN
        RAISE EXCEPTION 'cannot rewrite persisted Chat MatchSquad teardown receipt';
    END IF;
    IF OLD.voice_teardown_receipt_id IS NOT NULL AND ROW(NEW.voice_teardown_receipt_id,NEW.voice_teardown_receipt_bytes)
       IS DISTINCT FROM ROW(OLD.voice_teardown_receipt_id,OLD.voice_teardown_receipt_bytes) THEN
        RAISE EXCEPTION 'cannot rewrite persisted Voice MatchSquad teardown receipt';
    END IF;
    IF OLD.state='complete' AND (NEW.state<>OLD.state OR NEW.aggregate_completed_at IS DISTINCT FROM OLD.aggregate_completed_at) THEN
        RAISE EXCEPTION 'cannot rewrite completed MatchSquad teardown aggregate';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER matchmaking_match_squad_teardown_fence BEFORE UPDATE OR DELETE ON matchmaking_match_squad_teardowns
    FOR EACH ROW EXECUTE FUNCTION guard_matchmaking_match_squad_lifecycle();

ALTER FUNCTION guard_matchmaking_match_squad_teardown_participant() RENAME TO guard_matchmaking_match_squad_teardown_participant_pre021;
DROP TRIGGER matchmaking_match_squad_teardown_participant_fence ON matchmaking_match_squad_teardown_participants;
CREATE FUNCTION guard_matchmaking_match_squad_teardown_participant() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF TG_OP='DELETE' THEN RAISE EXCEPTION 'cannot delete MatchSquad provider participant evidence'; END IF;
    IF ROW(NEW.aggregate_id,NEW.provider,NEW.operation_id) IS DISTINCT FROM ROW(OLD.aggregate_id,OLD.provider,OLD.operation_id) THEN
        RAISE EXCEPTION 'cannot rewrite MatchSquad provider participant identity';
    END IF;
    IF ROW(NEW.request_sha256,NEW.request_bytes) IS DISTINCT FROM ROW(OLD.request_sha256,OLD.request_bytes) AND NOT (
       OLD.state='WAITING_FOR_CREATION' AND NEW.state='NOT_STARTED' AND OLD.request_sha256 IS NULL AND OLD.request_bytes IS NULL AND
       NEW.request_sha256 IS NOT NULL AND NEW.request_bytes IS NOT NULL AND EXISTS (
         SELECT 1 FROM matchmaking_match_squad_teardowns a JOIN matchmaking_match_squad_operations o ON o.match_id=a.match_id
         WHERE a.aggregate_id=OLD.aggregate_id AND a.purpose='PROVISION_COMPENSATION' AND a.state='pending'
         AND o.provision_disposition='ABANDONED' AND
         ((OLD.provider='chat' AND o.chat_dispatch_state='RECEIPT_KNOWN' AND o.chat_receipt_id IS NOT NULL
           AND a.chat_teardown_operation_id=NEW.operation_id AND a.chat_teardown_request_sha256=NEW.request_sha256 AND a.chat_teardown_request_bytes=NEW.request_bytes) OR
          (OLD.provider='voice' AND o.voice_dispatch_state='RECEIPT_KNOWN' AND o.voice_receipt_id IS NOT NULL
           AND a.voice_teardown_operation_id=NEW.operation_id AND a.voice_teardown_request_sha256=NEW.request_sha256 AND a.voice_teardown_request_bytes=NEW.request_bytes)))) THEN
        RAISE EXCEPTION 'cannot rewrite MatchSquad provider participant request';
    END IF;
    IF OLD.receipt_id IS NOT NULL AND ROW(NEW.receipt_id,NEW.receipt_bytes) IS DISTINCT FROM ROW(OLD.receipt_id,OLD.receipt_bytes) THEN
        RAISE EXCEPTION 'cannot rewrite MatchSquad provider participant receipt';
    END IF;
    IF OLD.state='COMPLETE' AND NEW.state<>OLD.state THEN RAISE EXCEPTION 'cannot reopen completed MatchSquad provider participant'; END IF;
    IF OLD.state='CONTRACT_MISMATCH' AND NEW.state<>OLD.state THEN RAISE EXCEPTION 'cannot retry a MatchSquad provider contract mismatch'; END IF;
    IF OLD.state<>NEW.state AND NOT (
       (OLD.state='WAITING_FOR_CREATION' AND NEW.state='NOT_STARTED') OR
       (OLD.state='NOT_STARTED' AND NEW.state IN ('IN_FLIGHT','COMPLETE','RETRYABLE_FAILURE','CONTRACT_MISMATCH')) OR
       (OLD.state='IN_FLIGHT' AND NEW.state IN ('COMPLETE','RETRYABLE_FAILURE','CONTRACT_MISMATCH')) OR
       (OLD.state='RETRYABLE_FAILURE' AND NEW.state IN ('IN_FLIGHT','COMPLETE','CONTRACT_MISMATCH'))) THEN
        RAISE EXCEPTION 'invalid MatchSquad provider participant state transition';
    END IF;
    RETURN NEW;
END;
$$;
CREATE TRIGGER matchmaking_match_squad_teardown_participant_fence BEFORE UPDATE OR DELETE ON matchmaking_match_squad_teardown_participants
    FOR EACH ROW EXECUTE FUNCTION guard_matchmaking_match_squad_teardown_participant();

COMMIT;
