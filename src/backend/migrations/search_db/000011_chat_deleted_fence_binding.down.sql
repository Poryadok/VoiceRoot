DO $$
BEGIN
    LOCK TABLE search_space_purged_chat_fences IN ACCESS EXCLUSIVE MODE;
    IF EXISTS (
        SELECT 1 FROM search_space_purged_chat_fences
        WHERE deletion_operation_id IS NOT NULL OR event_generation IS NOT NULL
           OR manifest_id IS NOT NULL OR manifest_sha256 IS NOT NULL
    ) THEN
        RAISE EXCEPTION 'cannot remove Search ChatDeleted bindings while bound terminal evidence exists';
    END IF;
    EXECUTE 'ALTER TABLE search_space_purged_chat_fences
        DROP CONSTRAINT search_purged_chat_binding_complete_or_legacy,
        DROP COLUMN deletion_operation_id,
        DROP COLUMN event_generation,
        DROP COLUMN manifest_id,
        DROP COLUMN manifest_sha256';
END $$;
