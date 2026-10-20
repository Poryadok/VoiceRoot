DO $$ BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'Bot target retirement DOWN requires READ COMMITTED';
    END IF;
END $$;
LOCK TABLE bot_deletion_operations,bot_uninstall_operations,
    bot_deletion_installations,bot_uninstall_installations,
    bot_deletion_effects,bot_uninstall_effects IN ACCESS EXCLUSIVE MODE;
DO $$ DECLARE family TEXT; target TEXT; effect TEXT; root TEXT; retained BOOLEAN;
BEGIN
    IF EXISTS(SELECT 1 FROM bot_deletion_installations WHERE disposition<>'ACTIVE')
       OR EXISTS(SELECT 1 FROM bot_uninstall_installations WHERE disposition<>'ACTIVE')
       OR EXISTS(SELECT 1 FROM bot_deletion_effects WHERE state='LOCAL_P3_COVERED')
       OR EXISTS(SELECT 1 FROM bot_uninstall_effects WHERE state='LOCAL_P3_COVERED') THEN
        RAISE EXCEPTION 'Bot target retirement DOWN refuses retained local results';
    END IF;
    FOREACH family IN ARRAY ARRAY['bot_deletion','bot_uninstall'] LOOP
        root := family || '_operations'; target := family || '_installations'; effect := family || '_effects';
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT target_retirement_shape,ALTER COLUMN bot_id SET NOT NULL,ALTER COLUMN space_id SET NOT NULL,ALTER COLUMN actor_profile_id SET NOT NULL,ALTER COLUMN bot_generation SET NOT NULL,ALTER COLUMN cancel_operation_id SET NOT NULL,ALTER COLUMN chat_ids SET NOT NULL,ALTER COLUMN legacy_evidence SET NOT NULL,ALTER COLUMN scope_bytes SET NOT NULL,ALTER COLUMN scope_sha256 SET NOT NULL',target);
        EXECUTE format($sql$ALTER TABLE %I ADD CHECK(bot_generation>=0),ADD CHECK(jsonb_typeof(chat_ids)='array'),ADD CHECK(octet_length(scope_sha256)=32),ADD CHECK(legacy_evidence OR (installation_id IS NOT NULL AND install_operation_id IS NOT NULL AND bot_generation>0))$sql$,target);
        EXECUTE format('ALTER TABLE %I DROP CONSTRAINT effect_retirement_shape,ALTER COLUMN kind SET NOT NULL',effect);
        EXECUTE format($sql$ALTER TABLE %I ADD CHECK(kind IN ('SPACE_CLEANUP','ROLE_REMOVE','MESSAGING_UNPIN')),ADD CHECK(state IN ('PENDING','COMPLETE')),ADD CHECK((request_bytes IS NULL)=(request_sha256 IS NULL)),ADD CHECK(request_sha256 IS NULL OR octet_length(request_sha256)=32),ADD CHECK((receipt_bytes IS NULL)=(receipt_sha256 IS NULL)),ADD CHECK(receipt_sha256 IS NULL OR octet_length(receipt_sha256)=32),ADD CHECK(state<>'COMPLETE' OR (request_bytes IS NOT NULL AND receipt_bytes IS NOT NULL)),ADD CHECK((kind='MESSAGING_UNPIN')=(chat_id IS NOT NULL))$sql$,effect);
        EXECUTE format('ALTER TABLE %I DROP COLUMN effect_ids,DROP COLUMN disposition,DROP COLUMN local_result_version,DROP COLUMN local_result_id,DROP COLUMN locally_completed_at',target);
        EXECUTE format('ALTER TABLE %I DROP COLUMN target_ids,DROP COLUMN target_membership_sealed',root);
    END LOOP;
END $$;
