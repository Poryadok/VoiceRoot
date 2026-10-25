-- A Bot producer's immutable installation is distinct from ordinary membership
-- generation. No legacy member is adopted or rewritten by this migration.
BEGIN;
DO $$ BEGIN
 IF to_regclass('public.space_effective_member_heads') IS NULL OR NOT EXISTS(
  SELECT 1 FROM pg_proc p JOIN pg_language l ON l.oid=p.prolang
  WHERE p.oid=to_regprocedure('public.space_effective_member_head_guard()')
  AND l.lanname='plpgsql' AND p.provolatile='v' AND NOT p.prosecdef AND p.proconfig IS NULL
  AND encode(sha256(convert_to(p.prosrc,'UTF8')),'hex')='fe84ecc060f27aeff57a711df113f6477cba90cd297ba49ac38699ab9c3b7e13')
 OR EXISTS(SELECT 1 FROM (VALUES
  ('public.space_effective_member_liveness_guard()','plpgsql','v','trigger','f892f9d0c2b9b001788c07c52e5c502ad0b5aca05dbc2c83c7c451cb32d982d5'),
  ('public.space_effective_membership_terminal_cleanup()','plpgsql','v','trigger','ea2a9b0dd7826274837561c24fe142fa140a0b690e419e45ec2911aa96f4ee0b'),
  ('public.space_membership_terminal_cleanup_allowed(uuid)','plpgsql','s','boolean','34f62a88777b36e9c762cacdfd3cb869941179b2254d08e57741dc653cff3be7'),
  ('public.space_terminal_purge_ready(uuid)','sql','s','boolean','7521d5ff86a3598c19cab651792fc49042e06997b8415df3ab3ae6ba9a82a2f3')
 ) expected(signature,language,volatility,result,body_sha256) WHERE NOT EXISTS(
  SELECT 1 FROM pg_proc p JOIN pg_language l ON l.oid=p.prolang
  WHERE p.oid=to_regprocedure(expected.signature) AND l.lanname=expected.language
  AND p.provolatile::text=expected.volatility AND p.prorettype=to_regtype(expected.result)
  AND NOT p.prosecdef AND p.proconfig IS NULL
  AND encode(sha256(convert_to(p.prosrc,'UTF8')),'hex')=expected.body_sha256)) THEN
  RAISE EXCEPTION 'native Bot journal requires qualified effective ledger41' USING ERRCODE='55000';
 END IF;
END $$;

CREATE TABLE space_bot_installation_attempts (
 install_operation_id UUID PRIMARY KEY CHECK(install_operation_id<>'00000000-0000-0000-0000-000000000000'),
 installation_id UUID NOT NULL UNIQUE CHECK(installation_id<>'00000000-0000-0000-0000-000000000000'),
 space_id UUID NOT NULL CHECK(space_id<>'00000000-0000-0000-0000-000000000000'),
 bot_id UUID NOT NULL CHECK(bot_id<>'00000000-0000-0000-0000-000000000000'),
 profile_id UUID NOT NULL CHECK(profile_id<>'00000000-0000-0000-0000-000000000000'),
 generation BIGINT NOT NULL CHECK(generation>0),
 state TEXT NOT NULL CHECK(state IN ('PUBLISHED','CANCELED')),
 membership_generation BIGINT CHECK(membership_generation>0),
 add_request_sha256 BYTEA CHECK(octet_length(add_request_sha256)=32),
 native_publication_sha256 BYTEA CHECK(octet_length(native_publication_sha256)=32),
 published_at TIMESTAMPTZ CHECK(isfinite(published_at)),
 UNIQUE(space_id,bot_id,generation),
 CHECK((membership_generation IS NULL AND add_request_sha256 IS NULL AND native_publication_sha256 IS NULL AND published_at IS NULL)
  OR (membership_generation IS NOT NULL AND add_request_sha256 IS NOT NULL AND native_publication_sha256 IS NOT NULL AND published_at IS NOT NULL)),
 CHECK(state<>'PUBLISHED' OR membership_generation IS NOT NULL)
);
CREATE TABLE space_bot_installation_heads (
 space_id UUID NOT NULL CHECK(space_id<>'00000000-0000-0000-0000-000000000000'),
 bot_id UUID NOT NULL CHECK(bot_id<>'00000000-0000-0000-0000-000000000000'),
 profile_id UUID NOT NULL CHECK(profile_id<>'00000000-0000-0000-0000-000000000000'),
 installation_id UUID NOT NULL UNIQUE CHECK(installation_id<>'00000000-0000-0000-0000-000000000000'),
 install_operation_id UUID NOT NULL UNIQUE CHECK(install_operation_id<>'00000000-0000-0000-0000-000000000000'),
 generation BIGINT NOT NULL CHECK(generation>0),
 retired_through BIGINT NOT NULL CHECK(retired_through>=0 AND retired_through<=generation),
 membership_generation BIGINT CHECK(membership_generation>0),
 state TEXT NOT NULL CHECK(state IN ('INSTALLED','CANCELED')),
 PRIMARY KEY(space_id,bot_id),
 CHECK((state='CANCELED')=(retired_through=generation)),
 CHECK(state<>'INSTALLED' OR membership_generation IS NOT NULL)
);
CREATE TABLE space_bot_installation_cancellations (
 operation_id UUID PRIMARY KEY CHECK(operation_id<>'00000000-0000-0000-0000-000000000000'),
 install_operation_id UUID NOT NULL CHECK(install_operation_id<>'00000000-0000-0000-0000-000000000000'),
 space_id UUID NOT NULL CHECK(space_id<>'00000000-0000-0000-0000-000000000000'),
 request_sha256 BYTEA NOT NULL CHECK(octet_length(request_sha256)=32),
 completed_at TIMESTAMPTZ NOT NULL CHECK(isfinite(completed_at)),
 CHECK(operation_id<>install_operation_id)
);

