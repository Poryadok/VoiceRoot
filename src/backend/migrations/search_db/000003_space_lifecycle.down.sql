DO $$
BEGIN
    LOCK TABLE search_space_lifecycle_fences, search_space_lifecycle_operations,
        search_space_lifecycle_receipts, search_space_purge_receipts,
        search_space_chat_manifests, search_space_chat_manifest_pages,
        search_space_chat_manifest_items, search_space_purged_chat_fences IN ACCESS EXCLUSIVE MODE;
    IF EXISTS (SELECT 1 FROM search_space_lifecycle_fences)
       OR EXISTS (SELECT 1 FROM search_space_lifecycle_operations)
       OR EXISTS (SELECT 1 FROM search_space_lifecycle_receipts)
       OR EXISTS (SELECT 1 FROM search_space_purge_receipts)
       OR EXISTS (SELECT 1 FROM search_space_chat_manifests)
       OR EXISTS (SELECT 1 FROM search_space_chat_manifest_pages)
       OR EXISTS (SELECT 1 FROM search_space_chat_manifest_items)
       OR EXISTS (SELECT 1 FROM search_space_purged_chat_fences) THEN
        RAISE EXCEPTION 'cannot remove search lifecycle schema while durable lifecycle evidence exists';
    END IF;
END $$;

DROP TRIGGER search_space_lifecycle_gate ON space_search_documents;
DROP TRIGGER search_chat_lifecycle_gate ON chat_search_documents;
DROP TRIGGER search_message_lifecycle_gate ON message_search_documents;
DROP FUNCTION search_reject_frozen_projection();
DROP TABLE search_space_purged_chat_fences;
DROP TABLE search_space_chat_manifest_items;
DROP TABLE search_space_chat_manifest_pages;
DROP TABLE search_space_chat_manifests;
DROP TABLE search_space_purge_receipts;
DROP TABLE search_space_lifecycle_receipts;
DROP TABLE search_space_lifecycle_operations;
DROP TABLE search_space_lifecycle_fences;
