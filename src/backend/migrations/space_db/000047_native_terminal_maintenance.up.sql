-- Fixed negative lifecycle operations. No positive membership scope is opened.
-- The retained46 seal is deliberately NOT changed by migration application.
BEGIN;
ALTER TABLE native_membership_guard.spaces ADD COLUMN terminal_maintenance_phase text;
ALTER TABLE native_membership_guard.spaces ADD CONSTRAINT terminal_maintenance_phase_check
 CHECK(terminal_maintenance_phase IS NULL OR
  (state='TERMINAL' AND terminal_maintenance_phase IN ('FULL','COMPACT','EXPIRED')));

CREATE TABLE native_membership_guard.terminal_maintenance_scope (
 xid bigint NOT NULL, pid integer NOT NULL, space_id uuid NOT NULL,
 operation text NOT NULL CHECK(operation IN ('ACK','COMPACT','EXPIRE_EVENTS','EXPIRE')),
 event_id uuid, PRIMARY KEY(xid,space_id),
 CHECK((operation='ACK')=(event_id IS NOT NULL))
);
CREATE TABLE native_membership_guard.catalog47_transition (
 singleton boolean PRIMARY KEY CHECK(singleton), epoch uuid NOT NULL,
 migration_sha256 bytea NOT NULL CHECK(octet_length(migration_sha256)=32),
 old_seal_sha256 bytea NOT NULL CHECK(octet_length(old_seal_sha256)=32),
 new_seal_sha256 bytea NOT NULL CHECK(octet_length(new_seal_sha256)=32),
 transitioned_at timestamptz NOT NULL
);
REVOKE ALL ON native_membership_guard.terminal_maintenance_scope,
 native_membership_guard.catalog47_transition FROM PUBLIC;

CREATE FUNCTION native_membership_guard.terminal_serving_scope(s uuid) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER
SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE role_oid oid;
BEGIN
 PERFORM native_membership_guard.scope(s);
 SELECT oid INTO role_oid FROM pg_roles WHERE rolname=session_user AND rolcanlogin
  AND NOT rolsuper AND NOT rolcreaterole AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls;
 IF role_oid IS NULL OR has_schema_privilege(session_user,'public','CREATE')
 OR has_schema_privilege(session_user,'native_membership_guard','CREATE')
 OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
  WHERE ((n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR n.nspname='native_membership_guard')
  AND pg_has_role(role_oid,c.relowner,'MEMBER'))
 OR EXISTS(SELECT 1 FROM pg_roles r WHERE (r.rolsuper OR r.rolcreaterole OR r.rolreplication OR r.rolbypassrls) AND pg_has_role(role_oid,r.oid,'MEMBER'))
 OR has_parameter_privilege(session_user,'session_replication_role','SET') THEN
  RAISE EXCEPTION 'terminal maintenance serving role can bypass enforcement' USING ERRCODE='55000';
 END IF;
 IF EXISTS(SELECT 1 FROM public.spaces WHERE id=s)
 OR NOT EXISTS(SELECT 1 FROM public.space_permanent_purge_fences WHERE space_id=s)
 OR EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE space_id=s)
 OR EXISTS(SELECT 1 FROM native_membership_guard.profiles WHERE space_id=s)
 OR EXISTS(SELECT 1 FROM public.space_ordinary_member_heads WHERE space_id=s)
 OR EXISTS(SELECT 1 FROM public.space_community_member_heads WHERE space_id=s)
 OR EXISTS(SELECT 1 FROM public.space_effective_member_heads WHERE space_id=s) THEN
  RAISE EXCEPTION 'terminal maintenance is negative only' USING ERRCODE='55000';
 END IF;
 PERFORM 1 FROM native_membership_guard.spaces WHERE space_id=s AND state='TERMINAL'
  AND epoch='94db7e08-e958-44fa-94b2-988410ab0044'
  AND terminal_xid IS NOT NULL AND octet_length(terminal_token_sha256)=32 FOR UPDATE;
 IF NOT FOUND THEN RAISE EXCEPTION 'terminal maintenance lacks native P3 disposition' USING ERRCODE='55000'; END IF;
