-- Accepted410: DB-enforced writer participation and per-Space UNKNOWN cutover.
-- Native source39/42 and their typed proof validators retain their authority.
-- This migration does not declare all Go writers adapted or a serving role safe.
BEGIN;
DO $$ BEGIN
 IF current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'membership guard installation requires READ COMMITTED' USING ERRCODE='55000';
 END IF;
END $$;
-- ACCESS EXCLUSIVE drains old source writers before the epoch is published.
LOCK TABLE spaces,space_members,space_ordinary_member_heads,space_community_member_heads,
 community_roster_members,community_owner_authority,space_membership_ownership_heads,
 space_member_admission_intents,space_native_member_publications,space_native_creator_operations,
 space_bot_installation_attempts,space_bot_installation_heads,space_bot_installation_cancellations,
 space_effective_member_heads,space_bans,ownership_journal,space_lifecycle_aggregates,
 space_permanent_purge_fences,community_owner_recovery_operations,community_bootstrap_operations,
 space_lifecycle_operations,space_lifecycle_participants,space_lifecycle_completions,
 space_lifecycle_outbox,space_deletion_tombstones IN ACCESS EXCLUSIVE MODE;

CREATE SCHEMA native_membership_guard;
REVOKE ALL ON SCHEMA native_membership_guard FROM PUBLIC;
CREATE TABLE native_membership_guard.epoch (
 singleton boolean PRIMARY KEY CHECK(singleton),
 epoch uuid NOT NULL CHECK(epoch='94db7e08-e958-44fa-94b2-988410ab0044'),
 catalog_sha256 bytea NOT NULL CHECK(octet_length(catalog_sha256)=32)
);
CREATE TABLE native_membership_guard.spaces (
 space_id uuid PRIMARY KEY, epoch uuid NOT NULL,
 cutover_id uuid NOT NULL, revision bigint NOT NULL CHECK(revision>0),
 state text NOT NULL CHECK(state IN ('COMPLETE','TERMINAL')),
 source_revision bigint NOT NULL, evaluated_at timestamptz NOT NULL
);
CREATE TABLE native_membership_guard.frames (
 xid bigint NOT NULL,space_id uuid NOT NULL,pid integer NOT NULL,
 token uuid NOT NULL UNIQUE,epoch uuid NOT NULL,started_at timestamptz NOT NULL,
 mode text NOT NULL CHECK(mode IN ('MUTATION','INITIALIZE','BORN')),
 sequence bigint NOT NULL DEFAULT 0,completed_sequence bigint NOT NULL DEFAULT -1,
 completed_at timestamptz,completed_digest bytea,last_source_at timestamptz,
 PRIMARY KEY(xid,space_id)
);
CREATE TABLE native_membership_guard.profiles (
 xid bigint NOT NULL,space_id uuid NOT NULL,profile_id uuid NOT NULL,
 old_head jsonb,old_sources jsonb NOT NULL,
 ordinary_cut boolean NOT NULL DEFAULT false,community_cut boolean NOT NULL DEFAULT false,
 PRIMARY KEY(xid,space_id,profile_id),
 FOREIGN KEY(xid,space_id) REFERENCES native_membership_guard.frames ON DELETE CASCADE
);
REVOKE ALL ON ALL TABLES IN SCHEMA native_membership_guard FROM PUBLIC;

CREATE FUNCTION native_membership_guard.coverage() RETURNS text[] LANGUAGE sql IMMUTABLE AS $$
 SELECT ARRAY['spaces','space_members','space_ordinary_member_heads','space_community_member_heads',
 'community_roster_members','community_owner_authority','space_membership_ownership_heads',
 'space_member_admission_intents','space_native_member_publications','space_native_creator_operations',
 'space_bot_installation_attempts','space_bot_installation_heads','space_bot_installation_cancellations',
 'space_effective_member_heads','space_bans','ownership_journal','space_lifecycle_aggregates',
 'space_permanent_purge_fences','community_owner_recovery_operations','community_bootstrap_operations',
 'space_lifecycle_operations','space_lifecycle_participants','space_lifecycle_completions',
 'space_lifecycle_outbox','space_deletion_tombstones']::text[]
