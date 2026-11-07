DO $$
BEGIN
    -- A stronger isolation level can retain an empty snapshot from before a
    -- writer committed, even after this transaction acquires the table lock.
    IF current_setting('transaction_isolation') NOT IN ('read committed', 'read uncommitted') THEN
        RAISE EXCEPTION 'chat deleted event outbox rollback requires read committed isolation'
            USING ERRCODE = '55000';
    END IF;
    -- Serialize with first writers before checking retained evidence. Keep the
    -- lock through DROP so an event cannot commit between the probe and DDL.
    LOCK TABLE chat_deleted_event_outbox IN ACCESS EXCLUSIVE MODE;
    IF EXISTS (SELECT 1 FROM chat_deleted_event_outbox) THEN
        RAISE EXCEPTION 'chat deleted event outbox contains retained evidence';
    END IF;
    DROP TABLE chat_deleted_event_outbox;
END $$;
