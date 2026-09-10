-- Preserve every persisted receipt or irreversible decision. Reserved-only
-- journals do not use the added columns and remain valid after this rollback.
BEGIN;
LOCK TABLE ownership_journal IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM ownership_journal
        WHERE state <> 'reserved' OR auth_receipt_id IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'cannot remove ownership journal decision evidence';
    END IF;
END
$$;

ALTER TABLE ownership_journal
    DROP CONSTRAINT ownership_journal_proof_confirmed_has_receipt,
    DROP CONSTRAINT ownership_journal_reserved_has_no_receipt,
    DROP CONSTRAINT ownership_journal_auth_receipt_matches_binding,
    DROP CONSTRAINT ownership_journal_auth_receipt_all_or_none,
    DROP COLUMN auth_verified_factors,
    DROP COLUMN auth_consumed_at,
    DROP COLUMN auth_session_epoch,
    DROP COLUMN auth_operation_id,
    DROP COLUMN auth_new_owner_profile_id,
    DROP COLUMN auth_space_id,
    DROP COLUMN auth_profile_id,
    DROP COLUMN auth_account_id,
    DROP COLUMN auth_receipt_id;

COMMIT;
