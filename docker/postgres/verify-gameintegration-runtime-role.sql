\set ON_ERROR_STOP on
-- Migrations use the privileged identity; the application must not forge this ledger.
REVOKE ALL PRIVILEGES ON TABLE public.schema_migrations FROM gameintegration_runtime;
-- Reassert the no-temporary-DDL boundary after every provision/migration run.
REVOKE TEMPORARY ON DATABASE game_integration_db FROM PUBLIC;
REVOKE TEMPORARY ON DATABASE game_integration_db FROM gameintegration_runtime;

DO $$
DECLARE
    role_row pg_roles%ROWTYPE;
    migration_version bigint;
    migration_dirty boolean;
    migration_rows bigint;
    table_row record;
    sequence_row record;
BEGIN
    IF current_database() <> 'game_integration_db' THEN
        RAISE EXCEPTION 'unexpected verification database';
    END IF;

    SELECT count(*), max(version), bool_or(dirty)
      INTO migration_rows, migration_version, migration_dirty
      FROM schema_migrations;
    IF migration_rows <> 1 OR migration_version <> 31 OR migration_dirty IS DISTINCT FROM false THEN
        RAISE EXCEPTION 'GIS schema version is not clean at expected revision';
    END IF;
    IF has_table_privilege('gameintegration_runtime', 'public.schema_migrations', 'INSERT')
       OR has_table_privilege('gameintegration_runtime', 'public.schema_migrations', 'UPDATE')
       OR has_table_privilege('gameintegration_runtime', 'public.schema_migrations', 'DELETE') THEN
        RAISE EXCEPTION 'GIS runtime role can mutate migration ledger';
    END IF;

    SELECT * INTO role_row FROM pg_roles WHERE rolname = 'gameintegration_runtime';
    IF NOT FOUND OR NOT role_row.rolcanlogin OR role_row.rolsuper OR role_row.rolinherit
       OR role_row.rolcreaterole OR role_row.rolcreatedb OR role_row.rolreplication
       OR role_row.rolbypassrls THEN
        RAISE EXCEPTION 'GIS runtime role attributes do not match restricted contract';
    END IF;
    IF NOT has_database_privilege('gameintegration_runtime', current_database(), 'CONNECT')
       OR has_database_privilege('gameintegration_runtime', current_database(), 'CREATE')
       OR has_database_privilege('gameintegration_runtime', current_database(), 'TEMP')
       OR NOT has_schema_privilege('gameintegration_runtime', 'public', 'USAGE')
       OR has_schema_privilege('gameintegration_runtime', 'public', 'CREATE') THEN
        RAISE EXCEPTION 'GIS runtime database or schema privileges do not match contract';
    END IF;

    FOR table_row IN
        SELECT c.oid, c.relname, c.relowner
          FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
         WHERE n.nspname = 'public' AND c.relkind IN ('r', 'p', 'v', 'm', 'f')
           AND c.relname <> 'schema_migrations'
    LOOP
        IF table_row.relowner = role_row.oid
           OR NOT has_table_privilege('gameintegration_runtime', table_row.oid, 'SELECT')
           OR NOT has_table_privilege('gameintegration_runtime', table_row.oid, 'INSERT')
           OR NOT has_table_privilege('gameintegration_runtime', table_row.oid, 'UPDATE')
           OR NOT has_table_privilege('gameintegration_runtime', table_row.oid, 'DELETE')
           OR has_table_privilege('gameintegration_runtime', table_row.oid, 'TRUNCATE')
           OR has_table_privilege('gameintegration_runtime', table_row.oid, 'TRIGGER') THEN
            RAISE EXCEPTION 'GIS runtime table grants do not match contract';
        END IF;
    END LOOP;

    FOR sequence_row IN
        SELECT c.oid, c.relowner
          FROM pg_class c JOIN pg_namespace n ON n.oid = c.relnamespace
         WHERE n.nspname = 'public' AND c.relkind = 'S'
    LOOP
        IF sequence_row.relowner = role_row.oid
           OR NOT has_sequence_privilege('gameintegration_runtime', sequence_row.oid, 'USAGE')
           OR NOT has_sequence_privilege('gameintegration_runtime', sequence_row.oid, 'SELECT')
           OR has_sequence_privilege('gameintegration_runtime', sequence_row.oid, 'UPDATE') THEN
            RAISE EXCEPTION 'GIS runtime sequence grants do not match contract';
        END IF;
    END LOOP;
END $$;

SELECT json_build_object(
    'schema', 'voice-gis-verifier-v1',
    'database', current_database(),
    'migration_version', version,
    'dirty', dirty,
    'runtime_role', 'gameintegration_runtime',
    'role_restricted', true,
    'table_grants', 'select_insert_update_delete',
    'sequence_grants', 'usage_select',
    'ddl_denied', true
)::text
  FROM schema_migrations;
