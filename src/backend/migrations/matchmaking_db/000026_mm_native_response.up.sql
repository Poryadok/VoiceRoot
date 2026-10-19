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
-- One immutable semantic decision per authenticated actor operation. Public
-- IDs and retained provenance remain personal operational evidence. Serving
-- roles need SELECT/INSERT only; final owner erasure is a separate guarded cut.
CREATE TABLE public.matchmaking_match_response_receipts (
 actor_account_id UUID NOT NULL,
 actor_profile_id UUID NOT NULL,
 operation_id UUID NOT NULL,
 request_sha256 BYTEA NOT NULL CHECK(octet_length(request_sha256)=32),
 match_id UUID NOT NULL REFERENCES public.matches(id) ON DELETE RESTRICT,
 proposal_id UUID NOT NULL REFERENCES public.match_proposals(id) ON DELETE RESTRICT,
 search_session_id UUID NOT NULL REFERENCES public.search_sessions(id) ON DELETE RESTRICT,
 session_generation BIGINT NOT NULL CHECK(session_generation>=0),
 actor_session_epoch BIGINT NOT NULL CHECK(actor_session_epoch>0),
 accept BOOLEAN NOT NULL,
 source_cut_sha256 BYTEA NOT NULL CHECK(octet_length(source_cut_sha256)=32),
 source_manifest_sha256 BYTEA NOT NULL CHECK(octet_length(source_manifest_sha256)=32),
 source_origin_bytes BYTEA NOT NULL CHECK(octet_length(source_origin_bytes) BETWEEN 1 AND 1048576),
 source_origin_sha256 BYTEA NOT NULL CHECK(octet_length(source_origin_sha256)=32),
 outcome TEXT NOT NULL CHECK(outcome IN ('ACCEPTED','DECLINED','EXPIRED','ALREADY_RESPONDED')),
 created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY(actor_account_id,actor_profile_id,operation_id)
);
CREATE INDEX mm_response_receipts_match ON public.matchmaking_match_response_receipts(match_id,proposal_id);
COMMIT;
