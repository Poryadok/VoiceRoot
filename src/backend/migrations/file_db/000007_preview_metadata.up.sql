ALTER TABLE files ADD COLUMN preview_metadata_revision bigint NOT NULL DEFAULT 1 CHECK (preview_metadata_revision > 0);
CREATE TABLE file_preview_event_outbox (
 event_id uuid PRIMARY KEY, file_id uuid NOT NULL,
 metadata_revision bigint NOT NULL CHECK (metadata_revision > 0), payload bytea NOT NULL,
 created_at timestamptz NOT NULL DEFAULT clock_timestamp(), published_at timestamptz,
 UNIQUE(file_id,metadata_revision)
);
CREATE INDEX file_preview_event_pending ON file_preview_event_outbox(file_id,metadata_revision) WHERE published_at IS NULL;
