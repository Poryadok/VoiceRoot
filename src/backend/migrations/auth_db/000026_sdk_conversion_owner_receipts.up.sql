ALTER TABLE sdk_conversion_operations
  ADD COLUMN preview_revision CHAR(64),
  ADD COLUMN preview_json JSONB,
  ADD COLUMN confirmed_at TIMESTAMPTZ,
  ADD COLUMN transfer_generation INTEGER NOT NULL DEFAULT 0 CHECK (transfer_generation BETWEEN 0 AND 10),
  ADD COLUMN last_owner_error TEXT;

CREATE TABLE sdk_conversion_owner_receipts (
  operation_id UUID NOT NULL REFERENCES sdk_conversion_operations(operation_id),
  owner VARCHAR(16) NOT NULL CHECK (owner IN ('gis','user','voice')),
  stage VARCHAR(16) NOT NULL CHECK (stage IN ('freeze','tombstone','fence','transfer','activate')),
  generation INTEGER NOT NULL DEFAULT 0 CHECK (generation BETWEEN 0 AND 10),
  request_hash CHAR(64) NOT NULL CHECK (request_hash ~ '^[0-9a-f]{64}$'),
  receipt_id UUID NOT NULL,
  receipt_json JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL,
  PRIMARY KEY(operation_id, owner, stage, generation),
  UNIQUE(owner, receipt_id)
);

CREATE INDEX sdk_conversion_recovery_idx
  ON sdk_conversion_operations(state, created_at, operation_id)
  WHERE state IN ('confirmed','frozen','owners_ready','activated');
