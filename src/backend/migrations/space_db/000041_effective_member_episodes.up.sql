-- Forward41 follows native39 and admission40; prototype38 was never integrated.
-- BE-066 c23 compact union episode. Installation is NOT all-writer cutover:
-- protected readers remain unavailable until native writer guards/cutover and
-- exact catalog qualification are assembled. Never reconstruct old join time.
BEGIN;
-- Exact native39/current34 owner contracts, not a same-named boolean callback.
DO $$ BEGIN
 IF NOT EXISTS(SELECT 1 FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_language l ON l.oid=p.prolang
  WHERE p.oid=to_regprocedure('public.space_membership_terminal_cleanup_allowed(uuid)')
  AND p.prorettype='boolean'::regtype AND p.pronargs=1 AND NOT p.prosecdef AND p.proconfig IS NULL
  AND p.provolatile='s' AND l.lanname='plpgsql'
  AND encode(sha256(convert_to(p.prosrc,'UTF8')),'hex')='34f62a88777b36e9c762cacdfd3cb869941179b2254d08e57741dc653cff3be7')
 OR NOT EXISTS(SELECT 1 FROM pg_catalog.pg_proc p JOIN pg_catalog.pg_language l ON l.oid=p.prolang
  WHERE p.oid=to_regprocedure('public.space_terminal_purge_ready(uuid)')
  AND p.prorettype='boolean'::regtype AND p.pronargs=1 AND NOT p.prosecdef AND p.proconfig IS NULL
  AND p.provolatile='s' AND l.lanname='sql'
  AND encode(sha256(convert_to(p.prosrc,'UTF8')),'hex')='7521d5ff86a3598c19cab651792fc49042e06997b8415df3ab3ae6ba9a82a2f3') THEN
  RAISE EXCEPTION 'effective ledger requires exact native39 and canonical eleven-participant P3' USING ERRCODE='55000';
 END IF;
END $$;
CREATE TABLE space_effective_member_heads (
 space_id UUID NOT NULL CHECK(space_id<>'00000000-0000-0000-0000-000000000000'),
 profile_id UUID NOT NULL CHECK(profile_id<>'00000000-0000-0000-0000-000000000000'),
 generation BIGINT NOT NULL CHECK(generation>=0),
 retired_through BIGINT NOT NULL CHECK(retired_through>=0 AND retired_through<=generation),
 ledger_revision BIGINT NOT NULL CHECK(ledger_revision>0),
 episode_id UUID CHECK(episode_id<>'00000000-0000-0000-0000-000000000000'),
 state TEXT NOT NULL CHECK(state IN ('NONE','ACTIVE','RETIRED')),
 boundary_state TEXT NOT NULL CHECK(boundary_state IN ('KNOWN','UNKNOWN')),
 history_visible_since TIMESTAMPTZ,
 covered_until TIMESTAMPTZ,
 observed_at TIMESTAMPTZ NOT NULL CHECK(isfinite(observed_at)),
 cutover_id UUID NOT NULL CHECK(cutover_id<>'00000000-0000-0000-0000-000000000000'),
 ordinary_witness TEXT CHECK(length(ordinary_witness) BETWEEN 1 AND 256),
 ordinary_legacy BOOLEAN NOT NULL DEFAULT false,
 community_witness TEXT CHECK(length(community_witness) BETWEEN 1 AND 256),
 community_until TIMESTAMPTZ CHECK(isfinite(community_until)),
 PRIMARY KEY(space_id,profile_id),
 CHECK((boundary_state='KNOWN')=(history_visible_since IS NOT NULL)),
 CHECK(history_visible_since IS NULL OR (isfinite(history_visible_since) AND history_visible_since<=observed_at)),
 CHECK(NOT ordinary_legacy OR ordinary_witness IS NOT NULL),
 CHECK((community_witness IS NULL)=(community_until IS NULL)),
 CHECK((state='ACTIVE')=(ordinary_witness IS NOT NULL OR community_witness IS NOT NULL)),
 CHECK((state='ACTIVE')=(covered_until IS NOT NULL)),
 CHECK(state<>'ACTIVE' OR (generation>retired_through AND episode_id IS NOT NULL
  AND ((ordinary_witness IS NOT NULL AND covered_until='infinity'::timestamptz)
    OR (ordinary_witness IS NULL AND covered_until=community_until AND covered_until>observed_at)))),
 CHECK(state<>'NONE' OR (generation=0 AND retired_through=0 AND episode_id IS NULL AND boundary_state='UNKNOWN')),
 CHECK(state<>'RETIRED' OR (generation>0 AND retired_through=generation AND episode_id IS NOT NULL))
);

