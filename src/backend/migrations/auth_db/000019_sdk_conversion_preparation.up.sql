-- Prepared SDK conversions are distinct from legacy guest promotion operations.
CREATE TABLE sdk_conversion_operations (
  operation_id UUID PRIMARY KEY,
  mode VARCHAR(16) NOT NULL CHECK (mode IN ('new','existing')),
  state VARCHAR(16) NOT NULL DEFAULT 'prepared'
    CHECK (state IN ('prepared','previewed','confirmed','frozen','owners_ready','activated','retired','cancelled')),
  revision BIGINT NOT NULL DEFAULT 1 CHECK (revision > 0),
  source_account_id UUID NOT NULL REFERENCES sdk_identities(account_id),
  device_id UUID NOT NULL REFERENCES sdk_devices(device_id),
  public_jwk TEXT NOT NULL,
  application_id UUID NOT NULL,
  environment_id UUID NOT NULL,
  source_generation BIGINT NOT NULL CHECK (source_generation > 0),
  binding_id UUID NOT NULL,
  idempotency_key UUID NOT NULL,
  request_hash CHAR(64) NOT NULL,
  registered_account_id UUID REFERENCES accounts(id),
  target_account_id UUID REFERENCES accounts(id),
  target_profile_id UUID,
  target_epoch BIGINT,
  approval_jti TEXT,
  approval_expires_at TIMESTAMPTZ,
  profile_revision BIGINT,
  created_at TIMESTAMPTZ NOT NULL,
  UNIQUE(source_account_id, mode, idempotency_key),
  CHECK ((target_account_id IS NULL) = (target_profile_id IS NULL)),
  CHECK (mode='new' OR registered_account_id IS NULL)
);
CREATE UNIQUE INDEX sdk_conversion_one_confirmed_source_idx
  ON sdk_conversion_operations(source_account_id)
  WHERE state IN ('confirmed','frozen','owners_ready','activated','retired');

CREATE TABLE sdk_registration_intents (
  intent_id UUID PRIMARY KEY,
  operation_id UUID NOT NULL UNIQUE REFERENCES sdk_conversion_operations(operation_id),
  expires_at TIMESTAMPTZ NOT NULL,
  consumed_at TIMESTAMPTZ,
  account_id UUID UNIQUE REFERENCES accounts(id),
  CHECK ((consumed_at IS NULL) = (account_id IS NULL))
);
CREATE INDEX sdk_registration_intents_expiry_idx ON sdk_registration_intents(expires_at);
