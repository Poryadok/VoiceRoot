\getenv gis_runtime_password GAME_INTEGRATION_DB_PASSWORD
SELECT format(
  'CREATE ROLE gameintegration_runtime WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS NOREPLICATION PASSWORD %L',
  :'gis_runtime_password'
)
WHERE NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = 'gameintegration_runtime')
\gexec
SELECT format(
  'ALTER ROLE gameintegration_runtime WITH LOGIN NOSUPERUSER NOCREATEDB NOCREATEROLE NOINHERIT NOBYPASSRLS NOREPLICATION PASSWORD %L',
  :'gis_runtime_password'
)
\gexec

-- PostgreSQL grants TEMP to PUBLIC by default. The GIS runtime role is a
-- member of PUBLIC, so remove that database-scoped privilege explicitly.
REVOKE TEMPORARY ON DATABASE game_integration_db FROM PUBLIC;
REVOKE TEMPORARY ON DATABASE game_integration_db FROM gameintegration_runtime;
