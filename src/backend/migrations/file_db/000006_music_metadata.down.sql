DROP TABLE file_music_event_outbox;
ALTER TABLE files
    DROP CONSTRAINT files_music_metadata_plaintext,
    DROP COLUMN music_title,
    DROP COLUMN music_artist,
    DROP COLUMN music_album,
    DROP COLUMN music_metadata_revision,
    DROP COLUMN upload_intent;
