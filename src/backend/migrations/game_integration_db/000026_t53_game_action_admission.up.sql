CREATE TABLE gis_game_action_invocations (
  invocation_id UUID PRIMARY KEY,
  account_id UUID NOT NULL,
  profile_id UUID NOT NULL,
  session_id UUID NOT NULL,
  session_epoch BIGINT NOT NULL CHECK (session_epoch > 0),
  roster_revision BIGINT NOT NULL CHECK (roster_revision > 0),
  application_id UUID NOT NULL,
  environment_id UUID NOT NULL,
  installation_id UUID NOT NULL,
  message_id UUID NOT NULL,
  chat_id UUID NOT NULL,
  card_revision BIGINT NOT NULL CHECK (card_revision > 0),
  action_id UUID NOT NULL,
  arguments_sha256 BYTEA NOT NULL CHECK (octet_length(arguments_sha256)=32),
  state_version TEXT NOT NULL CHECK (length(state_version) BETWEEN 1 AND 256),
  idempotency_key UUID NOT NULL,
  request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256)=32),
  challenge_sha256 BYTEA NOT NULL UNIQUE CHECK (octet_length(challenge_sha256)=32),
  challenge_secret_ciphertext BYTEA NOT NULL,
  challenge_status TEXT NOT NULL CHECK (challenge_status IN ('unused','consumed','expired','revoked')),
  confirm_idempotency_key UUID,
  confirm_request_sha256 BYTEA CHECK (confirm_request_sha256 IS NULL OR octet_length(confirm_request_sha256)=32),
  expires_at TIMESTAMPTZ NOT NULL,
  operation_id UUID NOT NULL UNIQUE,
  created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
  consumed_at TIMESTAMPTZ,
  UNIQUE(account_id,profile_id,idempotency_key)
);

CREATE TABLE gis_game_action_operations (
  operation_id UUID PRIMARY KEY,
  invocation_id UUID NOT NULL UNIQUE REFERENCES gis_game_action_invocations(invocation_id),
  account_id UUID NOT NULL,
  profile_id UUID NOT NULL,
  application_id UUID NOT NULL,
  environment_id UUID NOT NULL,
  installation_id UUID NOT NULL,
  message_id UUID NOT NULL,
  card_revision BIGINT NOT NULL,
  action_id UUID NOT NULL,
  arguments_sha256 BYTEA NOT NULL CHECK (octet_length(arguments_sha256)=32),
  state_version TEXT NOT NULL,
  operation_status TEXT NOT NULL CHECK (operation_status IN ('accepted','cancelled','reconciliation_required')),
  accepted_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX gis_game_action_expiry_idx ON gis_game_action_invocations(expires_at)
  WHERE challenge_status='unused';
