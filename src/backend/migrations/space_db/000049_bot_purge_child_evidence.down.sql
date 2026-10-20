BEGIN;
DO $$ BEGIN
 IF current_setting('transaction_isolation')<>'read committed' THEN RAISE EXCEPTION 'P3 child49 DOWN requires READ COMMITTED before locks' USING ERRCODE='55000'; END IF;
END $$;
LOCK TABLE public.schema_migrations,native_membership_guard.epoch,public.spaces,public.space_lifecycle_aggregates,native_membership_guard.bot_purge_children,native_membership_guard.catalog49_transition IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM native_membership_guard.bot_purge_children) OR EXISTS(SELECT 1 FROM native_membership_guard.catalog49_transition) THEN
  RAISE EXCEPTION 'P3 child49 DOWN cannot erase retained result or admitted catalog' USING ERRCODE='55000';
 END IF;
END $$;
DROP TRIGGER zzzzzzzz_bot_purge49_parent_terminal ON public.spaces;
DROP TRIGGER zzzzzzzz_bot_purge49_parent_minimize ON public.space_lifecycle_aggregates;
DROP TABLE native_membership_guard.bot_purge_children;
DROP TABLE native_membership_guard.catalog49_transition;
DROP FUNCTION public.space_bot_purge_child_saved(uuid,uuid,bytea);
DROP FUNCTION public.space_bot_purge_child_record(uuid,uuid,uuid,bigint,uuid,bytea,bigint,uuid,uuid,uuid,uuid,uuid,bigint,bytea,bytea,uuid,bytea,timestamptz);
DROP FUNCTION public.space_bot_purge_child_expire(uuid);
DROP FUNCTION native_membership_guard.bot_purge_child_parent(uuid,uuid,bigint,uuid,bytea,bigint);
DROP FUNCTION native_membership_guard.bot_purge_child_immutable();
DROP FUNCTION native_membership_guard.bot_purge_child_parent_completed();
COMMIT;
