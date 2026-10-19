BEGIN;
DO $$ BEGIN
    IF current_setting('transaction_isolation')<>'read committed' THEN
        RAISE EXCEPTION 'MM native migration requires fresh read committed checks';
    END IF;
END $$;
SELECT pg_catalog.pg_advisory_xact_lock(2060719636617962328);
LOCK TABLE public.mm_native_admission_head IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
    IF EXISTS(SELECT 1 FROM public.mm_native_admission_head WHERE state='ADMITTED') THEN
        RAISE EXCEPTION 'close MM native admission before schema transition';
    END IF;
END $$;
-- Existing rows remain unqualified. Never infer a native dispatch witness from
-- projected_at alone or backfill one from current ownership/history.
ALTER TABLE public.matchmaking_party_search_queue_intents
    ADD COLUMN projected_state_sha256 BYTEA,
    ADD COLUMN projected_admission_stamp_sha256 BYTEA,
    ADD CONSTRAINT mm_native_projection_state_binding CHECK (
      projected_state_sha256 IS NULL OR
      (octet_length(projected_state_sha256)=32 AND projected_at IS NOT NULL)),
    ADD CONSTRAINT mm_native_projection_stamp_binding CHECK (
      projected_admission_stamp_sha256 IS NULL OR
      (octet_length(projected_admission_stamp_sha256)=32 AND projected_state_sha256 IS NOT NULL));
CREATE FUNCTION public.mm_native_projection_invalidate() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
    IF NEW.projected_at IS NULL OR NEW.revision IS DISTINCT FROM OLD.revision
       OR NEW.action IS DISTINCT FROM OLD.action THEN
        NEW.projected_at=NULL;
        NEW.projected_state_sha256=NULL;
        NEW.projected_admission_stamp_sha256=NULL;
    END IF;
    RETURN NEW;
END $$;
CREATE TRIGGER mm_native_projection_invalidation
 BEFORE UPDATE OF revision,action,projected_at ON public.matchmaking_party_search_queue_intents
 FOR EACH ROW EXECUTE FUNCTION public.mm_native_projection_invalidate();
COMMIT;
