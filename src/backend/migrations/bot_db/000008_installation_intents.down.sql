BEGIN;
-- Acquire the writer-conflicting lock before checking retained authority.
LOCK TABLE bot_installation_intents IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM bot_installation_intents) THEN
        RAISE EXCEPTION 'installation intent and generation evidence must not be dropped';
    END IF;
END $$;
DROP TABLE bot_installation_intents;
COMMIT;
