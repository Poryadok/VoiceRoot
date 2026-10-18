-- BE-046: requires BE-137 receipt provenance (000007) and BE-045 machine issuer (000006).
CREATE TABLE report_threshold_evaluations (
    report_id UUID PRIMARY KEY REFERENCES reports(id) ON DELETE CASCADE,
    next_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    claim_id UUID NULL,
    claim_until TIMESTAMPTZ NULL,
    completed_at TIMESTAMPTZ NULL,
    outcome VARCHAR(32) NULL,
    failure_code VARCHAR(32) NULL,
    CHECK ((claim_id IS NULL) = (claim_until IS NULL))
);
CREATE INDEX report_threshold_evaluations_ready_idx ON report_threshold_evaluations(next_at,report_id) WHERE completed_at IS NULL;

-- The feature can activate after intake has already validated modern receipts.
-- Legacy/unverified reports remain manual and never acquire invented provenance.
INSERT INTO report_threshold_evaluations(report_id)
SELECT id FROM reports WHERE target_type='user' AND validation_state='VALID'
AND status IN ('pending','reviewing') AND reporter_account_id IS NOT NULL
AND validated_target_account_id IS NOT NULL AND reporter_account_id<>validated_target_account_id
AND created_at>clock_timestamp()-interval '24 hours';

CREATE TABLE report_threshold_account_cutoffs (
    account_id UUID PRIMARY KEY,
    closed_at TIMESTAMPTZ NOT NULL
);

CREATE TABLE report_threshold_cases (
    id UUID PRIMARY KEY,
    account_id UUID NOT NULL,
    subject_id UUID NOT NULL,
    lifecycle_revision BIGINT NOT NULL CHECK (lifecycle_revision > 0),
    target_profile_id UUID NOT NULL,
    profile_revision BIGINT NOT NULL CHECK (profile_revision > 0),
    evaluation_id UUID NOT NULL UNIQUE,
    metric_version TEXT NOT NULL CHECK (metric_version='POTENTIAL_PUBLIC_PROFILE_ACCOUNTS_V1'),
    audience_accounts BIGINT NOT NULL CHECK (audience_accounts > 0),
    threshold_accounts BIGINT NOT NULL CHECK (threshold_accounts >= 10),
    reporter_accounts BIGINT NOT NULL CHECK (reporter_accounts >= threshold_accounts),
    snapshot_id UUID NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    observation_completed_at TIMESTAMPTZ NOT NULL,
    context_expires_at TIMESTAMPTZ NOT NULL,
    created_at TIMESTAMPTZ NOT NULL,
    closed_at TIMESTAMPTZ NULL,
    CHECK (observed_at <= observation_completed_at AND observation_completed_at <= created_at
           AND created_at < context_expires_at AND context_expires_at <= observed_at + interval '30 seconds')
);
CREATE UNIQUE INDEX report_threshold_cases_active_account_idx ON report_threshold_cases(account_id) WHERE closed_at IS NULL;
CREATE INDEX report_threshold_cases_subject_idx ON report_threshold_cases(subject_id);
ALTER TABLE sanctions ADD CONSTRAINT sanctions_machine_case_fk FOREIGN KEY(machine_case_id) REFERENCES report_threshold_cases(id);

CREATE TABLE report_threshold_case_reports (
    case_id UUID NOT NULL REFERENCES report_threshold_cases(id) ON DELETE CASCADE,
    report_id UUID NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
    PRIMARY KEY(case_id,report_id)
);

-- Original bytes and event identity survive retries; network dispatch is at least once.
CREATE TABLE report_threshold_outbox (
    id UUID PRIMARY KEY,
    case_id UUID NOT NULL REFERENCES report_threshold_cases(id) ON DELETE CASCADE,
    kind VARCHAR(64) NOT NULL,
    subject TEXT NOT NULL,
    payload BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK(attempts >= 0),
    lease_id UUID NULL,
    lease_until TIMESTAMPTZ NULL,
    delivered_at TIMESTAMPTZ NULL,
    cancelled_at TIMESTAMPTZ NULL,
    failure_code VARCHAR(32) NULL,
    UNIQUE(case_id,kind),
    CHECK ((lease_id IS NULL) = (lease_until IS NULL))
);
CREATE INDEX report_threshold_outbox_ready_idx ON report_threshold_outbox(available_at,id) WHERE delivered_at IS NULL AND cancelled_at IS NULL;
