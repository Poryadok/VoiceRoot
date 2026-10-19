-- BE-137 c18/c19, refined c23/c24: recovery is not submission eligibility.
ALTER TABLE reports ADD COLUMN reporter_provenance TEXT NOT NULL DEFAULT 'LEGACY_UNVERIFIED'
    CHECK (reporter_provenance IN ('LEGACY_UNVERIFIED','VERIFIED_AT_SUBMISSION','OWNER_RECOVERED_LEGACY'));
-- Migration8's sole production writer is InsertSubjectReport: protected Gateway
-- identity plus fresh Auth, bound atomically at submission. An account alone is
-- deliberately insufficient. Old binaries must be fenced before this rollout.
UPDATE reports SET reporter_provenance='VERIFIED_AT_SUBMISSION'
 WHERE reporter_account_id IS NOT NULL AND reporter_subject_id IS NOT NULL
 AND reporter_subject_revision > 0;
ALTER TABLE reports ADD CONSTRAINT reports_recovered_provenance_binding CHECK (
 reporter_provenance='LEGACY_UNVERIFIED' OR
 (reporter_account_id IS NOT NULL AND reporter_subject_id IS NOT NULL AND reporter_subject_revision>0));
ALTER TABLE reports ADD COLUMN validated_target_provenance TEXT NOT NULL DEFAULT 'LEGACY_UNVERIFIED'
 CHECK (validated_target_provenance IN ('LEGACY_UNVERIFIED','VERIFIED_AT_VALIDATION','OWNER_RECOVERED_LEGACY'));
UPDATE reports SET validated_target_provenance='VERIFIED_AT_VALIDATION'
 WHERE validated_target_account_id IS NOT NULL AND validated_target_subject_id IS NOT NULL
 AND validated_target_subject_revision>0;
ALTER TABLE reports ADD CONSTRAINT reports_recovered_target_binding CHECK (
 validated_target_provenance='LEGACY_UNVERIFIED' OR
 (target_type='user' AND validated_target_account_id IS NOT NULL AND validated_target_subject_id IS NOT NULL AND validated_target_subject_revision>0));

CREATE TABLE report_privacy_journal_metadata (
 singleton BOOLEAN PRIMARY KEY DEFAULT TRUE CHECK (singleton),
 journal_generation UUID NOT NULL DEFAULT gen_random_uuid()
);
INSERT INTO report_privacy_journal_metadata(singleton) VALUES(TRUE);

-- No FK to reports: dispositions must survive raw report deletion and old backup
-- replay. Sequence is diagnostic, never a completeness or commit-order proof.
CREATE TABLE report_privacy_journal (
 report_id UUID NOT NULL,
 slot TEXT NOT NULL CHECK (slot IN ('REPORTER','USER_TARGET')),
 disposition TEXT NOT NULL CHECK (disposition IN ('BOUND','REDACTED')),
 subject_id UUID,
 evidence_kind TEXT NOT NULL CHECK (evidence_kind IN ('AUTH_SUBJECT','AUTH_TERMINAL','USER_OWNER_FACT','USER_ERASURE_FACT')),
 evidence_id UUID NOT NULL,
 evidence_version BIGINT NOT NULL CHECK (evidence_version>0),
 sequence BIGINT GENERATED ALWAYS AS IDENTITY UNIQUE,
 PRIMARY KEY(report_id,slot),
 CHECK ((disposition='BOUND' AND subject_id IS NOT NULL AND evidence_kind IN ('AUTH_SUBJECT','USER_OWNER_FACT'))
     OR (disposition='REDACTED' AND evidence_kind IN ('AUTH_TERMINAL','USER_ERASURE_FACT')))
);
