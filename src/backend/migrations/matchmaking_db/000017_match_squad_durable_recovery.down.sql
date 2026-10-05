DO $$
BEGIN
    PERFORM set_config('lock_timeout', '2s', true);
    LOCK TABLE matches,
        matchmaking_match_leave_operations,
        matchmaking_match_squad_operations,
        matchmaking_match_squad_teardowns,
        matchmaking_match_squad_teardown_participants,
        matchmaking_match_squad_compaction_intents,
        matchmaking_match_squad_completion_events
        IN ACCESS EXCLUSIVE MODE NOWAIT;

    IF EXISTS (
        SELECT 1 FROM matchmaking_match_squad_teardowns
        WHERE purpose <> 'FINAL_LEAVE'
           OR required_providers <> ARRAY['chat', 'voice']::TEXT[]
           OR chat_teardown_operation_id IS NULL
           OR chat_teardown_request_sha256 IS NULL
           OR chat_teardown_request_bytes IS NULL
           OR voice_teardown_operation_id IS NULL
           OR voice_teardown_request_sha256 IS NULL
           OR voice_teardown_request_bytes IS NULL
           OR teardown_attempt_count <> 0
           OR teardown_claim_generation <> 0
           OR teardown_claim_until IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'cannot remove MatchSquad recovery schema while compensation, provider omissions, or teardown claims exist';
    END IF;
    IF EXISTS (
        SELECT 1 FROM matchmaking_match_squad_operations
        WHERE state = 'compensating'
           OR provision_attempt_count <> 0
           OR provision_claim_generation <> 0
           OR provision_claim_until IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'cannot remove MatchSquad recovery schema while provisioning attempts or claims exist';
    END IF;
    IF EXISTS (
        SELECT 1 FROM matchmaking_match_squad_teardown_participants
        WHERE attempt_count <> 0 OR claim_generation <> 0 OR claim_until IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'cannot remove MatchSquad recovery schema while provider retry attempts or claims exist';
    END IF;
    IF EXISTS (
        SELECT 1 FROM matchmaking_match_squad_compaction_intents
        WHERE attempt_count <> 0 OR claim_generation <> 0 OR claim_until IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'cannot remove MatchSquad recovery schema while compaction retry attempts or claims exist';
    END IF;

    DROP TRIGGER matchmaking_match_squad_completion_purpose_guard ON matchmaking_match_squad_completion_events;
    DROP TRIGGER matchmaking_match_squad_compaction_required_guard ON matchmaking_match_squad_compaction_intents;
    DROP TRIGGER matchmaking_match_squad_participant_required_guard ON matchmaking_match_squad_teardown_participants;
    DROP TRIGGER matchmaking_match_squad_recovery_binding_fence ON matchmaking_match_squad_teardowns;
    DROP FUNCTION require_matchmaking_match_squad_completion_purpose();
    DROP FUNCTION require_matchmaking_match_squad_provider_required();
    DROP FUNCTION guard_matchmaking_match_squad_recovery_binding();

    DROP INDEX matchmaking_match_squad_compaction_retry_due_idx;
    DROP INDEX matchmaking_match_squad_participant_due_idx;
    DROP INDEX matchmaking_match_squad_teardown_due_idx;
    DROP INDEX matchmaking_match_squad_provisioning_due_idx;

    ALTER TABLE matchmaking_match_squad_teardowns
        DROP CONSTRAINT matchmaking_match_squad_teardown_completion_check,
        DROP CONSTRAINT matchmaking_match_squad_teardown_voice_binding_check,
        DROP CONSTRAINT matchmaking_match_squad_teardown_chat_binding_check,
        DROP CONSTRAINT matchmaking_match_squad_teardown_required_set_check;

    ALTER TABLE matchmaking_match_squad_teardowns
        ALTER COLUMN chat_teardown_operation_id SET NOT NULL,
        ALTER COLUMN chat_teardown_request_sha256 SET NOT NULL,
        ALTER COLUMN chat_teardown_request_bytes SET NOT NULL,
        ALTER COLUMN voice_teardown_operation_id SET NOT NULL,
        ALTER COLUMN voice_teardown_request_sha256 SET NOT NULL,
        ALTER COLUMN voice_teardown_request_bytes SET NOT NULL;

    ALTER TABLE matchmaking_match_squad_teardowns
        ADD CONSTRAINT matchmaking_match_squad_teardowns_check3 CHECK (
            state <> 'complete' OR
            (chat_teardown_receipt_id IS NOT NULL AND voice_teardown_receipt_id IS NOT NULL)
        );

    ALTER TABLE matchmaking_match_squad_compaction_intents
        DROP COLUMN last_error_class,
        DROP COLUMN claim_until,
        DROP COLUMN claim_generation,
        DROP COLUMN last_progress_at,
        DROP COLUMN next_attempt_at,
        DROP COLUMN attempt_count;
    ALTER TABLE matchmaking_match_squad_teardown_participants
        DROP COLUMN last_error_class,
        DROP COLUMN claim_until,
        DROP COLUMN claim_generation,
        DROP COLUMN last_progress_at,
        DROP COLUMN next_attempt_at,
        DROP COLUMN attempt_count;
    ALTER TABLE matchmaking_match_squad_teardowns
        DROP COLUMN teardown_last_error_class,
        DROP COLUMN teardown_claim_until,
        DROP COLUMN teardown_claim_generation,
        DROP COLUMN teardown_last_progress_at,
        DROP COLUMN teardown_next_attempt_at,
        DROP COLUMN teardown_attempt_count,
        DROP COLUMN required_providers,
        DROP COLUMN purpose;
    ALTER TABLE matchmaking_match_squad_operations
        DROP COLUMN provision_last_error_class,
        DROP COLUMN provision_claim_until,
        DROP COLUMN provision_claim_generation,
        DROP COLUMN provision_last_progress_at,
        DROP COLUMN provision_next_attempt_at,
        DROP COLUMN provision_attempt_count;
END;
$$;
