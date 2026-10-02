-- Exact request bytes make Auth assertion retries return the immutable original result.
CREATE TABLE sdk_device_authority_issues (
  request_id UUID PRIMARY KEY,
  request_bytes BYTEA NOT NULL,
  session_token_hash CHAR(64) NOT NULL,
  assertion TEXT NOT NULL,
  device_id UUID NOT NULL REFERENCES sdk_devices(device_id),
  key_id UUID NOT NULL REFERENCES sdk_device_keys(key_id),
  issued_at TIMESTAMPTZ NOT NULL,
  expires_at TIMESTAMPTZ NOT NULL
);
