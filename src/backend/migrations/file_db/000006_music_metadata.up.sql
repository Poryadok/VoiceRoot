ALTER TABLE files
    ADD COLUMN upload_intent TEXT NOT NULL DEFAULT '' CHECK (upload_intent IN ('', 'music')),
    ADD COLUMN music_metadata_revision BIGINT NOT NULL DEFAULT 0 CHECK (music_metadata_revision >= 0),
    ADD COLUMN music_title TEXT NULL CHECK (octet_length(music_title) <= 1024),
    ADD COLUMN music_artist TEXT NULL CHECK (octet_length(music_artist) <= 1024),
    ADD COLUMN music_album TEXT NULL CHECK (octet_length(music_album) <= 1024),
    ADD CONSTRAINT files_music_metadata_plaintext CHECK (
        (music_title IS NULL AND music_artist IS NULL AND music_album IS NULL)
        OR (NOT is_e2e AND file_type = 'audio')
    );

CREATE TABLE file_music_event_outbox (
    event_id UUID PRIMARY KEY,
    file_id UUID NOT NULL REFERENCES files(id),
    metadata_revision BIGINT NOT NULL CHECK (metadata_revision > 0),
    payload BYTEA NOT NULL,
    published_at TIMESTAMPTZ NULL,
    UNIQUE (file_id, metadata_revision)
);