$$;
CREATE FUNCTION native_membership_guard.catalog() RETURNS bytea LANGUAGE sql STABLE
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
 SELECT sha256(convert_to(jsonb_build_object(
  'tables',(SELECT jsonb_agg(jsonb_build_array(c.oid,c.relname,c.relowner,c.relkind,c.relpersistence,c.relrowsecurity,c.relforcerowsecurity,
    CASE WHEN n.nspname='native_membership_guard' THEN c.relacl END) ORDER BY c.oid) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE (n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR (n.nspname='native_membership_guard' AND c.relkind='r')),
  'columns',(SELECT jsonb_agg(to_jsonb(a) ORDER BY a.attrelid,a.attnum) FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE a.attnum>0 AND NOT a.attisdropped AND ((n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR n.nspname='native_membership_guard')),
  'triggers',(SELECT jsonb_agg(to_jsonb(t) ORDER BY t.oid) FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE (n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR n.nspname='native_membership_guard'),
  'constraints',(SELECT jsonb_agg(to_jsonb(k) ORDER BY k.oid) FROM pg_constraint k JOIN pg_class c ON c.oid=k.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace
   WHERE (n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR n.nspname='native_membership_guard'),
  'functions',(SELECT jsonb_agg(jsonb_build_array(p.oid,pg_get_functiondef(p.oid),p.proowner,p.proacl) ORDER BY p.oid)
   FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
   WHERE p.prokind='f' AND n.nspname IN ('public','native_membership_guard'))
 )::text,'UTF8'))
$$;
CREATE FUNCTION native_membership_guard.scope(s uuid) RETURNS void LANGUAGE plpgsql
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE key64 bigint;
BEGIN
 IF s IS NULL OR s='00000000-0000-0000-0000-000000000000' OR current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'membership guard requires scoped READ COMMITTED' USING ERRCODE='55000';
 END IF;
 IF EXISTS(SELECT 1 FROM pg_catalog.pg_class WHERE relnamespace=pg_catalog.pg_my_temp_schema()) THEN
  RAISE EXCEPTION 'membership scope forbids temporary relation shadowing' USING ERRCODE='55000';
 END IF;
 -- Identical SHA256 namespace/NUL/UUID first-eight-byte signed key to Go.
 key64:=('x'||substr(encode(sha256(convert_to('voice.space.mutation.v1','UTF8')||decode('00','hex')||uuid_send(s)),'hex'),1,16))::bit(64)::bigint;
 IF NOT EXISTS(SELECT 1 FROM pg_locks WHERE locktype='advisory' AND pid=pg_backend_pid()
  AND granted AND mode='ExclusiveLock' AND objsubid=1 AND classid::bigint=((key64>>32)&4294967295) AND objid::bigint=(key64&4294967295)) THEN
  RAISE EXCEPTION 'membership guard requires same-backend Space fence' USING ERRCODE='55000';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM native_membership_guard.epoch WHERE singleton AND epoch='94db7e08-e958-44fa-94b2-988410ab0044'
  AND catalog_sha256=native_membership_guard.catalog()) THEN
  RAISE EXCEPTION 'membership guard catalog is not exact' USING ERRCODE='55000';
 END IF;
END $$;

CREATE FUNCTION native_membership_guard.cohort(s uuid) RETURNS SETOF uuid LANGUAGE sql STABLE
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
 SELECT profile_id FROM public.space_members WHERE space_id=s
 UNION SELECT profile_id FROM public.space_ordinary_member_heads WHERE space_id=s
 UNION SELECT profile_id FROM public.space_community_member_heads WHERE space_id=s
 UNION SELECT profile_id FROM public.community_roster_members WHERE space_id=s
 UNION SELECT profile_id FROM public.space_effective_member_heads WHERE space_id=s
 UNION SELECT profile_id FROM public.space_member_admission_intents WHERE space_id=s
 UNION SELECT profile_id FROM public.space_native_member_publications WHERE space_id=s
 UNION SELECT profile_id FROM public.space_bot_installation_heads WHERE space_id=s
 UNION SELECT profile_id FROM public.space_bot_installation_attempts WHERE space_id=s
$$;

-- This is a DB transition witness, not a replacement for the typed native
-- protobuf/operation validator. Serving additionally qualifies that provider.
CREATE FUNCTION native_membership_guard.sources(s uuid,p uuid,at_time timestamptz) RETURNS jsonb LANGUAGE sql STABLE
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
 SELECT jsonb_build_object('ordinary',o.witness,'legacy',COALESCE(o.legacy,false),'community',c.witness,'until',c.deadline)
 FROM (SELECT 1) seed
 LEFT JOIN LATERAL (
  SELECT 'o/'||h.generation||'/'||h.publication_id||'/'||h.legacy::text witness,h.legacy
  FROM public.space_ordinary_member_heads h JOIN public.space_members m
   ON m.space_id=h.space_id AND m.profile_id=h.profile_id AND m.membership_generation=h.generation AND m.admission_operation_id=h.publication_id
  WHERE h.space_id=s AND h.profile_id=p AND h.state='ACTIVE'
   AND (h.legacy OR h.generation>h.retired_through)
   AND NOT EXISTS(SELECT 1 FROM public.space_bans b WHERE b.space_id=s AND b.account_id=h.account_id)
   AND (h.legacy OR (h.publication_kind=1 AND EXISTS(SELECT 1 FROM public.space_member_admission_intents i
     WHERE i.operation_id=h.publication_id AND i.space_id=s AND i.profile_id=p AND i.generation=h.generation AND i.state='PUBLISHED'
      AND i.binding_sha256=h.admission_binding_sha256 AND i.role_receipt_sha256=h.role_receipt_sha256))
    OR (h.publication_kind BETWEEN 3 AND 6 AND EXISTS(SELECT 1 FROM public.space_native_member_publications n
     WHERE n.operation_id=h.publication_id AND n.space_id=s AND n.profile_id=p AND n.generation=h.generation
      AND n.kind=h.publication_kind AND n.state='PUBLISHED' AND n.binding_sha256=h.native_publication_sha256)))
 ) o ON true
 LEFT JOIN LATERAL (
  SELECT 'c/'||h.owner_generation||'/'||h.generation||'/'||h.episode_id witness,
   LEAST(m.lease_expires_at,a.roster_lease_expires_at) deadline
  FROM public.space_community_member_heads h JOIN public.community_roster_members m
   ON m.space_id=h.space_id AND m.profile_id=h.profile_id AND m.owner_generation=h.owner_generation
  JOIN public.community_owner_authority a ON a.space_id=m.space_id AND a.owner_generation=m.owner_generation
  WHERE h.space_id=s AND h.profile_id=p AND h.state='ACTIVE' AND h.generation>h.retired_through AND a.status='active'
   AND m.revoked_at IS NULL AND m.source_revision>0 AND m.lease_expires_at>at_time AND a.roster_lease_expires_at>at_time
 ) c ON true
$$;

CREATE FUNCTION native_membership_guard.head_sources(h jsonb,at_time timestamptz) RETURNS jsonb LANGUAGE sql IMMUTABLE AS $$
 SELECT jsonb_build_object('ordinary',h->>'ordinary_witness','legacy',COALESCE((h->>'ordinary_legacy')::boolean,false),
 'community',CASE WHEN (h->>'community_until')::timestamptz>at_time THEN h->>'community_witness' END,
 'until',CASE WHEN (h->>'community_until')::timestamptz>at_time THEN (h->>'community_until')::timestamptz END)
$$;

CREATE FUNCTION native_membership_guard.digest_space(s uuid) RETURNS bytea LANGUAGE plpgsql STABLE
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE name text; raw jsonb; result jsonb:='{}';
BEGIN
 FOREACH name IN ARRAY native_membership_guard.coverage() LOOP
  EXECUTE format('SELECT COALESCE(jsonb_agg(to_jsonb(x) ORDER BY to_jsonb(x)::text),''[]''::jsonb) FROM public.%I x WHERE %I=$1',name,CASE WHEN name='spaces' THEN 'id' ELSE 'space_id' END) INTO raw USING s;
  result:=result||jsonb_build_object(name,raw);
 END LOOP;
 RETURN sha256(convert_to(result::text,'UTF8'));
END $$;

CREATE FUNCTION native_membership_guard.open_frame(s uuid,requested uuid[],kind text) RETURNS uuid LANGUAGE plpgsql
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE token_id uuid:=gen_random_uuid();p uuid;at_time timestamptz;h jsonb;src jsonb;
BEGIN
 PERFORM native_membership_guard.scope(s);
 IF requested IS NULL OR array_position(requested,NULL) IS NOT NULL OR '00000000-0000-0000-0000-000000000000'::uuid=ANY(requested)
 OR EXISTS(SELECT 1 FROM public.space_permanent_purge_fences WHERE space_id=s) THEN
  RAISE EXCEPTION 'invalid or terminal membership capture' USING ERRCODE='55000';
 END IF;
 IF EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE xid=txid_current() AND space_id=s) THEN
  RAISE EXCEPTION 'membership frame already exists' USING ERRCODE='55000';
 END IF;
 IF kind='MUTATION' AND NOT EXISTS(SELECT 1 FROM native_membership_guard.spaces WHERE space_id=s AND state='COMPLETE'
  AND epoch='94db7e08-e958-44fa-94b2-988410ab0044') THEN
  IF EXISTS(SELECT 1 FROM public.spaces WHERE id=s) OR EXISTS(SELECT 1 FROM native_membership_guard.cohort(s)) THEN
   RAISE EXCEPTION 'existing Space requires administrative full cutover' USING ERRCODE='55000';
  END IF;
  kind:='BORN';
  INSERT INTO native_membership_guard.spaces VALUES(s,'94db7e08-e958-44fa-94b2-988410ab0044',gen_random_uuid(),1,'COMPLETE',0,clock_timestamp());
 END IF;
 at_time:=clock_timestamp();
 INSERT INTO native_membership_guard.frames(xid,space_id,pid,token,epoch,started_at,mode)
 VALUES(txid_current(),s,pg_backend_pid(),token_id,'94db7e08-e958-44fa-94b2-988410ab0044',at_time,kind);
 FOR p IN SELECT v FROM (SELECT native_membership_guard.cohort(s) v UNION SELECT unnest(requested)) all_profiles ORDER BY v LOOP
  SELECT to_jsonb(x) INTO h FROM public.space_effective_member_heads x WHERE space_id=s AND profile_id=p FOR UPDATE;
  src:=native_membership_guard.sources(s,p,at_time);
  IF kind<>'INITIALIZE' AND ((h IS NULL AND (src->>'ordinary' IS NOT NULL OR src->>'community' IS NOT NULL))
   OR (h IS NOT NULL AND native_membership_guard.head_sources(h,at_time)<>src)) THEN
   RAISE EXCEPTION 'membership capture has unproved previous sources' USING ERRCODE='55000';
  END IF;
  INSERT INTO native_membership_guard.profiles(xid,space_id,profile_id,old_head,old_sources) VALUES(txid_current(),s,p,h,src);
 END LOOP;
 RETURN token_id;
END $$;

CREATE FUNCTION public.space_native_membership_begin(s uuid,profiles uuid[]) RETURNS uuid LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
BEGIN RETURN native_membership_guard.open_frame(s,profiles,'MUTATION'); END $$;

CREATE FUNCTION public.space_native_membership_profiles(s uuid,token_id uuid) RETURNS SETOF uuid LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
BEGIN
 PERFORM native_membership_guard.scope(s);
 IF NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE xid=txid_current() AND space_id=s AND pid=pg_backend_pid() AND token=token_id) THEN
  RAISE EXCEPTION 'membership cohort belongs to another capture' USING ERRCODE='55000';
 END IF;
 RETURN QUERY SELECT p.profile_id FROM native_membership_guard.profiles p WHERE p.xid=txid_current() AND p.space_id=s ORDER BY p.profile_id;
END $$;

CREATE FUNCTION native_membership_guard.terminal(s uuid) RETURNS boolean LANGUAGE sql STABLE
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
 SELECT public.space_terminal_purge_ready(s) AND EXISTS(SELECT 1 FROM public.space_permanent_purge_fences f
  JOIN public.space_lifecycle_aggregates a ON a.space_id=f.space_id
  WHERE f.space_id=s AND a.deletion_operation_id=f.deletion_operation_id AND a.generation=f.purge_generation
   AND a.phase='PURGED' AND a.local_purge_completed)
$$;

CREATE FUNCTION native_membership_guard.source_before() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE s uuid;r jsonb;old_space uuid;p uuid;frame native_membership_guard.frames;
BEGIN
 IF TG_OP='TRUNCATE' THEN RAISE EXCEPTION 'membership source TRUNCATE forbidden' USING ERRCODE='55000'; END IF;
 IF TG_TABLE_NAME='space_permanent_purge_fences' THEN
  IF TG_OP='DELETE' OR (TG_OP='UPDATE' AND to_jsonb(NEW) IS DISTINCT FROM to_jsonb(OLD)) THEN
   RAISE EXCEPTION 'permanent Space retirement cannot be erased or rebound' USING ERRCODE='55000';
  END IF;
  IF TG_OP='UPDATE' THEN RETURN NEW; END IF;
 END IF;
 IF TG_TABLE_NAME='spaces' AND TG_OP='UPDATE' AND to_jsonb(NEW)->>'owner_profile_id' IS NOT DISTINCT FROM to_jsonb(OLD)->>'owner_profile_id' THEN RETURN NEW; END IF;
 r:=CASE WHEN TG_OP='DELETE' THEN to_jsonb(OLD) ELSE to_jsonb(NEW) END;
 s:=(r->>TG_ARGV[0])::uuid;
 IF TG_OP='UPDATE' THEN old_space:=(to_jsonb(OLD)->>TG_ARGV[0])::uuid;
  IF old_space IS DISTINCT FROM s THEN RAISE EXCEPTION 'membership source cannot move Space' USING ERRCODE='55000'; END IF;
 END IF;
 -- Only the exact existing terminal workflow may bypass interactive capture.
 -- Existing native guards still impose their own permanent/current11 checks.
 IF (TG_OP='DELETE' OR (TG_OP='UPDATE' AND TG_TABLE_NAME IN ('space_ordinary_member_heads','space_community_member_heads','space_membership_ownership_heads')))
  AND native_membership_guard.terminal(s) THEN
  IF TG_OP='DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
 END IF;
 PERFORM native_membership_guard.scope(s);
 SELECT * INTO frame FROM native_membership_guard.frames WHERE xid=txid_current() AND space_id=s AND pid=pg_backend_pid() FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'native source mutation requires DB capture' USING ERRCODE='55000'; END IF;
 p:=(r->>'profile_id')::uuid;
 IF p IS NOT NULL AND NOT EXISTS(SELECT 1 FROM native_membership_guard.profiles WHERE xid=txid_current() AND space_id=s AND profile_id=p) THEN
  RAISE EXCEPTION 'native mutation omitted affected profile' USING ERRCODE='55000';
 END IF;
 UPDATE native_membership_guard.frames SET sequence=sequence+1,completed_sequence=-1,completed_digest=NULL,completed_at=NULL
 WHERE xid=txid_current() AND space_id=s;
 IF TG_OP='DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
END $$;

CREATE FUNCTION native_membership_guard.source_after() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE s uuid;r native_membership_guard.profiles;src jsonb;at_time timestamptz;
BEGIN
 IF TG_TABLE_NAME='space_effective_member_heads' THEN RETURN NULL; END IF;
 IF TG_TABLE_NAME='spaces' AND TG_OP='UPDATE' AND to_jsonb(NEW)->>'owner_profile_id' IS NOT DISTINCT FROM to_jsonb(OLD)->>'owner_profile_id' THEN RETURN NULL; END IF;
 s:=((CASE WHEN TG_OP='DELETE' THEN to_jsonb(OLD) ELSE to_jsonb(NEW) END)->>TG_ARGV[0])::uuid;
 IF NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE xid=txid_current() AND space_id=s) THEN RETURN NULL; END IF;
 at_time:=clock_timestamp();
 -- Capture intermediate semantic disappearance too: ban/unban ABA cannot hide
 -- a real loss merely because its final ordinary witness equals the old one.
 FOR r IN SELECT * FROM native_membership_guard.profiles WHERE xid=txid_current() AND space_id=s LOOP
  src:=native_membership_guard.sources(s,r.profile_id,at_time);
  UPDATE native_membership_guard.profiles SET
   ordinary_cut=ordinary_cut OR (r.old_sources->>'ordinary' IS NOT NULL AND r.old_sources->>'ordinary' IS DISTINCT FROM src->>'ordinary'),
   community_cut=community_cut OR (r.old_sources->>'community' IS NOT NULL AND r.old_sources->>'community' IS DISTINCT FROM src->>'community')
   WHERE xid=r.xid AND space_id=s AND profile_id=r.profile_id;
 END LOOP;
 UPDATE native_membership_guard.frames SET last_source_at=clock_timestamp(),sequence=sequence+1,completed_sequence=-1,
  completed_digest=NULL,completed_at=NULL WHERE xid=txid_current() AND space_id=s;
 RETURN NULL;
