ALTER TABLE matchmaking_match_squad_operations
    ADD COLUMN provision_attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (provision_attempt_count >= 0),
    ADD COLUMN provision_next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    ADD COLUMN provision_last_progress_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    ADD COLUMN provision_claim_generation BIGINT NOT NULL DEFAULT 0 CHECK (provision_claim_generation >= 0),
    ADD COLUMN provision_claim_until TIMESTAMPTZ,
    ADD COLUMN provision_last_error_class TEXT;

ALTER TABLE matchmaking_match_squad_teardowns
    ADD COLUMN purpose TEXT NOT NULL DEFAULT 'FINAL_LEAVE' CHECK (purpose IN ('FINAL_LEAVE', 'PROVISION_COMPENSATION')),
    ADD COLUMN required_providers TEXT[] NOT NULL DEFAULT ARRAY['chat', 'voice']::TEXT[],
    ADD COLUMN teardown_attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (teardown_attempt_count >= 0),
    ADD COLUMN teardown_next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    ADD COLUMN teardown_last_progress_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    ADD COLUMN teardown_claim_generation BIGINT NOT NULL DEFAULT 0 CHECK (teardown_claim_generation >= 0),
    ADD COLUMN teardown_claim_until TIMESTAMPTZ,
    ADD COLUMN teardown_last_error_class TEXT;

ALTER TABLE matchmaking_match_squad_teardowns
    ALTER COLUMN chat_teardown_operation_id DROP NOT NULL,
    ALTER COLUMN chat_teardown_request_sha256 DROP NOT NULL,
    ALTER COLUMN chat_teardown_request_bytes DROP NOT NULL,
    ALTER COLUMN voice_teardown_operation_id DROP NOT NULL,
    ALTER COLUMN voice_teardown_request_sha256 DROP NOT NULL,
    ALTER COLUMN voice_teardown_request_bytes DROP NOT NULL;

-- 000016 required both receipts before a teardown could become complete. The
-- recovery purpose introduced here permits Chat-only compensation, so replace
-- that legacy check with the purpose-aware completion constraint below.
ALTER TABLE matchmaking_match_squad_teardowns
    DROP CONSTRAINT matchmaking_match_squad_teardowns_check3;

ALTER TABLE matchmaking_match_squad_teardowns
    ADD CONSTRAINT matchmaking_match_squad_teardown_required_set_check CHECK (
        (purpose = 'FINAL_LEAVE' AND required_providers = ARRAY['chat', 'voice']::TEXT[]) OR
        (purpose = 'PROVISION_COMPENSATION' AND required_providers = ARRAY['chat']::TEXT[])
    ),
    ADD CONSTRAINT matchmaking_match_squad_teardown_chat_binding_check CHECK (
        (('chat' = ANY(required_providers) AND chat_teardown_operation_id IS NOT NULL AND chat_teardown_request_sha256 IS NOT NULL AND chat_teardown_request_bytes IS NOT NULL) OR
         (NOT ('chat' = ANY(required_providers)) AND chat_teardown_operation_id IS NULL AND chat_teardown_request_sha256 IS NULL AND chat_teardown_request_bytes IS NULL))
    ),
    ADD CONSTRAINT matchmaking_match_squad_teardown_voice_binding_check CHECK (
        (('voice' = ANY(required_providers) AND voice_teardown_operation_id IS NOT NULL AND voice_teardown_request_sha256 IS NOT NULL AND voice_teardown_request_bytes IS NOT NULL) OR
         (NOT ('voice' = ANY(required_providers)) AND voice_teardown_operation_id IS NULL AND voice_teardown_request_sha256 IS NULL AND voice_teardown_request_bytes IS NULL))
    ),
    ADD CONSTRAINT matchmaking_match_squad_teardown_completion_check CHECK (
        state <> 'complete' OR
        (purpose = 'FINAL_LEAVE' AND chat_teardown_receipt_id IS NOT NULL AND voice_teardown_receipt_id IS NOT NULL) OR
        (purpose = 'PROVISION_COMPENSATION' AND chat_teardown_receipt_id IS NOT NULL)
    );

