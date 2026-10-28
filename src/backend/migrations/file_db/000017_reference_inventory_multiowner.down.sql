BEGIN;
LOCK TABLE file_reference_inventory_imports, file_reference_inventory_items,
 file_reference_inventory_holds IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS (SELECT 1 FROM file_reference_inventory_imports)
 OR EXISTS (SELECT 1 FROM file_reference_inventory_items)
 OR EXISTS (SELECT 1 FROM file_reference_inventory_holds) THEN
  RAISE EXCEPTION 'retained reference inventory proof prevents rollback';
 END IF;
END $$;
DROP INDEX file_inventory_import_component;
ALTER TABLE file_reference_inventory_imports DROP CONSTRAINT file_inventory_import_component_fk;
ALTER TABLE file_reference_inventory_imports DROP COLUMN component;
COMMIT;
