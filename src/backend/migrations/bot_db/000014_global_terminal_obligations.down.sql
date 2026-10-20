BEGIN;
DO $$ BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'Bot global terminal DOWN requires READ COMMITTED isolation';
    END IF;
END $$;
LOCK TABLE bot_deletion_operations IN ACCESS EXCLUSIVE MODE;
LOCK TABLE bot_deletion_terminal_effects IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM bot_deletion_terminal_effects) THEN
        RAISE EXCEPTION 'retained Bot global terminal obligations must not be dropped';
    END IF;
END $$;
DROP TABLE bot_deletion_terminal_effects;
COMMIT;