ALTER TABLE matchmaking_match_squad_teardown_participants
    ADD COLUMN attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    ADD COLUMN next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    ADD COLUMN last_progress_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    ADD COLUMN claim_generation BIGINT NOT NULL DEFAULT 0 CHECK (claim_generation >= 0),
    ADD COLUMN claim_until TIMESTAMPTZ,
    ADD COLUMN last_error_class TEXT;

ALTER TABLE matchmaking_match_squad_compaction_intents
    ADD COLUMN attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
    ADD COLUMN next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    ADD COLUMN last_progress_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    ADD COLUMN claim_generation BIGINT NOT NULL DEFAULT 0 CHECK (claim_generation >= 0),
    ADD COLUMN claim_until TIMESTAMPTZ,
    ADD COLUMN last_error_class TEXT;

CREATE INDEX matchmaking_match_squad_provisioning_due_idx
    ON matchmaking_match_squad_operations (provision_next_attempt_at, match_id)
    WHERE state IN ('provisioning', 'compensating');

CREATE INDEX matchmaking_match_squad_teardown_due_idx
    ON matchmaking_match_squad_teardowns (teardown_next_attempt_at, aggregate_id)
    WHERE state = 'pending';

CREATE INDEX matchmaking_match_squad_participant_due_idx
    ON matchmaking_match_squad_teardown_participants (next_attempt_at, aggregate_id, provider)
    WHERE state IN ('NOT_STARTED', 'IN_FLIGHT', 'RETRYABLE_FAILURE');

CREATE INDEX matchmaking_match_squad_compaction_retry_due_idx
    ON matchmaking_match_squad_compaction_intents (next_attempt_at, aggregate_id, provider)
    WHERE completed_at IS NULL;

CREATE FUNCTION guard_matchmaking_match_squad_recovery_binding() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF ROW(NEW.aggregate_id, NEW.match_id, NEW.purpose, NEW.required_providers)
       IS DISTINCT FROM ROW(OLD.aggregate_id, OLD.match_id, OLD.purpose, OLD.required_providers) THEN
        RAISE EXCEPTION 'cannot rewrite MatchSquad teardown purpose or required provider set';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER matchmaking_match_squad_recovery_binding_fence
    BEFORE UPDATE ON matchmaking_match_squad_teardowns
    FOR EACH ROW EXECUTE FUNCTION guard_matchmaking_match_squad_recovery_binding();

CREATE FUNCTION require_matchmaking_match_squad_provider_required() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    required TEXT[];
BEGIN
    SELECT required_providers INTO required
    FROM matchmaking_match_squad_teardowns
    WHERE aggregate_id = NEW.aggregate_id;
    IF required IS NULL OR NOT (NEW.provider = ANY(required)) THEN
        RAISE EXCEPTION 'MatchSquad provider is not required by teardown purpose';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER matchmaking_match_squad_participant_required_guard
    BEFORE INSERT ON matchmaking_match_squad_teardown_participants
    FOR EACH ROW EXECUTE FUNCTION require_matchmaking_match_squad_provider_required();

CREATE TRIGGER matchmaking_match_squad_compaction_required_guard
    BEFORE INSERT ON matchmaking_match_squad_compaction_intents
    FOR EACH ROW EXECUTE FUNCTION require_matchmaking_match_squad_provider_required();

CREATE FUNCTION require_matchmaking_match_squad_completion_purpose() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE
    current_purpose TEXT;
BEGIN
    SELECT purpose INTO current_purpose
    FROM matchmaking_match_squad_teardowns
    WHERE aggregate_id = NEW.aggregate_id;
    IF current_purpose IS DISTINCT FROM 'FINAL_LEAVE' THEN
        RAISE EXCEPTION 'provisioning compensation cannot create a MatchSquad completion event';
    END IF;
    RETURN NEW;
END;
$$;

CREATE TRIGGER matchmaking_match_squad_completion_purpose_guard
    BEFORE INSERT ON matchmaking_match_squad_completion_events
    FOR EACH ROW EXECUTE FUNCTION require_matchmaking_match_squad_completion_purpose();
