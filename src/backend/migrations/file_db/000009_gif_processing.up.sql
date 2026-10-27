ALTER TABLE files DROP CONSTRAINT files_upload_intent_check;
ALTER TABLE files ADD CONSTRAINT files_upload_intent_check CHECK (upload_intent IN ('', 'music', 'gif'));
ALTER TABLE files ADD CONSTRAINT files_gif_plaintext CHECK (upload_intent <> 'gif' OR NOT is_e2e);

CREATE TABLE file_gif_jobs (
 file_id UUID PRIMARY KEY REFERENCES files(id),
 processor_version TEXT NOT NULL,
 state TEXT NOT NULL DEFAULT 'queued' CHECK (state IN ('queued','running','complete','failed')),
 attempt INTEGER NOT NULL DEFAULT 0 CHECK (attempt >= 0),
 attempt_token UUID NULL,
 lease_until TIMESTAMPTZ NULL,
 next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);
CREATE INDEX file_gif_jobs_pending ON file_gif_jobs(next_attempt_at) WHERE state IN ('queued','running');

CREATE TABLE file_gif_outputs (
 file_id UUID PRIMARY KEY REFERENCES files(id),
 full_descriptor JSONB NOT NULL,
 thumbnail_descriptor JSONB NOT NULL,
 processor_version TEXT NOT NULL
);

-- Every attempt declares keys durably before storage calls. A losing attempt can
-- only collect its own keys; the committed winner marks its keys retained.
CREATE TABLE file_gif_cleanup (
 object_key TEXT PRIMARY KEY,
 file_id UUID NOT NULL REFERENCES files(id),
 attempt_token UUID NULL,
 retained BOOLEAN NOT NULL DEFAULT false,
 not_before TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 recheck_until TIMESTAMPTZ NULL,
 completed_at TIMESTAMPTZ NULL
);
CREATE INDEX file_gif_cleanup_pending ON file_gif_cleanup(not_before) WHERE NOT retained AND completed_at IS NULL;
