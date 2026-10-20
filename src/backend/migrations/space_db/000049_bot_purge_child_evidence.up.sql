-- BE170c90/c114: own Space P3 child evidence, not a twelfth participant.
BEGIN;
DO $$ BEGIN
 IF current_setting('transaction_isolation')<>'read committed' THEN RAISE EXCEPTION 'P3 child49 requires READ COMMITTED' USING ERRCODE='55000'; END IF;
 IF to_regclass('native_membership_guard.catalog48_transition') IS NULL THEN RAISE EXCEPTION 'P3 child49 requires exact48 storage' USING ERRCODE='55000'; END IF;
END $$;
LOCK TABLE public.schema_migrations,native_membership_guard.epoch,public.spaces,public.space_lifecycle_aggregates,public.space_lifecycle_participants IN ACCESS EXCLUSIVE MODE;
CREATE TABLE native_membership_guard.bot_purge_children (
 operation_id uuid PRIMARY KEY CHECK(operation_id<>'00000000-0000-0000-0000-000000000000'),
 receipt_id uuid NOT NULL UNIQUE CHECK(receipt_id<>'00000000-0000-0000-0000-000000000000'),
 space_id uuid NOT NULL CHECK(space_id<>'00000000-0000-0000-0000-000000000000'),
 deletion_operation_id uuid NOT NULL CHECK(deletion_operation_id<>'00000000-0000-0000-0000-000000000000'),
 generation bigint NOT NULL CHECK(generation>0),
 manifest_id uuid NOT NULL CHECK(manifest_id<>'00000000-0000-0000-0000-000000000000'),
 manifest_sha256 bytea NOT NULL CHECK(octet_length(manifest_sha256)=32),
 manifest_item_count bigint NOT NULL CHECK(manifest_item_count>=0),
 request_bytes bytea NOT NULL CHECK(octet_length(request_bytes) BETWEEN 1 AND 65536),
 request_sha256 bytea NOT NULL CHECK(octet_length(request_sha256)=32 AND request_sha256=sha256(request_bytes)),
 receipt_bytes bytea NOT NULL CHECK(octet_length(receipt_bytes) BETWEEN 1 AND 65536),
 receipt_sha256 bytea NOT NULL CHECK(octet_length(receipt_sha256)=32 AND receipt_sha256=sha256(receipt_bytes)),
 completed_at timestamptz NOT NULL CHECK(isfinite(completed_at)),
 parent_completed_at timestamptz CHECK(isfinite(parent_completed_at) AND parent_completed_at>=completed_at),
 CHECK(operation_id<>deletion_operation_id)
);
CREATE INDEX bot_purge_children_space ON native_membership_guard.bot_purge_children(space_id);
CREATE TABLE native_membership_guard.catalog49_transition (
 singleton boolean PRIMARY KEY CHECK(singleton),epoch uuid NOT NULL,
 migration_sha256 bytea NOT NULL CHECK(octet_length(migration_sha256)=32),
 old_seal_sha256 bytea NOT NULL CHECK(octet_length(old_seal_sha256)=32),
 new_seal_sha256 bytea NOT NULL CHECK(octet_length(new_seal_sha256)=32),
 transitioned_at timestamptz NOT NULL CHECK(isfinite(transitioned_at))
);
REVOKE ALL ON native_membership_guard.bot_purge_children,native_membership_guard.catalog49_transition FROM PUBLIC;

CREATE FUNCTION native_membership_guard.bot_purge_child_parent(s uuid,deletion uuid,gen bigint,manifest uuid,manifest_sha bytea,items bigint) RETURNS boolean
 LANGUAGE sql STABLE SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
 SELECT EXISTS(SELECT 1 FROM public.space_lifecycle_aggregates a
 JOIN public.space_lifecycle_operations o ON o.operation_id=a.deletion_operation_id AND o.space_id=a.space_id
 WHERE a.space_id=s AND a.deletion_operation_id=deletion AND a.generation=gen
 AND a.manifest_id=manifest AND a.manifest_sha256=manifest_sha AND a.manifest_item_count=items
 AND a.phase='PURGING' AND a.purge_decided_at IS NOT NULL AND a.completed_at IS NULL AND NOT a.local_purge_completed
 AND o.method='DELETE' AND o.auth_receipt_bytes IS NOT NULL AND octet_length(o.auth_receipt_bytes)>0
 AND EXISTS(SELECT 1 FROM public.space_lifecycle_participants p WHERE p.space_id=s AND p.deletion_operation_id=deletion
  AND p.generation=gen AND p.participant_id=1 AND p.request_kind='ROLE_RETIREMENT' AND p.progress='COMPLETE'
  AND p.receipt_bytes IS NOT NULL AND octet_length(p.receipt_sha256)=32
  AND p.manifest_id=manifest AND p.manifest_sha256=manifest_sha AND p.manifest_item_count=items))
