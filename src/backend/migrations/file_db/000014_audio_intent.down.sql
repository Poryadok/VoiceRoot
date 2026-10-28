DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM files WHERE upload_intent = 'audio') THEN
        RAISE EXCEPTION 'cannot roll back while audio uploads exist';
    END IF;
END $$;
ALTER TABLE files DROP CONSTRAINT files_upload_intent_check;
ALTER TABLE files ADD CONSTRAINT files_upload_intent_check
    CHECK (upload_intent IN ('', 'music', 'gif', 'sticker'));
