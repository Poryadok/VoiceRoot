-- Preserve canonical v13 grants and every saved lifecycle receipt.
DO $$
DECLARE
 name TEXT;
BEGIN
 LOCK TABLE public.role_space_deletion_fences,public.role_space_deletion_fence_receipts IN ACCESS EXCLUSIVE MODE;
 FOREACH name IN ARRAY ARRAY['role_space_deletion_fences','role_space_deletion_fence_receipts'] LOOP
  IF NOT EXISTS(SELECT 1 FROM pg_class WHERE oid=to_regclass('public.'||name) AND relkind='r' AND relpersistence='p' AND NOT relrowsecurity AND NOT relforcerowsecurity)
   OR EXISTS(SELECT 1 FROM pg_policy WHERE polrelid=to_regclass('public.'||name))
   OR EXISTS(SELECT 1 FROM pg_rewrite WHERE ev_class=to_regclass('public.'||name))
   OR EXISTS(SELECT 1 FROM pg_inherits WHERE inhrelid=to_regclass('public.'||name) OR inhparent=to_regclass('public.'||name)) THEN
   RAISE EXCEPTION 'Role lifecycle relation behavior prevents rollback' USING ERRCODE='55000';
  END IF;
 END LOOP;
 -- Fail rather than treating rows hidden by a policy as an empty relation.
 PERFORM set_config('row_security','off',true);
 IF EXISTS(SELECT 1 FROM public.role_space_deletion_fences) OR EXISTS(SELECT 1 FROM public.role_space_deletion_fence_receipts) THEN
  RAISE EXCEPTION 'Role lifecycle evidence prevents rollback' USING ERRCODE='55000';
 END IF;
 DROP TABLE public.role_space_deletion_fence_receipts;
 DROP FUNCTION public.role_deletion_fence_receipt_immutable();
 DROP TABLE public.role_space_deletion_fences;
END $$;
