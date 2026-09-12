DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM file_references)
       OR EXISTS (SELECT 1 FROM file_reference_operations)
       OR EXISTS (SELECT 1 FROM file_access_capabilities)
       OR EXISTS (SELECT 1 FROM file_space_lifecycle_fences)
       OR EXISTS (SELECT 1 FROM file_space_deletion_manifests)
       OR EXISTS (SELECT 1 FROM file_space_deletion_manifest_chunks)
       OR EXISTS (SELECT 1 FROM file_space_deletion_producer_releases)
       OR EXISTS (SELECT 1 FROM file_space_purge_receipts)
       OR EXISTS (SELECT 1 FROM file_blobs WHERE state IN ('GC_PENDING', 'GC_COMPLETE'))
    THEN
        RAISE EXCEPTION 'file reference lifecycle DOWN refused: durable evidence or GC_PENDING/GC_COMPLETE work exists';
    END IF;
END $$;

DROP TABLE file_space_purge_receipts;
DROP TABLE file_space_deletion_producer_releases;
DROP TABLE file_space_deletion_manifest_chunks;
DROP TABLE file_space_deletion_manifests;
DROP TABLE file_space_lifecycle_fences;
DROP TABLE file_access_capabilities;
DROP TABLE file_reference_operations;
DROP TABLE file_references;
DROP TRIGGER files_sync_blob_after_update ON files;
DROP FUNCTION file_sync_blob_after_update();
DROP TRIGGER files_assign_blob_before_insert ON files;
DROP FUNCTION file_assign_blob_on_insert();
DROP INDEX files_blob_id_idx;
ALTER TABLE files DROP CONSTRAINT files_blob_id_fk;
ALTER TABLE files DROP COLUMN blob_id;
ALTER TABLE files ADD CONSTRAINT files_r2_key_key UNIQUE (r2_key);
DROP TABLE file_blobs;
