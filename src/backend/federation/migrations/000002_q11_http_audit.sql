CREATE TABLE federation_http_audit (
 id UUID PRIMARY KEY,
 actor_class TEXT NOT NULL CHECK (actor_class IN ('operator','node')),
 actor_fingerprint_sha256 TEXT NOT NULL CHECK (actor_fingerprint_sha256 ~ '^[0-9a-f]{64}$'),
 target_node_id UUID,
 target_space_id UUID,
 action TEXT NOT NULL CHECK (action IN ('node.snapshot.read','node.approve')),
 result TEXT NOT NULL CHECK (result IN ('denied','conflict')),
 http_status INTEGER NOT NULL CHECK (http_status IN (403,409)),
 reason_code TEXT NOT NULL CHECK (reason_code IN (
   'certificate_role_mismatch',
   'node_certificate_mismatch',
   'node_scope_mismatch',
   'credential_revoked',
   'approval_conflict'
 )),
 request_id UUID NOT NULL,
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE FUNCTION reject_federation_http_audit_mutation() RETURNS trigger
LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'federation HTTP audit is append-only' USING ERRCODE = '42501';
END;
$$;

CREATE TRIGGER federation_http_audit_no_row_mutation
BEFORE UPDATE OR DELETE ON federation_http_audit
FOR EACH ROW EXECUTE FUNCTION reject_federation_http_audit_mutation();

CREATE TRIGGER federation_http_audit_no_truncate
BEFORE TRUNCATE ON federation_http_audit
FOR EACH STATEMENT EXECUTE FUNCTION reject_federation_http_audit_mutation();
