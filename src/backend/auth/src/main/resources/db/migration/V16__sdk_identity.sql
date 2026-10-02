-- SDK identity is a distinct Auth principal, never a legacy guest or regular session.
CREATE TABLE sdk_identities (
  account_id UUID PRIMARY KEY,
  actor_id UUID NOT NULL UNIQUE,
  application_id UUID NOT NULL,
  environment_id UUID NOT NULL,
  issuer VARCHAR(255) NOT NULL,
  provider_subject VARCHAR(255) NOT NULL,
  ownership_generation BIGINT NOT NULL DEFAULT 1 CHECK (ownership_generation > 0),
  status VARCHAR(16) NOT NULL DEFAULT 'active' CHECK (status IN ('active','suspended','deleted','retired')),
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE(application_id, environment_id, issuer, provider_subject)
);

CREATE TABLE sdk_devices (
  device_id UUID PRIMARY KEY,
  account_id UUID NOT NULL REFERENCES sdk_identities(account_id),
  thumbprint VARCHAR(64) NOT NULL,
  public_jwk TEXT NOT NULL,
  revoked_at TIMESTAMPTZ,
  UNIQUE(account_id, thumbprint)
);

CREATE TABLE sdk_challenges (
  challenge_id UUID PRIMARY KEY,
  application_id UUID NOT NULL,
  environment_id UUID NOT NULL,
  client_id VARCHAR(255) NOT NULL,
  nonce VARCHAR(64) NOT NULL UNIQUE,
  public_jwk TEXT NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ
);
CREATE INDEX sdk_challenges_expiry_idx ON sdk_challenges(expires_at);

CREATE TABLE sdk_sessions (
  token_hash CHAR(64) PRIMARY KEY,
  account_id UUID NOT NULL REFERENCES sdk_identities(account_id),
  device_id UUID NOT NULL REFERENCES sdk_devices(device_id),
  ownership_generation BIGINT NOT NULL CHECK (ownership_generation > 0),
  game_subject VARCHAR(255) NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL
);
CREATE INDEX sdk_sessions_device_idx ON sdk_sessions(device_id);
CREATE INDEX sdk_sessions_expiry_idx ON sdk_sessions(expires_at);
