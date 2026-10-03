CREATE TABLE player_binding_exchange_operations (
  operation_id UUID PRIMARY KEY,
  challenge_id UUID NOT NULL UNIQUE REFERENCES player_binding_challenges(challenge_id),
  request_sha256 CHAR(64) NOT NULL CHECK (request_sha256 ~ '^[0-9a-f]{64}$'),
  status TEXT NOT NULL CHECK (status IN ('pending','claimed','created','completed','failed')),
  handoff_jws TEXT,
  auth_claim_id UUID UNIQUE,
  assertion_jti UUID UNIQUE,
  binding_id UUID UNIQUE REFERENCES player_bindings(binding_id),
  result JSONB,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK ((status='pending' AND handoff_jws IS NULL AND auth_claim_id IS NULL AND assertion_jti IS NULL AND binding_id IS NULL AND result IS NULL)
      OR (status='claimed' AND handoff_jws IS NOT NULL AND auth_claim_id IS NOT NULL AND assertion_jti IS NOT NULL AND binding_id IS NULL AND result IS NULL)
      OR (status='created' AND handoff_jws IS NOT NULL AND auth_claim_id IS NOT NULL AND assertion_jti IS NOT NULL AND binding_id IS NOT NULL AND result IS NULL)
      OR (status='completed' AND handoff_jws IS NOT NULL AND auth_claim_id IS NOT NULL AND assertion_jti IS NOT NULL AND binding_id IS NOT NULL AND result IS NOT NULL)
      OR (status='failed' AND handoff_jws IS NOT NULL AND auth_claim_id IS NOT NULL AND assertion_jti IS NOT NULL AND binding_id IS NULL AND result IS NOT NULL))
);

CREATE TABLE player_binding_completion_outbox (
  operation_id UUID PRIMARY KEY REFERENCES player_binding_exchange_operations(operation_id),
  claim_id UUID NOT NULL UNIQUE,
  assertion_jti UUID NOT NULL UNIQUE,
  binding_id UUID REFERENCES player_bindings(binding_id),
  outcome TEXT NOT NULL CHECK (outcome IN ('succeeded','failed')),
  attempt_count INTEGER NOT NULL DEFAULT 0 CHECK (attempt_count >= 0),
  next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  last_attempt_at TIMESTAMPTZ,
  completed_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  CHECK ((outcome='succeeded' AND binding_id IS NOT NULL) OR (outcome='failed' AND binding_id IS NULL))
);

CREATE INDEX player_binding_completion_outbox_pending_idx
  ON player_binding_completion_outbox(next_attempt_at) WHERE completed_at IS NULL;