$$;

CREATE FUNCTION native_membership_guard.bot_purge_child_immutable() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
BEGIN
 IF TG_TABLE_NAME='bot_purge_children' AND TG_OP IN ('INSERT','UPDATE','DELETE') THEN
  IF TG_OP='INSERT' THEN
   PERFORM native_membership_guard.scope(NEW.space_id);
   PERFORM native_membership_guard.bot_terminal_scope();
   IF NEW.parent_completed_at IS NULL
    AND native_membership_guard.bot_purge_child_parent(NEW.space_id,NEW.deletion_operation_id,NEW.generation,NEW.manifest_id,NEW.manifest_sha256,NEW.manifest_item_count)
    AND EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE xid=txid_current() AND pid=pg_backend_pid() AND space_id=NEW.space_id)
    AND NEW.completed_at>=transaction_timestamp() AND NEW.completed_at<=clock_timestamp() THEN RETURN NEW; END IF;
  ELSIF TG_OP='UPDATE' THEN
   PERFORM native_membership_guard.scope(OLD.space_id);
   IF OLD.parent_completed_at IS NULL AND NEW.parent_completed_at IS NOT NULL
    AND (to_jsonb(NEW)-'parent_completed_at')=(to_jsonb(OLD)-'parent_completed_at')
    AND public.space_membership_terminal_cleanup_allowed(OLD.space_id)
    AND EXISTS(SELECT 1 FROM public.space_lifecycle_aggregates a WHERE a.space_id=OLD.space_id
     AND a.deletion_operation_id=OLD.deletion_operation_id AND a.generation=OLD.generation
     AND a.manifest_id=OLD.manifest_id AND a.manifest_sha256=OLD.manifest_sha256 AND a.manifest_item_count=OLD.manifest_item_count
     AND a.phase='PURGED' AND a.local_purge_completed AND a.completed_at=NEW.parent_completed_at)
    AND EXISTS(SELECT 1 FROM public.space_permanent_purge_fences f WHERE f.space_id=OLD.space_id
     AND f.deletion_operation_id=OLD.deletion_operation_id AND f.purge_generation=OLD.generation AND f.purged_at=NEW.parent_completed_at) THEN RETURN NEW; END IF;
  ELSE
   PERFORM native_membership_guard.scope(OLD.space_id);
   PERFORM native_membership_guard.bot_terminal_scope();
   IF OLD.parent_completed_at IS NOT NULL AND OLD.parent_completed_at+interval '30 days'<=clock_timestamp()
    AND EXISTS(SELECT 1 FROM public.space_permanent_purge_fences f WHERE f.space_id=OLD.space_id
     AND f.deletion_operation_id=OLD.deletion_operation_id AND f.purge_generation=OLD.generation AND f.purged_at=OLD.parent_completed_at)
    AND EXISTS(SELECT 1 FROM native_membership_guard.spaces r WHERE r.space_id=OLD.space_id AND r.state='TERMINAL') THEN RETURN OLD; END IF;
  END IF;
 END IF;
 RAISE EXCEPTION 'P3 native child evidence cannot be rebound, refreshed, or erased early' USING ERRCODE='55000';
END $$;