END $$;

DO $$ DECLARE name text; BEGIN
 FOREACH name IN ARRAY native_membership_guard.coverage() LOOP
  EXECUTE format('CREATE TRIGGER a000_native_membership_capture BEFORE INSERT OR UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION native_membership_guard.source_before(%L)',name,CASE WHEN name='spaces' THEN 'id' ELSE 'space_id' END);
  EXECUTE format('CREATE TRIGGER a000_native_membership_no_truncate BEFORE TRUNCATE ON public.%I FOR EACH STATEMENT EXECUTE FUNCTION native_membership_guard.source_before()',name);
  EXECUTE format('ALTER TABLE public.%I ENABLE ALWAYS TRIGGER a000_native_membership_capture',name);
  EXECUTE format('ALTER TABLE public.%I ENABLE ALWAYS TRIGGER a000_native_membership_no_truncate',name);
  EXECUTE format('CREATE TRIGGER zzzzzz_native_membership_observe AFTER INSERT OR UPDATE OR DELETE ON public.%I FOR EACH ROW EXECUTE FUNCTION native_membership_guard.source_after(%L)',name,CASE WHEN name='spaces' THEN 'id' ELSE 'space_id' END);
  EXECUTE format('ALTER TABLE public.%I ENABLE ALWAYS TRIGGER zzzzzz_native_membership_observe',name);
 END LOOP;
