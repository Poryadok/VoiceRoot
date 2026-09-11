-- Add immutable authoritative Role terminal receipt evidence.
ALTER TABLE ownership_journal
    ADD COLUMN role_terminal_receipt_bytes BYTEA,
    ADD COLUMN role_terminal_receipt_hash BYTEA,
    ADD CONSTRAINT ownership_journal_terminal_receipt_all_or_none CHECK (
        (role_terminal_receipt_bytes IS NULL AND role_terminal_receipt_hash IS NULL)
        OR
        (role_terminal_receipt_bytes IS NOT NULL
            AND role_terminal_receipt_hash IS NOT NULL
            AND octet_length(role_terminal_receipt_hash) = 32)
    ),
    ADD CONSTRAINT ownership_journal_terminal_receipt_state CHECK (
        role_terminal_receipt_bytes IS NULL OR state IN ('completed','aborted')
    ),
    ADD CONSTRAINT ownership_journal_completed_evidence CHECK (
        state <> 'completed' OR (
            role_terminal_receipt_bytes IS NOT NULL
            AND role_receipt_bytes IS NOT NULL
            AND pending_audit_action IS NOT NULL
        )
    ),
    ADD CONSTRAINT ownership_journal_aborted_evidence CHECK (
        state <> 'aborted' OR (
            role_terminal_receipt_bytes IS NOT NULL
            AND pending_audit_action IS NULL
        )
    );

CREATE FUNCTION guard_ownership_journal_terminal_evidence()
RETURNS trigger
LANGUAGE plpgsql
AS $$
BEGIN
    IF OLD.role_terminal_receipt_bytes IS NOT NULL AND (
        NEW.role_terminal_receipt_bytes IS DISTINCT FROM OLD.role_terminal_receipt_bytes
        OR NEW.role_terminal_receipt_hash IS DISTINCT FROM OLD.role_terminal_receipt_hash
    ) THEN
        RAISE EXCEPTION 'ownership journal terminal receipt evidence is immutable';
    END IF;
    IF OLD.state IN ('completed', 'aborted') AND NEW.state IS DISTINCT FROM OLD.state THEN
        RAISE EXCEPTION 'ownership journal terminal outcome is immutable';
    END IF;
    RETURN NEW;
END
$$;

CREATE TRIGGER ownership_journal_terminal_evidence_immutable
BEFORE UPDATE ON ownership_journal
FOR EACH ROW EXECUTE FUNCTION guard_ownership_journal_terminal_evidence();
