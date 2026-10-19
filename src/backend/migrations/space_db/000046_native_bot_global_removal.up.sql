-- BE170c69/c70 native-only global removal. Installing this closed extension
-- invalidates the retained membership seal; it never promotes that seal.
BEGIN;
DO $$ BEGIN
 IF current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'native global removal installation requires READ COMMITTED' USING ERRCODE='55000';
 END IF;
 IF to_regclass('native_membership_guard.epoch') IS NULL
 OR to_regclass('public.space_bot_installation_cancellations') IS NULL
 OR to_regprocedure('public.guard_join_legacy_selection_immutable()') IS NULL THEN
  RAISE EXCEPTION 'native global removal requires the forward45 cohort' USING ERRCODE='55000';
 END IF;
END $$;
LOCK TABLE public.schema_migrations,native_membership_guard.epoch,
 public.space_bot_installation_attempts,public.spaces IN ACCESS EXCLUSIVE MODE;

-- A permanent application revocation, not a User ownership/association fact.
-- The opaque actor binding includes the random root ID and exact root digest.
CREATE TABLE native_membership_guard.bot_terminals (
 bot_id UUID PRIMARY KEY CHECK(bot_id<>'00000000-0000-0000-0000-000000000000'),
 decision_id UUID NOT NULL UNIQUE CHECK(decision_id<>'00000000-0000-0000-0000-000000000000'),
 decision_sha256 BYTEA NOT NULL CHECK(octet_length(decision_sha256)=32),
 actor_binding_sha256 BYTEA NOT NULL CHECK(octet_length(actor_binding_sha256)=32),
 created_at TIMESTAMPTZ NOT NULL CHECK(isfinite(created_at))
);
CREATE TABLE native_membership_guard.bot_removal_receipts (
 operation_id UUID PRIMARY KEY CHECK(operation_id<>'00000000-0000-0000-0000-000000000000'),
 receipt_id UUID NOT NULL UNIQUE CHECK(receipt_id<>'00000000-0000-0000-0000-000000000000'),
 bot_id UUID NOT NULL REFERENCES native_membership_guard.bot_terminals(bot_id),
 decision_id UUID NOT NULL CHECK(decision_id<>'00000000-0000-0000-0000-000000000000'),
 decision_sha256 BYTEA NOT NULL CHECK(octet_length(decision_sha256)=32),
 space_id UUID NOT NULL CHECK(space_id<>'00000000-0000-0000-0000-000000000000'),
 request_sha256 BYTEA NOT NULL CHECK(octet_length(request_sha256)=32),
 request_bytes BYTEA NOT NULL CHECK(octet_length(request_bytes)>0 AND octet_length(request_bytes)<=65536),
 evidence_sha256 BYTEA NOT NULL CHECK(octet_length(evidence_sha256)=32),
 frozen_scope_sha256 BYTEA NOT NULL CHECK(octet_length(frozen_scope_sha256)=32),
 receipt_bytes BYTEA NOT NULL CHECK(octet_length(receipt_bytes)>0 AND octet_length(receipt_bytes)<=65536),
 completed_at TIMESTAMPTZ NOT NULL CHECK(isfinite(completed_at)),
 CHECK(sha256(request_bytes)=request_sha256)
);
-- A successful trusted transition makes DOWN permanently non-destructive,
-- including when no Bot has yet been removed. No UP-time activation record.
CREATE TABLE native_membership_guard.catalog46_transition (
 singleton BOOLEAN PRIMARY KEY CHECK(singleton),
 epoch UUID NOT NULL,
 migration_sha256 BYTEA NOT NULL CHECK(octet_length(migration_sha256)=32),
 old_seal_sha256 BYTEA NOT NULL CHECK(octet_length(old_seal_sha256)=32),
 new_seal_sha256 BYTEA NOT NULL CHECK(octet_length(new_seal_sha256)=32),
 transitioned_at TIMESTAMPTZ NOT NULL CHECK(isfinite(transitioned_at))
);
REVOKE ALL ON native_membership_guard.bot_terminals,native_membership_guard.bot_removal_receipts,
 native_membership_guard.catalog46_transition FROM PUBLIC;

CREATE FUNCTION native_membership_guard.bot_global_lock(bot UUID) RETURNS VOID
 LANGUAGE plpgsql VOLATILE SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
BEGIN
 IF bot IS NULL OR bot='00000000-0000-0000-0000-000000000000' THEN
  RAISE EXCEPTION 'native Bot identity is required' USING ERRCODE='55000';
 END IF;
 PERFORM pg_advisory_xact_lock(('x'||substr(encode(sha256(
  convert_to('voice.space.bot-global-terminal.v1','UTF8')||decode('00','hex')||uuid_send(bot)),'hex'),1,16))::bit(64)::bigint);
END $$;
REVOKE ALL ON FUNCTION native_membership_guard.bot_global_lock(UUID) FROM PUBLIC;

-- Called after global operation locks and BEFORE any Space fence, including
-- the positive Add replay path. It holds the shared Bot lock until COMMIT.
CREATE FUNCTION public.space_bot_native_terminal(bot UUID) RETURNS BOOLEAN
 LANGUAGE plpgsql VOLATILE SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
