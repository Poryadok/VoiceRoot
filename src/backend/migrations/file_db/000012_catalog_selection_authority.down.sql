DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM file_catalog_selection_heads) THEN
        RAISE EXCEPTION 'catalog selection authority must not be forgotten';
    END IF;
END $$;
DROP TRIGGER file_catalog_selection_file_terminal ON files;
DROP FUNCTION file_catalog_close_selection();
DROP TRIGGER file_catalog_selection_asset_retirement ON file_catalog_assets;
DROP FUNCTION file_catalog_retire_selection();
ALTER TABLE file_catalog_import_operations DROP CONSTRAINT catalog_operation_selection_proof,
    DROP COLUMN authority_protocol,DROP COLUMN selection_head_id,DROP COLUMN selection_revision;
ALTER TABLE file_catalog_assets DROP CONSTRAINT catalog_asset_selection_proof,
    DROP COLUMN authority_protocol,DROP COLUMN selection_head_id,DROP COLUMN selection_revision;
DROP TABLE file_catalog_selection_heads;
