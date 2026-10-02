-- T15 Auth-owned public signing-key generations. Private key material never enters Auth.
ALTER TABLE sdk_devices ADD COLUMN authority_revision BIGINT NOT NULL DEFAULT 1
  CHECK (authority_revision > 0);
ALTER TABLE sdk_challenges ADD COLUMN purpose VARCHAR(16) NOT NULL DEFAULT 'enroll'
  CHECK (purpose IN ('enroll','rotate','recover'));
ALTER TABLE sdk_challenges ADD COLUMN device_id UUID;
ALTER TABLE sdk_challenges ADD COLUMN account_id UUID;
ALTER TABLE sdk_challenges ADD COLUMN replaces_device_id UUID;

CREATE TABLE sdk_device_keys (
  key_id UUID PRIMARY KEY,
  device_id UUID NOT NULL REFERENCES sdk_devices(device_id),
  application_id UUID NOT NULL,
  environment_id UUID NOT NULL,
  public_jwk TEXT NOT NULL,
  key_thumbprint VARCHAR(64) NOT NULL,
  generation BIGINT NOT NULL CHECK (generation > 0),
  not_before TIMESTAMPTZ NOT NULL,
  not_after TIMESTAMPTZ NOT NULL,
  status VARCHAR(16) NOT NULL CHECK (status IN ('active','overlap','revoked','expired')),
  revoked_at TIMESTAMPTZ,
  UNIQUE (device_id, generation)
);
CREATE UNIQUE INDEX sdk_device_keys_one_active_idx
  ON sdk_device_keys(device_id) WHERE status = 'active';
CREATE INDEX sdk_device_keys_device_idx ON sdk_device_keys(device_id, generation DESC);

CREATE TABLE sdk_device_key_operations (
  request_id UUID PRIMARY KEY,
  operation VARCHAR(16) NOT NULL CHECK (operation IN ('rotate','recover','revoke')),
  request_bytes BYTEA NOT NULL,
  account_id UUID NOT NULL REFERENCES sdk_identities(account_id),
  device_id UUID NOT NULL REFERENCES sdk_devices(device_id),
  key_id UUID NOT NULL REFERENCES sdk_device_keys(key_id),
  generation BIGINT NOT NULL CHECK (generation > 0),
  authority_revision BIGINT NOT NULL CHECK (authority_revision > 0),
  created_at TIMESTAMPTZ NOT NULL
);

-- Existing bootstrap devices keep their established public key and gain an Auth key ID.
INSERT INTO sdk_device_keys (
  key_id, device_id, application_id, environment_id, public_jwk, key_thumbprint,
  generation, not_before, not_after, status, revoked_at
)
SELECT gen_random_uuid(), d.device_id, i.application_id, i.environment_id,
       d.public_jwk, d.thumbprint, 1, now(), now() + interval '90 days',
       CASE WHEN d.revoked_at IS NULL THEN 'active' ELSE 'revoked' END, d.revoked_at
FROM sdk_devices d
JOIN sdk_identities i ON i.account_id = d.account_id;
