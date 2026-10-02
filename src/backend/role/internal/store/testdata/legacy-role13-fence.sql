-- Space lifecycle heads serialize with every existing Role scope.
CREATE TABLE role_space_deletion_fences (
 space_id UUID PRIMARY KEY,
 deletion_operation_id UUID NOT NULL,
 generation BIGINT NOT NULL CHECK(generation>0),
 state TEXT NOT NULL CHECK(state IN ('FROZEN','LIVE','PURGE_DECIDED')),
 manifest_id UUID NOT NULL,
 manifest_sha256 BYTEA NOT NULL CHECK(octet_length(manifest_sha256)=32),
 manifest_item_count NUMERIC(20,0) NOT NULL CHECK(manifest_item_count>=0 AND manifest_item_count<=18446744073709551615)
);
CREATE TABLE role_space_deletion_fence_receipts (
 space_id UUID NOT NULL,
 deletion_operation_id UUID NOT NULL,
 generation BIGINT NOT NULL CHECK(generation>0),
 state TEXT NOT NULL CHECK(state IN ('FROZEN','LIVE','PURGE_DECIDED')),
 manifest_id UUID NOT NULL,
 manifest_sha256 BYTEA NOT NULL CHECK(octet_length(manifest_sha256)=32),
 manifest_item_count NUMERIC(20,0) NOT NULL CHECK(manifest_item_count>=0 AND manifest_item_count<=18446744073709551615),
 receipt_id UUID NOT NULL UNIQUE,
 applied_at TIMESTAMPTZ NOT NULL,
 request_sha256 BYTEA NOT NULL CHECK(octet_length(request_sha256)=32),
 request_bytes BYTEA CHECK(request_bytes IS NULL OR octet_length(request_bytes)>0),
 receipt_bytes BYTEA CHECK(receipt_bytes IS NULL OR octet_length(receipt_bytes)>0),
 CHECK((request_bytes IS NULL)=(receipt_bytes IS NULL)),
 PRIMARY KEY(space_id,generation)
);
CREATE FUNCTION role_deletion_fence_receipt_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='INSERT' AND NEW.request_bytes IS NOT NULL AND NEW.receipt_bytes IS NOT NULL THEN RETURN NEW; END IF;
 IF TG_OP='UPDATE'
 AND (to_jsonb(NEW)-'request_bytes'-'receipt_bytes')=(to_jsonb(OLD)-'request_bytes'-'receipt_bytes')
 AND OLD.request_bytes IS NOT NULL AND NEW.request_bytes IS NULL AND NEW.receipt_bytes IS NULL
 AND EXISTS(SELECT 1 FROM role_space_lifecycle WHERE space_id=OLD.space_id
            AND retired_at IS NOT NULL AND clock_timestamp()>=retired_at+interval '30 days') THEN
  RETURN NEW;
 END IF;
 RAISE EXCEPTION 'Role lifecycle receipt is immutable' USING ERRCODE='55000';
END $$;
CREATE TRIGGER role_deletion_fence_receipt_immutable BEFORE INSERT OR UPDATE OR DELETE ON role_space_deletion_fence_receipts
 FOR EACH ROW EXECUTE FUNCTION role_deletion_fence_receipt_immutable();
