ALTER TABLE player_bindings
    ADD COLUMN authority_epoch BIGINT NOT NULL DEFAULT 1 CHECK (authority_epoch > 0),
    ADD COLUMN conversion_operation_id UUID,
    ADD COLUMN pending_target_account_id UUID,
    ADD COLUMN pending_target_profile_id UUID,
    ADD COLUMN active_grant_digest TEXT,
    DROP CONSTRAINT player_bindings_status_check,
    ADD CONSTRAINT player_bindings_status_check
        CHECK (status IN ('pending','active','revoking','revoked','frozen','transferring','retired'));

CREATE TABLE player_binding_conversion_receipts (
    operation_id UUID NOT NULL,
    stage TEXT NOT NULL CHECK (stage IN ('freeze','transfer','activate','retire')),
    transfer_generation INTEGER NOT NULL DEFAULT 0 CHECK (transfer_generation >= 0),
    binding_id UUID NOT NULL REFERENCES player_bindings(binding_id),
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash)=32),
    receipt_bytes BYTEA NOT NULL,
    receipt_hash BYTEA NOT NULL CHECK (octet_length(receipt_hash)=32),
    binding_revision BIGINT NOT NULL CHECK (binding_revision > 0),
    authority_epoch BIGINT NOT NULL CHECK (authority_epoch > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY(operation_id,stage,transfer_generation)
);
