ALTER TABLE ownership_outbox
    ADD COLUMN attempt_count INTEGER NOT NULL DEFAULT 0,
    ADD COLUMN next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    ADD COLUMN lease_token UUID,
    ADD COLUMN lease_expires_at TIMESTAMPTZ,
    ADD COLUMN last_failure_at TIMESTAMPTZ,
    ADD COLUMN delivered_at TIMESTAMPTZ,
    ADD CONSTRAINT ownership_outbox_attempt_count_nonnegative CHECK (attempt_count >= 0),
    ADD CONSTRAINT ownership_outbox_lease_all_or_none CHECK ((lease_token IS NULL) = (lease_expires_at IS NULL)),
    ADD CONSTRAINT ownership_outbox_failure_all_or_none CHECK ((attempt_count = 0) = (last_failure_at IS NULL)),
    ADD CONSTRAINT ownership_outbox_delivered_releases_lease CHECK (
        delivered_at IS NULL OR (lease_token IS NULL AND lease_expires_at IS NULL)
    );

CREATE INDEX ownership_outbox_delivery_ready_idx
    ON ownership_outbox(next_attempt_at, created_at, event_id)
    WHERE ready AND delivered_at IS NULL;
