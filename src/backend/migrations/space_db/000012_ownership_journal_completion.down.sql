-- Preserve terminal outcomes and every public effect which can expose them.
BEGIN;
LOCK TABLE ownership_journal IN ACCESS EXCLUSIVE MODE;
LOCK TABLE ownership_outbox IN ACCESS EXCLUSIVE MODE;
LOCK TABLE audit_log IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM ownership_journal
        WHERE state IN ('completed','aborted')
            OR role_terminal_receipt_bytes IS NOT NULL
            OR role_terminal_receipt_hash IS NOT NULL
    ) OR EXISTS (
        SELECT 1 FROM ownership_outbox WHERE ready
    ) OR EXISTS (
        SELECT 1 FROM audit_log a
        JOIN ownership_journal j ON j.audit_id = a.id
    ) THEN
        RAISE EXCEPTION 'cannot remove ownership journal terminal evidence';
    END IF;
END
$$;

DROP TRIGGER ownership_journal_terminal_evidence_immutable ON ownership_journal;
DROP FUNCTION guard_ownership_journal_terminal_evidence();

ALTER TABLE ownership_journal
    DROP CONSTRAINT ownership_journal_aborted_evidence,
    DROP CONSTRAINT ownership_journal_completed_evidence,
    DROP CONSTRAINT ownership_journal_terminal_receipt_state,
    DROP CONSTRAINT ownership_journal_terminal_receipt_all_or_none,
    DROP COLUMN role_terminal_receipt_hash,
    DROP COLUMN role_terminal_receipt_bytes;

COMMIT;