BEGIN
 PERFORM native_membership_guard.bot_global_lock(bot);
 RETURN EXISTS(SELECT 1 FROM native_membership_guard.bot_terminals WHERE bot_id=bot);
END $$;

CREATE FUNCTION native_membership_guard.bot_global_immutable() RETURNS trigger
 LANGUAGE plpgsql VOLATILE SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
BEGIN
 IF TG_OP='DELETE' AND TG_TABLE_NAME='bot_removal_receipts' THEN
  IF public.space_membership_terminal_cleanup_allowed(OLD.space_id) THEN RETURN OLD; END IF;
 END IF;
 RAISE EXCEPTION 'native global removal proof is immutable' USING ERRCODE='55000';
END $$;
REVOKE ALL ON FUNCTION native_membership_guard.bot_global_immutable() FROM PUBLIC;
DO $$ DECLARE tab TEXT; BEGIN
 FOREACH tab IN ARRAY ARRAY['bot_terminals','bot_removal_receipts','catalog46_transition'] LOOP
  EXECUTE format('CREATE TRIGGER bot_global_immutable BEFORE UPDATE OR DELETE ON native_membership_guard.%I FOR EACH ROW EXECUTE FUNCTION native_membership_guard.bot_global_immutable()',tab);
  EXECUTE format('CREATE TRIGGER bot_global_no_truncate BEFORE TRUNCATE ON native_membership_guard.%I FOR EACH STATEMENT EXECUTE FUNCTION native_membership_guard.bot_global_immutable()',tab);
 END LOOP;
END $$;

CREATE FUNCTION native_membership_guard.bot_add_terminal_guard() RETURNS trigger
 LANGUAGE plpgsql VOLATILE SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
BEGIN
 PERFORM native_membership_guard.bot_global_lock(NEW.bot_id);
 IF NEW.state='PUBLISHED' AND EXISTS(SELECT 1 FROM native_membership_guard.bot_terminals WHERE bot_id=NEW.bot_id) THEN
  RAISE EXCEPTION 'globally deleted Bot cannot publish a native installation' USING ERRCODE='55000';
 END IF;
 IF NEW.state='PUBLISHED' AND EXISTS(SELECT 1 FROM public.space_bot_installation_attempts
  WHERE bot_id=NEW.bot_id AND profile_id<>NEW.profile_id AND install_operation_id<>NEW.install_operation_id) THEN
  RAISE EXCEPTION 'native Bot actor conflicts with retained installation identity' USING ERRCODE='23505';
 END IF;
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION native_membership_guard.bot_add_terminal_guard() FROM PUBLIC;
CREATE FUNCTION native_membership_guard.bot_terminal_registration_guard() RETURNS trigger
 LANGUAGE plpgsql VOLATILE SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
BEGIN
 PERFORM native_membership_guard.bot_global_lock(NEW.bot_id);
 RETURN NEW;
END $$;
REVOKE ALL ON FUNCTION native_membership_guard.bot_terminal_registration_guard() FROM PUBLIC;
CREATE TRIGGER bot_terminal_registration BEFORE INSERT ON native_membership_guard.bot_terminals
 FOR EACH ROW EXECUTE FUNCTION native_membership_guard.bot_terminal_registration_guard();
ALTER TABLE native_membership_guard.bot_terminals ENABLE ALWAYS TRIGGER bot_terminal_registration;
CREATE TRIGGER a001_bot_global_terminal BEFORE INSERT OR UPDATE ON public.space_bot_installation_attempts
 FOR EACH ROW EXECUTE FUNCTION native_membership_guard.bot_add_terminal_guard();
ALTER TABLE public.space_bot_installation_attempts ENABLE ALWAYS TRIGGER a001_bot_global_terminal;

CREATE FUNCTION public.space_bot_global_removal_saved(op UUID,s UUID) RETURNS BYTEA
 LANGUAGE plpgsql VOLATILE SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE saved BYTEA;
BEGIN
 PERFORM native_membership_guard.scope(s);
 SELECT receipt_bytes INTO saved FROM native_membership_guard.bot_removal_receipts WHERE operation_id=op;
 RETURN saved;
END $$;

CREATE FUNCTION public.space_bot_global_removal_record(
 op UUID,decision UUID,root_sha BYTEA,bot UUID,s UUID,cancellation UUID,installation_op UUID,
 evidence_sha BYTEA,request_raw BYTEA,frozen_sha BYTEA,receipt UUID,result_raw BYTEA,completed TIMESTAMPTZ)
 RETURNS BYTEA LANGUAGE plpgsql VOLATILE SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE actor UUID; actor_binding BYTEA; prior native_membership_guard.bot_removal_receipts; terminal native_membership_guard.bot_terminals;
