BEGIN;
-- Reject stale snapshots BEFORE destructive lock acquisition. Table locks
-- cannot refresh an earlier REPEATABLE READ empty-table observation.
DO $$ BEGIN
 IF current_setting('transaction_isolation')<>'read committed' THEN
  RAISE EXCEPTION 'native global removal rollback requires READ COMMITTED' USING ERRCODE='55000';
 END IF;
END $$;
LOCK TABLE native_membership_guard.bot_terminals,native_membership_guard.bot_removal_receipts,
 native_membership_guard.catalog46_transition,public.space_bot_installation_attempts,public.spaces IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM native_membership_guard.bot_terminals)
 OR EXISTS(SELECT 1 FROM native_membership_guard.bot_removal_receipts)
 OR EXISTS(SELECT 1 FROM native_membership_guard.catalog46_transition) THEN
  RAISE EXCEPTION 'retained native global removal or activation proof forbids rollback' USING ERRCODE='55000';
 END IF;
END $$;
DROP TRIGGER zzzzzz_bot_global_receipt_terminal ON public.spaces;
DROP TRIGGER a001_bot_global_terminal ON public.space_bot_installation_attempts;
DROP TABLE native_membership_guard.bot_removal_receipts,native_membership_guard.bot_terminals,
 native_membership_guard.catalog46_transition;
DROP FUNCTION native_membership_guard.bot_receipt_terminal_cleanup(),native_membership_guard.bot_add_terminal_guard(),
 native_membership_guard.bot_terminal_registration_guard(),native_membership_guard.bot_global_immutable(),
 public.space_bot_global_removal_saved(UUID,UUID),
 public.space_bot_global_removal_record(UUID,UUID,BYTEA,UUID,UUID,UUID,UUID,BYTEA,BYTEA,BYTEA,UUID,BYTEA,TIMESTAMPTZ),
 public.space_bot_native_terminal(UUID),native_membership_guard.bot_global_lock(UUID);
-- Deliberately preserve the retained epoch seal. No automatic45 reseal.
COMMIT;
