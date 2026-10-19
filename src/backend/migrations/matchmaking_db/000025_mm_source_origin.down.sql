BEGIN;
-- Gate first, then lock every retained relation before the fresh emptiness
-- check. A blocked non-cooperating first INSERT must not be lost by rollback.
SELECT pg_catalog.pg_advisory_xact_lock(2060719636617962328);
LOCK TABLE public.mm_native_admission_head, public.mm_native_admission_receipts,
    public.mm_native_continuity_records, public.mm_native_party_origins IN ACCESS EXCLUSIVE MODE;
DO $$
BEGIN
    IF current_setting('transaction_isolation')<>'read committed' THEN
        RAISE EXCEPTION 'MM native rollback requires fresh read committed checks';
    END IF;
    IF EXISTS(SELECT 1 FROM public.mm_native_admission_head)
       OR EXISTS(SELECT 1 FROM public.mm_native_admission_receipts)
       OR EXISTS(SELECT 1 FROM public.mm_native_continuity_records)
       OR EXISTS(SELECT 1 FROM public.mm_native_party_origins) THEN
        RAISE EXCEPTION 'MM native admission history must be retained';
    END IF;
END $$;
DROP TABLE public.mm_native_party_origins, public.mm_native_continuity_records, public.mm_native_admission_receipts;
DROP FUNCTION public.mm_native_history_immutable();
ALTER TABLE public.mm_native_admission_head
    DROP CONSTRAINT mm_native_history_pair,
    DROP COLUMN database_lineage_id,
    DROP COLUMN continuity_history_root_sha256;
COMMIT;
