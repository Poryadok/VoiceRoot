\getenv space_runtime_password SPACE_DB_PASSWORD
SELECT format(
  'CREATE ROLE space_runtime WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS NOREPLICATION PASSWORD %L',
  :'space_runtime_password'
)
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'space_runtime')
\gexec

-- Never adopt an installer/owner identity as a serving identity.
DO $$
BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'space_runtime'
      AND (rolsuper OR rolcreatedb OR rolcreaterole OR rolreplication OR rolbypassrls))
    OR EXISTS (SELECT 1 FROM pg_auth_members WHERE member = 'space_runtime'::regrole)
    OR EXISTS (SELECT 1 FROM pg_class WHERE relowner = 'space_runtime'::regrole)
    OR EXISTS (SELECT 1 FROM pg_namespace WHERE nspowner = 'space_runtime'::regrole)
    OR pg_catalog.has_parameter_privilege('space_runtime', 'session_replication_role', 'SET')
    OR EXISTS (SELECT 1 FROM pg_namespace WHERE nspname = 'native_membership_guard'
      AND has_schema_privilege('space_runtime', oid, 'CREATE'))
  THEN
    RAISE EXCEPTION 'space_runtime must be a non-owner serving role';
  END IF;
END $$;

SELECT format('ALTER ROLE space_runtime WITH LOGIN PASSWORD %L', :'space_runtime_password')
\gexec
REVOKE CREATE ON SCHEMA public FROM PUBLIC;
REVOKE CREATE ON SCHEMA public FROM space_runtime;
GRANT USAGE ON SCHEMA public TO space_runtime;
GRANT SELECT, INSERT, UPDATE, DELETE ON ALL TABLES IN SCHEMA public TO space_runtime;
GRANT USAGE, SELECT ON ALL SEQUENCES IN SCHEMA public TO space_runtime;
