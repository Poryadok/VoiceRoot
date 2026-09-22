-- A durable pre-consume phase distinguishes a live request from recovery.
ALTER TABLE ownership_journal DROP CONSTRAINT ownership_journal_state_check;
ALTER TABLE ownership_journal ADD CONSTRAINT ownership_journal_state_check CHECK (state IN (
    'reserved', 'consume_started', 'proof_confirmed', 'prepared', 'commit_decided',
    'abort_decided', 'completed', 'aborted'
));
ALTER TABLE ownership_journal ADD CONSTRAINT ownership_journal_consume_started_has_no_receipt CHECK (
    state <> 'consume_started' OR auth_receipt_id IS NULL
);
