BEGIN;
DO $$ BEGIN
    IF current_setting('transaction_isolation')<>'read committed' THEN
        RAISE EXCEPTION 'MM native rollback requires fresh read committed checks';
    END IF;
END $$;
SELECT pg_catalog.pg_advisory_xact_lock(2060719636617962328);
LOCK TABLE public.mm_native_admission_head, public.mm_native_admission_receipts,
 public.mm_native_continuity_records, public.mm_native_party_origins,
 public.matchmaking_native_timeout_outbox IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM public.mm_native_admission_head)
       OR EXISTS(SELECT 1 FROM public.mm_native_admission_receipts)
       OR EXISTS(SELECT 1 FROM public.mm_native_continuity_records)
       OR EXISTS(SELECT 1 FROM public.mm_native_party_origins)
       OR EXISTS(SELECT 1 FROM public.matchmaking_native_timeout_outbox) THEN
        RAISE EXCEPTION 'MM native timeout and admission evidence must be retained';
    END IF;
END $$;
DROP TABLE public.matchmaking_native_timeout_outbox;
DROP FUNCTION public.matchmaking_native_timeout_immutable();
COMMIT;
