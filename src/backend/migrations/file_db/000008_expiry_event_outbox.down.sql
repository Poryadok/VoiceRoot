DO $$
BEGIN
    LOCK TABLE file_expiry_event_outbox IN ACCESS EXCLUSIVE MODE NOWAIT;
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'expiry outbox rollback requires fresh READ COMMITTED checks'
            USING ERRCODE = '55000';
    END IF;
    IF EXISTS (SELECT 1 FROM file_expiry_event_outbox) THEN
        RAISE EXCEPTION 'expiry outbox rollback would erase durable event evidence'
            USING ERRCODE = '55000';
    END IF;
    DROP TABLE file_expiry_event_outbox;
END $$;
