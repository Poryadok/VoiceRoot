ALTER TABLE chat_space_lifecycle_fences
    ADD COLUMN source_manifest_id UUID,
    ADD COLUMN source_manifest_sha256 BYTEA,
    ADD COLUMN source_manifest_item_count BIGINT;

UPDATE chat_space_lifecycle_fences
SET source_manifest_id = manifest_id,
    source_manifest_sha256 = manifest_sha256,
    source_manifest_item_count = manifest_item_count
WHERE generation > 0;

ALTER TABLE chat_space_lifecycle_fences
    ADD CONSTRAINT chat_space_lifecycle_source_manifest_check CHECK (
      (generation = 0 AND source_manifest_id IS NULL AND source_manifest_sha256 IS NULL AND source_manifest_item_count IS NULL)
      OR
      (generation > 0 AND source_manifest_id IS NOT NULL AND source_manifest_sha256 IS NOT NULL
       AND octet_length(source_manifest_sha256) = 32 AND source_manifest_item_count IS NOT NULL AND source_manifest_item_count >= 0)
    );
