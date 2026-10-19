-- Personal replay fingerprints have a bounded terminal evidence window.
BEGIN;
LOCK TABLE audit_event_receipts IN ACCESS EXCLUSIVE MODE;
ALTER TABLE audit_event_receipts ADD COLUMN fingerprint_minimized boolean NOT NULL DEFAULT false;
ALTER TABLE audit_event_receipts DROP CONSTRAINT audit_event_receipts_check,
 ADD CONSTRAINT audit_event_receipts_payload_check CHECK(COALESCE(
  (origin='local' AND fingerprint_version IS NULL AND fingerprint IS NULL AND NOT fingerprint_minimized)
  OR (origin IN ('role','chat') AND NOT fingerprint_minimized AND fingerprint_version=1 AND octet_length(fingerprint)=32)
  OR (origin IN ('role','chat') AND fingerprint_minimized AND fingerprint_version IS NULL AND fingerprint IS NULL),false));

CREATE OR REPLACE FUNCTION guard_space_audit_receipt_v1()
RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='INSERT' AND NOT NEW.fingerprint_minimized THEN RETURN NEW; END IF;
 IF TG_OP='UPDATE' THEN
  IF NOT OLD.fingerprint_minimized AND NEW.fingerprint_minimized
   AND OLD.origin IN ('role','chat')
   AND NEW.audit_event_id=OLD.audit_event_id AND NEW.space_id=OLD.space_id AND NEW.origin=OLD.origin
   AND NEW.fingerprint_version IS NULL AND NEW.fingerprint IS NULL
   AND EXISTS(SELECT 1 FROM space_permanent_purge_fences p WHERE p.space_id=OLD.space_id
    AND clock_timestamp() >= p.purged_at+interval '30 days') THEN
   RETURN NEW;
  END IF;
 END IF;
 RAISE EXCEPTION 'audit receipts permit only due terminal fingerprint minimization' USING ERRCODE='55000';
END $$;
DROP TRIGGER audit_event_receipts_immutable ON audit_event_receipts;
CREATE TRIGGER audit_event_receipts_immutable BEFORE INSERT OR UPDATE OR DELETE ON audit_event_receipts
 FOR EACH ROW EXECUTE FUNCTION guard_space_audit_receipt_v1();
COMMIT;