CREATE FUNCTION space_effective_member_head_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN
  IF public.space_membership_terminal_cleanup_allowed(OLD.space_id) THEN RETURN OLD; END IF;
  RAISE EXCEPTION 'effective membership floor requires terminal P3 minimization' USING ERRCODE='55000';
 ELSIF TG_OP='TRUNCATE' THEN
  RAISE EXCEPTION 'effective membership floors cannot be truncated' USING ERRCODE='55000';
 END IF;
 IF ROW(NEW.space_id,NEW.profile_id,NEW.cutover_id) IS DISTINCT FROM ROW(OLD.space_id,OLD.profile_id,OLD.cutover_id)
 OR NEW.generation<OLD.generation OR NEW.retired_through<OLD.retired_through
 OR NEW.ledger_revision<>OLD.ledger_revision+1 OR NEW.observed_at<OLD.observed_at THEN
  RAISE EXCEPTION 'effective membership identity, floor or revision regressed' USING ERRCODE='55000';
 END IF;
 IF NEW.generation=OLD.generation THEN
  IF ROW(NEW.episode_id,NEW.boundary_state,NEW.history_visible_since)
   IS DISTINCT FROM ROW(OLD.episode_id,OLD.boundary_state,OLD.history_visible_since)
   OR (OLD.state<>'ACTIVE' AND NEW.state='ACTIVE') THEN
   RAISE EXCEPTION 'effective episode cannot be rebound or revived' USING ERRCODE='55000';
  END IF;
 ELSIF NEW.generation<>OLD.generation+1 OR NEW.retired_through<OLD.generation
 OR NEW.episode_id IS NOT DISTINCT FROM OLD.episode_id OR NEW.state<>'ACTIVE' THEN
  RAISE EXCEPTION 'new effective episode requires a fresh successor' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER effective_member_head_guard BEFORE UPDATE OR DELETE ON space_effective_member_heads
 FOR EACH ROW EXECUTE FUNCTION space_effective_member_head_guard();
CREATE TRIGGER effective_member_head_truncate BEFORE TRUNCATE ON space_effective_member_heads
 FOR EACH STATEMENT EXECUTE FUNCTION space_effective_member_head_guard();

CREATE FUNCTION space_effective_member_liveness_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 -- Freeze is reversible and is not a membership-history cut. Permanent P3 is
 -- irreversible. Lock the live Space row so a late writer cannot cross deletion.
 IF EXISTS(SELECT 1 FROM public.space_permanent_purge_fences WHERE space_id=NEW.space_id) THEN
  RAISE EXCEPTION 'effective membership cannot cross permanent Space retirement' USING ERRCODE='55000';
 END IF;
 PERFORM 1 FROM public.spaces WHERE id=NEW.space_id FOR KEY SHARE;
 IF NOT FOUND OR EXISTS(SELECT 1 FROM public.space_permanent_purge_fences WHERE space_id=NEW.space_id) THEN
  RAISE EXCEPTION 'effective membership requires a nonretired live Space' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER effective_member_liveness_guard BEFORE INSERT OR UPDATE ON space_effective_member_heads
 FOR EACH ROW EXECUTE FUNCTION space_effective_member_liveness_guard();

CREATE FUNCTION space_effective_membership_terminal_cleanup() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NOT public.space_membership_terminal_cleanup_allowed(OLD.id) THEN
  RAISE EXCEPTION 'effective membership minimization requires native terminal P3' USING ERRCODE='55000';
 END IF;
 DELETE FROM public.space_effective_member_heads WHERE space_id=OLD.id;
 RETURN OLD;
END $$;
CREATE TRIGGER zz_space_effective_membership_terminal_cleanup AFTER DELETE ON spaces
 FOR EACH ROW EXECUTE FUNCTION space_effective_membership_terminal_cleanup();
COMMIT;
