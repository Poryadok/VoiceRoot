DO $$ BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION USING ERRCODE = '55000',
            MESSAGE = 'attachment send intent rollback requires read committed';
    END IF;
    -- Keep the current-snapshot guard and DROP under the same writer fence.
    LOCK TABLE messaging_attachment_send_intents IN ACCESS EXCLUSIVE MODE;
    IF EXISTS(SELECT 1 FROM messaging_attachment_send_intents) THEN
        RAISE EXCEPTION 'attachment send intent rollback requires drained, exported evidence';
    END IF;
    DROP TABLE messaging_attachment_send_intents;
END $$;
