-- BE-137: durable opaque intake; expand before removing legacy writers.
ALTER TABLE reports
    ADD COLUMN reporter_account_id UUID NULL,
    ADD COLUMN validated_target_account_id UUID NULL,
    ADD COLUMN validation_state VARCHAR(16),
    ADD COLUMN validation_publication_legacy BOOLEAN NOT NULL DEFAULT false,
    ADD COLUMN validation_attempts INTEGER NOT NULL DEFAULT 0 CHECK (validation_attempts >= 0),
    ADD COLUMN validation_next_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ADD COLUMN validation_claim UUID NULL,
    ADD COLUMN validation_claim_until TIMESTAMPTZ NULL,
    ADD COLUMN validation_failure_code VARCHAR(32) NULL;

-- Closed records remain historical; unresolved history must be checked, but
-- its already-published report-created event must never be replayed.
UPDATE reports
SET validation_state = CASE WHEN status IN ('resolved', 'dismissed') THEN 'VALID' ELSE 'PENDING' END,
    validation_publication_legacy = true;

ALTER TABLE reports
    ALTER COLUMN validation_state SET DEFAULT 'PENDING',
    ALTER COLUMN validation_state SET NOT NULL,
    ADD CONSTRAINT reports_validation_state_check CHECK (validation_state IN ('PENDING', 'VALID', 'MISSING', 'DELETED'));

CREATE INDEX reports_validation_ready_idx ON reports(validation_next_at, created_at, id)
    WHERE validation_state = 'PENDING';

CREATE TABLE report_intake_outbox (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    report_id UUID NOT NULL REFERENCES reports(id) ON DELETE CASCADE,
    kind VARCHAR(64) NOT NULL,
    subject TEXT NOT NULL,
    payload BYTEA NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    available_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    lease_id UUID NULL,
    lease_until TIMESTAMPTZ NULL,
    delivered_at TIMESTAMPTZ NULL,
    failure_code VARCHAR(32) NULL,
    UNIQUE(report_id, kind)
);

CREATE INDEX report_intake_outbox_ready_idx ON report_intake_outbox(available_at, created_at, id)
    WHERE delivered_at IS NULL;