END $$;

CREATE FUNCTION native_membership_guard.verify_completion(s uuid,seal boolean DEFAULT true) RETURNS void LANGUAGE plpgsql
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE f native_membership_guard.frames;r native_membership_guard.profiles;h jsonb;src jsonb;at_time timestamptz;
 preserve boolean;ordinary_bridge boolean;community_bridge boolean;old_generation bigint;expected_generation bigint;
BEGIN
 PERFORM native_membership_guard.scope(s);
 SELECT * INTO f FROM native_membership_guard.frames WHERE xid=txid_current() AND space_id=s AND pid=pg_backend_pid() FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'membership completion has no capture' USING ERRCODE='55000'; END IF;
 IF EXISTS(SELECT 1 FROM native_membership_guard.cohort(s) p WHERE NOT EXISTS(SELECT 1 FROM native_membership_guard.profiles c
  WHERE c.xid=txid_current() AND c.space_id=s AND c.profile_id=p)) THEN
  RAISE EXCEPTION 'membership completion cohort expanded without capture' USING ERRCODE='55000';
 END IF;
 -- Lock ALL potentially waiting ledger rows before the one common final time.
 PERFORM 1 FROM public.space_effective_member_heads WHERE space_id=s ORDER BY profile_id FOR UPDATE;
 at_time:=clock_timestamp();
 IF at_time<f.started_at OR at_time>=f.started_at+interval '5 seconds' THEN
  RAISE EXCEPTION 'membership completion exceeded finite decision budget' USING ERRCODE='55000';
 END IF;
 FOR r IN SELECT * FROM native_membership_guard.profiles WHERE xid=txid_current() AND space_id=s ORDER BY profile_id LOOP
  SELECT to_jsonb(x) INTO h FROM public.space_effective_member_heads x WHERE space_id=s AND profile_id=r.profile_id;
  src:=native_membership_guard.sources(s,r.profile_id,at_time);
  IF h IS NULL OR native_membership_guard.head_sources(h,at_time)<>src
   OR (h->>'observed_at')::timestamptz<f.started_at OR (h->>'observed_at')::timestamptz>at_time
   OR ((h->>'state')='ACTIVE') IS DISTINCT FROM (src->>'ordinary' IS NOT NULL OR src->>'community' IS NOT NULL)
   OR (h->>'community_witness' IS NOT NULL AND (h->>'community_until')::timestamptz<=at_time) THEN
   RAISE EXCEPTION 'membership completion head/source mismatch' USING ERRCODE='55000';
  END IF;
  old_generation:=COALESCE((r.old_head->>'generation')::bigint,0);
  IF f.mode='INITIALIZE' THEN
   IF (h->>'state' IN ('ACTIVE','NONE') AND (h->>'boundary_state'<>'UNKNOWN' OR h->>'history_visible_since' IS NOT NULL))
    OR (h->>'state'='ACTIVE' AND ((h->>'generation')::bigint<>old_generation+1 OR h->>'episode_id' IS NOT DISTINCT FROM r.old_head->>'episode_id')) THEN
    RAISE EXCEPTION 'first full cutover must establish UNKNOWN successor' USING ERRCODE='55000';
   END IF;
  ELSE
   ordinary_bridge:=NOT r.ordinary_cut AND r.old_sources->>'ordinary' IS NOT NULL AND r.old_sources->>'ordinary'=src->>'ordinary';
   community_bridge:=NOT r.community_cut AND r.old_sources->>'community' IS NOT NULL AND r.old_sources->>'community'=src->>'community';
   preserve:=r.old_head->>'state'='ACTIVE' AND (ordinary_bridge OR community_bridge);
   IF preserve AND NOT ordinary_bridge AND (r.old_sources->>'until')::timestamptz<=at_time THEN
    RAISE EXCEPTION 'OLD finite bridge expired before completion' USING ERRCODE='55000';
   END IF;
   IF preserve THEN
    IF ROW(h->>'generation',h->>'episode_id',h->>'boundary_state',h->>'history_visible_since')
     IS DISTINCT FROM ROW(r.old_head->>'generation',r.old_head->>'episode_id',r.old_head->>'boundary_state',r.old_head->>'history_visible_since') THEN
     RAISE EXCEPTION 'continuous membership history rebound' USING ERRCODE='55000';
    END IF;
   ELSIF src->>'ordinary' IS NOT NULL OR src->>'community' IS NOT NULL THEN
    IF (h->>'generation')::bigint<>old_generation+1 OR h->>'episode_id' IS NOT DISTINCT FROM r.old_head->>'episode_id'
     OR (h->>'retired_through')::bigint<old_generation
     OR (src->>'legacy')::boolean IS DISTINCT FROM (h->>'boundary_state'='UNKNOWN')
     OR (h->>'boundary_state'='KNOWN' AND (h->>'history_visible_since')::timestamptz<COALESCE(f.last_source_at,f.started_at)) THEN
     RAISE EXCEPTION 'new membership episode lacks actual publication boundary' USING ERRCODE='55000';
    END IF;
   ELSIF (h->>'generation')::bigint<>old_generation OR (h->>'retired_through')::bigint<>old_generation THEN
    RAISE EXCEPTION 'membership retirement floor mismatch' USING ERRCODE='55000';
   END IF;
  END IF;
 END LOOP;
 IF seal THEN
  UPDATE native_membership_guard.frames SET completed_sequence=sequence,completed_at=at_time,
   completed_digest=native_membership_guard.digest_space(s) WHERE xid=txid_current() AND space_id=s;
 END IF;