END $$;

-- Structural certification carries no extra participant identity/hash/deadline.
-- Only intact FULL proof can acquire an initially absent certification.
CREATE FUNCTION native_membership_guard.terminal_representation(s uuid) RETURNS text
LANGUAGE plpgsql SECURITY DEFINER
SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE phase text; a public.space_lifecycle_aggregates;
BEGIN
 PERFORM native_membership_guard.terminal_serving_scope(s);
 SELECT terminal_maintenance_phase INTO phase FROM native_membership_guard.spaces WHERE space_id=s;
 IF phase IS NULL OR phase='FULL' THEN
  IF NOT public.space_membership_terminal_cleanup_allowed(s) THEN
   RAISE EXCEPTION 'terminal FULL proof is not intact' USING ERRCODE='55000';
  END IF;
  SELECT * INTO a FROM public.space_lifecycle_aggregates WHERE space_id=s FOR UPDATE;
  PERFORM 1 FROM public.space_lifecycle_participants WHERE space_id=s ORDER BY generation,participant_id,request_kind FOR UPDATE;
  IF a.evidence_compacted OR EXISTS(SELECT 1 FROM public.space_lifecycle_completions WHERE space_id=s)
  OR (SELECT count(*) FROM public.space_lifecycle_participants p WHERE p.space_id=s
   AND p.deletion_operation_id=a.deletion_operation_id AND p.generation=a.generation
   AND p.progress='COMPLETE' AND p.completed_at IS NOT NULL AND p.receipt_bytes IS NOT NULL
   AND p.manifest_id=a.manifest_id AND p.manifest_sha256=a.manifest_sha256 AND p.manifest_item_count=a.manifest_item_count
   AND ((p.participant_id=1 AND p.request_kind='ROLE_RETIREMENT') OR (p.participant_id BETWEEN 2 AND 11 AND p.request_kind='PURGE')))<>11 THEN
   RAISE EXCEPTION 'terminal FULL current11 evidence differs' USING ERRCODE='55000';
  END IF;
  UPDATE native_membership_guard.spaces SET terminal_maintenance_phase='FULL' WHERE space_id=s AND terminal_maintenance_phase IS NULL;
  RETURN 'FULL';
 ELSIF phase='COMPACT' THEN
  SELECT * INTO a FROM public.space_lifecycle_aggregates WHERE space_id=s FOR UPDATE;
  IF NOT FOUND OR a.phase<>'PURGED' OR NOT a.local_purge_completed OR NOT a.evidence_compacted
  OR NOT EXISTS(SELECT 1 FROM public.space_deletion_tombstones t JOIN public.space_permanent_purge_fences f ON f.space_id=t.space_id
   WHERE t.space_id=s AND f.deletion_operation_id=a.deletion_operation_id AND f.purge_generation=a.generation)
  OR EXISTS(SELECT 1 FROM public.space_lifecycle_participants WHERE space_id=s)
  OR (SELECT count(*) FROM public.space_lifecycle_completions WHERE space_id=s)<>11
  OR (SELECT count(*) FROM public.space_lifecycle_completions p WHERE p.space_id=s AND p.deletion_operation_id=a.deletion_operation_id
   AND p.generation=a.generation AND p.manifest_sha256=a.manifest_sha256
   AND ((p.participant_id=1 AND p.request_kind='ROLE_RETIREMENT') OR (p.participant_id BETWEEN 2 AND 11 AND p.request_kind='PURGE')))<>11 THEN
   RAISE EXCEPTION 'terminal COMPACT representation differs' USING ERRCODE='55000';
  END IF;
  RETURN phase;
 ELSIF phase='EXPIRED' THEN
  IF EXISTS(SELECT 1 FROM public.space_lifecycle_aggregates WHERE space_id=s)
  OR EXISTS(SELECT 1 FROM public.space_deletion_tombstones WHERE space_id=s)
  OR EXISTS(SELECT 1 FROM public.space_lifecycle_participants WHERE space_id=s)
  OR EXISTS(SELECT 1 FROM public.space_lifecycle_completions WHERE space_id=s)
  OR EXISTS(SELECT 1 FROM public.space_lifecycle_operations WHERE space_id=s) THEN
   RAISE EXCEPTION 'terminal EXPIRED representation differs' USING ERRCODE='55000';
  END IF;
  RETURN phase;
 END IF;
 RAISE EXCEPTION 'unknown terminal representation' USING ERRCODE='55000';
