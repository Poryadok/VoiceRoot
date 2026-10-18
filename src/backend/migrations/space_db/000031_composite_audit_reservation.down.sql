BEGIN;
LOCK TABLE audit_log, audit_event_receipts, space_chat_creation_fences IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
  IF EXISTS(SELECT 1 FROM space_chat_creation_fences) THEN
    RAISE EXCEPTION 'cannot remove reservation protection with retained composite fences';
  END IF;
END $$;
CREATE OR REPLACE FUNCTION reserve_space_audit_id_v1() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
  INSERT INTO audit_event_receipts(audit_event_id,space_id,origin,fingerprint_version,fingerprint)
  VALUES(NEW.id,NEW.space_id,COALESCE(NEW.ingress_origin,'local'),NEW.ingress_fingerprint_version,NEW.ingress_fingerprint);
  RETURN NEW;
END;
$$;
DROP FUNCTION lock_space_audit_id_v1(uuid);
COMMIT;
