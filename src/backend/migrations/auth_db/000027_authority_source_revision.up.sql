-- Owning authorization set clock. Sequence allocation and per-row revisions
-- cannot account for a newly inserted or deleted row in a complete snapshot.
DO $$ DECLARE relation REGCLASS; BEGIN
 FOREACH relation IN ARRAY ARRAY['public.accounts'::regclass,'public.sdk_identities'::regclass,
  'public.sdk_devices'::regclass,'public.sdk_device_keys'::regclass,'public.sdk_sessions'::regclass,
  'public.sdk_linked_sessions'::regclass,'public.sdk_authorizations'::regclass,
  'public.sdk_conversion_operations'::regclass,'public.sdk_game_message_grants'::regclass,
  'public.sdk_game_binding_handoff_claims'::regclass] LOOP
  EXECUTE format('LOCK TABLE %s IN ACCESS EXCLUSIVE MODE',relation);
  IF NOT EXISTS(SELECT 1 FROM pg_class WHERE oid=relation AND relkind='r' AND relpersistence='p' AND NOT relrowsecurity AND NOT relforcerowsecurity)
   OR EXISTS(SELECT 1 FROM pg_policy WHERE polrelid=relation)
   OR EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=relation)
   OR EXISTS(SELECT 1 FROM pg_inherits WHERE inhrelid=relation OR inhparent=relation) THEN
   RAISE EXCEPTION 'Auth authority source relation is not canonical' USING ERRCODE='55000';
  END IF;
 END LOOP;
END $$;
CREATE TABLE auth_authority_revision (
 singleton BOOLEAN PRIMARY KEY CHECK(singleton),
 revision BIGINT NOT NULL CHECK(revision>0)
);
INSERT INTO auth_authority_revision VALUES(TRUE,1);
CREATE FUNCTION auth_authority_floor_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP IN ('DELETE','TRUNCATE') THEN
  RAISE EXCEPTION 'Auth authority floor cannot be removed' USING ERRCODE='55000';
 ELSIF TG_OP='UPDATE' THEN
  IF NEW.singleton IS DISTINCT FROM OLD.singleton OR NEW.revision::numeric<>OLD.revision::numeric+1 THEN
   RAISE EXCEPTION 'Auth authority must advance exactly once' USING ERRCODE='55000';
  END IF;
 ELSIF NEW.singleton IS DISTINCT FROM TRUE OR NEW.revision<>1 THEN
  RAISE EXCEPTION 'Auth authority must start at one' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER auth_authority_floor_change BEFORE INSERT OR UPDATE OR DELETE ON auth_authority_revision
 FOR EACH ROW EXECUTE FUNCTION auth_authority_floor_guard();
CREATE TRIGGER auth_authority_floor_truncate BEFORE TRUNCATE ON auth_authority_revision
 FOR EACH STATEMENT EXECUTE FUNCTION auth_authority_floor_guard();
CREATE FUNCTION auth_authority_changed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 UPDATE public.auth_authority_revision SET revision=revision+1 WHERE singleton;
 IF NOT FOUND THEN RAISE EXCEPTION 'Auth authority floor missing' USING ERRCODE='55000'; END IF;
 RETURN NULL;
END $$;
-- Bulk removal invalidates the source just like row mutations. Missing account
-- and SDK facts are closed state; the durable owner floor itself is never reset.
DO $$ DECLARE source TEXT; BEGIN
 FOREACH source IN ARRAY ARRAY['accounts','sdk_identities','sdk_devices','sdk_device_keys','sdk_sessions',
  'sdk_linked_sessions','sdk_authorizations','sdk_conversion_operations','sdk_game_message_grants','sdk_game_binding_handoff_claims'] LOOP
  EXECUTE format('CREATE TRIGGER %I AFTER INSERT OR UPDATE OR DELETE ON public.%I FOR EACH STATEMENT EXECUTE FUNCTION public.auth_authority_changed()', 'auth_source_'||source,source);
  EXECUTE format('CREATE TRIGGER %I AFTER TRUNCATE ON public.%I FOR EACH STATEMENT EXECUTE FUNCTION public.auth_authority_changed()', 'auth_source_'||source||'_truncate',source);
 END LOOP;
END $$;