END $$;

-- Exact deterministic protobuf value encoding of the existing saved record.
CREATE FUNCTION native_membership_guard.terminal_varint(v bigint) RETURNS bytea
LANGUAGE plpgsql IMMUTABLE SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE result bytea:=''::bytea;
BEGIN
 IF v<0 THEN RAISE EXCEPTION 'negative lifecycle wire scalar' USING ERRCODE='55000'; END IF;
 WHILE v>=128 LOOP result:=result||decode(lpad(to_hex((v&127)|128),2,'0'),'hex');v:=v>>7;END LOOP;
 RETURN result||decode(lpad(to_hex(v),2,'0'),'hex');
END $$;
CREATE FUNCTION native_membership_guard.terminal_wire_bytes(tag integer,v bytea) RETURNS bytea
LANGUAGE sql IMMUTABLE SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
 SELECT native_membership_guard.terminal_varint(tag*8+2)||native_membership_guard.terminal_varint(octet_length(v))||v
$$;
CREATE FUNCTION native_membership_guard.terminal_event_wire(e public.space_lifecycle_outbox) RETURNS bytea
LANGUAGE plpgsql IMMUTABLE SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE stamp bytea:=''::bytea;seconds bigint;nanos bigint;
BEGIN
 seconds:=floor(extract(epoch FROM e.occurred_at))::bigint;
 nanos:=((extract(epoch FROM e.occurred_at)-seconds)*1000000000)::bigint;
 IF seconds<>0 THEN stamp:=decode('08','hex')||native_membership_guard.terminal_varint(seconds); END IF;
 IF nanos<>0 THEN stamp:=stamp||decode('10','hex')||native_membership_guard.terminal_varint(nanos); END IF;
 RETURN native_membership_guard.terminal_wire_bytes(1,convert_to(e.event_id::text,'UTF8'))
 ||native_membership_guard.terminal_wire_bytes(2,convert_to(e.event_type,'UTF8'))
 ||native_membership_guard.terminal_wire_bytes(3,convert_to(e.space_id::text,'UTF8'))
 ||native_membership_guard.terminal_wire_bytes(4,convert_to(e.deletion_operation_id::text,'UTF8'))
 ||decode('28','hex')||native_membership_guard.terminal_varint(e.generation)
 ||native_membership_guard.terminal_wire_bytes(6,stamp);
END $$;

