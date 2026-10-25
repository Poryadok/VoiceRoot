BEGIN;
SET LOCAL lock_timeout='2s';
LOCK TABLE ONLY public.auth_identity_ddl_barrier IN ACCESS EXCLUSIVE MODE;
DO $$ BEGIN
  RAISE EXCEPTION USING ERRCODE='55000', MESSAGE='identity DDL gate downgrade refused; use verified matching recovery';
END $$;
COMMIT;