END $$;

CREATE FUNCTION public.space_native_membership_complete(s uuid,token_id uuid) RETURNS void LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
BEGIN
 IF NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE xid=txid_current() AND space_id=s AND pid=pg_backend_pid() AND token=token_id) THEN
  RAISE EXCEPTION 'membership completion token belongs to another scope' USING ERRCODE='55000';
 END IF;
 PERFORM native_membership_guard.verify_completion(s);
END $$;

CREATE FUNCTION native_membership_guard.commit_check() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE f native_membership_guard.frames;
BEGIN
 SELECT * INTO f FROM native_membership_guard.frames WHERE xid=NEW.xid AND space_id=NEW.space_id;
 IF NOT FOUND THEN RETURN NULL; END IF;
 IF f.xid<>txid_current() OR f.pid<>pg_backend_pid() OR f.completed_sequence<>f.sequence
  OR f.completed_digest IS DISTINCT FROM native_membership_guard.digest_space(f.space_id) THEN
  RAISE EXCEPTION 'native membership frame is dirty or uncompleted at commit' USING ERRCODE='55000';
 END IF;
 -- Revalidate after deferred/late waiting SQL, without minting completion.
 PERFORM native_membership_guard.verify_completion(f.space_id,false);
 DELETE FROM native_membership_guard.frames WHERE xid=f.xid AND space_id=f.space_id;
 RETURN NULL;
