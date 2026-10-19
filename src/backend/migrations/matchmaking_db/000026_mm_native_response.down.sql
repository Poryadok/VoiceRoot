BEGIN;
-- An inherited pinned RR snapshot cannot certify absence, even after locks.
DO $$ BEGIN
    IF current_setting('transaction_isolation')<>'read committed' THEN
        RAISE EXCEPTION 'MM native rollback requires fresh read committed checks';
    END IF;
END $$;
SELECT pg_catalog.pg_advisory_xact_lock(2060719636617962328);
LOCK TABLE public.mm_native_admission_head, public.mm_native_admission_receipts,
 public.mm_native_continuity_records, public.mm_native_party_origins,
 public.matchmaking_party_search_queue_intents,
 public.matchmaking_match_response_receipts IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM public.mm_native_admission_head)
       OR EXISTS(SELECT 1 FROM public.mm_native_admission_receipts)
       OR EXISTS(SELECT 1 FROM public.mm_native_continuity_records)
       OR EXISTS(SELECT 1 FROM public.mm_native_party_origins)
       OR EXISTS(SELECT 1 FROM public.matchmaking_match_response_receipts)
       OR EXISTS(SELECT 1 FROM public.matchmaking_party_search_queue_intents
         WHERE projected_state_sha256 IS NOT NULL OR projected_admission_stamp_sha256 IS NOT NULL) THEN
        RAISE EXCEPTION 'MM native projection and admission evidence must be retained';
    END IF;
END $$;
DROP TABLE public.matchmaking_match_response_receipts;
DROP TRIGGER mm_native_projection_invalidation ON public.matchmaking_party_search_queue_intents;
DROP FUNCTION public.mm_native_projection_invalidate();
ALTER TABLE public.matchmaking_party_search_queue_intents
 DROP CONSTRAINT mm_native_projection_state_binding,
 DROP CONSTRAINT mm_native_projection_stamp_binding,
 DROP COLUMN projected_state_sha256,
 DROP COLUMN projected_admission_stamp_sha256;
COMMIT;
