-- Private, account-serialized committed evidence. No plaintext is retained.
ALTER TABLE sanctions ALTER COLUMN issued_by DROP NOT NULL;
ALTER TABLE sanctions ADD COLUMN issuer_service text;
ALTER TABLE sanctions ADD COLUMN machine_case_id uuid;
ALTER TABLE sanctions ADD CONSTRAINT sanctions_issuer_kind CHECK (
  (issued_by IS NOT NULL AND issuer_service IS NULL AND machine_case_id IS NULL) OR
  (issued_by IS NULL AND issuer_service = 'service:moderation/automod' AND report_id IS NULL AND
    ((type = 'perm_ban' AND machine_case_id IS NULL) OR (type = 'shadow_ban' AND machine_case_id IS NOT NULL)))
);

CREATE TABLE automod_terminal_subjects (
  subject_id uuid PRIMARY KEY,
  purged_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
CREATE TABLE automod_subjects (
  subject_id uuid PRIMARY KEY,
  account_id uuid NOT NULL UNIQUE,
  -- Other moderation functions can bind a fresh Auth subject before its first
  -- Messaging send. Only the private producer provision binds a source epoch.
  source_epoch uuid,
  lifecycle_revision bigint NOT NULL CHECK (lifecycle_revision > 0),
  lifecycle_state text NOT NULL CHECK (lifecycle_state IN ('LIVE','DELETED')),
  purge_at timestamptz,
  through_sequence bigint NOT NULL DEFAULT 0 CHECK (through_sequence >= 0),
  last_source_at timestamptz,
  muted_until timestamptz,
  post_mute_count integer NOT NULL DEFAULT 0 CHECK (post_mute_count BETWEEN 0 AND 10),
  restriction_revision bigint NOT NULL DEFAULT 0 CHECK (restriction_revision >= 0),
  CHECK ((lifecycle_state='LIVE' AND purge_at IS NULL) OR (lifecycle_state='DELETED' AND purge_at IS NOT NULL))
);
CREATE TABLE automod_inbox (
  subject_id uuid NOT NULL REFERENCES automod_subjects ON DELETE CASCADE,
  sequence bigint NOT NULL CHECK (sequence > 0),
  event_id uuid NOT NULL,
  recorded_at timestamptz NOT NULL,
  original_hash bytea NOT NULL CHECK (octet_length(original_hash)=32),
  payload jsonb,
  applied_at timestamptz,
  PRIMARY KEY (subject_id, sequence),
  UNIQUE (subject_id, event_id)
);
CREATE INDEX automod_inbox_expiry_idx ON automod_inbox (recorded_at);
CREATE TABLE automod_patterns (
  subject_id uuid NOT NULL REFERENCES automod_subjects ON DELETE CASCADE,
  pattern_id uuid NOT NULL,
  state jsonb NOT NULL,
  last_seen_at timestamptz NOT NULL,
  PRIMARY KEY(subject_id, pattern_id)
);
CREATE TABLE automod_pattern_aliases (
  subject_id uuid NOT NULL,
  key_id text NOT NULL,
  digest bytea NOT NULL CHECK (octet_length(digest)=32),
  pattern_id uuid NOT NULL,
  PRIMARY KEY(subject_id, key_id, digest),
  FOREIGN KEY(subject_id, pattern_id) REFERENCES automod_patterns ON DELETE CASCADE
);
CREATE TABLE automod_actions (
  subject_id uuid NOT NULL REFERENCES automod_subjects ON DELETE CASCADE,
  sequence bigint NOT NULL,
  kind text NOT NULL CHECK (kind IN ('mute','perm_ban')),
  source_at timestamptz NOT NULL,
  applied_at timestamptz NOT NULL,
  expires_at timestamptz,
  sanction_id uuid REFERENCES sanctions,
  PRIMARY KEY(subject_id, sequence)
);
CREATE TABLE automod_event_outbox (
  event_id uuid PRIMARY KEY,
  subject_id uuid NOT NULL REFERENCES automod_subjects ON DELETE CASCADE,
  payload jsonb NOT NULL,
  created_at timestamptz NOT NULL DEFAULT clock_timestamp(),
  published_at timestamptz
);
CREATE TABLE automod_restriction_sync (
  subject_id uuid PRIMARY KEY REFERENCES automod_subjects ON DELETE CASCADE,
  requested_at timestamptz NOT NULL DEFAULT clock_timestamp()
);