END $$;
CREATE CONSTRAINT TRIGGER membership_frame_completed AFTER INSERT OR UPDATE ON native_membership_guard.frames
 DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION native_membership_guard.commit_check();

CREATE FUNCTION public.space_native_membership_require(s uuid) RETURNS void LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE role_oid oid;
BEGIN
 PERFORM native_membership_guard.scope(s);
 SELECT oid INTO role_oid FROM pg_roles WHERE rolname=session_user AND NOT rolsuper AND NOT rolcreaterole AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls;
 IF role_oid IS NULL OR has_schema_privilege(session_user,'public','CREATE') OR has_schema_privilege(session_user,'native_membership_guard','CREATE')
 OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
  WHERE ((n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR n.nspname='native_membership_guard')
  AND pg_has_role(role_oid,c.relowner,'MEMBER'))
 OR EXISTS(SELECT 1 FROM pg_roles r WHERE (r.rolsuper OR r.rolcreaterole OR r.rolreplication OR r.rolbypassrls) AND pg_has_role(role_oid,r.oid,'MEMBER'))
 OR has_parameter_privilege(session_user,'session_replication_role','SET') THEN
  RAISE EXCEPTION 'membership serving role can bypass enforcement' USING ERRCODE='55000';
 END IF;
 PERFORM 1 FROM native_membership_guard.spaces WHERE space_id=s AND state='COMPLETE' AND epoch='94db7e08-e958-44fa-94b2-988410ab0044' FOR SHARE;
 IF NOT FOUND OR EXISTS(SELECT 1 FROM public.space_permanent_purge_fences WHERE space_id=s)
 OR NOT EXISTS(SELECT 1 FROM public.spaces WHERE id=s)
 OR EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE xid=txid_current() AND space_id=s AND completed_sequence<>sequence) THEN
  RAISE EXCEPTION 'Space native membership cutover is not complete' USING ERRCODE='55000';
 END IF;
