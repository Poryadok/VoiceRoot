DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM role_space_deletion_fences) OR EXISTS(SELECT 1 FROM role_space_deletion_fence_receipts) THEN
  RAISE EXCEPTION 'Role lifecycle evidence prevents rollback' USING ERRCODE='55000';
 END IF;
END $$;
DROP TABLE role_space_deletion_fence_receipts;
DROP FUNCTION role_deletion_fence_receipt_immutable();
DROP TABLE role_space_deletion_fences;