CREATE FUNCTION native_membership_guard.bot_purge_child_parent_completed() RETURNS trigger LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE parent public.space_lifecycle_aggregates;
BEGIN
 IF TG_TABLE_NAME='space_lifecycle_aggregates' THEN
  IF TG_OP='UPDATE' THEN
   IF OLD.evidence_compacted OR NOT NEW.evidence_compacted THEN RETURN NEW; END IF;
  END IF;
  -- The existing fixed terminal helpers cannot compact/expire a parent and
  -- strand its child payload. This is not a terminal bypass: both the old
  -- source guard and the exact child deadline/provenance checks still run.
  PERFORM public.space_bot_purge_child_expire(OLD.space_id);
  IF EXISTS(SELECT 1 FROM native_membership_guard.bot_purge_children WHERE space_id=OLD.space_id) THEN
   RAISE EXCEPTION 'P3 parent cannot outlive unexpired or unknown child evidence' USING ERRCODE='55000';
  END IF;
  IF TG_OP='DELETE' THEN RETURN OLD; ELSE RETURN NEW; END IF;
 END IF;
 IF NOT public.space_membership_terminal_cleanup_allowed(OLD.id) THEN RAISE EXCEPTION 'P3 native child requires actual current11 terminal provenance' USING ERRCODE='55000'; END IF;
 SELECT * INTO STRICT parent FROM public.space_lifecycle_aggregates WHERE space_id=OLD.id;
 IF parent.phase<>'PURGED' OR NOT parent.local_purge_completed OR parent.completed_at IS NULL
  OR EXISTS(SELECT 1 FROM native_membership_guard.bot_purge_children c WHERE c.space_id=OLD.id AND
   (c.deletion_operation_id<>parent.deletion_operation_id OR c.generation<>parent.generation
    OR c.manifest_id IS DISTINCT FROM parent.manifest_id OR c.manifest_sha256 IS DISTINCT FROM parent.manifest_sha256
    OR c.manifest_item_count IS DISTINCT FROM parent.manifest_item_count OR c.parent_completed_at IS NOT NULL)) THEN
  RAISE EXCEPTION 'P3 child parent differs from actual first terminal transition' USING ERRCODE='55000';
 END IF;
 UPDATE native_membership_guard.bot_purge_children SET parent_completed_at=parent.completed_at WHERE space_id=OLD.id;
 RETURN OLD;
END $$;

CREATE FUNCTION public.space_bot_purge_child_saved(op uuid,s uuid,request_raw bytea) RETURNS bytea LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE saved native_membership_guard.bot_purge_children;
BEGIN
 PERFORM native_membership_guard.scope(s);
 PERFORM native_membership_guard.bot_terminal_scope();
 SELECT * INTO saved FROM native_membership_guard.bot_purge_children WHERE operation_id=op;
 IF NOT FOUND THEN RETURN NULL; END IF;
 IF saved.space_id<>s OR saved.request_bytes<>request_raw OR saved.request_sha256<>sha256(request_raw) THEN
  RAISE EXCEPTION 'P3 native child operation was rebound' USING ERRCODE='23505';
 END IF;
 IF saved.receipt_sha256<>sha256(saved.receipt_bytes) THEN RAISE EXCEPTION 'P3 native child result corrupt' USING ERRCODE='55000'; END IF;
 IF saved.parent_completed_at IS NULL THEN
  IF NOT EXISTS(SELECT 1 FROM native_membership_guard.spaces r WHERE r.space_id=s AND r.state='COMPLETE')
   OR NOT native_membership_guard.bot_purge_child_parent(s,saved.deletion_operation_id,saved.generation,saved.manifest_id,saved.manifest_sha256,saved.manifest_item_count) THEN
   RAISE EXCEPTION 'P3 native child pending parent unavailable' USING ERRCODE='55000';
  END IF;
 ELSE
  IF saved.parent_completed_at+interval '30 days'<=clock_timestamp()
   OR NOT EXISTS(SELECT 1 FROM native_membership_guard.spaces r WHERE r.space_id=s AND r.state='TERMINAL')
   OR NOT EXISTS(SELECT 1 FROM public.space_permanent_purge_fences f WHERE f.space_id=s
    AND f.deletion_operation_id=saved.deletion_operation_id AND f.purge_generation=saved.generation AND f.purged_at=saved.parent_completed_at)
   OR NOT EXISTS(SELECT 1 FROM public.space_lifecycle_aggregates a WHERE a.space_id=s AND a.phase='PURGED' AND a.local_purge_completed
    AND a.deletion_operation_id=saved.deletion_operation_id AND a.generation=saved.generation AND a.completed_at=saved.parent_completed_at
    AND a.manifest_id=saved.manifest_id AND a.manifest_sha256=saved.manifest_sha256 AND a.manifest_item_count=saved.manifest_item_count) THEN
   RAISE EXCEPTION 'P3 native child full evidence unavailable' USING ERRCODE='55000';
  END IF;
 END IF;
 RETURN saved.receipt_bytes;
END $$;

CREATE FUNCTION public.space_bot_purge_child_record(op uuid,s uuid,deletion uuid,gen bigint,manifest uuid,manifest_sha bytea,items bigint,
 bot uuid,installation uuid,actor uuid,installation_op uuid,cancellation uuid,botgen bigint,native_sha bytea,
 request_raw bytea,receipt uuid,result_raw bytea,completed timestamptz) RETURNS bytea LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE prior bytea;
