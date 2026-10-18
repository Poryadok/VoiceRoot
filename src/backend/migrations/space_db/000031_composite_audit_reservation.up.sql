BEGIN;

-- Serialize both reservation tables without inventing a receipt/fingerprint
-- before the original external fact exists. UPDATE minimization takes no lock.
LOCK TABLE audit_log, audit_event_receipts, space_chat_creation_fences IN ACCESS EXCLUSIVE MODE;
CREATE FUNCTION lock_space_audit_id_v1(event_id uuid) RETURNS void
LANGUAGE sql AS $$
  SELECT pg_advisory_xact_lock(hashtext('voice.space.audit-id.v1'),hashtext(event_id::text));
$$;

DO $$ BEGIN
  -- Producer/Space/state equality cannot prove the full original Chat fact.
  -- Creator is in protobuf evidence and receipt hashes may outlive audit rows.
  -- Refuse ambiguous pre-upgrade overlaps without erasing either record. Even
  -- apparently matching Chat rows require explicit verified reconciliation.
  IF EXISTS (
    SELECT 1 FROM space_chat_creation_fences f JOIN audit_event_receipts r USING(audit_event_id)
  ) THEN
    RAISE EXCEPTION 'ambiguous existing composite/audit ID overlap requires explicit verified reconciliation';
  END IF;
END $$;

CREATE OR REPLACE FUNCTION reserve_space_audit_id_v1() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE reserved space_chat_creation_fences%ROWTYPE;
BEGIN
  PERFORM lock_space_audit_id_v1(NEW.id);
  SELECT * INTO reserved FROM space_chat_creation_fences WHERE audit_event_id=NEW.id;
  IF FOUND AND NOT (
    COALESCE(NEW.ingress_origin,'local')='chat' AND reserved.terminal_state=1
    AND NEW.space_id=reserved.space_id AND NEW.action='chat_created'
    AND NEW.target_type='chat' AND NEW.target_id=reserved.chat_id AND NEW.details='{}'::jsonb
  ) THEN
    RAISE EXCEPTION 'audit ID reserved by composite operation' USING ERRCODE='23505';
  END IF;
  INSERT INTO audit_event_receipts(audit_event_id,space_id,origin,fingerprint_version,fingerprint)
  VALUES(NEW.id,NEW.space_id,COALESCE(NEW.ingress_origin,'local'),NEW.ingress_fingerprint_version,NEW.ingress_fingerprint);
  RETURN NEW;
END;
$$;
COMMIT;
