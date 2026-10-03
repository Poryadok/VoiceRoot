CREATE TABLE notification_space_chat_manifests (
  space_id UUID NOT NULL,
  deletion_operation_id UUID NOT NULL,
  schedule_generation BIGINT NOT NULL CHECK (schedule_generation > 0),
  manifest_bytes BYTEA NOT NULL,
  sealed BOOLEAN NOT NULL DEFAULT FALSE,
  retain_until TIMESTAMPTZ,
  PRIMARY KEY (space_id, deletion_operation_id)
);
CREATE TABLE notification_space_chat_manifest_pages (
  space_id UUID NOT NULL,
  deletion_operation_id UUID NOT NULL,
  page_index BIGINT NOT NULL CHECK (page_index >= 0),
  request_bytes BYTEA NOT NULL,
  receipt_bytes BYTEA NOT NULL,
  chat_ids UUID[] NOT NULL,
  PRIMARY KEY (space_id, deletion_operation_id, page_index),
  FOREIGN KEY (space_id, deletion_operation_id) REFERENCES notification_space_chat_manifests ON DELETE CASCADE
);
-- Non-content scope keys survive full receipt/page retention cleanup.
CREATE TABLE notification_space_chat_fences (
  chat_id UUID PRIMARY KEY,
  space_id UUID NOT NULL,
  blocked BOOLEAN NOT NULL DEFAULT TRUE
);
CREATE INDEX notification_space_chat_fences_space_idx ON notification_space_chat_fences(space_id);
