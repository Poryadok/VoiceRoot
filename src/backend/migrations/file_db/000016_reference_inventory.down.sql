BEGIN;
LOCK TABLE file_reference_inventory_gate, file_reference_inventory_cohorts,
 file_reference_inventory_components, file_reference_inventory_imports,
 file_reference_inventory_receipts, file_reference_inventory_items,
 file_reference_inventory_holds, file_reference_inventory_deletes IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM file_reference_inventory_cohorts)
 OR EXISTS(SELECT 1 FROM file_reference_inventory_imports)
 OR EXISTS(SELECT 1 FROM file_reference_inventory_receipts)
 OR EXISTS(SELECT 1 FROM file_reference_inventory_items)
 OR EXISTS(SELECT 1 FROM file_reference_inventory_holds)
 OR EXISTS(SELECT 1 FROM file_reference_inventory_deletes)
 OR EXISTS(SELECT 1 FROM file_reference_inventory_gate WHERE blocked) THEN
  RAISE EXCEPTION 'retained reference inventory work prevents rollback';
 END IF;
END $$;
DROP TABLE file_reference_inventory_deletes,file_reference_inventory_holds, file_reference_inventory_items,
 file_reference_inventory_receipts, file_reference_inventory_imports,
 file_reference_inventory_components, file_reference_inventory_cohorts,
 file_reference_inventory_gate;
COMMIT;