CREATE FUNCTION space_bot_installation_liveness_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM public.space_permanent_purge_fences WHERE space_id=NEW.space_id) THEN
  RAISE EXCEPTION 'Bot installation cannot cross permanent Space retirement' USING ERRCODE='55000';
 END IF;
 PERFORM 1 FROM public.spaces WHERE id=NEW.space_id FOR KEY SHARE;
 IF NOT FOUND OR EXISTS(SELECT 1 FROM public.space_permanent_purge_fences WHERE space_id=NEW.space_id) THEN
  RAISE EXCEPTION 'Bot installation requires a live Space identity' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
CREATE FUNCTION space_bot_installation_attempt_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN
  IF public.space_membership_terminal_cleanup_allowed(OLD.space_id) THEN RETURN OLD; END IF;
 ELSIF TG_OP='UPDATE' THEN
  IF ROW(NEW.install_operation_id,NEW.installation_id,NEW.space_id,NEW.bot_id,NEW.profile_id,NEW.generation,
    NEW.membership_generation,NEW.add_request_sha256,NEW.native_publication_sha256,NEW.published_at)
   IS NOT DISTINCT FROM ROW(OLD.install_operation_id,OLD.installation_id,OLD.space_id,OLD.bot_id,OLD.profile_id,OLD.generation,
    OLD.membership_generation,OLD.add_request_sha256,OLD.native_publication_sha256,OLD.published_at)
   AND (NEW.state=OLD.state OR (OLD.state='PUBLISHED' AND NEW.state='CANCELED')) THEN RETURN NEW; END IF;
 END IF;
 RAISE EXCEPTION 'Bot installation intent and native provenance cannot be rebound or erased' USING ERRCODE='55000';
END $$;
CREATE FUNCTION space_bot_installation_head_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' THEN
  IF public.space_membership_terminal_cleanup_allowed(OLD.space_id) THEN RETURN OLD; END IF;
  RAISE EXCEPTION 'Bot installation floor requires terminal P3 minimization' USING ERRCODE='55000';
 ELSIF TG_OP='TRUNCATE' THEN
  RAISE EXCEPTION 'Bot installation floors cannot be truncated' USING ERRCODE='55000';
 ELSIF TG_OP='UPDATE' THEN
  IF NEW.space_id IS DISTINCT FROM OLD.space_id OR NEW.bot_id IS DISTINCT FROM OLD.bot_id
   OR NEW.generation<OLD.generation OR NEW.retired_through<OLD.retired_through THEN
   RAISE EXCEPTION 'Bot installation identity or floor regressed' USING ERRCODE='55000';
  END IF;
  IF NEW.generation=OLD.generation THEN
   IF ROW(NEW.profile_id,NEW.installation_id,NEW.install_operation_id,NEW.membership_generation)
    IS DISTINCT FROM ROW(OLD.profile_id,OLD.installation_id,OLD.install_operation_id,OLD.membership_generation)
    OR (OLD.state='CANCELED' AND NEW.state<>'CANCELED') THEN
    RAISE EXCEPTION 'Bot installation incarnation cannot be rebound or revived' USING ERRCODE='55000';
   END IF;
  ELSIF OLD.state<>'CANCELED' OR NEW.generation<>OLD.generation+1 OR NEW.retired_through<OLD.generation
   OR NEW.installation_id=OLD.installation_id OR NEW.install_operation_id=OLD.install_operation_id THEN
   RAISE EXCEPTION 'Bot reinstall requires the exact canceled predecessor and new identity' USING ERRCODE='55000';
  END IF;
 END IF;
 IF NOT EXISTS(SELECT 1 FROM public.space_bot_installation_attempts a
  WHERE a.install_operation_id=NEW.install_operation_id AND a.installation_id=NEW.installation_id
  AND a.space_id=NEW.space_id AND a.bot_id=NEW.bot_id AND a.profile_id=NEW.profile_id AND a.generation=NEW.generation
  AND a.membership_generation IS NOT DISTINCT FROM NEW.membership_generation
  AND a.state=CASE NEW.state WHEN 'INSTALLED' THEN 'PUBLISHED' ELSE 'CANCELED' END) THEN
  RAISE EXCEPTION 'Bot installation head requires its exact durable attempt' USING ERRCODE='55000';
 END IF;
 IF NEW.state='INSTALLED' AND NOT EXISTS(SELECT 1 FROM public.space_ordinary_member_heads h
  JOIN public.space_members m ON m.space_id=h.space_id AND m.profile_id=h.profile_id
   AND m.membership_generation=h.generation AND m.admission_operation_id=h.publication_id
  JOIN public.space_bot_installation_attempts a ON a.install_operation_id=h.publication_id
   AND a.native_publication_sha256=h.native_publication_sha256
  WHERE h.space_id=NEW.space_id AND h.profile_id=NEW.profile_id AND h.generation=NEW.membership_generation
   AND h.publication_id=NEW.install_operation_id AND h.state='ACTIVE' AND h.publication_kind=6) THEN
  RAISE EXCEPTION 'installed Bot requires its exact current native publication' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