CREATE FUNCTION public.space_native_terminal_event_delivered(s uuid,id uuid,expected_bytes bytea,expected_sha bytea) RETURNS void
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE e public.space_lifecycle_outbox;representation text;started timestamptz:=clock_timestamp();
BEGIN
 representation:=native_membership_guard.terminal_representation(s);
 SELECT * INTO e FROM public.space_lifecycle_outbox WHERE space_id=s AND event_id=id FOR UPDATE;
 IF NOT FOUND OR e.state NOT IN ('READY','DELIVERED') OR e.event_bytes IS DISTINCT FROM expected_bytes
 OR e.event_sha256 IS DISTINCT FROM expected_sha OR e.event_bytes IS DISTINCT FROM native_membership_guard.terminal_event_wire(e)
 OR e.event_sha256 IS DISTINCT FROM sha256(convert_to('voice.space.store.v1.LifecycleOutboxRecord','UTF8')||decode('00','hex')||e.event_bytes)
 OR (e.event_type='space.deleted' AND (e.purge_decided_at IS NULL OR NOT EXISTS(
  SELECT 1 FROM public.space_permanent_purge_fences f WHERE f.space_id=s AND f.deletion_operation_id=e.deletion_operation_id AND f.purge_generation=e.generation))) THEN
  RAISE EXCEPTION 'terminal event differs from exact saved typed value' USING ERRCODE='55000';
 END IF;
 IF e.state='READY' THEN
  INSERT INTO native_membership_guard.terminal_maintenance_scope VALUES(txid_current(),pg_backend_pid(),s,'ACK',id);
  UPDATE public.space_lifecycle_outbox SET state='DELIVERED',delivered_at=clock_timestamp() WHERE event_id=id;
  DELETE FROM native_membership_guard.terminal_maintenance_scope WHERE xid=txid_current() AND space_id=s;
 END IF;
 PERFORM native_membership_guard.terminal_serving_scope(s);
 IF clock_timestamp()>=started+interval '5 seconds' THEN RAISE EXCEPTION 'terminal event deadline expired' USING ERRCODE='55000'; END IF;
END $$;

CREATE FUNCTION public.space_native_terminal_compact(s uuid) RETURNS boolean
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE representation text;a public.space_lifecycle_aggregates;started timestamptz:=clock_timestamp();
BEGIN
 representation:=native_membership_guard.terminal_representation(s);
 IF representation<>'FULL' THEN RETURN false; END IF;
 SELECT * INTO a FROM public.space_lifecycle_aggregates WHERE space_id=s FOR UPDATE;
 IF a.completed_at IS NULL OR clock_timestamp()<a.completed_at+interval '30 days' THEN RETURN false; END IF;
 -- An unrelated still-replayable outcome may not disappear in this operation.
 IF EXISTS(SELECT 1 FROM public.space_lifecycle_operations WHERE space_id=s
  AND (state<>'COMPLETED' OR completed_at IS NULL OR completed_at+interval '30 days'>clock_timestamp())) THEN
  RAISE EXCEPTION 'terminal compaction would erase unexpired operation' USING ERRCODE='55000';
 END IF;
 INSERT INTO native_membership_guard.terminal_maintenance_scope VALUES(txid_current(),pg_backend_pid(),s,'COMPACT',NULL);
 INSERT INTO public.space_lifecycle_completions(space_id,deletion_operation_id,generation,participant_id,request_kind,request_sha256,receipt_sha256,manifest_sha256,completed_at)
 SELECT space_id,deletion_operation_id,generation,participant_id,request_kind,request_sha256,receipt_sha256,manifest_sha256,completed_at
 FROM public.space_lifecycle_participants WHERE space_id=s AND deletion_operation_id=a.deletion_operation_id AND generation=a.generation
 AND ((participant_id=1 AND request_kind='ROLE_RETIREMENT') OR (participant_id BETWEEN 2 AND 11 AND request_kind='PURGE'));
 IF EXISTS((SELECT space_id,deletion_operation_id,generation,participant_id,request_kind,request_sha256,receipt_sha256,manifest_sha256,completed_at
  FROM public.space_lifecycle_participants WHERE space_id=s AND deletion_operation_id=a.deletion_operation_id AND generation=a.generation
  AND ((participant_id=1 AND request_kind='ROLE_RETIREMENT') OR (participant_id BETWEEN 2 AND 11 AND request_kind='PURGE')))
  EXCEPT SELECT * FROM public.space_lifecycle_completions WHERE space_id=s)
 OR (SELECT count(*) FROM public.space_lifecycle_completions WHERE space_id=s)<>11 THEN
  RAISE EXCEPTION 'terminal compact tuple set mismatch' USING ERRCODE='55000';
 END IF;
 DELETE FROM public.space_lifecycle_participants WHERE space_id=s;
 DELETE FROM public.space_lifecycle_operations WHERE space_id=s AND state='COMPLETED' AND completed_at+interval '30 days'<=clock_timestamp();
 UPDATE public.space_lifecycle_aggregates SET evidence_compacted=true WHERE space_id=s;
 UPDATE native_membership_guard.spaces SET terminal_maintenance_phase='COMPACT' WHERE space_id=s;
 DELETE FROM native_membership_guard.terminal_maintenance_scope WHERE xid=txid_current() AND space_id=s;
 PERFORM native_membership_guard.terminal_representation(s);
 IF clock_timestamp()>=started+interval '5 seconds' THEN RAISE EXCEPTION 'terminal compaction deadline expired' USING ERRCODE='55000'; END IF;
 RETURN true;
