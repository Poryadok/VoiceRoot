-- Existing-data adoption only: owned operation Job, never blind rerun.
-- Duplicate roles fail the entire transaction. No ALTER/password rotation.
\set ON_ERROR_STOP on
SET log_statement = 'none';
SET log_min_error_statement = 'panic';
\getenv auth_password AUTH_DB_PASSWORD
\getenv user_password USER_DB_PASSWORD
\getenv admin_password PGPASSWORD
SELECT length(:'auth_password') >= 32 AND length(:'user_password') >= 32
  AND :'auth_password' <> :'user_password' AND :'auth_password' <> :'admin_password'
  AND :'user_password' <> :'admin_password' AS passwords_valid
\gset
\if :passwords_valid
\else
  DO $$ BEGIN RAISE EXCEPTION 'independent runtime passwords required'; END $$;
\endif
BEGIN;
DO $$
BEGIN
  IF current_user <> 'voice' OR NOT (SELECT rolsuper FROM pg_roles WHERE rolname=current_user) THEN
    RAISE EXCEPTION 'dedicated adoption admin required';
  END IF;
  IF (SELECT count(*) FROM pg_database WHERE datname IN ('auth_db','user_db')
      AND pg_get_userbyid(datdba)='voice') <> 2 THEN
    RAISE EXCEPTION 'existing admin database ownership required';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname IN ('auth_runtime','user_runtime')) THEN
    RAISE EXCEPTION 'existing runtime roles cannot be adopted or rekeyed';
  END IF;
END $$;
-- CREATE has no IF NOT EXISTS: concurrent foreign creation aborts atomically.
SELECT format('CREATE ROLE %I LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS PASSWORD %L', role, password)
FROM (VALUES ('auth_runtime', :'auth_password'), ('user_runtime', :'user_password')) AS runtime(role,password)
\gexec
REVOKE CONNECT, TEMPORARY ON DATABASE auth_db, user_db FROM PUBLIC;
REVOKE ALL ON DATABASE auth_db, user_db FROM auth_runtime, user_runtime;
GRANT CONNECT ON DATABASE auth_db TO auth_runtime;
GRANT CONNECT, TEMPORARY ON DATABASE user_db TO user_runtime;
COMMIT;
