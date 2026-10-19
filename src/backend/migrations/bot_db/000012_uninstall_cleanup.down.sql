BEGIN;
DO $$ BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'Bot uninstall DOWN requires READ COMMITTED isolation';
    END IF;
END $$;
LOCK TABLE bots,bot_installation_intents,bot_uninstall_operations,bot_uninstall_installations,bot_uninstall_effects IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM bot_uninstall_operations)
        OR EXISTS(SELECT 1 FROM bot_uninstall_installations)
        OR EXISTS(SELECT 1 FROM bot_uninstall_effects) THEN
        RAISE EXCEPTION 'retained Bot uninstall decisions and receipts must not be dropped';
    END IF;
END $$;
DROP TABLE bot_uninstall_effects,bot_uninstall_installations,bot_uninstall_operations;
COMMIT;
