BEGIN;
LOCK TABLE audit_log, audit_event_receipts IN ACCESS EXCLUSIVE MODE;
DO $$
DECLARE local_expression text;
BEGIN
    IF EXISTS(SELECT 1 FROM audit_event_receipts) THEN
        RAISE EXCEPTION 'cannot roll back audit ID receipts while reserved IDs exist' USING ERRCODE = '55000';
    END IF;
    SELECT prosrc INTO STRICT local_expression FROM pg_proc
    WHERE oid = 'space_audit_local_registry_v1(text,text,jsonb)'::regprocedure;
    EXECUTE 'ALTER TABLE audit_log DROP CONSTRAINT audit_log_closed_registry';
    EXECUTE 'ALTER TABLE audit_log ADD CONSTRAINT audit_log_closed_registry CHECK (' || substring(local_expression FROM 8) || ')';
END
$$;
DROP TRIGGER audit_log_reserve_global_id ON audit_log;
DROP FUNCTION reserve_space_audit_id_v1();
DROP TABLE audit_event_receipts;
DROP FUNCTION guard_space_audit_receipt_v1();
ALTER TABLE audit_log DROP COLUMN ingress_origin,
    DROP COLUMN ingress_fingerprint_version, DROP COLUMN ingress_fingerprint;
DROP FUNCTION space_audit_external_registry_v1(text,text,text,uuid,uuid,jsonb);
DROP FUNCTION space_audit_uint64_v1(jsonb,boolean);
DROP FUNCTION space_audit_local_registry_v1(text,text,jsonb);
COMMIT;
