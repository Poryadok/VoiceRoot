-- Do not discard admitted new work, unknown legacy work, or publication evidence.
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM reports WHERE NOT validation_publication_legacy OR validation_state = 'PENDING')
       OR EXISTS (SELECT 1 FROM report_intake_outbox) THEN
        RAISE EXCEPTION 'report intake evidence prevents rollback';
    END IF;
END $$;

DROP TABLE report_intake_outbox;
DROP INDEX reports_validation_ready_idx;
ALTER TABLE reports
    DROP COLUMN reporter_account_id,
    DROP COLUMN validated_target_account_id,
    DROP CONSTRAINT reports_validation_state_check,
    DROP COLUMN validation_state,
    DROP COLUMN validation_publication_legacy,
    DROP COLUMN validation_attempts,
    DROP COLUMN validation_next_at,
    DROP COLUMN validation_claim,
    DROP COLUMN validation_claim_until,
    DROP COLUMN validation_failure_code;
