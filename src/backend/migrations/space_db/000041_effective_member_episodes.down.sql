-- A retained episode/floor is privacy authority, not disposable cache data.
-- One atomic statement also leaves a failed migration connection usable for
-- the pinned driver's advisory unlock; it never commits an enclosing caller TX.
DO $migration$ BEGIN
-- Table locks cannot refresh an inherited transaction snapshot. Refuse it
-- before waiting, otherwise its old empty view could discard a committed head.
IF current_setting('transaction_isolation') <> 'read committed' THEN
 RAISE EXCEPTION 'effective membership rollback requires READ COMMITTED' USING ERRCODE='55000';
END IF;
-- Serialize with a first writer BEFORE observing emptiness. Holding only the
-- later DROP lock permits a writer to commit between the check and destruction.
LOCK TABLE public.spaces,public.space_effective_member_heads IN ACCESS EXCLUSIVE MODE;
 IF EXISTS(SELECT 1 FROM space_effective_member_heads) THEN
  RAISE EXCEPTION 'effective membership history requires roll-forward migration' USING ERRCODE='55000';
 END IF;
DROP TRIGGER zz_space_effective_membership_terminal_cleanup ON public.spaces;
DROP TABLE public.space_effective_member_heads;
DROP FUNCTION public.space_effective_member_head_guard();
DROP FUNCTION public.space_effective_member_liveness_guard();
DROP FUNCTION public.space_effective_membership_terminal_cleanup();
END $migration$;
