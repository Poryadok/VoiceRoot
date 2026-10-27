-- DEV fresh bootstrap only, after admin-owned migrations and before apps start.
-- Existing deployments require separately reviewed ownership/custody planning.
\set ON_ERROR_STOP on
-- Session-only: errors involving generated role statements must not log secrets.
SET log_statement = 'none';
SET log_min_error_statement = 'panic';
\getenv auth_password AUTH_DB_PASSWORD
\getenv user_password USER_DB_PASSWORD
\getenv admin_password PGPASSWORD
SELECT length(:'auth_password') > 0 AND length(:'user_password') > 0
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
    RAISE EXCEPTION 'dedicated bootstrap admin required';
  END IF;
  IF (SELECT count(*) FROM pg_database WHERE datname IN ('auth_db','user_db')
      AND pg_get_userbyid(datdba)='voice') <> 2 THEN
    RAISE EXCEPTION 'existing admin database ownership required';
  END IF;
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname IN ('auth_runtime','user_runtime')
      AND (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls OR rolinherit OR NOT rolcanlogin))
     OR EXISTS (SELECT 1 FROM pg_auth_members WHERE member IN
         (SELECT oid FROM pg_roles WHERE rolname IN ('auth_runtime','user_runtime')))
     OR EXISTS (SELECT 1 FROM pg_shdepend WHERE deptype='o' AND refobjid IN
         (SELECT oid FROM pg_roles WHERE rolname IN ('auth_runtime','user_runtime'))) THEN
    RAISE EXCEPTION 'unexpected existing runtime role authority';
  END IF;
END $$;
-- quote_literal via format %L keeps independently issued values out of SQL syntax.
SELECT format('CREATE ROLE %I LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOREPLICATION NOBYPASSRLS PASSWORD %L', role, password)
FROM (VALUES ('auth_runtime', :'auth_password'), ('user_runtime', :'user_password')) AS runtime(role,password)
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname=role)
\gexec
SELECT format('ALTER ROLE %I PASSWORD %L', role, password)
FROM (VALUES ('auth_runtime', :'auth_password'), ('user_runtime', :'user_password')) AS runtime(role,password)
\gexec
REVOKE CONNECT, TEMPORARY ON DATABASE auth_db, user_db FROM PUBLIC;
REVOKE ALL ON DATABASE auth_db, user_db FROM auth_runtime, user_runtime;
GRANT CONNECT ON DATABASE auth_db TO auth_runtime;
-- User's identity-association transactions use pg_temp preparation/batch tables.
GRANT CONNECT, TEMPORARY ON DATABASE user_db TO user_runtime;
COMMIT;

\connect auth_db
BEGIN;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON SCHEMA public FROM auth_runtime;
GRANT USAGE ON SCHEMA public TO auth_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO auth_runtime;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO auth_runtime;
ALTER DEFAULT PRIVILEGES FOR ROLE voice IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO auth_runtime;
ALTER DEFAULT PRIVILEGES FOR ROLE voice IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO auth_runtime;
-- Application never owns or advances the migration ledger.
SELECT 'REVOKE ALL ON TABLE public.flyway_schema_history FROM auth_runtime'
WHERE to_regclass('public.flyway_schema_history') IS NOT NULL
\gexec
SELECT 'GRANT SELECT ON TABLE public.flyway_schema_history TO auth_runtime'
WHERE to_regclass('public.flyway_schema_history') IS NOT NULL
\gexec
-- The canonical identity gate permits only owner ACLs plus PUBLIC SELECT.
-- Runtime takes ACCESS SHARE through PUBLIC; any direct role ACL is invalid.
SELECT 'REVOKE ALL ON TABLE public.auth_identity_ddl_barrier FROM auth_runtime'
WHERE to_regclass('public.auth_identity_ddl_barrier') IS NOT NULL
\gexec
COMMIT;

\connect user_db
BEGIN;
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE ALL ON SCHEMA public FROM user_runtime;
GRANT USAGE ON SCHEMA public TO user_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO user_runtime;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO user_runtime;
ALTER DEFAULT PRIVILEGES FOR ROLE voice IN SCHEMA public GRANT SELECT, INSERT, UPDATE, DELETE ON TABLES TO user_runtime;
ALTER DEFAULT PRIVILEGES FOR ROLE voice IN SCHEMA public GRANT USAGE, SELECT ON SEQUENCES TO user_runtime;
SELECT 'REVOKE ALL ON TABLE public.schema_migrations FROM user_runtime'
WHERE to_regclass('public.schema_migrations') IS NOT NULL
\gexec
-- User's canonical schema guard reads the ledger at startup.
SELECT 'GRANT SELECT ON TABLE public.schema_migrations TO user_runtime'
WHERE to_regclass('public.schema_migrations') IS NOT NULL
\gexec
COMMIT;
