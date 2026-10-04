DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM matchmaking_match_leave_operations) THEN
        RAISE EXCEPTION 'cannot remove actor-scoped Matchmaking leave idempotency records';
    END IF;
    IF EXISTS (SELECT 1 FROM matchmaking_match_squad_operations) THEN
        RAISE EXCEPTION 'cannot remove MatchSquad provisioning operations or ownership fences';
    END IF;
    IF EXISTS (SELECT 1 FROM matchmaking_match_squad_teardowns) THEN
        RAISE EXCEPTION 'cannot remove pending, completed, or terminal MatchSquad teardown evidence';
    END IF;
    IF EXISTS (SELECT 1 FROM matchmaking_match_squad_teardown_participants) THEN
        RAISE EXCEPTION 'cannot remove MatchSquad provider participant requests or receipts';
    END IF;
    IF EXISTS (SELECT 1 FROM matchmaking_match_squad_compaction_intents) THEN
        RAISE EXCEPTION 'cannot remove pending or completed MatchSquad compaction evidence';
    END IF;
    IF EXISTS (SELECT 1 FROM matchmaking_match_squad_completion_events) THEN
        RAISE EXCEPTION 'cannot remove MatchSquad completion event delivery evidence';
    END IF;
END;
$$;

DROP TRIGGER matchmaking_match_squad_completion_event_fence ON matchmaking_match_squad_completion_events;
DROP TRIGGER matchmaking_match_squad_teardown_participant_fence ON matchmaking_match_squad_teardown_participants;
DROP TRIGGER matchmaking_match_squad_compaction_fence ON matchmaking_match_squad_compaction_intents;
DROP TRIGGER matchmaking_match_squad_compaction_due_guard ON matchmaking_match_squad_compaction_intents;
DROP TRIGGER matchmaking_match_squad_teardown_fence ON matchmaking_match_squad_teardowns;
DROP TRIGGER matchmaking_match_squad_operation_fence ON matchmaking_match_squad_operations;
DROP FUNCTION require_matchmaking_match_squad_compaction_due();
DROP FUNCTION guard_matchmaking_match_squad_completion_event();
DROP FUNCTION guard_matchmaking_match_squad_teardown_participant();
DROP FUNCTION guard_matchmaking_match_squad_compaction();
DROP FUNCTION guard_matchmaking_match_squad_lifecycle();
DROP FUNCTION guard_matchmaking_match_squad_operation();
DROP INDEX matchmaking_match_squad_compaction_due_idx;
DROP INDEX matchmaking_match_squad_completion_events_pending_idx;
DROP INDEX matchmaking_match_squad_teardown_participants_pending_idx;
DROP TABLE matchmaking_match_squad_completion_events;
DROP TABLE matchmaking_match_squad_teardown_participants;
DROP TABLE matchmaking_match_squad_compaction_intents;
DROP TABLE matchmaking_match_squad_teardowns;
DROP TABLE matchmaking_match_squad_operations;
DROP TABLE matchmaking_match_leave_operations;
