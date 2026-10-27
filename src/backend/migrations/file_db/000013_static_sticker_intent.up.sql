ALTER TABLE files DROP CONSTRAINT files_upload_intent_check;
ALTER TABLE files ADD CONSTRAINT files_upload_intent_check
    CHECK (upload_intent IN ('', 'music', 'gif', 'sticker'));
ALTER TABLE files ADD CONSTRAINT files_static_sticker_source
    CHECK (upload_intent <> 'sticker' OR
           (NOT is_e2e AND story_id IS NULL AND mime_type IN ('image/png', 'image/webp')));
