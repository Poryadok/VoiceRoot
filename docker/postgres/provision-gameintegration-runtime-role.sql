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
