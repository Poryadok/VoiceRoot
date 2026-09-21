ALTER TABLE ownership_journal DROP CONSTRAINT ownership_journal_consume_started_has_no_receipt;
ALTER TABLE ownership_journal DROP CONSTRAINT ownership_journal_state_check;
UPDATE ownership_journal SET state='reserved' WHERE state='consume_started';
ALTER TABLE ownership_journal ADD CONSTRAINT ownership_journal_state_check CHECK (state IN (
    'reserved', 'proof_confirmed', 'prepared', 'commit_decided',
    'abort_decided', 'completed', 'aborted'
));
