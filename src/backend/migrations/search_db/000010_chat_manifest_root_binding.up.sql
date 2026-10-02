-- Chat's source manifest is distinct from the aggregate Space root. Preserve
-- both bindings, including a canonical zero-count source page.
ALTER TABLE search_space_chat_manifests
 ADD COLUMN root_manifest_id TEXT,
 ADD COLUMN root_manifest_sha256 BYTEA,
 ADD COLUMN root_manifest_item_count BIGINT;

-- The previous importer required source == root before storing any header.
-- Record that existing binding without rewriting source or receipt evidence.
UPDATE search_space_chat_manifests SET
 root_manifest_id=manifest_id,
 root_manifest_sha256=manifest_sha256,
 root_manifest_item_count=item_count;

ALTER TABLE search_space_chat_manifests
 ALTER COLUMN root_manifest_id SET NOT NULL,
 ALTER COLUMN root_manifest_sha256 SET NOT NULL,
 ALTER COLUMN root_manifest_item_count SET NOT NULL,
 ADD CONSTRAINT search_chat_root_id_valid CHECK(root_manifest_id<>''),
 ADD CONSTRAINT search_chat_root_sha_valid CHECK(octet_length(root_manifest_sha256)=32),
 ADD CONSTRAINT search_chat_root_count_valid CHECK(root_manifest_item_count>=0),
 DROP CONSTRAINT search_space_chat_manifests_item_count_check,
 ADD CONSTRAINT search_space_chat_manifests_item_count_check CHECK(item_count>=0);
