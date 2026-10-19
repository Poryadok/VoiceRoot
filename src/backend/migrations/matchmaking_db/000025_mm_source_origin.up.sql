BEGIN;

-- Canonical wire receipt bytes are evidence, never a JSON-text hash. Only the
-- independent controller may INSERT history while holding the exclusive gate.
CREATE TABLE public.mm_native_admission_receipts (
    receipt_sha256 BYTEA PRIMARY KEY CHECK (octet_length(receipt_sha256)=32),
    receipt_bytes BYTEA NOT NULL CHECK (octet_length(receipt_bytes)>0 AND octet_length(receipt_bytes)<=1048576),
    database_lineage_id UUID NOT NULL,
    authority_incarnation UUID NOT NULL,
    transition_id UUID NOT NULL,
    admission_generation BIGINT NOT NULL CHECK (admission_generation>0),
    environment_id UUID NOT NULL,
    installation_id UUID NOT NULL,
    UNIQUE (authority_incarnation, transition_id, admission_generation)
);
CREATE TABLE public.mm_native_continuity_records (
    record_sha256 BYTEA PRIMARY KEY CHECK (octet_length(record_sha256)=32),
    record_bytes BYTEA NOT NULL CHECK (octet_length(record_bytes)>0 AND octet_length(record_bytes)<=1048576),
    predecessor_receipt_sha256 BYTEA NOT NULL REFERENCES public.mm_native_admission_receipts(receipt_sha256),
    successor_receipt_sha256 BYTEA NOT NULL UNIQUE REFERENCES public.mm_native_admission_receipts(receipt_sha256),
    history_root_sha256 BYTEA NOT NULL UNIQUE CHECK (octet_length(history_root_sha256)=32),
    CHECK (predecessor_receipt_sha256<>successor_receipt_sha256),
    CHECK (history_root_sha256=record_sha256)
);
ALTER TABLE public.mm_native_admission_head
    ADD COLUMN database_lineage_id UUID,
    ADD COLUMN continuity_history_root_sha256 BYTEA CHECK (octet_length(continuity_history_root_sha256)=32),
    ADD CONSTRAINT mm_native_history_pair CHECK ((database_lineage_id IS NULL)=(continuity_history_root_sha256 IS NULL));

-- No historical capture is enrolled. The application creates this private
-- origin in the SAME transaction as the new party, all sessions and intents.
CREATE TABLE public.mm_native_party_origins (
    party_id UUID PRIMARY KEY REFERENCES public.parties(id),
    origin_sha256 BYTEA NOT NULL UNIQUE CHECK (octet_length(origin_sha256)=32),
    origin_bytes BYTEA NOT NULL CHECK (octet_length(origin_bytes)>0 AND octet_length(origin_bytes)<=65536),
    original_receipt_sha256 BYTEA NOT NULL REFERENCES public.mm_native_admission_receipts(receipt_sha256),
    database_lineage_id UUID NOT NULL,
    creation_operation_id UUID NOT NULL UNIQUE,
    private_source_manifest_sha256 BYTEA NOT NULL CHECK (octet_length(private_source_manifest_sha256)=32)
);
REVOKE ALL ON public.mm_native_admission_receipts, public.mm_native_continuity_records, public.mm_native_party_origins FROM PUBLIC;

CREATE FUNCTION public.mm_native_history_immutable() RETURNS trigger
LANGUAGE plpgsql SET search_path = pg_catalog AS $$
BEGIN
    RAISE EXCEPTION 'MM native history must be retained';
END $$;
CREATE TRIGGER mm_native_receipt_immutable BEFORE UPDATE OR DELETE ON public.mm_native_admission_receipts
    FOR EACH ROW EXECUTE FUNCTION public.mm_native_history_immutable();
CREATE TRIGGER mm_native_receipt_no_truncate BEFORE TRUNCATE ON public.mm_native_admission_receipts
    FOR EACH STATEMENT EXECUTE FUNCTION public.mm_native_history_immutable();
CREATE TRIGGER mm_native_continuity_immutable BEFORE UPDATE OR DELETE ON public.mm_native_continuity_records
    FOR EACH ROW EXECUTE FUNCTION public.mm_native_history_immutable();
CREATE TRIGGER mm_native_continuity_no_truncate BEFORE TRUNCATE ON public.mm_native_continuity_records
    FOR EACH STATEMENT EXECUTE FUNCTION public.mm_native_history_immutable();
CREATE TRIGGER mm_native_origin_immutable BEFORE UPDATE OR DELETE ON public.mm_native_party_origins
    FOR EACH ROW EXECUTE FUNCTION public.mm_native_history_immutable();
CREATE TRIGGER mm_native_origin_no_truncate BEFORE TRUNCATE ON public.mm_native_party_origins
    FOR EACH STATEMENT EXECUTE FUNCTION public.mm_native_history_immutable();

COMMIT;
