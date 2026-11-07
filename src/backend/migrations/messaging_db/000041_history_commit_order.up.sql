-- A UUIDv7 is identity, not commit order: IDs can be allocated before a pause
-- or on replicas with different clocks. Keep the public ID and serialize its
-- private history position with every insertion in the same chat.
BEGIN;
LOCK TABLE messages IN ACCESS EXCLUSIVE MODE;

CREATE SEQUENCE message_history_position_seq AS BIGINT;
ALTER TABLE messages ADD COLUMN history_position BIGINT;
DO $$
DECLARE
    fence_mode "char";
BEGIN
    -- This metadata-only backfill must include retained frozen Space rows.
    -- The exclusive lock excludes all application writers; disable only their
    -- payload lifecycle gate for this UPDATE and restore its exact prior mode
    -- before committing. Any failure rolls back the DDL and trigger state.
    SELECT tgenabled INTO fence_mode FROM pg_trigger
    WHERE tgrelid='messages'::regclass AND tgname='messaging_space_chat_mutation_gate';
    IF fence_mode IS NOT NULL AND fence_mode <> 'D' THEN
        ALTER TABLE messages DISABLE TRIGGER messaging_space_chat_mutation_gate;
    END IF;
    WITH ordered AS (
        SELECT id, row_number() OVER (ORDER BY id) AS position FROM messages
    )
    UPDATE messages SET history_position = ordered.position
    FROM ordered WHERE messages.id = ordered.id;
    IF fence_mode = 'O' THEN
        ALTER TABLE messages ENABLE TRIGGER messaging_space_chat_mutation_gate;
    ELSIF fence_mode = 'A' THEN
        ALTER TABLE messages ENABLE ALWAYS TRIGGER messaging_space_chat_mutation_gate;
    ELSIF fence_mode = 'R' THEN
        ALTER TABLE messages ENABLE REPLICA TRIGGER messaging_space_chat_mutation_gate;
    END IF;
END;
$$;
SELECT setval('message_history_position_seq',
    COALESCE((SELECT MAX(history_position) FROM messages), 1),
    EXISTS(SELECT 1 FROM messages));
ALTER TABLE messages ALTER COLUMN history_position SET NOT NULL;
ALTER SEQUENCE message_history_position_seq OWNED BY messages.history_position;

CREATE FUNCTION messaging_assign_history_position() RETURNS TRIGGER AS $$
BEGIN
    IF TG_OP = 'UPDATE' THEN
        IF NEW.history_position IS DISTINCT FROM OLD.history_position
           OR NEW.chat_id IS DISTINCT FROM OLD.chat_id THEN
            RAISE EXCEPTION 'message history position is immutable' USING ERRCODE = '55000';
        END IF;
        RETURN NEW;
    END IF;
    -- Same lock/order as lockMessageMutation. nextval alone would still allow
    -- B to commit and advance a reader's cursor while A is uncommitted.
    PERFORM pg_advisory_xact_lock(hashtextextended(
        'voice.messaging.message-mutation.v1/' || NEW.chat_id::text, 0));
    NEW.history_position := nextval('message_history_position_seq');
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Run before Space/fence triggers to preserve chat -> Space lock ordering for
-- raw SQL and legacy producers as well as the explicit Go transactions.
CREATE TRIGGER a_messaging_history_position
    BEFORE INSERT OR UPDATE ON messages
    FOR EACH ROW EXECUTE FUNCTION messaging_assign_history_position();
CREATE UNIQUE INDEX messages_chat_history_position_idx ON messages(chat_id, history_position);
CREATE INDEX messages_thread_history_position_idx ON messages(chat_id, thread_parent_id, history_position)
    WHERE deleted_at IS NULL;
COMMIT;
