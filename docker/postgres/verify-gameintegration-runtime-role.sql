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
    authority_relation regclass;
    changed_function oid;
    floor_function oid;
    source_relation text;
BEGIN
    IF current_database() <> 'game_integration_db' THEN
        RAISE EXCEPTION 'unexpected verification database';
    END IF;

    SELECT count(*), max(version), bool_or(dirty)
      INTO migration_rows, migration_version, migration_dirty
      FROM schema_migrations;
    IF migration_rows <> 1 OR migration_version <> 33 OR migration_dirty IS DISTINCT FROM false THEN
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

    -- Revision 32's retained singleton and its exact invoker functions are
    -- part of the authority cut. The runtime role needs its existing DML
    -- grants for invoker triggers; do not revoke them here.
    authority_relation := 'public.gis_authority_revision'::regclass;
    IF NOT EXISTS (SELECT 1 FROM pg_class c WHERE c.oid=authority_relation AND c.relkind='r'
        AND c.relpersistence='p' AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity
        AND c.relowner<>role_row.oid)
       OR EXISTS (SELECT 1 FROM pg_policy WHERE polrelid=authority_relation)
       OR EXISTS (SELECT 1 FROM pg_rewrite WHERE ev_class=authority_relation)
       OR EXISTS (SELECT 1 FROM pg_inherits WHERE inhrelid=authority_relation OR inhparent=authority_relation)
       OR (SELECT count(*) FROM pg_attribute WHERE attrelid=authority_relation AND attnum>0 AND NOT attisdropped)<>2
       OR NOT EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid=authority_relation AND attname='singleton'
                      AND atttypid='boolean'::regtype AND attnotnull AND NOT attisdropped)
       OR NOT EXISTS (SELECT 1 FROM pg_attribute WHERE attrelid=authority_relation AND attname='revision'
                      AND atttypid='bigint'::regtype AND attnotnull AND NOT attisdropped)
       OR (SELECT count(*) FROM pg_constraint WHERE conrelid=authority_relation AND contype<>'n')<>3
       OR (SELECT count(*) FROM pg_constraint WHERE conrelid=authority_relation AND convalidated AND
           ((contype='p' AND pg_get_constraintdef(oid)='PRIMARY KEY (singleton)') OR
            (contype='c' AND pg_get_constraintdef(oid) IN ('CHECK (singleton)','CHECK ((revision > 0))'))))<>3
       OR (SELECT count(*) FROM public.gis_authority_revision)<>1
       OR NOT EXISTS (SELECT 1 FROM public.gis_authority_revision WHERE singleton IS TRUE AND revision>0) THEN
        RAISE EXCEPTION 'GIS authority singleton is invalid';
    END IF;
    SELECT p.oid INTO changed_function FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
     JOIN pg_language l ON l.oid=p.prolang
     WHERE n.nspname='public' AND p.proname='gis_authority_changed' AND p.pronargs=0
       AND p.prorettype='trigger'::regtype AND l.lanname='plpgsql' AND NOT p.prosecdef
       AND p.proconfig IS NULL AND p.proowner<>role_row.oid
       AND encode(sha256(convert_to(p.prosrc,'UTF8')),'hex')='2df3536b5e8688cf132121f4ae5e1d5901bbcb7809e4fa82710cb93f63846bd3';
    SELECT p.oid INTO floor_function FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
     JOIN pg_language l ON l.oid=p.prolang
     WHERE n.nspname='public' AND p.proname='gis_authority_floor_guard' AND p.pronargs=0
       AND p.prorettype='trigger'::regtype AND l.lanname='plpgsql' AND NOT p.prosecdef
       AND p.proconfig IS NULL AND p.proowner<>role_row.oid
       AND encode(sha256(convert_to(p.prosrc,'UTF8')),'hex')='6ee87591548481e6e663df34b2ace732564c07cd129b439c31c37055fc4ad70b';
    IF changed_function IS NULL OR floor_function IS NULL THEN
        RAISE EXCEPTION 'GIS authority functions are invalid';
    END IF;
    FOREACH source_relation IN ARRAY ARRAY['applications','environments','player_bindings','installations',
        'game_resource_mappings','game_resource_binding_chats','gis_sessions',
        'gis_app_notification_consents','service_credentials'] LOOP
        IF NOT EXISTS (SELECT 1 FROM pg_class c WHERE c.oid=to_regclass('public.'||source_relation)
                        AND c.relkind='r' AND c.relpersistence='p' AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity)
           OR EXISTS (SELECT 1 FROM pg_policy WHERE polrelid=to_regclass('public.'||source_relation))
           OR EXISTS (SELECT 1 FROM pg_rewrite WHERE ev_class=to_regclass('public.'||source_relation))
           OR EXISTS (SELECT 1 FROM pg_inherits WHERE inhrelid=to_regclass('public.'||source_relation)
                      OR inhparent=to_regclass('public.'||source_relation))
           OR (SELECT count(*) FROM pg_trigger t WHERE t.tgrelid=to_regclass('public.'||source_relation)
               AND NOT t.tgisinternal) <> (CASE WHEN source_relation IN ('applications','environments') THEN 3 ELSE 2 END)
           OR (SELECT count(*) FROM pg_trigger t
             WHERE t.tgrelid=to_regclass('public.'||source_relation) AND NOT t.tgisinternal
               AND t.tgenabled='O' AND t.tgqual IS NULL AND t.tgnargs=0
               AND ((t.tgname='gis_authority_changed' AND t.tgtype=28 AND t.tgfoid=changed_function)
                 OR (t.tgname='gis_authority_no_truncate' AND t.tgtype=34 AND t.tgfoid=floor_function)))<>2 THEN
            RAISE EXCEPTION 'GIS authority source triggers are invalid';
        END IF;
        IF source_relation IN ('applications','environments') AND NOT EXISTS (
            SELECT 1 FROM pg_trigger t JOIN pg_proc p ON p.oid=t.tgfoid
              JOIN pg_namespace n ON n.oid=p.pronamespace JOIN pg_language l ON l.oid=p.prolang
             WHERE t.tgrelid=to_regclass('public.'||source_relation) AND NOT t.tgisinternal
               AND t.tgname=source_relation||'_revoke_credentials_on_status_change'
               AND t.tgtype=17 AND t.tgenabled='O' AND t.tgqual IS NULL AND t.tgnargs=0
               AND t.tgattr::text=(SELECT attnum::text FROM pg_attribute WHERE attrelid=t.tgrelid AND attname='status' AND NOT attisdropped)
               AND n.nspname='public' AND l.lanname='plpgsql' AND p.pronargs=0 AND p.prorettype='trigger'::regtype
               AND NOT p.prosecdef AND p.proconfig IS NULL AND p.proowner<>role_row.oid
               AND ((source_relation='applications' AND p.proname='revoke_credentials_on_application_lifecycle_change'
                     AND encode(sha256(convert_to(p.prosrc,'UTF8')),'hex')='bc0cad6f1f2d5e15d57c4f16146b2d3f765ff8b9235a88d2d88d792390024db6')
                 OR (source_relation='environments' AND p.proname='revoke_credentials_on_environment_lifecycle_change'
                     AND encode(sha256(convert_to(p.prosrc,'UTF8')),'hex')='220baf09b91fdec934e7fd425e569a4df8ecd228c6b17a88cb026356a6806eaa'))) THEN
            RAISE EXCEPTION 'GIS lifecycle source trigger is invalid';
        END IF;
    END LOOP;
    IF (SELECT count(*) FROM pg_trigger WHERE tgrelid=authority_relation AND NOT tgisinternal)<>2
       OR (SELECT count(*) FROM pg_trigger t WHERE t.tgrelid=authority_relation AND NOT t.tgisinternal
          AND t.tgenabled='O' AND t.tgqual IS NULL AND t.tgnargs=0 AND t.tgfoid=floor_function
          AND ((t.tgname='gis_authority_floor_change' AND t.tgtype=31)
            OR (t.tgname='gis_authority_floor_truncate' AND t.tgtype=34)))<>2 THEN
        RAISE EXCEPTION 'GIS authority floor triggers are invalid';
    END IF;
END $$;

SELECT json_build_object(
    'schema', 'voice-gis-verifier-v3',
    'database', current_database(),
    'migration_version', version,
    'dirty', dirty,
    'runtime_role', 'gameintegration_runtime',
    'role_restricted', true,
    'table_grants', 'select_insert_update_delete',
    'sequence_grants', 'usage_select',
    'ddl_denied', true,
    'schema_ledger_runtime_mutation_denied', true,
    'authority_singleton_verified', true,
    'authority_triggers_verified', true,
    'authority_functions_verified', true
)::text
  FROM schema_migrations;
