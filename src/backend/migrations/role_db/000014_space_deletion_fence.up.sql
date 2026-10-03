-- Version 13 is reserved for the canonical game-session grant migration.
-- Adopt the exact pre-repair fence-only v13 catalog without rewriting evidence.
-- golang-migrate executes this while version 14 is dirty; direct recovery may
-- start only from a recorded clean v13. Startup must wait for clean v14.
DO $role14$
DECLARE
 grant_ddl TEXT := $grant_ddl$CREATE TABLE game_session_grant_sessions (
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    session_id UUID NOT NULL,
    voice_room_id UUID,
    roster_revision BIGINT NOT NULL DEFAULT 0 CHECK (roster_revision >= 0),
    profile_set_sha256 BYTEA NOT NULL CHECK (octet_length(profile_set_sha256) = 32),
    status TEXT NOT NULL CHECK (status IN ('active', 'revoked')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (application_id, environment_id, session_id),
    CHECK (status <> 'active' OR voice_room_id IS NOT NULL)
);

CREATE TABLE game_session_grants (
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    session_id UUID NOT NULL,
    voice_room_id UUID NOT NULL,
    profile_id UUID NOT NULL,
    permission TEXT NOT NULL DEFAULT 'VOICE_JOIN' CHECK (permission = 'VOICE_JOIN'),
    roster_revision BIGINT NOT NULL CHECK (roster_revision > 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
    PRIMARY KEY (application_id, environment_id, session_id, profile_id),
    FOREIGN KEY (application_id, environment_id, session_id)
        REFERENCES game_session_grant_sessions(application_id, environment_id, session_id)
        ON DELETE RESTRICT
);

CREATE INDEX game_session_grants_voice_lookup_idx
    ON game_session_grants(application_id, environment_id, session_id, voice_room_id, profile_id);

CREATE TABLE game_session_grant_operations (
    operation_id UUID PRIMARY KEY,
    operation_kind TEXT NOT NULL CHECK (operation_kind IN ('apply', 'revoke')),
    application_id UUID NOT NULL,
    environment_id UUID NOT NULL,
    session_id UUID NOT NULL,
    request_sha256 BYTEA NOT NULL CHECK (octet_length(request_sha256) = 32),
    receipt_id UUID NOT NULL UNIQUE,
    roster_revision BIGINT NOT NULL CHECK (roster_revision >= 0),
    applied_profile_set_sha256 BYTEA NOT NULL CHECK (octet_length(applied_profile_set_sha256) = 32),
    outcome TEXT NOT NULL CHECK (outcome IN ('APPLIED', 'REPLAYED', 'STALE', 'REVOKED')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp()
);

CREATE INDEX game_session_grant_operations_session_idx
    ON game_session_grant_operations(application_id, environment_id, session_id, created_at);
$grant_ddl$;
 fence_ddl TEXT := $fence_ddl$-- Space lifecycle heads serialize with every existing Role scope.
CREATE TABLE role_space_deletion_fences (
 space_id UUID PRIMARY KEY,
 deletion_operation_id UUID NOT NULL,
 generation BIGINT NOT NULL CHECK(generation>0),
 state TEXT NOT NULL CHECK(state IN ('FROZEN','LIVE','PURGE_DECIDED')),
 manifest_id UUID NOT NULL,
 manifest_sha256 BYTEA NOT NULL CHECK(octet_length(manifest_sha256)=32),
 manifest_item_count NUMERIC(20,0) NOT NULL CHECK(manifest_item_count>=0 AND manifest_item_count<=18446744073709551615)
);
CREATE TABLE role_space_deletion_fence_receipts (
 space_id UUID NOT NULL,
 deletion_operation_id UUID NOT NULL,
 generation BIGINT NOT NULL CHECK(generation>0),
 state TEXT NOT NULL CHECK(state IN ('FROZEN','LIVE','PURGE_DECIDED')),
 manifest_id UUID NOT NULL,
 manifest_sha256 BYTEA NOT NULL CHECK(octet_length(manifest_sha256)=32),
 manifest_item_count NUMERIC(20,0) NOT NULL CHECK(manifest_item_count>=0 AND manifest_item_count<=18446744073709551615),
 receipt_id UUID NOT NULL UNIQUE,
 applied_at TIMESTAMPTZ NOT NULL,
 request_sha256 BYTEA NOT NULL CHECK(octet_length(request_sha256)=32),
 request_bytes BYTEA CHECK(request_bytes IS NULL OR octet_length(request_bytes)>0),
 receipt_bytes BYTEA CHECK(receipt_bytes IS NULL OR octet_length(receipt_bytes)>0),
 CHECK((request_bytes IS NULL)=(receipt_bytes IS NULL)),
 PRIMARY KEY(space_id,generation)
);
$fence_ddl$;
 immutable_ddl TEXT := $immutable_ddl$CREATE FUNCTION role_deletion_fence_receipt_immutable() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP='INSERT' AND NEW.request_bytes IS NOT NULL AND NEW.receipt_bytes IS NOT NULL THEN RETURN NEW; END IF;
 IF TG_OP='UPDATE'
 AND (to_jsonb(NEW)-'request_bytes'-'receipt_bytes')=(to_jsonb(OLD)-'request_bytes'-'receipt_bytes')
 AND OLD.request_bytes IS NOT NULL AND NEW.request_bytes IS NULL AND NEW.receipt_bytes IS NULL
 AND EXISTS(SELECT 1 FROM role_space_lifecycle WHERE space_id=OLD.space_id
            AND retired_at IS NOT NULL AND clock_timestamp()>=retired_at+interval '30 days') THEN
  RETURN NEW;
 END IF;
 RAISE EXCEPTION 'Role lifecycle receipt is immutable' USING ERRCODE='55000';
END $$;
CREATE TRIGGER role_deletion_fence_receipt_immutable BEFORE INSERT OR UPDATE OR DELETE ON role_space_deletion_fence_receipts
 FOR EACH ROW EXECUTE FUNCTION role_deletion_fence_receipt_immutable();
$immutable_ddl$;
 immutable_source TEXT := $immutable_source$
BEGIN
 IF TG_OP='INSERT' AND NEW.request_bytes IS NOT NULL AND NEW.receipt_bytes IS NOT NULL THEN RETURN NEW; END IF;
 IF TG_OP='UPDATE'
 AND (to_jsonb(NEW)-'request_bytes'-'receipt_bytes')=(to_jsonb(OLD)-'request_bytes'-'receipt_bytes')
 AND OLD.request_bytes IS NOT NULL AND NEW.request_bytes IS NULL AND NEW.receipt_bytes IS NULL
 AND EXISTS(SELECT 1 FROM role_space_lifecycle WHERE space_id=OLD.space_id
            AND retired_at IS NOT NULL AND clock_timestamp()>=retired_at+interval '30 days') THEN
  RETURN NEW;
 END IF;
 RAISE EXCEPTION 'Role lifecycle receipt is immutable' USING ERRCODE='55000';
END $immutable_source$;
 names TEXT[] := ARRAY['game_session_grant_sessions','game_session_grants','game_session_grant_operations','role_space_deletion_fences','role_space_deletion_fence_receipts'];
 expected TEXT;
 name TEXT;
 grant_count INTEGER;
 fence_count INTEGER;
 marker_count INTEGER;
 marker_valid BOOLEAN;
 existing_fences BOOLEAN;
 immutable_oid OID;
BEGIN
 IF to_regclass('public.schema_migrations') IS NULL THEN
  RAISE EXCEPTION 'Role v14 requires recorded migration state' USING ERRCODE='55000';
 END IF;
 LOCK TABLE public.schema_migrations IN SHARE MODE;
 SELECT count(*),bool_and((version=13 AND NOT dirty) OR (version=14 AND dirty))
 INTO marker_count,marker_valid FROM public.schema_migrations;
 IF marker_count<>1 OR marker_valid IS DISTINCT FROM TRUE THEN
  RAISE EXCEPTION 'Role v14 migration state is invalid' USING ERRCODE='55000';
 END IF;
 PERFORM set_config('search_path','public,pg_temp',true);
 SELECT count(*) INTO grant_count FROM unnest(names[1:3]) n WHERE to_regclass('public.'||n) IS NOT NULL;
 SELECT count(*) INTO fence_count FROM unnest(names[4:5]) n WHERE to_regclass('public.'||n) IS NOT NULL;
 IF grant_count NOT IN (0,3) OR fence_count NOT IN (0,2) OR (grant_count=0 AND fence_count=0) THEN
  RAISE EXCEPTION 'Role v13 schema is partial or unrecognized' USING ERRCODE='55000';
 END IF;
 existing_fences := fence_count=2;
 FOREACH name IN ARRAY names LOOP
  IF to_regclass('public.'||name) IS NOT NULL THEN
   IF NOT EXISTS(SELECT 1 FROM pg_class WHERE oid=to_regclass('public.'||name) AND relkind='r') THEN
    RAISE EXCEPTION 'Role v13 relation kind is invalid' USING ERRCODE='55000';
   END IF;
   EXECUTE format('LOCK TABLE public.%I IN ACCESS EXCLUSIVE MODE',name);
  END IF;
 END LOOP;
 IF NOT existing_fences AND to_regprocedure('public.role_deletion_fence_receipt_immutable()') IS NOT NULL THEN
  RAISE EXCEPTION 'Role v13 fence function is orphaned' USING ERRCODE='55000';
 END IF;
 expected := replace(grant_ddl||fence_ddl,'CREATE TABLE','CREATE TEMP TABLE');
 FOREACH name IN ARRAY names LOOP
  expected := replace(expected,name,'_role14_'||name);
 END LOOP;
 EXECUTE expected;
 EXECUTE $signature$
 CREATE FUNCTION pg_temp.role14_signature(target OID) RETURNS JSONB LANGUAGE SQL AS $fn$
 SELECT jsonb_build_object(
 'columns',(SELECT coalesce(jsonb_agg(jsonb_build_array(a.attnum,a.attname,a.atttypid,a.atttypmod,a.attcollation,a.attnotnull,a.attidentity,a.attgenerated,pg_get_expr(d.adbin,d.adrelid)) ORDER BY a.attnum),'[]') FROM pg_attribute a LEFT JOIN pg_attrdef d ON d.adrelid=a.attrelid AND d.adnum=a.attnum WHERE a.attrelid=target AND a.attnum>0 AND NOT a.attisdropped),
 'constraints',(SELECT coalesce(jsonb_agg(x.value ORDER BY x.value::text),'[]') FROM (SELECT jsonb_build_array(contype,condeferrable,condeferred,convalidated,replace(pg_get_constraintdef(oid,true),'_role14_','')) value FROM pg_constraint WHERE conrelid=target) x),
 'indexes',(SELECT coalesce(jsonb_agg(x.value ORDER BY x.value::text),'[]') FROM (SELECT jsonb_build_array(i.indisunique,i.indisprimary,i.indisexclusion,i.indisvalid,i.indisready,i.indnkeyatts,i.indnatts,i.indclass::text,i.indcollation::text,i.indoption::text,pg_get_expr(i.indpred,i.indrelid),ARRAY(SELECT pg_get_indexdef(i.indexrelid,n,true) FROM generate_series(1,i.indnatts) n ORDER BY n)) value FROM pg_index i WHERE i.indrelid=target) x)
 );
 $fn$;
 $signature$;
 IF grant_count=0 THEN EXECUTE grant_ddl; END IF;
 IF fence_count=0 THEN EXECUTE fence_ddl; END IF;
 FOREACH name IN ARRAY names LOOP
  IF NOT EXISTS(SELECT 1 FROM pg_class WHERE oid=to_regclass('public.'||name) AND relkind='r' AND relpersistence='p' AND NOT relrowsecurity AND NOT relforcerowsecurity)
   OR EXISTS(SELECT 1 FROM pg_policy WHERE polrelid=to_regclass('public.'||name))
   OR EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=to_regclass('public.'||name))
   OR EXISTS(SELECT 1 FROM pg_inherits WHERE inhrelid=to_regclass('public.'||name) OR inhparent=to_regclass('public.'||name)) THEN
   RAISE EXCEPTION 'Role v13 relation behavior is not canonical' USING ERRCODE='55000';
  END IF;
  IF pg_temp.role14_signature(to_regclass('public.'||name)) IS DISTINCT FROM pg_temp.role14_signature(to_regclass('pg_temp._role14_'||name)) THEN
   RAISE EXCEPTION 'Role v13 schema does not match canonical DDL' USING ERRCODE='55000';
  END IF;
  IF name<>'role_space_deletion_fence_receipts' AND EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid=to_regclass('public.'||name) AND NOT tgisinternal) THEN
   RAISE EXCEPTION 'Role v13 schema has unexpected triggers' USING ERRCODE='55000';
  END IF;
 END LOOP;
 IF NOT existing_fences THEN EXECUTE immutable_ddl; END IF;
 immutable_oid := to_regprocedure('public.role_deletion_fence_receipt_immutable()');
 IF NOT EXISTS(SELECT 1 FROM pg_proc p JOIN pg_language l ON l.oid=p.prolang WHERE p.oid=immutable_oid AND p.pronargs=0 AND p.prorettype='trigger'::regtype AND l.lanname='plpgsql' AND p.provolatile='v' AND p.proparallel='u' AND NOT p.prosecdef AND NOT p.proisstrict AND p.proconfig IS NULL AND btrim(replace(p.prosrc,E'\r',''),E'\n\r\t ')=btrim(immutable_source,E'\n\r\t ')) THEN
  RAISE EXCEPTION 'Role v13 receipt function is not canonical' USING ERRCODE='55000';
 END IF;
 IF (SELECT count(*) FROM pg_trigger WHERE tgrelid='public.role_space_deletion_fence_receipts'::regclass AND NOT tgisinternal)<>1 OR NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgrelid='public.role_space_deletion_fence_receipts'::regclass AND tgname='role_deletion_fence_receipt_immutable' AND tgfoid=immutable_oid AND tgtype=31 AND tgenabled='O' AND tgnargs=0 AND tgqual IS NULL AND NOT tgisinternal AND NOT tgdeferrable AND NOT tginitdeferred) THEN
  RAISE EXCEPTION 'Role v13 receipt trigger is not canonical' USING ERRCODE='55000';
 END IF;
 DROP TABLE pg_temp._role14_game_session_grants,pg_temp._role14_game_session_grant_sessions,pg_temp._role14_game_session_grant_operations,pg_temp._role14_role_space_deletion_fence_receipts,pg_temp._role14_role_space_deletion_fences;
 DROP FUNCTION pg_temp.role14_signature(OID);
END
$role14$;
