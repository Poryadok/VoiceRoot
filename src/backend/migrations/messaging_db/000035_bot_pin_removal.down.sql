DO $$
BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'Bot pin-removal rollback requires READ COMMITTED';
    END IF;
END $$;

LOCK TABLE bot_pin_removal_decisions,bot_pin_actor_bindings,
    bot_pin_removal_revisions,bot_pin_removal_receipts IN ACCESS EXCLUSIVE MODE;

DO $$
BEGIN
    IF EXISTS(SELECT 1 FROM bot_pin_removal_decisions)
       OR EXISTS(SELECT 1 FROM bot_pin_actor_bindings)
       OR EXISTS(SELECT 1 FROM bot_pin_removal_revisions)
       OR EXISTS(SELECT 1 FROM bot_pin_removal_receipts) THEN
        RAISE EXCEPTION 'Bot pin-removal authority is retained; rollback refused';
    END IF;
END $$;

DROP TABLE bot_pin_removal_receipts;
DROP TABLE bot_pin_removal_revisions;
DROP TABLE bot_pin_actor_bindings;
DROP TABLE bot_pin_removal_decisions;
