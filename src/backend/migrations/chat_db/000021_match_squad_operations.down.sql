DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM chat_match_squad_operations) THEN
        RAISE EXCEPTION 'refusing to drop MatchSquad receipts or terminal ownership fences';
    END IF;
END
$$;

DROP TRIGGER chat_match_squad_delete_fence ON chats;
DROP FUNCTION reject_match_squad_chat_delete();
DROP TABLE chat_match_squad_operations;
