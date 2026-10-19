BEGIN;
DO $$ BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'Bot Chat scope DOWN requires READ COMMITTED isolation';
    END IF;
END $$;
-- All observation writers first hold their Bot mutation lock. Wait for that
-- actual writer before taking a fresh RC snapshot of retained provenance.
LOCK TABLE bots,bot_installation_intents,bot_chat_whitelist,bot_chat_scope_observations IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM bot_chat_scope_observations)
        OR EXISTS(SELECT 1 FROM bot_installation_intents WHERE chat_scope_observation_ids<>'[]'::jsonb)
        OR EXISTS(SELECT 1 FROM bot_chat_whitelist WHERE scope_observation_id IS NOT NULL OR origin_scope_observation_id IS NOT NULL OR source_installation_id IS NOT NULL) THEN
        RAISE EXCEPTION 'Bot Chat source authority is retained';
    END IF;
END $$;
ALTER TABLE bot_chat_whitelist DROP COLUMN scope_observation_id;
ALTER TABLE bot_chat_whitelist DROP COLUMN origin_scope_observation_id;
ALTER TABLE bot_chat_whitelist DROP COLUMN source_installation_id;
ALTER TABLE bot_installation_intents DROP COLUMN chat_scope_observation_ids;
DROP TABLE bot_chat_scope_observations;
COMMIT;
