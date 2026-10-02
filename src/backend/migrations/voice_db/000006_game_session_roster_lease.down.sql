BEGIN;
DROP TRIGGER voice_game_session_roster_receipt_delete_guard ON voice_game_session_roster_receipts;
DROP FUNCTION voice_game_session_roster_receipt_delete_guard_fn();
DROP TRIGGER voice_game_session_roster_receipt_guard ON voice_game_session_roster_receipts;
DROP FUNCTION voice_game_session_roster_receipt_guard_fn();
DROP TABLE voice_game_session_roster_receipts;
ALTER TABLE voice_game_session_operations
    DROP COLUMN lease_expires_at,
    DROP COLUMN lease_fenced_at,
    DROP COLUMN roster_profile_ids,
    DROP COLUMN roster_body_hash,
    DROP COLUMN roster_revision;
DROP TRIGGER voice_game_session_operation_delete_guard ON voice_game_session_operations;
DROP FUNCTION voice_game_session_operation_delete_guard_fn();
DROP TRIGGER voice_game_session_operation_immutable_guard ON voice_game_session_operations;
DROP FUNCTION voice_game_session_operation_immutable_guard_fn();
CREATE FUNCTION voice_game_session_operation_immutable_guard_fn() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
    RAISE EXCEPTION 'game session operation receipt is immutable' USING ERRCODE = '55000';
END;
$$;
CREATE TRIGGER voice_game_session_operation_immutable_guard
BEFORE UPDATE OR DELETE ON voice_game_session_operations
FOR EACH ROW EXECUTE FUNCTION voice_game_session_operation_immutable_guard_fn();
COMMIT;
