ALTER TABLE messages ADD COLUMN file_preview_bindings jsonb NOT NULL DEFAULT '[]'::jsonb CHECK (jsonb_typeof(file_preview_bindings)='array');
CREATE INDEX messages_file_preview_bindings ON messages USING gin(file_preview_bindings jsonb_path_ops) WHERE deleted_at IS NULL AND NOT is_e2e;
CREATE TABLE messaging_file_preview_snapshots (
 file_id uuid PRIMARY KEY, metadata_revision bigint NOT NULL CHECK (metadata_revision > 0),
 payload bytea NOT NULL, terminal boolean NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE messaging_file_preview_pending (
 file_id uuid NOT NULL REFERENCES messaging_file_preview_snapshots(file_id), chat_id uuid NOT NULL,
 updated_at timestamptz NOT NULL DEFAULT clock_timestamp(), PRIMARY KEY(file_id,chat_id)
);
CREATE TABLE messaging_file_preview_reconciliation (
 singleton boolean PRIMARY KEY DEFAULT true CHECK (singleton),
 after_id uuid NOT NULL DEFAULT '00000000-0000-0000-0000-000000000000',
 pass bigint NOT NULL DEFAULT 0, updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
INSERT INTO messaging_file_preview_reconciliation(singleton) VALUES(true);
CREATE TABLE messaging_file_preview_errors (
 message_id uuid PRIMARY KEY, attempts bigint NOT NULL DEFAULT 1, reason text NOT NULL,
 next_attempt_at timestamptz NOT NULL DEFAULT clock_timestamp(), updated_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
