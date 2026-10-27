DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM story_media_reference_intents) OR EXISTS(SELECT 1 FROM story_deletion_receipts) THEN
        RAISE EXCEPTION 'pending Story media claims require reconciliation';
    END IF;
END $$;
DROP TABLE story_media_reference_intents;
DROP TABLE story_deletion_receipts;
