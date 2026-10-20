DO $$ BEGIN
    IF current_setting('transaction_isolation') <> 'read committed' THEN
        RAISE EXCEPTION 'Bot target retirement migration requires READ COMMITTED';
    END IF;
END $$;

LOCK TABLE bot_deletion_operations,bot_uninstall_operations,
    bot_deletion_installations,bot_uninstall_installations,
    bot_deletion_effects,bot_uninstall_effects IN ACCESS EXCLUSIVE MODE;

-- These are opaque accepted identity ledgers, not permanent personal hashes.
-- A pre-migration root is sealed only from its actual retained rows while all
-- writers are excluded. A later old writer defaults to unsealed and cannot be
-- reduced by the new application as a zero-target success.
ALTER TABLE bot_deletion_operations
    ADD COLUMN target_ids UUID[] NOT NULL DEFAULT '{}',
    ADD COLUMN target_membership_sealed BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE bot_uninstall_operations
    ADD COLUMN target_ids UUID[] NOT NULL DEFAULT '{}',
    ADD COLUMN target_membership_sealed BOOLEAN NOT NULL DEFAULT false;

DO $$ DECLARE family TEXT; target TEXT; effect TEXT; root TEXT; c RECORD;
BEGIN
    FOREACH family IN ARRAY ARRAY['bot_deletion','bot_uninstall'] LOOP
        root := family || '_operations'; target := family || '_installations'; effect := family || '_effects';
        EXECUTE format('ALTER TABLE %I ADD COLUMN effect_ids UUID[] NOT NULL DEFAULT ''{}'', ADD COLUMN disposition TEXT NOT NULL DEFAULT ''ACTIVE'', ADD COLUMN local_result_version INTEGER, ADD COLUMN local_result_id UUID, ADD COLUMN locally_completed_at TIMESTAMPTZ',target);
        EXECUTE format('UPDATE %I t SET effect_ids=ARRAY(SELECT e.effect_id FROM %I e WHERE e.target_id=t.target_id ORDER BY e.effect_id)',target,effect);
        EXECUTE format('UPDATE %I o SET target_ids=ARRAY(SELECT t.target_id FROM %I t WHERE t.operation_id=o.operation_id ORDER BY t.target_id),target_membership_sealed=true',root,target);
        -- Replace only these owned table checks with the discriminated form;
        -- primary keys, foreign keys and unique identity constraints stay.
        FOR c IN SELECT conname FROM pg_constraint WHERE conrelid=target::regclass AND contype='c' LOOP
            EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I',target,c.conname);
        END LOOP;
        FOR c IN SELECT conname FROM pg_constraint WHERE conrelid=effect::regclass AND contype='c' LOOP
            EXECUTE format('ALTER TABLE %I DROP CONSTRAINT %I',effect,c.conname);
        END LOOP;
        EXECUTE format('ALTER TABLE %I ALTER COLUMN bot_id DROP NOT NULL,ALTER COLUMN space_id DROP NOT NULL,ALTER COLUMN actor_profile_id DROP NOT NULL,ALTER COLUMN bot_generation DROP NOT NULL,ALTER COLUMN cancel_operation_id DROP NOT NULL,ALTER COLUMN chat_ids DROP NOT NULL,ALTER COLUMN legacy_evidence DROP NOT NULL,ALTER COLUMN scope_bytes DROP NOT NULL,ALTER COLUMN scope_sha256 DROP NOT NULL',target);
        EXECUTE format($sql$ALTER TABLE %I ADD CONSTRAINT target_retirement_shape CHECK (
            (disposition='ACTIVE' AND local_result_version IS NULL AND local_result_id IS NULL AND locally_completed_at IS NULL
             AND bot_id IS NOT NULL AND space_id IS NOT NULL AND actor_profile_id IS NOT NULL
             AND bot_generation IS NOT NULL AND bot_generation>=0 AND cancel_operation_id IS NOT NULL
             AND chat_ids IS NOT NULL AND jsonb_typeof(chat_ids)='array' AND legacy_evidence IS NOT NULL
             AND scope_bytes IS NOT NULL AND scope_sha256 IS NOT NULL AND octet_length(scope_sha256)=32
             AND (legacy_evidence OR (installation_id IS NOT NULL AND install_operation_id IS NOT NULL AND bot_generation>0)))
            OR (disposition='LOCAL_P3_COVERED' AND local_result_version=1 AND local_result_version IS NOT NULL AND local_result_id IS NOT NULL AND local_result_id<>'00000000-0000-0000-0000-000000000000'::uuid AND locally_completed_at IS NOT NULL
             AND bot_id IS NULL AND space_id IS NULL AND actor_profile_id IS NULL AND installation_id IS NULL
             AND install_operation_id IS NULL AND bot_generation IS NULL AND cancel_operation_id IS NULL
             AND chat_ids IS NULL AND legacy_evidence IS NULL AND scope_bytes IS NULL AND scope_sha256 IS NULL))$sql$,target);
        EXECUTE format('ALTER TABLE %I ALTER COLUMN kind DROP NOT NULL',effect);
        EXECUTE format($sql$ALTER TABLE %I ADD CONSTRAINT effect_retirement_shape CHECK (
            (state IN ('PENDING','COMPLETE') AND kind IS NOT NULL AND kind IN ('SPACE_CLEANUP','ROLE_REMOVE','MESSAGING_UNPIN')
             AND ((kind='MESSAGING_UNPIN')=(chat_id IS NOT NULL))
             AND ((request_bytes IS NULL)=(request_sha256 IS NULL)) AND (request_sha256 IS NULL OR octet_length(request_sha256)=32)
             AND ((receipt_bytes IS NULL)=(receipt_sha256 IS NULL)) AND (receipt_sha256 IS NULL OR octet_length(receipt_sha256)=32)
             AND (state<>'COMPLETE' OR (request_bytes IS NOT NULL AND receipt_bytes IS NOT NULL)))
            OR (state='LOCAL_P3_COVERED' AND kind IS NULL AND chat_id IS NULL AND request_bytes IS NULL
             AND request_sha256 IS NULL AND receipt_bytes IS NULL AND receipt_sha256 IS NULL))$sql$,effect);
    END LOOP;
END $$;