END $$;

-- Only migration/deployment owner may invoke this administrative producer.
-- The Go initializer validates every typed native source before this function.
CREATE FUNCTION public.space_native_membership_initialize(s uuid) RETURNS void LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE token_id uuid;p uuid;src jsonb;h public.space_effective_member_heads;at_time timestamptz;active boolean;rev bigint;
BEGIN
 PERFORM native_membership_guard.scope(s);
 IF EXISTS(SELECT 1 FROM native_membership_guard.spaces WHERE space_id=s) THEN
  RAISE EXCEPTION 'membership cutover cannot be repeated or repaired' USING ERRCODE='55000';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM public.spaces WHERE id=s) THEN RAISE EXCEPTION 'cutover requires existing Space' USING ERRCODE='55000'; END IF;
 token_id:=native_membership_guard.open_frame(s,ARRAY[]::uuid[],'INITIALIZE');
 at_time:=clock_timestamp();
 FOR p IN SELECT profile_id FROM native_membership_guard.profiles WHERE xid=txid_current() AND space_id=s ORDER BY profile_id LOOP
  src:=native_membership_guard.sources(s,p,at_time);active:=src->>'ordinary' IS NOT NULL OR src->>'community' IS NOT NULL;
  SELECT * INTO h FROM public.space_effective_member_heads WHERE space_id=s AND profile_id=p;
  IF FOUND THEN
   UPDATE public.space_effective_member_heads SET generation=h.generation+CASE WHEN active THEN 1 ELSE 0 END,
    retired_through=h.generation,ledger_revision=h.ledger_revision+1,
    episode_id=CASE WHEN active THEN gen_random_uuid() ELSE h.episode_id END,
    state=CASE WHEN active THEN 'ACTIVE' WHEN h.generation>0 THEN 'RETIRED' ELSE 'NONE' END,
    boundary_state=CASE WHEN active OR h.generation=0 THEN 'UNKNOWN' ELSE h.boundary_state END,
    history_visible_since=CASE WHEN active OR h.generation=0 THEN NULL ELSE h.history_visible_since END,
    covered_until=CASE WHEN src->>'ordinary' IS NOT NULL THEN 'infinity'::timestamptz ELSE (src->>'until')::timestamptz END,
    observed_at=at_time,ordinary_witness=src->>'ordinary',ordinary_legacy=(src->>'legacy')::boolean,
    community_witness=src->>'community',community_until=(src->>'until')::timestamptz WHERE space_id=s AND profile_id=p;
  ELSE
   INSERT INTO public.space_effective_member_heads(space_id,profile_id,generation,retired_through,ledger_revision,episode_id,state,boundary_state,
    covered_until,observed_at,cutover_id,ordinary_witness,ordinary_legacy,community_witness,community_until)
   VALUES(s,p,CASE WHEN active THEN 1 ELSE 0 END,0,1,CASE WHEN active THEN gen_random_uuid() END,CASE WHEN active THEN 'ACTIVE' ELSE 'NONE' END,'UNKNOWN',
    CASE WHEN src->>'ordinary' IS NOT NULL THEN 'infinity'::timestamptz ELSE (src->>'until')::timestamptz END,
    at_time,gen_random_uuid(),src->>'ordinary',(src->>'legacy')::boolean,src->>'community',(src->>'until')::timestamptz);
  END IF;
 END LOOP;
 PERFORM native_membership_guard.verify_completion(s);
 SELECT access_epoch INTO rev FROM public.space_voice_access_epochs WHERE space_id=s;
 INSERT INTO native_membership_guard.spaces VALUES(s,'94db7e08-e958-44fa-94b2-988410ab0044',gen_random_uuid(),1,'COMPLETE',COALESCE(rev,0),clock_timestamp());
