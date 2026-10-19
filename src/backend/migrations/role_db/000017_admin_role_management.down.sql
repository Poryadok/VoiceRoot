-- Prior Admin grants cannot be distinguished from this additive correction.
-- A rollback must not silently revoke preexisting delegated permissions.
DO $$ BEGIN
  RAISE EXCEPTION 'Admin authority correction requires a reviewed forward migration' USING ERRCODE='55000';
END $$;
