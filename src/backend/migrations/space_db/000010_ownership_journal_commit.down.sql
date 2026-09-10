-- Preserve every Role receipt, local commit decision and private outbox row.
BEGIN;
LOCK TABLE ownership_journal IN ACCESS EXCLUSIVE MODE;
LOCK TABLE ownership_outbox IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM ownership_journal
        WHERE state IN ('prepared','commit_decided')
            OR role_receipt_bytes IS NOT NULL
            OR pending_audit_action IS NOT NULL
    ) OR EXISTS (SELECT 1 FROM ownership_outbox) THEN
        RAISE EXCEPTION 'cannot remove ownership journal commit evidence';
    END IF;
END
$$;

DROP TABLE ownership_outbox;

ALTER TABLE ownership_journal
    DROP CONSTRAINT ownership_journal_pending_audit_requires_role,
    DROP CONSTRAINT ownership_journal_pending_audit_state,
    DROP CONSTRAINT ownership_journal_commit_has_pending_artifacts,
    DROP CONSTRAINT ownership_journal_pending_audit_all_or_none,
    DROP CONSTRAINT ownership_journal_role_receipt_state,
    DROP CONSTRAINT ownership_journal_prepared_has_role_receipt,
    DROP CONSTRAINT ownership_journal_role_receipt_requires_auth,
    DROP CONSTRAINT ownership_journal_role_receipt_all_or_none,
    DROP COLUMN pending_audit_details,
    DROP COLUMN pending_audit_target_id,
    DROP COLUMN pending_audit_target_type,
    DROP COLUMN pending_audit_action,
    DROP COLUMN role_intent_hash,
    DROP COLUMN role_intent_bytes,
    DROP COLUMN role_receipt_hash,
    DROP COLUMN role_receipt_bytes;

COMMIT;