END $$;
REVOKE ALL ON FUNCTION public.space_native_membership_initialize(uuid) FROM PUBLIC;

CREATE FUNCTION native_membership_guard.terminal_cleanup() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
BEGIN
 IF NOT public.space_membership_terminal_cleanup_allowed(OLD.id) THEN RAISE EXCEPTION 'guard cleanup requires native/current11 terminal P3' USING ERRCODE='55000'; END IF;
 DELETE FROM native_membership_guard.frames WHERE space_id=OLD.id;
 INSERT INTO native_membership_guard.spaces VALUES(OLD.id,'94db7e08-e958-44fa-94b2-988410ab0044',gen_random_uuid(),1,'TERMINAL',0,clock_timestamp())
 ON CONFLICT(space_id) DO UPDATE SET state='TERMINAL',revision=native_membership_guard.spaces.revision+1,evaluated_at=clock_timestamp();
 RETURN OLD;
END $$;
CREATE TRIGGER zzzzz_native_membership_guard_terminal AFTER DELETE ON public.spaces
 FOR EACH ROW EXECUTE FUNCTION native_membership_guard.terminal_cleanup();

-- Protected helpers have no direct application entry points.
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA native_membership_guard FROM PUBLIC;
INSERT INTO native_membership_guard.epoch VALUES(true,'94db7e08-e958-44fa-94b2-988410ab0044',native_membership_guard.catalog());
COMMIT;
