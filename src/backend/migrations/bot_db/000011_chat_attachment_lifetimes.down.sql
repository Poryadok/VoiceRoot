BEGIN;
DO $$ BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'Bot attachment DOWN requires READ COMMITTED isolation';
    END IF;
END $$;
LOCK TABLE bots,bot_installation_intents,bot_chat_whitelist,bot_chat_attachments IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM bot_chat_attachments)
        OR EXISTS(SELECT 1 FROM bot_installation_intents WHERE chat_attachment_ids<>'[]'::jsonb)
        OR EXISTS(SELECT 1 FROM bot_chat_whitelist WHERE attachment_id IS NOT NULL) THEN
        RAISE EXCEPTION 'Bot attachment lifetimes are retained';
    END IF;
END $$;
ALTER TABLE bot_chat_whitelist DROP COLUMN attachment_id;
ALTER TABLE bot_installation_intents DROP COLUMN chat_attachment_ids;
DROP TABLE bot_chat_attachments;
COMMIT;
