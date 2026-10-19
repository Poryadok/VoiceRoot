BEGIN;
-- A preexisting repeatable snapshot cannot be made current by table locks.
DO $$ BEGIN
 IF current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'Bot journal rollback requires READ COMMITTED before locking' USING ERRCODE='55000';
 END IF;
END $$;
LOCK TABLE public.space_bot_installation_heads,public.space_bot_installation_attempts,
 public.space_bot_installation_cancellations,public.space_native_member_publications IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM public.space_bot_installation_heads)
 OR EXISTS(SELECT 1 FROM public.space_bot_installation_attempts)
 OR EXISTS(SELECT 1 FROM public.space_bot_installation_cancellations)
 OR EXISTS(SELECT 1 FROM public.space_native_member_publications WHERE kind=6) THEN
  RAISE EXCEPTION 'retained Bot installation floors or native provenance forbid rollback' USING ERRCODE='55000';
 END IF;
END $$;
DROP TRIGGER zzzz_space_bot_installation_terminal_cleanup ON public.spaces;
DROP TABLE public.space_bot_installation_heads,public.space_bot_installation_cancellations,public.space_bot_installation_attempts;
DROP FUNCTION public.space_bot_installation_terminal_cleanup(),public.space_bot_installation_cancellation_guard(),
 public.space_bot_installation_head_guard(),public.space_bot_installation_attempt_guard(),public.space_bot_installation_liveness_guard();
COMMIT;
