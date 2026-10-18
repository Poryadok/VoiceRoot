BEGIN;
LOCK TABLE matchmaking_match_squad_operations IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM matchmaking_match_squad_operations) THEN
        RAISE EXCEPTION 'cannot remove MatchSquad dispatch or settlement evidence';
    END IF;
END $$;
DROP TRIGGER matchmaking_match_squad_teardown_fence ON matchmaking_match_squad_teardowns;
DROP FUNCTION guard_matchmaking_match_squad_lifecycle();
ALTER FUNCTION guard_matchmaking_match_squad_lifecycle_pre021() RENAME TO guard_matchmaking_match_squad_lifecycle;
CREATE TRIGGER matchmaking_match_squad_teardown_fence BEFORE UPDATE OR DELETE ON matchmaking_match_squad_teardowns
    FOR EACH ROW EXECUTE FUNCTION guard_matchmaking_match_squad_lifecycle();
DROP TRIGGER matchmaking_match_squad_teardown_participant_fence ON matchmaking_match_squad_teardown_participants;
DROP FUNCTION guard_matchmaking_match_squad_teardown_participant();
ALTER FUNCTION guard_matchmaking_match_squad_teardown_participant_pre021() RENAME TO guard_matchmaking_match_squad_teardown_participant;
CREATE TRIGGER matchmaking_match_squad_teardown_participant_fence BEFORE UPDATE OR DELETE ON matchmaking_match_squad_teardown_participants
    FOR EACH ROW EXECUTE FUNCTION guard_matchmaking_match_squad_teardown_participant();
DROP TRIGGER matchmaking_match_squad_dispatch_fence ON matchmaking_match_squad_operations;
DROP FUNCTION guard_matchmaking_match_squad_dispatch();
DROP TRIGGER matchmaking_match_squad_abandoned_obligation_guard ON matchmaking_match_squad_teardowns;
DROP FUNCTION require_matchmaking_match_squad_abandoned_obligation();
ALTER TABLE matchmaking_match_squad_teardown_participants
    DROP CONSTRAINT matchmaking_match_squad_participant_request_binding,
    DROP CONSTRAINT matchmaking_match_squad_teardown_participants_state_check,
    ALTER COLUMN request_sha256 SET NOT NULL,
    ALTER COLUMN request_bytes SET NOT NULL,
    ADD CONSTRAINT matchmaking_match_squad_teardown_participants_state_check CHECK
      (state IN ('NOT_STARTED','IN_FLIGHT','COMPLETE','RETRYABLE_FAILURE','CONTRACT_MISMATCH'));
ALTER TABLE matchmaking_match_squad_teardowns
    DROP CONSTRAINT matchmaking_match_squad_teardown_required_set_check,
    DROP CONSTRAINT matchmaking_match_squad_teardown_chat_binding_check,
    DROP CONSTRAINT matchmaking_match_squad_teardown_voice_binding_check,
    DROP CONSTRAINT matchmaking_match_squad_teardown_completion_check,
    ADD CONSTRAINT matchmaking_match_squad_teardown_required_set_check CHECK (
        (purpose='FINAL_LEAVE' AND required_providers=ARRAY['chat','voice']::TEXT[]) OR
        (purpose='PROVISION_COMPENSATION' AND required_providers=ARRAY['chat']::TEXT[])
    ),
    ADD CONSTRAINT matchmaking_match_squad_teardown_chat_binding_check CHECK (
        ('chat'=ANY(required_providers) AND chat_teardown_operation_id IS NOT NULL AND chat_teardown_request_sha256 IS NOT NULL AND chat_teardown_request_bytes IS NOT NULL) OR
        (NOT ('chat'=ANY(required_providers)) AND chat_teardown_operation_id IS NULL AND chat_teardown_request_sha256 IS NULL AND chat_teardown_request_bytes IS NULL)
    ),
    ADD CONSTRAINT matchmaking_match_squad_teardown_voice_binding_check CHECK (
        ('voice'=ANY(required_providers) AND voice_teardown_operation_id IS NOT NULL AND voice_teardown_request_sha256 IS NOT NULL AND voice_teardown_request_bytes IS NOT NULL) OR
        (NOT ('voice'=ANY(required_providers)) AND voice_teardown_operation_id IS NULL AND voice_teardown_request_sha256 IS NULL AND voice_teardown_request_bytes IS NULL)
    ),
    ADD CONSTRAINT matchmaking_match_squad_teardown_completion_check CHECK (
        state<>'complete' OR
        (purpose='FINAL_LEAVE' AND chat_teardown_receipt_id IS NOT NULL AND voice_teardown_receipt_id IS NOT NULL) OR
        (purpose='PROVISION_COMPENSATION' AND chat_teardown_receipt_id IS NOT NULL)
    );
ALTER TABLE matchmaking_match_squad_operations
    DROP CONSTRAINT matchmaking_provision_abandon_binding,
    DROP CONSTRAINT matchmaking_provision_chat_dispatch_binding,
    DROP CONSTRAINT matchmaking_provision_voice_dispatch_binding,
    DROP CONSTRAINT matchmaking_provision_voice_dispatch_prerequisite,
    DROP COLUMN dispatch_protocol_version,
    DROP COLUMN provision_disposition,
    DROP COLUMN provision_revision,
    DROP COLUMN provision_abandoned_at,
    DROP COLUMN provision_abandon_reason,
    DROP COLUMN provision_abandon_evidence_sha256,
    DROP COLUMN chat_dispatch_state,
    DROP COLUMN chat_first_dispatch_id,
    DROP COLUMN chat_first_dispatch_at,
    DROP COLUMN voice_dispatch_state,
    DROP COLUMN voice_first_dispatch_id,
    DROP COLUMN voice_first_dispatch_at;
COMMIT;
