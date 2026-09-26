-- Consent prepares a link; no active Game Integration binding is implied.
CREATE TABLE sdk_authorizations (
  request_id UUID PRIMARY KEY,
  source_account_id UUID NOT NULL REFERENCES sdk_identities(account_id),
  device_id UUID NOT NULL REFERENCES sdk_devices(device_id),
  source_session_hash CHAR(64) NOT NULL,
  source_generation BIGINT NOT NULL CHECK (source_generation > 0),
  application_id UUID NOT NULL,
  environment_id UUID NOT NULL,
  idempotency_key UUID NOT NULL,
  request_hash CHAR(64) NOT NULL,
  redirect_uri TEXT NOT NULL,
  code_challenge VARCHAR(43) NOT NULL,
  client_state VARCHAR(128) NOT NULL,
  scopes TEXT NOT NULL,
  policy_revision BIGINT NOT NULL CHECK (policy_revision > 0),
  display_name TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  code_hash CHAR(64),
  code_expires_at TIMESTAMPTZ,
  target_account_id UUID REFERENCES accounts(id),
  target_profile_id UUID,
  target_epoch BIGINT,
  profile_revision BIGINT,
  approval_jti TEXT,
  approval_expires_at TIMESTAMPTZ,
  consumed_at TIMESTAMPTZ,
  UNIQUE(source_account_id, idempotency_key)
);
CREATE INDEX sdk_authorizations_expiry_idx ON sdk_authorizations(expires_at);

CREATE TABLE sdk_linked_sessions (
  token_hash CHAR(64) PRIMARY KEY,
  request_id UUID NOT NULL UNIQUE REFERENCES sdk_authorizations(request_id),
  consent_revision BIGSERIAL NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX sdk_linked_sessions_expiry_idx ON sdk_linked_sessions(expires_at);
