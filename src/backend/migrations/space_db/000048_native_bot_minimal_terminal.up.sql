-- BOT-TERMINAL-REGISTRATION-v1 BE170c95/c96. No legacy evidence erasure.
BEGIN;
DO $$ BEGIN
 IF current_setting('transaction_isolation')<>'read committed' THEN RAISE EXCEPTION 'minimal48 requires READ COMMITTED' USING ERRCODE='55000'; END IF;
 IF to_regclass('native_membership_guard.catalog47_transition') IS NULL THEN RAISE EXCEPTION 'minimal48 requires exact forward47 storage' USING ERRCODE='55000'; END IF;
END $$;
LOCK TABLE public.schema_migrations,native_membership_guard.epoch,native_membership_guard.bot_terminals,
 native_membership_guard.bot_removal_receipts,public.space_bot_installation_attempts,public.spaces IN ACCESS EXCLUSIVE MODE;
CREATE TABLE native_membership_guard.bot_terminal_registrations (
 bot_id uuid PRIMARY KEY CHECK(bot_id<>'00000000-0000-0000-0000-000000000000'),
 decision_id uuid NOT NULL UNIQUE CHECK(decision_id<>'00000000-0000-0000-0000-000000000000'),
 receipt_id uuid NOT NULL UNIQUE CHECK(receipt_id<>'00000000-0000-0000-0000-000000000000'),
 registered_at timestamptz NOT NULL CHECK(isfinite(registered_at))
);
CREATE TABLE native_membership_guard.bot_removal_receipts48 (
 operation_id UUID PRIMARY KEY CHECK(operation_id<>'00000000-0000-0000-0000-000000000000'),
 receipt_id UUID NOT NULL UNIQUE CHECK(receipt_id<>'00000000-0000-0000-0000-000000000000'),
 bot_id UUID NOT NULL REFERENCES native_membership_guard.bot_terminal_registrations(bot_id),
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
CREATE TABLE native_membership_guard.catalog48_transition (
 singleton boolean PRIMARY KEY CHECK(singleton),epoch uuid NOT NULL,
 migration_sha256 bytea NOT NULL CHECK(octet_length(migration_sha256)=32),
 old_seal_sha256 bytea NOT NULL CHECK(octet_length(old_seal_sha256)=32),
 new_seal_sha256 bytea NOT NULL CHECK(octet_length(new_seal_sha256)=32),
 transitioned_at timestamptz NOT NULL CHECK(isfinite(transitioned_at))
);
REVOKE ALL ON native_membership_guard.bot_terminal_registrations,native_membership_guard.bot_removal_receipts48,native_membership_guard.catalog48_transition FROM PUBLIC;
CREATE FUNCTION native_membership_guard.bot_terminal_scope() RETURNS void LANGUAGE plpgsql VOLATILE SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE role_oid oid;
BEGIN
 IF current_setting('transaction_isolation')<>'read committed' OR EXISTS(SELECT 1 FROM pg_catalog.pg_class WHERE relnamespace=pg_my_temp_schema()) THEN
  RAISE EXCEPTION 'minimal terminal requires unshadowed READ COMMITTED' USING ERRCODE='55000';
 END IF;
 IF NOT EXISTS(SELECT 1 FROM native_membership_guard.epoch WHERE singleton AND epoch='94db7e08-e958-44fa-94b2-988410ab0044' AND catalog_sha256=native_membership_guard.catalog()) THEN
  RAISE EXCEPTION 'minimal terminal catalog is not exact' USING ERRCODE='55000';
 END IF;
 SELECT oid INTO role_oid FROM pg_roles WHERE rolname=session_user AND NOT rolsuper AND NOT rolcreaterole AND NOT rolcreatedb AND NOT rolreplication AND NOT rolbypassrls;
 IF role_oid IS NULL OR has_schema_privilege(session_user,'public','CREATE') OR has_schema_privilege(session_user,'native_membership_guard','CREATE')
 OR EXISTS(SELECT 1 FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
  WHERE ((n.nspname='public' AND c.relname=ANY(native_membership_guard.coverage())) OR n.nspname='native_membership_guard') AND pg_has_role(role_oid,c.relowner,'MEMBER'))
 OR EXISTS(SELECT 1 FROM pg_roles r WHERE (r.rolsuper OR r.rolcreaterole OR r.rolreplication OR r.rolbypassrls) AND pg_has_role(role_oid,r.oid,'MEMBER'))
 OR has_parameter_privilege(session_user,'session_replication_role','SET') THEN
  RAISE EXCEPTION 'minimal terminal serving role can bypass enforcement' USING ERRCODE='55000';
 END IF;
END $$;

CREATE FUNCTION native_membership_guard.bot_terminal_minimal_record(bot uuid,decision uuid)
 RETURNS native_membership_guard.bot_terminal_registrations LANGUAGE plpgsql VOLATILE SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE saved native_membership_guard.bot_terminal_registrations;
BEGIN
 PERFORM native_membership_guard.bot_global_lock(bot);
 IF decision IS NULL OR decision='00000000-0000-0000-0000-000000000000' THEN RAISE EXCEPTION 'minimal terminal decision is required' USING ERRCODE='55000'; END IF;
 IF EXISTS(SELECT 1 FROM native_membership_guard.bot_terminals t WHERE (t.bot_id=bot AND t.decision_id<>decision) OR (t.decision_id=decision AND t.bot_id<>bot)) THEN
  RAISE EXCEPTION 'minimal terminal conflicts with retained root' USING ERRCODE='23505';
 END IF;
 INSERT INTO native_membership_guard.bot_terminal_registrations(bot_id,decision_id,receipt_id,registered_at)
 VALUES(bot,decision,gen_random_uuid(),clock_timestamp()) ON CONFLICT(bot_id) DO NOTHING;
 SELECT * INTO saved FROM native_membership_guard.bot_terminal_registrations WHERE bot_id=bot;
 IF saved.decision_id<>decision THEN RAISE EXCEPTION 'minimal terminal root cannot change' USING ERRCODE='23505'; END IF;
 RETURN saved;
END $$;

CREATE FUNCTION public.space_bot_global_terminal_register(bot uuid,decision uuid)
 RETURNS TABLE(receipt_id uuid,registered_at timestamptz) LANGUAGE plpgsql VOLATILE SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE saved native_membership_guard.bot_terminal_registrations;
BEGIN
 PERFORM native_membership_guard.bot_terminal_scope();
 saved:=native_membership_guard.bot_terminal_minimal_record(bot,decision);
 RETURN QUERY SELECT saved.receipt_id,saved.registered_at;
END $$;

CREATE FUNCTION public.space_bot_global_removal_saved48(op uuid,s uuid) RETURNS bytea
 LANGUAGE plpgsql VOLATILE SECURITY DEFINER SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE saved bytea;
BEGIN
 PERFORM native_membership_guard.scope(s);
 SELECT receipt_bytes INTO saved FROM native_membership_guard.bot_removal_receipts48 WHERE operation_id=op AND space_id=s;
 IF NOT FOUND THEN SELECT receipt_bytes INTO saved FROM native_membership_guard.bot_removal_receipts WHERE operation_id=op AND space_id=s; END IF;
 RETURN saved;
END $$;

CREATE FUNCTION public.space_bot_global_removal_record48(
 op UUID,decision UUID,root_sha BYTEA,bot UUID,s UUID,cancellation UUID,installation_op UUID,
 evidence_sha BYTEA,request_raw BYTEA,frozen_sha BYTEA,receipt UUID,result_raw BYTEA,completed TIMESTAMPTZ)
 RETURNS BYTEA LANGUAGE plpgsql VOLATILE SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
DECLARE actor UUID; actor_binding BYTEA; prior native_membership_guard.bot_removal_receipts48; terminal native_membership_guard.bot_terminals;
BEGIN
 PERFORM native_membership_guard.bot_global_lock(bot);
 PERFORM native_membership_guard.scope(s);
 PERFORM native_membership_guard.bot_terminal_scope();
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
 SELECT * INTO prior FROM native_membership_guard.bot_removal_receipts48 WHERE operation_id=op;
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
 -- Retained46 evidence is still compared while lawfully present. New roots
 -- never create personal-derived hashes in either permanent representation.
 SELECT * INTO terminal FROM native_membership_guard.bot_terminals WHERE bot_id=bot;
 IF FOUND AND (terminal.decision_id<>decision OR terminal.decision_sha256<>root_sha OR terminal.actor_binding_sha256<>
 sha256(convert_to('voice.space.bot-global-root-actor.v1','UTF8')||decode('00','hex')||uuid_send(decision)||root_sha||uuid_send(actor))) THEN
  RAISE EXCEPTION 'retained global root or actor conflicts' USING ERRCODE='55000';
 END IF;
 PERFORM native_membership_guard.bot_terminal_minimal_record(bot,decision);
 INSERT INTO native_membership_guard.bot_removal_receipts48(operation_id,receipt_id,bot_id,decision_id,decision_sha256,space_id,
  request_sha256,request_bytes,evidence_sha256,frozen_scope_sha256,receipt_bytes,completed_at)
 VALUES(op,receipt,bot,decision,root_sha,s,sha256(request_raw),request_raw,evidence_sha,frozen_sha,result_raw,completed);
 RETURN result_raw;
END $$;

CREATE FUNCTION native_membership_guard.bot_minimal48_immutable() RETURNS trigger LANGUAGE plpgsql VOLATILE SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
BEGIN
 IF TG_OP='DELETE' AND TG_TABLE_NAME='bot_removal_receipts48' THEN
  IF public.space_membership_terminal_cleanup_allowed(OLD.space_id) THEN RETURN OLD; END IF;
 END IF;
 RAISE EXCEPTION 'minimal terminal registration is immutable' USING ERRCODE='55000';
END $$;

CREATE FUNCTION native_membership_guard.bot_receipt48_terminal_cleanup() RETURNS trigger LANGUAGE plpgsql VOLATILE SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
BEGIN
 IF NOT public.space_membership_terminal_cleanup_allowed(OLD.id) THEN RAISE EXCEPTION 'minimal48 source evidence requires current11 terminal proof' USING ERRCODE='55000'; END IF;
 DELETE FROM native_membership_guard.bot_removal_receipts48 WHERE space_id=OLD.id;
 RETURN OLD;
END $$;

CREATE OR REPLACE FUNCTION public.space_bot_native_terminal(bot UUID) RETURNS BOOLEAN
 LANGUAGE plpgsql VOLATILE SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
BEGIN
 PERFORM native_membership_guard.bot_global_lock(bot);
 RETURN EXISTS(SELECT 1 FROM native_membership_guard.bot_terminals WHERE bot_id=bot) OR EXISTS(SELECT 1 FROM native_membership_guard.bot_terminal_registrations WHERE bot_id=bot);
END $$;

CREATE OR REPLACE FUNCTION native_membership_guard.bot_add_terminal_guard() RETURNS trigger
 LANGUAGE plpgsql VOLATILE SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
BEGIN
 PERFORM native_membership_guard.bot_global_lock(NEW.bot_id);
 IF NEW.state='PUBLISHED' AND (EXISTS(SELECT 1 FROM native_membership_guard.bot_terminals WHERE bot_id=NEW.bot_id) OR EXISTS(SELECT 1 FROM native_membership_guard.bot_terminal_registrations WHERE bot_id=NEW.bot_id)) THEN
  RAISE EXCEPTION 'globally deleted Bot cannot publish a native installation' USING ERRCODE='55000';
 END IF;
 IF NEW.state='PUBLISHED' AND EXISTS(SELECT 1 FROM public.space_bot_installation_attempts
  WHERE bot_id=NEW.bot_id AND profile_id<>NEW.profile_id AND install_operation_id<>NEW.install_operation_id) THEN
  RAISE EXCEPTION 'native Bot actor conflicts with retained installation identity' USING ERRCODE='23505';
 END IF;
 RETURN NEW;
END $$;

CREATE OR REPLACE FUNCTION public.space_bot_global_removal_record(
 op UUID,decision UUID,root_sha BYTEA,bot UUID,s UUID,cancellation UUID,installation_op UUID,
 evidence_sha BYTEA,request_raw BYTEA,frozen_sha BYTEA,receipt UUID,result_raw BYTEA,completed TIMESTAMPTZ)
 RETURNS BYTEA LANGUAGE plpgsql VOLATILE SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
BEGIN
 RAISE EXCEPTION 'legacy full writer is closed on minimal48' USING ERRCODE='55000';
END $$;
DO $$ DECLARE tab text;BEGIN
 FOREACH tab IN ARRAY ARRAY['bot_terminal_registrations','bot_removal_receipts48','catalog48_transition'] LOOP
  EXECUTE format('CREATE TRIGGER bot_minimal48_immutable BEFORE UPDATE OR DELETE ON native_membership_guard.%I FOR EACH ROW EXECUTE FUNCTION native_membership_guard.bot_minimal48_immutable()',tab);
  EXECUTE format('CREATE TRIGGER bot_minimal48_no_truncate BEFORE TRUNCATE ON native_membership_guard.%I FOR EACH STATEMENT EXECUTE FUNCTION native_membership_guard.bot_minimal48_immutable()',tab);
 END LOOP;
END $$;
CREATE TRIGGER bot_terminal48_registration BEFORE INSERT ON native_membership_guard.bot_terminal_registrations FOR EACH ROW EXECUTE FUNCTION native_membership_guard.bot_terminal_registration_guard();
ALTER TABLE native_membership_guard.bot_terminal_registrations ENABLE ALWAYS TRIGGER bot_terminal48_registration;
CREATE TRIGGER zzzzzzz_bot_minimal48_receipt_terminal AFTER DELETE ON public.spaces FOR EACH ROW EXECUTE FUNCTION native_membership_guard.bot_receipt48_terminal_cleanup();
REVOKE ALL ON FUNCTION native_membership_guard.bot_terminal_scope() FROM PUBLIC;
REVOKE ALL ON FUNCTION native_membership_guard.bot_terminal_minimal_record(uuid,uuid) FROM PUBLIC;
REVOKE ALL ON FUNCTION native_membership_guard.bot_minimal48_immutable() FROM PUBLIC;
REVOKE ALL ON FUNCTION native_membership_guard.bot_receipt48_terminal_cleanup() FROM PUBLIC;
COMMIT;
