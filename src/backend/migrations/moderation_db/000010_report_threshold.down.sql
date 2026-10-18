DROP TABLE IF EXISTS report_threshold_outbox;
DROP TABLE IF EXISTS report_threshold_case_reports;
ALTER TABLE sanctions DROP CONSTRAINT IF EXISTS sanctions_machine_case_fk;
DROP TABLE IF EXISTS report_threshold_cases;
DROP TABLE IF EXISTS report_threshold_account_cutoffs;
DROP TABLE IF EXISTS report_threshold_evaluations;
