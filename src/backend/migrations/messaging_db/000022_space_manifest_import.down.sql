DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM messaging_space_chat_manifests)
       OR EXISTS (SELECT 1 FROM messaging_space_chat_manifest_pages)
       OR EXISTS (SELECT 1 FROM messaging_space_chat_manifest_items) THEN
        RAISE EXCEPTION 'refusing to drop admitted Messaging Space manifest evidence';
    END IF;
END $$;

DROP TABLE messaging_space_chat_manifest_items;
DROP TABLE messaging_space_chat_manifest_pages;
DROP TABLE messaging_space_chat_manifests;
