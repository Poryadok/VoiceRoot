CREATE TABLE sdk_game_message_grants (
  grant_id UUID PRIMARY KEY,
  authorization_request_id UUID NOT NULL REFERENCES sdk_authorizations(request_id),
  application_id UUID NOT NULL,
  environment_id UUID NOT NULL,
  target_account_id UUID NOT NULL REFERENCES accounts(id),
  target_profile_id UUID NOT NULL,
  target_epoch BIGINT NOT NULL CHECK (target_epoch > 0),
  binding_id UUID NOT NULL UNIQUE,
  consent_revision BIGINT NOT NULL CHECK (consent_revision > 0),
  scopes TEXT NOT NULL,
  policy_revision BIGINT NOT NULL CHECK (policy_revision > 0),
  profile_revision BIGINT NOT NULL CHECK (profile_revision > 0),
  status VARCHAR(16) NOT NULL CHECK (status IN ('active','revoking','revoked')),
  authority_revision BIGINT NOT NULL CHECK (authority_revision > 0),
  created_at TIMESTAMPTZ NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL
);

CREATE UNIQUE INDEX sdk_game_message_grants_active_key_idx
  ON sdk_game_message_grants(application_id, environment_id, target_account_id, target_profile_id)
  WHERE status IN ('active','revoking');
CREATE INDEX sdk_game_message_grants_request_idx
  ON sdk_game_message_grants(authorization_request_id);

CREATE TABLE sdk_game_message_execution_permits (
  permit_jti UUID PRIMARY KEY,
  grant_id UUID NOT NULL REFERENCES sdk_game_message_grants(grant_id),
  operation_id UUID NOT NULL,
  request_sha256 CHAR(64) NOT NULL CHECK (request_sha256 ~ '^[0-9a-f]{64}$'),
  assertion_jti UUID NOT NULL,
  gis_permit_id UUID NOT NULL UNIQUE,
  binding_revision BIGINT NOT NULL CHECK (binding_revision > 0),
  permit_jws TEXT NOT NULL,
  issued_at_ms BIGINT NOT NULL,
  expires_at_ms BIGINT NOT NULL,
  completion_outcome VARCHAR(16) CHECK (completion_outcome IN ('committed','aborted')),
  completion_receipt JSONB,
  created_at TIMESTAMPTZ NOT NULL,
  completed_at TIMESTAMPTZ,
  UNIQUE(operation_id),
  CHECK (expires_at_ms > issued_at_ms AND expires_at_ms - issued_at_ms <= 3750),
  CHECK ((completion_outcome IS NULL AND completion_receipt IS NULL AND completed_at IS NULL)
      OR (completion_outcome IS NOT NULL AND completion_receipt IS NOT NULL AND completed_at IS NOT NULL))
);

CREATE INDEX sdk_game_message_execution_permits_drain_idx
  ON sdk_game_message_execution_permits(grant_id, expires_at_ms)
  WHERE completion_outcome IS NULL;
