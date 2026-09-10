-- A rollback must never discard ownership decisions, including terminal ones.
-- Lock before checking: an INSERT invisible to a prior snapshot must finish
-- before the evidence check, not between that check and DROP's lock upgrade.
BEGIN;
LOCK TABLE ownership_journal IN ACCESS EXCLUSIVE MODE;

DO $$
DECLARE
    has_outbox_evidence BOOLEAN := FALSE;
BEGIN
    -- Keep one fixed lock order when a later migration adds the outbox.
    IF to_regclass('ownership_outbox') IS NOT NULL THEN
        EXECUTE 'LOCK TABLE ownership_outbox IN ACCESS EXCLUSIVE MODE';
        EXECUTE 'SELECT EXISTS (SELECT 1 FROM ownership_outbox)' INTO has_outbox_evidence;
    END IF;
    IF EXISTS (SELECT 1 FROM ownership_journal) THEN
        RAISE EXCEPTION 'cannot remove ownership journal with durable evidence';
    END IF;
    IF has_outbox_evidence THEN
        RAISE EXCEPTION 'cannot remove ownership journal with outbox evidence';
    END IF;
END
$$;

DROP TABLE ownership_journal;
COMMIT;
