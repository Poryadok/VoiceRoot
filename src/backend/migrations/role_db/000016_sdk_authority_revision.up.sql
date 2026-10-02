-- SDK grants have no Space column; their persistent owner-wide clock joins the
-- per-Space Role clock in complete source reads. It never replaces either floor.
BEGIN;
LOCK TABLE public.schema_migrations,public.game_session_grant_sessions,public.game_session_grants IN ACCESS EXCLUSIVE MODE;
DO $$ DECLARE relation REGCLASS; BEGIN
 IF (SELECT count(*) FROM public.schema_migrations)<>1 OR NOT EXISTS(
  SELECT 1 FROM public.schema_migrations WHERE (version=15 AND NOT dirty) OR (version=16 AND dirty)) THEN
  RAISE EXCEPTION 'Role SDK authority requires clean15 or migrator dirty16' USING ERRCODE='55000';
 END IF;
 FOREACH relation IN ARRAY ARRAY['public.game_session_grant_sessions'::regclass,'public.game_session_grants'::regclass] LOOP
  IF NOT EXISTS(SELECT 1 FROM pg_class WHERE oid=relation AND relkind='r' AND relpersistence='p' AND NOT relrowsecurity AND NOT relforcerowsecurity)
   OR EXISTS(SELECT 1 FROM pg_policy WHERE polrelid=relation)
   OR EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=relation)
   OR EXISTS(SELECT 1 FROM pg_inherits WHERE inhrelid=relation OR inhparent=relation) THEN
   RAISE EXCEPTION 'Role SDK source relation is not canonical' USING ERRCODE='55000';
  END IF;
 END LOOP;
END $$;
CREATE TABLE role_sdk_authority_revision (
 singleton BOOLEAN PRIMARY KEY CHECK(singleton),
 revision BIGINT NOT NULL CHECK(revision>0)
);
INSERT INTO role_sdk_authority_revision VALUES(TRUE,1);
CREATE FUNCTION role_sdk_authority_floor_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP IN ('DELETE','TRUNCATE') THEN
  RAISE EXCEPTION 'Role SDK authority floor cannot be removed' USING ERRCODE='55000';
 ELSIF TG_OP='UPDATE' THEN
  IF NEW.singleton IS DISTINCT FROM OLD.singleton OR NEW.revision::numeric<>OLD.revision::numeric+1 THEN
   RAISE EXCEPTION 'Role SDK authority must advance exactly once' USING ERRCODE='55000';
  END IF;
 ELSIF NEW.singleton IS DISTINCT FROM TRUE OR NEW.revision<>1 THEN
  RAISE EXCEPTION 'Role SDK authority must start at one' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER role_sdk_authority_floor_change BEFORE INSERT OR UPDATE OR DELETE ON role_sdk_authority_revision
 FOR EACH ROW EXECUTE FUNCTION role_sdk_authority_floor_guard();
CREATE TRIGGER role_sdk_authority_floor_truncate BEFORE TRUNCATE ON role_sdk_authority_revision
 FOR EACH STATEMENT EXECUTE FUNCTION role_sdk_authority_floor_guard();
CREATE FUNCTION role_sdk_authority_changed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 UPDATE public.role_sdk_authority_revision SET revision=revision+1 WHERE singleton;
 IF NOT FOUND THEN RAISE EXCEPTION 'Role SDK authority floor missing' USING ERRCODE='55000'; END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER role_sdk_authority_sessions AFTER INSERT OR UPDATE OR DELETE ON game_session_grant_sessions
 FOR EACH STATEMENT EXECUTE FUNCTION role_sdk_authority_changed();
CREATE TRIGGER role_sdk_authority_grants AFTER INSERT OR UPDATE OR DELETE ON game_session_grants
 FOR EACH STATEMENT EXECUTE FUNCTION role_sdk_authority_changed();
CREATE TRIGGER role_sdk_authority_sessions_truncate BEFORE TRUNCATE ON game_session_grant_sessions
 FOR EACH STATEMENT EXECUTE FUNCTION role_sdk_authority_floor_guard();
CREATE TRIGGER role_sdk_authority_grants_truncate BEFORE TRUNCATE ON game_session_grants
 FOR EACH STATEMENT EXECUTE FUNCTION role_sdk_authority_floor_guard();
COMMIT;
