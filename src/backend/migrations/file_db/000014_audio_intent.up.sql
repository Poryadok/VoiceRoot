ALTER TABLE files DROP CONSTRAINT files_upload_intent_check;
ALTER TABLE files ADD CONSTRAINT files_upload_intent_check
    CHECK (upload_intent IN ('', 'music', 'gif', 'sticker', 'audio'));
