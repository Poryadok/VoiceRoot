CREATE TABLE IF NOT EXISTS voice_sdk_conversion_fence_receipts (
  operation_id UUID PRIMARY KEY,
  binding_id UUID NOT NULL,
  source_account_id UUID NOT NULL,
  source_actor_id UUID NOT NULL,
  source_profile_id UUID NOT NULL,
  target_account_id UUID NOT NULL,
  target_profile_id UUID NOT NULL,
  frozen_authority_epoch BIGINT NOT NULL CHECK (frozen_authority_epoch > 0),
  freeze_receipt_id UUID NOT NULL,
  request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
  receipt_id UUID NOT NULL UNIQUE,
  state TEXT NOT NULL CHECK (state IN ('pending', 'conflict', 'fenced', 'activated')),
  source_room_id TEXT,
  media_generation BIGINT NOT NULL DEFAULT 1 CHECK (media_generation > 0),
  target_session_conflict BOOLEAN NOT NULL DEFAULT FALSE,
  observed_ejection_at TIMESTAMPTZ,
  committed_at TIMESTAMPTZ,
  activation_receipt_id UUID UNIQUE,
  activation_response_receipt_id UUID UNIQUE,
  activation_request_hash BYTEA CHECK (activation_request_hash IS NULL OR octet_length(activation_request_hash) = 32),
  activated_at TIMESTAMPTZ,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  CHECK ((state = 'pending' AND committed_at IS NULL)
      OR (state IN ('conflict', 'fenced', 'activated') AND committed_at IS NOT NULL)),
  CHECK (state <> 'conflict' OR target_session_conflict),
  CHECK ((state = 'activated') = (activation_receipt_id IS NOT NULL)),
  CHECK ((state = 'activated') = (activation_response_receipt_id IS NOT NULL)),
  CHECK ((state = 'activated') = (activation_request_hash IS NOT NULL)),
  CHECK ((state = 'activated') = (activated_at IS NOT NULL))
);

CREATE INDEX IF NOT EXISTS voice_sdk_conversion_target_fence_idx
  ON voice_sdk_conversion_fence_receipts(target_account_id, target_profile_id)
  WHERE state IN ('pending', 'fenced');

CREATE INDEX IF NOT EXISTS voice_sdk_conversion_source_fence_idx
  ON voice_sdk_conversion_fence_receipts(source_account_id, source_profile_id)
  WHERE state IN ('pending', 'fenced');
