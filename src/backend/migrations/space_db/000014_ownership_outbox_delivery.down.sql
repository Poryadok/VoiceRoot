BEGIN;

LOCK TABLE ownership_outbox IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS (
        SELECT 1
        FROM ownership_outbox
        WHERE attempt_count <> 0
           OR lease_token IS NOT NULL
           OR lease_expires_at IS NOT NULL
           OR last_failure_at IS NOT NULL
           OR delivered_at IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'cannot remove ownership outbox delivery evidence';
    END IF;
END
$$;

DROP INDEX ownership_outbox_delivery_ready_idx;

ALTER TABLE ownership_outbox
    DROP CONSTRAINT ownership_outbox_delivered_releases_lease,
    DROP CONSTRAINT ownership_outbox_failure_all_or_none,
    DROP CONSTRAINT ownership_outbox_lease_all_or_none,
    DROP CONSTRAINT ownership_outbox_attempt_count_nonnegative,
    DROP COLUMN delivered_at,
    DROP COLUMN last_failure_at,
    DROP COLUMN lease_expires_at,
    DROP COLUMN lease_token,
    DROP COLUMN next_attempt_at,
    DROP COLUMN attempt_count;

COMMIT;
