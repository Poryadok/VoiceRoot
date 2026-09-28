CREATE TABLE player_binding_challenges (
  challenge_id UUID PRIMARY KEY,
  nonce VARCHAR(43) NOT NULL UNIQUE,
  application_id UUID NOT NULL,
  environment_id UUID NOT NULL,
  provider VARCHAR(32) NOT NULL,
  redirect_uri_sha256 CHAR(64) NOT NULL CHECK (redirect_uri_sha256 ~ '^[0-9a-f]{64}$'),
  pkce_challenge VARCHAR(43) NOT NULL,
  device_key_id UUID NOT NULL,
  device_key_thumbprint VARCHAR(43) NOT NULL,
  operation_id UUID NOT NULL UNIQUE,
  expires_at TIMESTAMPTZ NOT NULL,
  status VARCHAR(16) NOT NULL CHECK (status IN ('pending','consumed','expired')),
  consumed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK ((status='consumed' AND consumed_at IS NOT NULL) OR (status<>'consumed' AND consumed_at IS NULL))
);
CREATE INDEX player_binding_challenges_expiry_idx ON player_binding_challenges(status, expires_at);
