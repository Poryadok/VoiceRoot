DROP TRIGGER a_messaging_history_position ON messages;
DROP FUNCTION messaging_assign_history_position();
DROP INDEX messages_thread_history_position_idx;
DROP INDEX messages_chat_history_position_idx;
ALTER TABLE messages DROP COLUMN history_position;
