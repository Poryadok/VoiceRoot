BEGIN;

DROP TABLE IF EXISTS voice_space_lifecycle_purge_receipts;
DROP TABLE IF EXISTS voice_space_lifecycle_fence_receipts;
DROP TABLE IF EXISTS voice_space_lifecycle_fence_heads;

COMMIT;
