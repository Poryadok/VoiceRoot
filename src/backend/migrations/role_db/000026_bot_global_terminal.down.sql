BEGIN;
-- Reject stale snapshots BEFORE locks or emptiness checks. Never remove a
-- committed minimal terminal predicate/result or weaken published25 denial.
DO $$ BEGIN
 IF current_setting('transaction_isolation') <> 'read committed' THEN
  RAISE EXCEPTION 'Role terminal rollback requires READ COMMITTED' USING ERRCODE='55000';
 END IF;
END $$;
LOCK TABLE public.role_bot_global_terminals IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM public.role_bot_global_terminals) THEN
  RAISE EXCEPTION 'Role terminal evidence prevents rollback' USING ERRCODE='55000';
 END IF;
END $$;
DROP TABLE public.role_bot_global_terminals;
-- Published25 INVOKER function and all old evidence remain unchanged.
COMMIT;
