-- GIS source state and this singleton are one transactional authority cut.
-- No identity sequence or maximum surviving row revision can fence deletions.
BEGIN;
LOCK TABLE public.schema_migrations, public.applications, public.environments,
 public.player_bindings, public.installations, public.game_resource_mappings,
 public.game_resource_binding_chats, public.gis_sessions,
 public.gis_app_notification_consents, public.service_credentials
 IN ACCESS EXCLUSIVE MODE;
DO $$ DECLARE relation REGCLASS; BEGIN
 IF (SELECT count(*) FROM public.schema_migrations)<>1 OR NOT EXISTS(
  SELECT 1 FROM public.schema_migrations WHERE (version=31 AND NOT dirty) OR (version=32 AND dirty)) THEN
  RAISE EXCEPTION 'GIS authority requires clean31 or migrator dirty32' USING ERRCODE='55000';
 END IF;
 FOREACH relation IN ARRAY ARRAY['public.applications'::regclass,'public.environments'::regclass,
  'public.player_bindings'::regclass,'public.installations'::regclass,'public.game_resource_mappings'::regclass,
  'public.game_resource_binding_chats'::regclass,'public.gis_sessions'::regclass,
  'public.gis_app_notification_consents'::regclass,'public.service_credentials'::regclass] LOOP
  IF NOT EXISTS(SELECT 1 FROM pg_class WHERE oid=relation AND relkind='r' AND relpersistence='p' AND NOT relrowsecurity AND NOT relforcerowsecurity)
   OR EXISTS(SELECT 1 FROM pg_policy WHERE polrelid=relation)
   OR EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=relation)
   OR EXISTS(SELECT 1 FROM pg_inherits WHERE inhrelid=relation OR inhparent=relation) THEN
   RAISE EXCEPTION 'GIS source relation is not canonical' USING ERRCODE='55000';
  END IF;
 END LOOP;
END $$;
CREATE TABLE gis_authority_revision (
 singleton BOOLEAN PRIMARY KEY CHECK(singleton),
 revision BIGINT NOT NULL CHECK(revision>0)
);
INSERT INTO gis_authority_revision VALUES(TRUE,1);
CREATE FUNCTION gis_authority_floor_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP IN ('DELETE','TRUNCATE') THEN
  RAISE EXCEPTION 'GIS authority floor cannot be removed' USING ERRCODE='55000';
 ELSIF TG_OP='UPDATE' THEN
  IF NEW.singleton IS DISTINCT FROM OLD.singleton OR NEW.revision::numeric<>OLD.revision::numeric+1 THEN
   RAISE EXCEPTION 'GIS authority must advance exactly once' USING ERRCODE='55000';
  END IF;
 ELSIF NEW.singleton IS DISTINCT FROM TRUE OR NEW.revision<>1 THEN
  RAISE EXCEPTION 'GIS authority must start at one' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER gis_authority_floor_change BEFORE INSERT OR UPDATE OR DELETE ON gis_authority_revision
 FOR EACH ROW EXECUTE FUNCTION gis_authority_floor_guard();
CREATE TRIGGER gis_authority_floor_truncate BEFORE TRUNCATE ON gis_authority_revision
 FOR EACH STATEMENT EXECUTE FUNCTION gis_authority_floor_guard();
CREATE FUNCTION gis_authority_changed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 UPDATE public.gis_authority_revision SET revision=revision+1 WHERE singleton;
 IF NOT FOUND THEN RAISE EXCEPTION 'GIS authority floor missing' USING ERRCODE='55000'; END IF;
 RETURN NULL;
END $$;
DO $$ DECLARE name TEXT; BEGIN
 FOREACH name IN ARRAY ARRAY['applications','environments','player_bindings','installations',
  'game_resource_mappings','game_resource_binding_chats','gis_sessions',
  'gis_app_notification_consents','service_credentials'] LOOP
  EXECUTE format('CREATE TRIGGER gis_authority_changed AFTER INSERT OR UPDATE OR DELETE ON public.%I FOR EACH STATEMENT EXECUTE FUNCTION public.gis_authority_changed()',name);
  EXECUTE format('CREATE TRIGGER gis_authority_no_truncate BEFORE TRUNCATE ON public.%I FOR EACH STATEMENT EXECUTE FUNCTION public.gis_authority_floor_guard()',name);
 END LOOP;
END $$;
COMMIT;
