DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM chat_deleted_event_outbox) THEN
        RAISE EXCEPTION 'chat deleted event outbox contains retained evidence';
    END IF;
END $$;

DROP TABLE chat_deleted_event_outbox;