BEGIN
 PERFORM native_membership_guard.scope(s);
 PERFORM native_membership_guard.bot_terminal_scope();
 prior:=public.space_bot_purge_child_saved(op,s,request_raw);
 IF prior IS NOT NULL THEN RETURN prior; END IF;
 IF op IS NULL OR op IN (deletion,installation_op,cancellation) OR receipt IS NULL
  OR completed IS NULL OR completed<transaction_timestamp() OR completed>clock_timestamp()
  OR clock_timestamp()>=completed+interval '5 seconds'
  OR NOT native_membership_guard.bot_purge_child_parent(s,deletion,gen,manifest,manifest_sha,items)
  OR NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE xid=txid_current() AND pid=pg_backend_pid() AND space_id=s)
  OR NOT EXISTS(SELECT 1 FROM public.space_bot_installation_attempts a
   JOIN public.space_bot_installation_cancellations c ON c.space_id=a.space_id AND c.install_operation_id=a.install_operation_id
   WHERE a.space_id=s AND a.install_operation_id=installation_op AND a.bot_id=bot AND a.installation_id=installation
    AND a.profile_id=actor AND a.generation=botgen AND a.state='CANCELED' AND c.operation_id=cancellation AND c.request_sha256=native_sha)
  OR EXISTS(SELECT 1 FROM public.space_bot_installation_attempts WHERE install_operation_id=op)
  OR EXISTS(SELECT 1 FROM public.space_bot_installation_cancellations WHERE operation_id=op) THEN
  RAISE EXCEPTION 'P3 native child requires exact actual settled source and parent' USING ERRCODE='55000';
 END IF;
 INSERT INTO native_membership_guard.bot_purge_children(operation_id,receipt_id,space_id,deletion_operation_id,generation,manifest_id,manifest_sha256,manifest_item_count,
  request_bytes,request_sha256,receipt_bytes,receipt_sha256,completed_at)
 VALUES(op,receipt,s,deletion,gen,manifest,manifest_sha,items,request_raw,sha256(request_raw),result_raw,sha256(result_raw),completed);
 RETURN result_raw;
END $$;

CREATE FUNCTION public.space_bot_purge_child_expire(s uuid) RETURNS bigint LANGUAGE plpgsql SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE erased bigint;
BEGIN
 PERFORM native_membership_guard.scope(s);
 PERFORM native_membership_guard.bot_terminal_scope();
 DELETE FROM native_membership_guard.bot_purge_children WHERE space_id=s AND parent_completed_at IS NOT NULL AND parent_completed_at+interval '30 days'<=clock_timestamp();
 GET DIAGNOSTICS erased=ROW_COUNT;
 RETURN erased;
END $$;

CREATE TRIGGER bot_purge49_immutable BEFORE INSERT OR UPDATE OR DELETE ON native_membership_guard.bot_purge_children FOR EACH ROW EXECUTE FUNCTION native_membership_guard.bot_purge_child_immutable();
CREATE TRIGGER bot_purge49_no_truncate BEFORE TRUNCATE ON native_membership_guard.bot_purge_children FOR EACH STATEMENT EXECUTE FUNCTION native_membership_guard.bot_purge_child_immutable();
CREATE TRIGGER bot_purge49_transition_immutable BEFORE UPDATE OR DELETE ON native_membership_guard.catalog49_transition FOR EACH ROW EXECUTE FUNCTION native_membership_guard.bot_purge_child_immutable();
CREATE TRIGGER bot_purge49_transition_no_truncate BEFORE TRUNCATE ON native_membership_guard.catalog49_transition FOR EACH STATEMENT EXECUTE FUNCTION native_membership_guard.bot_purge_child_immutable();
CREATE TRIGGER zzzzzzzz_bot_purge49_parent_terminal AFTER DELETE ON public.spaces FOR EACH ROW EXECUTE FUNCTION native_membership_guard.bot_purge_child_parent_completed();
CREATE TRIGGER zzzzzzzz_bot_purge49_parent_minimize BEFORE UPDATE OR DELETE ON public.space_lifecycle_aggregates FOR EACH ROW EXECUTE FUNCTION native_membership_guard.bot_purge_child_parent_completed();
REVOKE ALL ON FUNCTION native_membership_guard.bot_purge_child_parent(uuid,uuid,bigint,uuid,bytea,bigint) FROM PUBLIC;
REVOKE ALL ON FUNCTION native_membership_guard.bot_purge_child_immutable() FROM PUBLIC;
REVOKE ALL ON FUNCTION native_membership_guard.bot_purge_child_parent_completed() FROM PUBLIC;
COMMIT;
