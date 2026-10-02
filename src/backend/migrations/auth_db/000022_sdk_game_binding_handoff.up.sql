ALTER TABLE sdk_authorizations
  ADD COLUMN game_binding_id UUID,
  ADD COLUMN game_binding_status VARCHAR(16) NOT NULL DEFAULT 'unbound'
    CHECK (game_binding_status IN ('unbound','active','revoking','revoked')),
  ADD COLUMN game_binding_authority_revision BIGINT NOT NULL DEFAULT 0
    CHECK (game_binding_authority_revision >= 0),
  ADD COLUMN game_binding_challenge_id UUID,
  ADD COLUMN game_binding_operation_id UUID,
  ADD CONSTRAINT sdk_authorizations_binding_challenge_pair_check
    CHECK ((game_binding_challenge_id IS NULL) = (game_binding_operation_id IS NULL));

CREATE UNIQUE INDEX sdk_authorizations_binding_operation_idx
  ON sdk_authorizations(game_binding_operation_id) WHERE game_binding_operation_id IS NOT NULL;

CREATE TABLE sdk_game_binding_handoff_issuances (
  authorization_request_id UUID PRIMARY KEY REFERENCES sdk_authorizations(request_id),
  challenge_id UUID NOT NULL UNIQUE,
  operation_id UUID NOT NULL UNIQUE,
  code_sha256 CHAR(64) NOT NULL CHECK (code_sha256 ~ '^[0-9a-f]{64}$'),
  verifier_sha256 CHAR(64) NOT NULL CHECK (verifier_sha256 ~ '^[0-9a-f]{64}$'),
  request_sha256 CHAR(64) NOT NULL CHECK (request_sha256 ~ '^[0-9a-f]{64}$'),
  device_id UUID NOT NULL,
  device_proof_sha256 CHAR(64) NOT NULL CHECK (device_proof_sha256 ~ '^[0-9a-f]{64}$'),
  assertion_jti UUID NOT NULL UNIQUE,
  handoff_jws TEXT NOT NULL,
  consent_revision BIGINT NOT NULL UNIQUE CHECK (consent_revision > 0),
  expires_at TIMESTAMPTZ NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE sdk_game_binding_handoff_claims (
  claim_id UUID PRIMARY KEY,
  authorization_request_id UUID NOT NULL REFERENCES sdk_authorizations(request_id),
  challenge_id UUID NOT NULL,
  assertion_jti UUID NOT NULL UNIQUE,
  operation_id UUID NOT NULL UNIQUE,
  request_sha256 CHAR(64) NOT NULL CHECK (request_sha256 ~ '^[0-9a-f]{64}$'),
  source_device_id UUID NOT NULL,
  state VARCHAR(16) NOT NULL CHECK (state IN ('claimed','completed','failed')),
  expires_at TIMESTAMPTZ NOT NULL,
  result_binding_id UUID,
  result_sha256 CHAR(64),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  completed_at TIMESTAMPTZ,
  CHECK ((state='claimed' AND completed_at IS NULL AND result_binding_id IS NULL AND result_sha256 IS NULL)
      OR (state='completed' AND completed_at IS NOT NULL AND result_binding_id IS NOT NULL AND result_sha256 IS NOT NULL)
      OR (state='failed' AND completed_at IS NOT NULL AND result_binding_id IS NULL AND result_sha256 IS NOT NULL))
);

CREATE INDEX sdk_game_binding_handoff_claims_drain_idx
  ON sdk_game_binding_handoff_claims(authorization_request_id, expires_at)
  WHERE state='claimed';

CREATE UNIQUE INDEX sdk_authorizations_game_binding_id_idx
  ON sdk_authorizations(game_binding_id) WHERE game_binding_id IS NOT NULL;
