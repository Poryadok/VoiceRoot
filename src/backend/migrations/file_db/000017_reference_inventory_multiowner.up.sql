BEGIN;
LOCK TABLE file_reference_inventory_imports, file_reference_inventory_items,
 file_reference_inventory_holds IN ACCESS EXCLUSIVE MODE;

ALTER TABLE file_reference_inventory_imports ADD COLUMN component text;
UPDATE file_reference_inventory_imports SET component='story.media.v1';
ALTER TABLE file_reference_inventory_imports ALTER COLUMN component SET NOT NULL;
ALTER TABLE file_reference_inventory_imports ADD CONSTRAINT file_inventory_import_component_fk
 FOREIGN KEY (migration_id,component) REFERENCES file_reference_inventory_components(migration_id,component);
CREATE UNIQUE INDEX file_inventory_import_component ON file_reference_inventory_imports(migration_id,component);

-- Migration 16 only admitted Story keys: no subresource or Space scope.
UPDATE file_reference_inventory_holds SET item_key =
 'key:' || split_part(substring(item_key FROM 5), '/', 1) || '/2/' ||
 split_part(substring(item_key FROM 5), '/', 2) || '/-/-'
 WHERE item_key LIKE 'key:%';
UPDATE file_reference_inventory_items SET item_key =
 'key:' || file_id::text || '/2/' || owner_id::text || '/-/-'
 WHERE item_key LIKE 'key:%';
COMMIT;
