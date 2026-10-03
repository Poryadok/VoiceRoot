ALTER TABLE chat_space_lifecycle_fences
    DROP CONSTRAINT IF EXISTS chat_space_lifecycle_source_manifest_check,
    DROP COLUMN IF EXISTS source_manifest_item_count,
    DROP COLUMN IF EXISTS source_manifest_sha256,
    DROP COLUMN IF EXISTS source_manifest_id;
