DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM files WHERE upload_intent = 'sticker') THEN
        RAISE EXCEPTION 'cannot roll back while sticker assets exist';
    END IF;
END $$;
ALTER TABLE files DROP CONSTRAINT files_static_sticker_source;
ALTER TABLE files DROP CONSTRAINT files_upload_intent_check;
ALTER TABLE files ADD CONSTRAINT files_upload_intent_check
    CHECK (upload_intent IN ('', 'music', 'gif'));
