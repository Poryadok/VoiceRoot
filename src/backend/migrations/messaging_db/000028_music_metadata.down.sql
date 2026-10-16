DROP INDEX IF EXISTS messages_music_file_refresh;
ALTER TABLE messages DROP CONSTRAINT IF EXISTS message_music_binding;
ALTER TABLE messages DROP COLUMN IF EXISTS music_metadata_revision,
 DROP COLUMN IF EXISTS music_source_chat_id, DROP COLUMN IF EXISTS music_file_id;
