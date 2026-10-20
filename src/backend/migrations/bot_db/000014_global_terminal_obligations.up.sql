BEGIN;
DO $$ BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'Bot global terminal migration requires READ COMMITTED isolation';
    END IF;
END $$;
LOCK TABLE bot_deletion_operations IN ACCESS EXCLUSIVE MODE;
CREATE TABLE bot_deletion_terminal_effects (
    operation_id UUID NOT NULL REFERENCES bot_deletion_operations(operation_id),
    participant TEXT NOT NULL CHECK (participant IN ('space','role')),
    request_bytes BYTEA,
    request_sha256 BYTEA CHECK (octet_length(request_sha256)=32),
    receipt_bytes BYTEA,
    receipt_sha256 BYTEA CHECK (octet_length(receipt_sha256)=32),
    state TEXT NOT NULL DEFAULT 'PENDING' CHECK (state IN ('PENDING','COMPLETE')),
    PRIMARY KEY (operation_id,participant),
    CHECK ((request_bytes IS NULL)=(request_sha256 IS NULL)),
    CHECK ((receipt_bytes IS NULL)=(receipt_sha256 IS NULL)),
    CHECK ((state='COMPLETE')=(receipt_bytes IS NOT NULL)),
    CHECK (receipt_bytes IS NULL OR request_bytes IS NOT NULL)
);
-- Retained roots have no proof of these independently required obligations.
-- Preserve their original owner decision; never manufacture a terminal receipt.
INSERT INTO bot_deletion_terminal_effects(operation_id,participant)
SELECT operation_id,participant FROM bot_deletion_operations
CROSS JOIN (VALUES ('space'),('role')) AS required(participant);
UPDATE bot_deletion_operations
SET state='RETRY_WAIT',completed_at=NULL,claimed_until=NULL,next_attempt_at=clock_timestamp()
WHERE state='COMPLETE';
COMMIT;
