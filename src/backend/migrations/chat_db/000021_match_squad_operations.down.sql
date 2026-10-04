BEGIN;
LOCK TABLE chat_match_squad_operations IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM chat_match_squad_operations) THEN
        RAISE EXCEPTION 'refusing to drop MatchSquad receipts or terminal ownership fences';
    END IF;
END
$$;

DROP TRIGGER chat_match_squad_delete_fence ON chats;
DROP TRIGGER chat_match_squad_operation_delete_fence ON chat_match_squad_operations;
DROP TRIGGER chat_match_squad_operation_update_guard ON chat_match_squad_operations;
DROP FUNCTION reject_match_squad_operation_delete();
DROP FUNCTION guard_match_squad_operation_update();
DROP FUNCTION reject_match_squad_chat_delete();
DROP TABLE chat_match_squad_operations;
COMMIT;
