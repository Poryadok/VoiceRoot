-- One transactional owner clock covers every profile, permanent inactive
-- overlay and historical SDK actor fence. Identity sequences are not clocks.
BEGIN;
LOCK TABLE public.schema_migrations,public.profiles,public.user_account_lifecycle,public.sdk_author_tombstones IN ACCESS EXCLUSIVE MODE;
DO $$ DECLARE relation REGCLASS; BEGIN
 IF (SELECT count(*) FROM public.schema_migrations)<>1 OR NOT EXISTS(
  SELECT 1 FROM public.schema_migrations WHERE (version=18 AND NOT dirty) OR (version=19 AND dirty)) THEN
  RAISE EXCEPTION 'User authority requires clean18 or migrator dirty19' USING ERRCODE='55000';
 END IF;
 FOREACH relation IN ARRAY ARRAY['public.profiles'::regclass,'public.user_account_lifecycle'::regclass,'public.sdk_author_tombstones'::regclass] LOOP
  IF NOT EXISTS(SELECT 1 FROM pg_class WHERE oid=relation AND relkind='r' AND relpersistence='p' AND NOT relrowsecurity AND NOT relforcerowsecurity)
   OR EXISTS(SELECT 1 FROM pg_policy WHERE polrelid=relation)
   OR EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=relation)
   OR EXISTS(SELECT 1 FROM pg_inherits WHERE inhrelid=relation OR inhparent=relation) THEN
   RAISE EXCEPTION 'User authority source relation is not canonical' USING ERRCODE='55000';
  END IF;
 END LOOP;
END $$;
CREATE TABLE user_authority_revision (
 singleton BOOLEAN PRIMARY KEY CHECK(singleton),
 revision BIGINT NOT NULL CHECK(revision>0)
);
INSERT INTO user_authority_revision VALUES(TRUE,1);
CREATE FUNCTION user_authority_floor_guard() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP IN ('DELETE','TRUNCATE') THEN
  RAISE EXCEPTION 'User authority floor cannot be removed' USING ERRCODE='55000';
 ELSIF TG_OP='UPDATE' THEN
  IF NEW.singleton IS DISTINCT FROM OLD.singleton OR NEW.revision::numeric<>OLD.revision::numeric+1 THEN
   RAISE EXCEPTION 'User authority must advance exactly once' USING ERRCODE='55000';
  END IF;
 ELSIF NEW.singleton IS DISTINCT FROM TRUE OR NEW.revision<>1 THEN
  RAISE EXCEPTION 'User authority must start at one' USING ERRCODE='55000';
 END IF;
 RETURN NEW;
END $$;
CREATE TRIGGER user_authority_floor_change BEFORE INSERT OR UPDATE OR DELETE ON user_authority_revision
 FOR EACH ROW EXECUTE FUNCTION user_authority_floor_guard();
CREATE TRIGGER user_authority_floor_truncate BEFORE TRUNCATE ON user_authority_revision
 FOR EACH STATEMENT EXECUTE FUNCTION user_authority_floor_guard();
CREATE FUNCTION user_authority_changed() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 UPDATE public.user_authority_revision SET revision=revision+1 WHERE singleton;
 IF NOT FOUND THEN RAISE EXCEPTION 'User authority floor missing' USING ERRCODE='55000'; END IF;
 RETURN NULL;
END $$;
CREATE TRIGGER user_authority_profiles AFTER INSERT OR UPDATE OR DELETE ON profiles
 FOR EACH STATEMENT EXECUTE FUNCTION user_authority_changed();
CREATE TRIGGER user_authority_inactive AFTER INSERT OR UPDATE OR DELETE ON user_account_lifecycle
 FOR EACH STATEMENT EXECUTE FUNCTION user_authority_changed();
CREATE TRIGGER user_authority_tombstones AFTER INSERT OR UPDATE OR DELETE ON sdk_author_tombstones
 FOR EACH STATEMENT EXECUTE FUNCTION user_authority_changed();
CREATE TRIGGER user_authority_profiles_truncate BEFORE TRUNCATE ON profiles
 FOR EACH STATEMENT EXECUTE FUNCTION user_authority_floor_guard();
CREATE FUNCTION reject_user_inactive_mutation() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 RAISE EXCEPTION 'User inactive account fences are immutable' USING ERRCODE='55000';
END $$;
CREATE TRIGGER user_account_lifecycle_no_update_delete BEFORE UPDATE OR DELETE ON user_account_lifecycle
 FOR EACH ROW EXECUTE FUNCTION reject_user_inactive_mutation();
CREATE TRIGGER user_account_lifecycle_no_truncate BEFORE TRUNCATE ON user_account_lifecycle
 FOR EACH STATEMENT EXECUTE FUNCTION reject_user_inactive_mutation();
COMMIT;
