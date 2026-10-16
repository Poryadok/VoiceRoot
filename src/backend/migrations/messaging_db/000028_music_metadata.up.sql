-- Private authority established from File at send/forward admission. Never
-- expose the source chat on a forwarded public attachment.
ALTER TABLE messages ADD COLUMN music_file_id UUID,
 ADD COLUMN music_source_chat_id UUID,
 ADD COLUMN music_metadata_revision BIGINT;
ALTER TABLE messages ADD CONSTRAINT message_music_binding CHECK (
 (music_file_id IS NULL AND music_source_chat_id IS NULL AND music_metadata_revision IS NULL)
 OR (music_file_id IS NOT NULL AND music_source_chat_id IS NOT NULL
     AND music_metadata_revision IS NOT NULL AND music_metadata_revision>0
     AND NOT is_e2e AND COALESCE(content_type,'')='music'));
CREATE INDEX messages_music_file_refresh ON messages(music_file_id,music_source_chat_id)
 WHERE music_file_id IS NOT NULL AND deleted_at IS NULL;