BEGIN
 PERFORM native_membership_guard.bot_global_lock(bot);
 PERFORM native_membership_guard.scope(s);
 IF op IS NULL OR op='00000000-0000-0000-0000-000000000000'
 OR decision IS NULL OR decision='00000000-0000-0000-0000-000000000000'
 OR receipt IS NULL OR receipt='00000000-0000-0000-0000-000000000000'
 OR octet_length(root_sha) IS DISTINCT FROM 32 OR octet_length(evidence_sha) IS DISTINCT FROM 32
 OR octet_length(frozen_sha) IS DISTINCT FROM 32 OR request_raw IS NULL OR octet_length(request_raw) NOT BETWEEN 1 AND 65536
 OR result_raw IS NULL OR octet_length(result_raw) NOT BETWEEN 1 AND 65536
 OR completed IS NULL OR NOT isfinite(completed) OR completed<transaction_timestamp()
 OR completed>clock_timestamp() OR clock_timestamp()>=completed+interval '5 seconds' THEN
  RAISE EXCEPTION 'invalid native global removal result' USING ERRCODE='55000';
 END IF;
 SELECT * INTO prior FROM native_membership_guard.bot_removal_receipts WHERE operation_id=op;
 IF FOUND THEN
  IF prior.bot_id<>bot OR prior.space_id<>s OR prior.decision_id<>decision OR prior.decision_sha256<>root_sha
   OR prior.request_bytes<>request_raw OR prior.request_sha256<>sha256(request_raw)
   OR prior.evidence_sha256<>evidence_sha OR prior.frozen_scope_sha256<>frozen_sha THEN
   RAISE EXCEPTION 'native global removal child was rebound' USING ERRCODE='55000';
  END IF;
  RETURN prior.receipt_bytes;
 END IF;
 IF NOT EXISTS(SELECT 1 FROM native_membership_guard.frames WHERE xid=txid_current() AND space_id=s AND pid=pg_backend_pid()) THEN
  RAISE EXCEPTION 'native global removal requires its actual mutation frame' USING ERRCODE='55000';
 END IF;
 SELECT a.profile_id INTO actor FROM public.space_bot_installation_attempts a
 JOIN public.space_bot_installation_cancellations c ON c.install_operation_id=a.install_operation_id AND c.space_id=a.space_id
 WHERE c.operation_id=cancellation AND c.install_operation_id=installation_op AND c.space_id=s AND c.request_sha256=evidence_sha
 AND a.bot_id=bot AND a.state='CANCELED';
 IF NOT FOUND OR EXISTS(SELECT 1 FROM public.space_bot_installation_heads WHERE space_id=s AND bot_id=bot AND state='INSTALLED') THEN
  RAISE EXCEPTION 'native global removal requires its exact settled source' USING ERRCODE='55000';
 END IF;
 IF EXISTS(SELECT 1 FROM public.space_bot_installation_attempts WHERE bot_id=bot AND profile_id<>actor) THEN
  RAISE EXCEPTION 'global Bot actor conflicts with retained installation identity' USING ERRCODE='23505';
 END IF;
 actor_binding:=sha256(convert_to('voice.space.bot-global-root-actor.v1','UTF8')||decode('00','hex')||uuid_send(decision)||root_sha||uuid_send(actor));
 INSERT INTO native_membership_guard.bot_terminals(bot_id,decision_id,decision_sha256,actor_binding_sha256,created_at)
 VALUES(bot,decision,root_sha,actor_binding,completed) ON CONFLICT(bot_id) DO NOTHING;
 SELECT * INTO terminal FROM native_membership_guard.bot_terminals WHERE bot_id=bot;
 IF terminal.decision_id<>decision OR terminal.decision_sha256<>root_sha OR terminal.actor_binding_sha256<>actor_binding THEN
  RAISE EXCEPTION 'global Bot removal root or actor conflicts' USING ERRCODE='55000';
 END IF;
 INSERT INTO native_membership_guard.bot_removal_receipts(operation_id,receipt_id,bot_id,decision_id,decision_sha256,space_id,
  request_sha256,request_bytes,evidence_sha256,frozen_scope_sha256,receipt_bytes,completed_at)
 VALUES(op,receipt,bot,decision,root_sha,s,sha256(request_raw),request_raw,evidence_sha,frozen_sha,result_raw,completed);
 RETURN result_raw;
END $$;

CREATE FUNCTION native_membership_guard.bot_receipt_terminal_cleanup() RETURNS trigger
 LANGUAGE plpgsql VOLATILE SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
BEGIN
 IF NOT public.space_membership_terminal_cleanup_allowed(OLD.id) THEN
  RAISE EXCEPTION 'global Bot receipt minimization requires native current11 terminal proof' USING ERRCODE='55000';
 END IF;
 DELETE FROM native_membership_guard.bot_removal_receipts WHERE space_id=OLD.id;
 -- Global application revocation contains no raw Space/profile/account link.
 RETURN OLD;
END $$;
REVOKE ALL ON FUNCTION native_membership_guard.bot_receipt_terminal_cleanup() FROM PUBLIC;
CREATE TRIGGER zzzzzz_bot_global_receipt_terminal AFTER DELETE ON public.spaces
 FOR EACH ROW EXECUTE FUNCTION native_membership_guard.bot_receipt_terminal_cleanup();
COMMIT;
