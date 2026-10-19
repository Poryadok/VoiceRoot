BEGIN;
DO $$ BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'Bot retired cleanup DOWN requires READ COMMITTED isolation';
    END IF;
END $$;
LOCK TABLE bots IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM bots WHERE retired_cleanup_unresolved) THEN
        RAISE EXCEPTION 'unresolved Bot retired cleanup must not be dropped';
    END IF;
END $$;
ALTER TABLE bots DROP COLUMN retired_cleanup_unresolved;
COMMIT;
