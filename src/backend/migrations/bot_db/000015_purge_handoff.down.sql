DO $$ BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'Bot purge handoff DOWN requires READ COMMITTED';
    END IF;
END $$;
LOCK TABLE bots,bot_space_lifecycle_heads,bot_installation_intents,bot_uninstall_operations,bot_space_purge_handoffs,bot_space_purge_sources IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM bot_space_purge_handoffs) OR EXISTS(SELECT 1 FROM bot_space_purge_sources) THEN
        RAISE EXCEPTION 'retained Bot purge handoff cannot be dropped';
    END IF;
    IF EXISTS(SELECT 1 FROM bot_uninstall_operations o WHERE NOT EXISTS(SELECT 1 FROM bot_installation_intents i WHERE i.installation_id=o.installation_id)) THEN
        RAISE EXCEPTION 'accepted uninstall journal outlives its installation projection';
    END IF;
END $$;
ALTER TABLE bot_uninstall_operations ADD CONSTRAINT bot_uninstall_operations_installation_id_fkey FOREIGN KEY(installation_id) REFERENCES bot_installation_intents(installation_id);
DROP TRIGGER bot_space_purge_receipt_handoff_guard ON public.bot_space_lifecycle_purge_receipts;
DROP FUNCTION public.bot_require_prepared_purge_handoff();
DROP TABLE bot_space_purge_sources,bot_space_purge_handoffs;
