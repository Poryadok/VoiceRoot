BEGIN;
-- Trusted offline installation before identity-owner capability activation.
-- Requires the existing authorized superuser migration channel; never runtime elevation.
-- Flyway supplies this file's transaction. Canonical UP wraps the identical body.
SET LOCAL lock_timeout='2s';
SET LOCAL statement_timeout='2s';
CREATE TABLE public.auth_identity_ddl_barrier (singleton boolean NOT NULL);
LOCK TABLE ONLY public.auth_identity_ddl_barrier IN ACCESS EXCLUSIVE MODE;
GRANT SELECT ON public.auth_identity_ddl_barrier TO PUBLIC;
CREATE FUNCTION public.auth_identity_ddl_gate() RETURNS event_trigger
LANGUAGE plpgsql SECURITY INVOKER AS $$
BEGIN
  LOCK TABLE ONLY public.auth_identity_ddl_barrier IN ACCESS EXCLUSIVE MODE NOWAIT;
  RETURN;
END $$;
REVOKE ALL ON FUNCTION public.auth_identity_ddl_gate() FROM PUBLIC;
CREATE EVENT TRIGGER auth_identity_ddl_gate ON ddl_command_start
  WHEN TAG IN ('CREATE FUNCTION', 'ALTER FUNCTION', 'DROP FUNCTION', 'CREATE PROCEDURE', 'ALTER PROCEDURE', 'DROP PROCEDURE', 'ALTER ROUTINE', 'DROP ROUTINE', 'CREATE EXTENSION', 'ALTER EXTENSION', 'DROP EXTENSION', 'CREATE LANGUAGE', 'ALTER LANGUAGE', 'DROP LANGUAGE', 'ALTER SCHEMA', 'DROP SCHEMA', 'DROP OWNED', 'GRANT', 'REVOKE', 'ALTER DEFAULT PRIVILEGES')
  EXECUTE FUNCTION public.auth_identity_ddl_gate();
COMMIT;
