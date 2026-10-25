BEGIN;
LOCK TABLE role_member_admission_receipts,role_member_admission_preparations,role_member_source_floors,role_member_contributions,role_member_owner_authority IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM role_member_admission_receipts)
  OR EXISTS(SELECT 1 FROM role_member_admission_preparations)
  OR EXISTS(SELECT 1 FROM role_member_source_floors)
  OR EXISTS(SELECT 1 FROM role_member_contributions)
  OR EXISTS(SELECT 1 FROM role_member_owner_authority) THEN
  RAISE EXCEPTION 'cannot remove saved member admission evidence' USING ERRCODE='55000';
 END IF;
END $$;
DROP TABLE role_member_admission_preparations;
DROP TABLE role_member_admission_receipts;
DROP TABLE role_member_source_floors;
DROP TABLE role_member_contributions;
DROP TABLE role_member_owner_authority;
DROP FUNCTION role_member_admission_receipt_immutable();
DROP FUNCTION role_member_source_floor_guard();
DROP FUNCTION role_member_contribution_guard();
DROP FUNCTION role_member_owner_authority_guard();
COMMIT;
