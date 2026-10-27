DO $$ BEGIN
    IF EXISTS (SELECT 1 FROM file_catalog_actor_gates)
       OR EXISTS (SELECT 1 FROM file_catalog_assets)
       OR EXISTS (SELECT 1 FROM file_catalog_import_operations)
       OR EXISTS (SELECT 1 FROM file_catalog_terminal_fences)
       OR EXISTS (SELECT 1 FROM file_catalog_writes)
       OR EXISTS (SELECT 1 FROM files WHERE catalog_asset OR catalog_source_size_unknown) THEN
        RAISE EXCEPTION 'catalog lifecycle authority cannot be removed';
    END IF;
END $$;
DROP TRIGGER file_catalog_guard_file ON files;
DROP FUNCTION file_catalog_guard_file();
DROP TRIGGER file_catalog_complete_operation ON files;
DROP FUNCTION file_catalog_complete_operation();
DROP TRIGGER file_catalog_immutable_source ON file_catalog_assets;
DROP FUNCTION file_catalog_immutable_source();
DROP TRIGGER file_catalog_guard_blob_gc ON file_blobs;
DROP FUNCTION file_catalog_guard_blob_gc();
DROP TABLE file_catalog_writes;
DROP TABLE file_catalog_terminal_fences;
DROP TABLE file_catalog_import_operations;
DROP TABLE file_catalog_assets;
DROP TABLE file_catalog_actor_gates;
ALTER TABLE files DROP CONSTRAINT files_catalog_plaintext;
ALTER TABLE files DROP CONSTRAINT files_size_bytes_check;
ALTER TABLE files DROP COLUMN catalog_source_size_unknown;
ALTER TABLE files DROP COLUMN catalog_asset;
ALTER TABLE files ADD CONSTRAINT files_size_bytes_check CHECK (size_bytes > 0 AND size_bytes <= 209715200);