END $$;

CREATE FUNCTION public.space_native_terminal_expire_events(s uuid) RETURNS bigint
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE removed bigint;started timestamptz:=clock_timestamp();
BEGIN
 PERFORM native_membership_guard.terminal_representation(s);
 INSERT INTO native_membership_guard.terminal_maintenance_scope VALUES(txid_current(),pg_backend_pid(),s,'EXPIRE_EVENTS',NULL);
 DELETE FROM public.space_lifecycle_outbox WHERE space_id=s AND state='DELIVERED' AND delivered_at+interval '30 days'<=clock_timestamp();
 GET DIAGNOSTICS removed=ROW_COUNT;
 DELETE FROM native_membership_guard.terminal_maintenance_scope WHERE xid=txid_current() AND space_id=s;
 PERFORM native_membership_guard.terminal_representation(s);
 IF clock_timestamp()>=started+interval '5 seconds' THEN RAISE EXCEPTION 'terminal event expiry deadline expired' USING ERRCODE='55000'; END IF;
 RETURN removed;
END $$;

CREATE FUNCTION public.space_native_terminal_expire(s uuid) RETURNS boolean
LANGUAGE plpgsql SECURITY DEFINER SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE representation text;started timestamptz:=clock_timestamp();
BEGIN
 representation:=native_membership_guard.terminal_representation(s);
 IF representation='FULL' THEN
  PERFORM public.space_native_terminal_compact(s);
  representation:=native_membership_guard.terminal_representation(s);
 END IF;
 IF representation='EXPIRED' THEN RETURN false; END IF;
 IF representation<>'COMPACT' THEN RETURN false; END IF;
 PERFORM 1 FROM public.space_deletion_tombstones WHERE space_id=s AND retain_until<=clock_timestamp() AT TIME ZONE 'UTC' FOR UPDATE;
 IF NOT FOUND THEN RETURN false; END IF;
 IF EXISTS(SELECT 1 FROM public.space_lifecycle_operations WHERE space_id=s) THEN
  RAISE EXCEPTION 'terminal expiry would erase operation evidence' USING ERRCODE='55000';
 END IF;
 INSERT INTO native_membership_guard.terminal_maintenance_scope VALUES(txid_current(),pg_backend_pid(),s,'EXPIRE',NULL);
 DELETE FROM public.space_lifecycle_aggregates WHERE space_id=s;
 DELETE FROM public.space_deletion_tombstones WHERE space_id=s;
 UPDATE native_membership_guard.spaces SET terminal_maintenance_phase='EXPIRED' WHERE space_id=s;
 DELETE FROM native_membership_guard.terminal_maintenance_scope WHERE xid=txid_current() AND space_id=s;
 PERFORM native_membership_guard.terminal_representation(s);
 IF clock_timestamp()>=started+interval '5 seconds' THEN RAISE EXCEPTION 'terminal expiry deadline expired' USING ERRCODE='55000'; END IF;
 RETURN true;
END $$;

