ALTER TABLE gis_game_action_invocations
  ADD COLUMN character_binding_id UUID,
  ADD COLUMN binding_revision BIGINT CHECK (binding_revision IS NULL OR binding_revision > 0);

ALTER TABLE gis_game_action_operations
  ADD COLUMN character_binding_id UUID,
  ADD COLUMN binding_revision BIGINT CHECK (binding_revision IS NULL OR binding_revision > 0),
  ADD COLUMN action_type TEXT,
  ADD COLUMN arguments_json JSONB,
  ADD COLUMN command_id UUID UNIQUE;

CREATE TABLE gis_game_command_outbox (
  command_id UUID PRIMARY KEY,
  operation_id UUID NOT NULL UNIQUE REFERENCES gis_game_action_operations(operation_id),
  application_id UUID NOT NULL,
  environment_id UUID NOT NULL,
  installation_id UUID NOT NULL,
  callback_url TEXT NOT NULL CHECK (length(callback_url) BETWEEN 1 AND 2048),
  credential_id UUID NOT NULL,
  canonical_body BYTEA NOT NULL CHECK (octet_length(canonical_body) BETWEEN 1 AND 16384),
  body_sha256 BYTEA NOT NULL CHECK (octet_length(body_sha256)=32),
  delivery_status TEXT NOT NULL CHECK (delivery_status IN ('pending','leased','accepted','reconciling','dead_letter')),
  attempt_count SMALLINT NOT NULL DEFAULT 0 CHECK (attempt_count BETWEEN 0 AND 6),
  next_attempt_at TIMESTAMPTZ NOT NULL,
  accepted_at TIMESTAMPTZ,
  terminal_at TIMESTAMPTZ NOT NULL,
  lease_id UUID,
  lease_until TIMESTAMPTZ,
  last_http_status SMALLINT,
  last_error_code TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
  CHECK ((lease_id IS NULL) = (lease_until IS NULL)),
  CHECK ((delivery_status='leased') = (lease_id IS NOT NULL))
);

CREATE INDEX gis_game_command_delivery_idx
  ON gis_game_command_outbox(next_attempt_at, created_at)
  WHERE delivery_status IN ('pending','leased','reconciling');