CREATE FUNCTION space_bot_installation_cancellation_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='DELETE' AND public.space_membership_terminal_cleanup_allowed(OLD.space_id) THEN RETURN OLD; END IF;
 RAISE EXCEPTION 'Bot cancellation outcome is immutable until terminal P3 minimization' USING ERRCODE='55000';
END $$;
CREATE FUNCTION space_bot_installation_terminal_cleanup() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF NOT public.space_membership_terminal_cleanup_allowed(OLD.id) THEN
  RAISE EXCEPTION 'Bot installation minimization requires native terminal P3' USING ERRCODE='55000';
 END IF;
 DELETE FROM public.space_bot_installation_heads WHERE space_id=OLD.id;
 DELETE FROM public.space_bot_installation_cancellations WHERE space_id=OLD.id;
 DELETE FROM public.space_bot_installation_attempts WHERE space_id=OLD.id;
 RETURN OLD;
END $$;
CREATE TRIGGER zzzz_space_bot_installation_terminal_cleanup AFTER DELETE ON spaces
 FOR EACH ROW EXECUTE FUNCTION space_bot_installation_terminal_cleanup();

CREATE TRIGGER bot_installation_attempt_guard BEFORE UPDATE OR DELETE ON space_bot_installation_attempts
 FOR EACH ROW EXECUTE FUNCTION space_bot_installation_attempt_guard();
CREATE TRIGGER bot_installation_attempt_truncate BEFORE TRUNCATE ON space_bot_installation_attempts
 FOR EACH STATEMENT EXECUTE FUNCTION space_bot_installation_attempt_guard();
CREATE TRIGGER bot_installation_head_guard BEFORE INSERT OR UPDATE OR DELETE ON space_bot_installation_heads
 FOR EACH ROW EXECUTE FUNCTION space_bot_installation_head_guard();
CREATE TRIGGER bot_installation_head_truncate BEFORE TRUNCATE ON space_bot_installation_heads
 FOR EACH STATEMENT EXECUTE FUNCTION space_bot_installation_head_guard();
CREATE TRIGGER bot_installation_cancellation_guard BEFORE UPDATE OR DELETE ON space_bot_installation_cancellations
 FOR EACH ROW EXECUTE FUNCTION space_bot_installation_cancellation_guard();
CREATE TRIGGER bot_installation_cancellation_truncate BEFORE TRUNCATE ON space_bot_installation_cancellations
 FOR EACH STATEMENT EXECUTE FUNCTION space_bot_installation_cancellation_guard();
DO $$ DECLARE name TEXT; BEGIN
 FOREACH name IN ARRAY ARRAY['space_bot_installation_heads','space_bot_installation_attempts','space_bot_installation_cancellations'] LOOP
  EXECUTE format('CREATE TRIGGER %I BEFORE INSERT OR UPDATE ON %I FOR EACH ROW EXECUTE FUNCTION space_bot_installation_liveness_guard()',name||'_liveness',name);
  EXECUTE format('CREATE TRIGGER %I AFTER INSERT OR UPDATE OR DELETE ON %I FOR EACH ROW EXECUTE FUNCTION space_authority_revision_changed(''space_id'')','space_authority_revision_'||name,name);
  EXECUTE format('CREATE TRIGGER %I BEFORE TRUNCATE ON %I FOR EACH STATEMENT EXECUTE FUNCTION space_authority_revision_floor_guard()','space_authority_revision_'||name||'_truncate',name);
 END LOOP;
END $$;
COMMIT;
