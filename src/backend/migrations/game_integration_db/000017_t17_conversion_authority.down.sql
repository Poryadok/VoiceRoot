DROP TABLE IF EXISTS player_binding_conversion_receipts;
ALTER TABLE player_bindings
    DROP COLUMN IF EXISTS active_grant_digest,
    DROP COLUMN IF EXISTS pending_target_profile_id,
    DROP COLUMN IF EXISTS pending_target_account_id,
    DROP COLUMN IF EXISTS conversion_operation_id,
    DROP COLUMN IF EXISTS authority_epoch,
    DROP CONSTRAINT IF EXISTS player_bindings_status_check,
    ADD CONSTRAINT player_bindings_status_check
        CHECK (status IN ('pending','active','revoking','revoked'));
