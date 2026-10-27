-- Protocol 1 imports have no trustworthy admission order. Keep them historical;
-- never infer revision order from timestamps, UUIDs, or response arrival.
CREATE TABLE file_catalog_selection_heads (
    id uuid PRIMARY KEY,
    key_version text NOT NULL,
    selector_hmac bytea NOT NULL CHECK (octet_length(selector_hmac)=32),
    revision bigint NOT NULL CHECK (revision>0),
    current_source_id uuid,
    current_file_id uuid,
    terminal_fence_id uuid,
    UNIQUE(key_version,selector_hmac),
    CHECK ((current_source_id IS NOT NULL AND current_file_id IS NOT NULL AND terminal_fence_id IS NULL)
        OR (current_source_id IS NULL AND current_file_id IS NULL AND terminal_fence_id IS NOT NULL))
);
ALTER TABLE file_catalog_assets
    ADD COLUMN authority_protocol integer NOT NULL DEFAULT 1 CHECK (authority_protocol IN (1,2)),
    ADD COLUMN selection_head_id uuid REFERENCES file_catalog_selection_heads(id),
    ADD COLUMN selection_revision bigint,
    ADD CONSTRAINT catalog_asset_selection_proof CHECK (
        (authority_protocol=1 AND selection_head_id IS NULL AND selection_revision IS NULL)
        OR (authority_protocol=2 AND selection_head_id IS NOT NULL AND selection_revision IS NOT NULL AND selection_revision>0));
ALTER TABLE file_catalog_import_operations
    ADD COLUMN authority_protocol integer NOT NULL DEFAULT 1 CHECK (authority_protocol IN (1,2)),
    ADD COLUMN selection_head_id uuid REFERENCES file_catalog_selection_heads(id),
    ADD COLUMN selection_revision bigint,
    ADD CONSTRAINT catalog_operation_selection_proof CHECK (
        (authority_protocol=1 AND selection_head_id IS NULL AND selection_revision IS NULL)
        OR (authority_protocol=2 AND selection_head_id IS NOT NULL AND selection_revision IS NOT NULL AND selection_revision>0));

-- Retiring a source never reveals an older READY source. The aggregate revision
-- survives detail/receipt compaction without raw actor/provider/URL fields.
CREATE FUNCTION file_catalog_retire_selection() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    UPDATE file_catalog_selection_heads SET current_source_id=NULL,current_file_id=NULL,
        terminal_fence_id=gen_random_uuid()
    WHERE current_file_id=OLD.file_id AND current_source_id=OLD.source_id
        AND id=OLD.selection_head_id AND revision=OLD.selection_revision;
    RETURN OLD;
END $$;
CREATE TRIGGER file_catalog_selection_asset_retirement BEFORE DELETE ON file_catalog_assets
FOR EACH ROW EXECUTE FUNCTION file_catalog_retire_selection();

CREATE FUNCTION file_catalog_close_selection() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.status IN ('failed','deleted','expired') THEN
        UPDATE file_catalog_selection_heads SET current_source_id=NULL,current_file_id=NULL,
            terminal_fence_id=gen_random_uuid()
        WHERE current_file_id=NEW.id;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER file_catalog_selection_file_terminal AFTER UPDATE OF status ON files
FOR EACH ROW WHEN (OLD.status IS DISTINCT FROM NEW.status) EXECUTE FUNCTION file_catalog_close_selection();