CREATE OR REPLACE FUNCTION native_membership_guard.source_before() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE s uuid;r jsonb;old_space uuid;p uuid;frame native_membership_guard.frames;maintenance native_membership_guard.terminal_maintenance_scope;
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
 -- A purged evidence representation may change only inside a fixed helper.
 -- This precedes the old terminal DELETE shortcut, including FK cascades.
 IF TG_TABLE_NAME=ANY(ARRAY['space_lifecycle_aggregates','space_lifecycle_operations',
  'space_lifecycle_participants','space_lifecycle_completions','space_lifecycle_outbox','space_deletion_tombstones'])
 AND NOT EXISTS(SELECT 1 FROM public.spaces WHERE id=s)
 AND EXISTS(SELECT 1 FROM public.space_permanent_purge_fences WHERE space_id=s) THEN
  PERFORM native_membership_guard.scope(s);
  SELECT * INTO maintenance FROM native_membership_guard.terminal_maintenance_scope
   WHERE xid=txid_current() AND pid=pg_backend_pid() AND space_id=s;
  IF NOT FOUND THEN RAISE EXCEPTION 'terminal evidence requires fixed certified maintenance' USING ERRCODE='55000'; END IF;
  IF maintenance.operation='ACK' AND TG_TABLE_NAME='space_lifecycle_outbox' AND TG_OP='UPDATE'
   AND (to_jsonb(OLD)->>'event_id')::uuid=maintenance.event_id AND to_jsonb(OLD)->>'state'='READY' AND to_jsonb(NEW)->>'state'='DELIVERED'
   AND to_jsonb(OLD)->>'delivered_at' IS NULL AND to_jsonb(NEW)->>'delivered_at' IS NOT NULL
   AND to_jsonb(NEW)-ARRAY['state','delivered_at']=to_jsonb(OLD)-ARRAY['state','delivered_at'] THEN RETURN NEW;
  ELSIF maintenance.operation='COMPACT' THEN
   IF TG_OP='INSERT' AND TG_TABLE_NAME='space_lifecycle_completions' THEN RETURN NEW; END IF;
   IF TG_OP='DELETE' AND TG_TABLE_NAME IN ('space_lifecycle_participants','space_lifecycle_operations') THEN RETURN OLD; END IF;
   IF TG_OP='UPDATE' AND TG_TABLE_NAME='space_lifecycle_aggregates'
    AND NOT (to_jsonb(OLD)->>'evidence_compacted')::boolean AND (to_jsonb(NEW)->>'evidence_compacted')::boolean
    AND to_jsonb(NEW)-'evidence_compacted'=to_jsonb(OLD)-'evidence_compacted' THEN RETURN NEW; END IF;
  ELSIF maintenance.operation='EXPIRE_EVENTS' AND TG_TABLE_NAME='space_lifecycle_outbox' AND TG_OP='DELETE'
   AND to_jsonb(OLD)->>'state'='DELIVERED' AND (to_jsonb(OLD)->>'delivered_at')::timestamptz+interval '30 days'<=clock_timestamp() THEN RETURN OLD;
  ELSIF maintenance.operation='EXPIRE' AND TG_OP='DELETE'
   AND TG_TABLE_NAME IN ('space_lifecycle_aggregates','space_lifecycle_completions','space_deletion_tombstones') THEN RETURN OLD;
  END IF;
  RAISE EXCEPTION 'write exceeds fixed terminal maintenance operation' USING ERRCODE='55000';
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


CREATE TRIGGER terminal_catalog47_immutable BEFORE UPDATE OR DELETE ON native_membership_guard.catalog47_transition FOR EACH ROW EXECUTE FUNCTION native_membership_guard.bot_global_immutable();
CREATE TRIGGER terminal_catalog47_no_truncate BEFORE TRUNCATE ON native_membership_guard.catalog47_transition FOR EACH STATEMENT EXECUTE FUNCTION native_membership_guard.bot_global_immutable();
REVOKE ALL ON ALL FUNCTIONS IN SCHEMA native_membership_guard FROM PUBLIC;
COMMIT;
