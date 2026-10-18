BEGIN;
LOCK TABLE audit_event_receipts IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM audit_event_receipts WHERE fingerprint_minimized) THEN
  RAISE EXCEPTION 'cannot restore erased audit fingerprints' USING ERRCODE='55000';
 END IF;
END $$;
ALTER TABLE audit_event_receipts DROP CONSTRAINT audit_event_receipts_payload_check,
 ADD CONSTRAINT audit_event_receipts_check CHECK(COALESCE(
  (origin='local' AND fingerprint_version IS NULL AND fingerprint IS NULL)
  OR (origin IN ('role','chat') AND fingerprint_version=1 AND octet_length(fingerprint)=32),false));
DROP TRIGGER audit_event_receipts_immutable ON audit_event_receipts;
CREATE OR REPLACE FUNCTION guard_space_audit_receipt_v1()
RETURNS trigger LANGUAGE plpgsql AS $$ BEGIN
 RAISE EXCEPTION 'audit ID receipts are immutable until authorized terminal minimization' USING ERRCODE='55000';
END $$;
CREATE TRIGGER audit_event_receipts_immutable BEFORE UPDATE OR DELETE ON audit_event_receipts
 FOR EACH ROW EXECUTE FUNCTION guard_space_audit_receipt_v1();
ALTER TABLE audit_event_receipts DROP COLUMN fingerprint_minimized;
COMMIT;
