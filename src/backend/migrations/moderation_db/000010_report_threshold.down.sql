-- Preserve admitted work, review cutoffs and publication/decision evidence.
-- Migration runners execute this complete script in one transaction.
LOCK TABLE report_threshold_evaluations, report_threshold_account_cutoffs,
    report_threshold_cases, report_threshold_case_reports, report_threshold_outbox
    IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM report_threshold_evaluations)
       OR EXISTS (SELECT 1 FROM report_threshold_account_cutoffs)
       OR EXISTS (SELECT 1 FROM report_threshold_cases)
       OR EXISTS (SELECT 1 FROM report_threshold_case_reports)
       OR EXISTS (SELECT 1 FROM report_threshold_outbox) THEN
        RAISE EXCEPTION 'report threshold evidence prevents rollback';
    END IF;
END $$;

DROP TABLE IF EXISTS report_threshold_outbox;
DROP TABLE IF EXISTS report_threshold_case_reports;
ALTER TABLE sanctions DROP CONSTRAINT IF EXISTS sanctions_machine_case_fk;
DROP TABLE IF EXISTS report_threshold_cases;
DROP TABLE IF EXISTS report_threshold_account_cutoffs;
DROP TABLE IF EXISTS report_threshold_evaluations;
