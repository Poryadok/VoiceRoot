DO $$ BEGIN
    IF current_setting('transaction_isolation')<>'read committed' THEN
        RAISE EXCEPTION 'composite rollback requires a fresh READ COMMITTED transaction';
    END IF;
END $$;
LOCK TABLE chat_space_composite_operations,chat_space_composite_outbox,
    chat_space_composite_drains,chat_create_requests,chat_space_lifecycle_fences IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM chat_space_composite_operations)
       OR EXISTS(SELECT 1 FROM chat_space_composite_outbox)
       OR EXISTS(SELECT 1 FROM chat_space_composite_drains)
       OR EXISTS(SELECT 1 FROM chat_create_requests WHERE execution_kind<>'legacy')
       OR EXISTS(SELECT 1 FROM chat_space_lifecycle_fences WHERE draining_composites) THEN
        RAISE EXCEPTION 'retained composite authority forbids rollback';
    END IF;
END $$;
DROP TABLE chat_space_composite_drains;
DROP TABLE chat_space_composite_outbox;
DROP TABLE chat_space_composite_operations;
ALTER TABLE chat_create_requests DROP CONSTRAINT chat_create_execution_kind,
    DROP COLUMN operation_id,DROP COLUMN execution_kind;
ALTER TABLE chat_space_lifecycle_fences DROP CONSTRAINT chat_space_lifecycle_fences_check;
ALTER TABLE chat_space_lifecycle_fences DROP CONSTRAINT chat_space_lifecycle_source_manifest_check;
ALTER TABLE chat_space_lifecycle_fences DROP COLUMN draining_composites;
ALTER TABLE chat_space_lifecycle_fences ADD CONSTRAINT chat_space_lifecycle_fences_check CHECK (
    (generation=0 AND state='LIVE' AND deletion_operation_id IS NULL AND schedule_generation IS NULL
        AND manifest_id IS NULL AND manifest_sha256 IS NULL AND manifest_item_count IS NULL)
    OR (generation>0 AND deletion_operation_id IS NOT NULL AND schedule_generation IS NOT NULL
        AND schedule_generation>0 AND manifest_id IS NOT NULL AND manifest_sha256 IS NOT NULL
        AND octet_length(manifest_sha256)=32 AND manifest_item_count IS NOT NULL AND manifest_item_count>=0)
);
ALTER TABLE chat_space_lifecycle_fences ADD CONSTRAINT chat_space_lifecycle_source_manifest_check CHECK (
    (generation=0 AND source_manifest_id IS NULL AND source_manifest_sha256 IS NULL AND source_manifest_item_count IS NULL)
    OR (generation>0 AND source_manifest_id IS NOT NULL AND source_manifest_sha256 IS NOT NULL
        AND octet_length(source_manifest_sha256)=32 AND source_manifest_item_count IS NOT NULL AND source_manifest_item_count>=0)
);
