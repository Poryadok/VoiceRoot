DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM messaging_attachment_send_intents) THEN
        RAISE EXCEPTION 'attachment send intent rollback requires drained, exported evidence';
    END IF;
END $$;
DROP TABLE messaging_attachment_send_intents;
