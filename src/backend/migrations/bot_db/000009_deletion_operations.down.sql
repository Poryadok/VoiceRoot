BEGIN;
DO $$ BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'Bot deletion DOWN requires READ COMMITTED isolation';
    END IF;
END $$;
LOCK TABLE bot_deletion_operations,bot_deletion_installations,bot_deletion_effects IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM bot_deletion_operations)
        OR EXISTS(SELECT 1 FROM bot_deletion_installations)
        OR EXISTS(SELECT 1 FROM bot_deletion_effects) THEN
        RAISE EXCEPTION 'retained Bot deletion decisions, fences and receipts must not be dropped';
    END IF;
END $$;
DROP TABLE bot_deletion_effects,bot_deletion_installations,bot_deletion_operations;
COMMIT;
