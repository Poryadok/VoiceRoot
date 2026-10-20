BEGIN;
DO $$ BEGIN IF current_setting('transaction_isolation')<>'read committed' THEN RAISE EXCEPTION 'minimal48 DOWN requires READ COMMITTED before locking' USING ERRCODE='55000'; END IF; END $$;
LOCK TABLE public.schema_migrations,native_membership_guard.epoch,public.space_bot_installation_attempts,public.spaces,
 native_membership_guard.bot_terminals,native_membership_guard.bot_removal_receipts,native_membership_guard.bot_terminal_registrations,
 native_membership_guard.bot_removal_receipts48,native_membership_guard.catalog48_transition IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM native_membership_guard.bot_terminal_registrations) OR EXISTS(SELECT 1 FROM native_membership_guard.bot_removal_receipts48) OR EXISTS(SELECT 1 FROM native_membership_guard.catalog48_transition) THEN
  RAISE EXCEPTION 'minimal48 retained terminal/result/activation cannot be downgraded' USING ERRCODE='55000';
 END IF;
END $$;
DROP TRIGGER zzzzzzz_bot_minimal48_receipt_terminal ON public.spaces;
DROP FUNCTION native_membership_guard.bot_receipt48_terminal_cleanup();
DROP FUNCTION public.space_bot_global_removal_record48(uuid,uuid,bytea,uuid,uuid,uuid,uuid,bytea,bytea,bytea,uuid,bytea,timestamptz);
DROP FUNCTION public.space_bot_global_removal_saved48(uuid,uuid);
DROP FUNCTION public.space_bot_global_terminal_register(uuid,uuid);
DROP FUNCTION native_membership_guard.bot_terminal_minimal_record(uuid,uuid);
DROP FUNCTION native_membership_guard.bot_terminal_scope();
DROP TABLE native_membership_guard.bot_removal_receipts48,native_membership_guard.bot_terminal_registrations,native_membership_guard.catalog48_transition;
DROP FUNCTION native_membership_guard.bot_minimal48_immutable();
DO $restore$ BEGIN EXECUTE $oldbody$CREATE OR REPLACE FUNCTION public.space_bot_native_terminal(bot UUID) RETURNS BOOLEAN
 LANGUAGE plpgsql VOLATILE SECURITY DEFINER
 SET search_path=pg_catalog,public,native_membership_guard,pg_temp AS $$
BEGIN
 PERFORM native_membership_guard.bot_global_lock(bot);
 RETURN EXISTS(SELECT 1 FROM native_membership_guard.bot_terminals WHERE bot_id=bot);
END $$;$oldbody$; END $restore$;
DO $restore$ BEGIN EXECUTE $oldbody$CREATE OR REPLACE FUNCTION native_membership_guard.bot_add_terminal_guard() RETURNS trigger
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
END $$;$oldbody$; END $restore$;
DO $restore$ BEGIN EXECUTE $oldbody$CREATE OR REPLACE FUNCTION public.space_bot_global_removal_record(
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
END $$;$oldbody$; END $restore$;
COMMIT;
