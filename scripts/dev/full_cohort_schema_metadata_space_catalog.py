"""Compile the source-pinned Space v25 catalog check into one read-only cut."""
import hashlib
from pathlib import Path
import re

from full_cohort_schema_metadata import Hold, COMMON_SQL

FILES = {
    'src/backend/space/internal/store/authority_source_catalog.go': 'cafa3c34705fd2dfa40901d5bb1c7bfb8c6ae9742f6d8b4133676e3624e0c324',
    'src/backend/pkg/authoritysource/postgres.go': 'b6fe91abb87f6ab189b90cc58c8b1ae20cb077739fa575d3d9c211d29e3c20b2',
    'src/backend/space/internal/store/tree_viewer.go': '61e4c6434e39fde33969d595b4a9538cb6d0fb2a2fe8c06e409230433ff20896',
    'src/backend/space/internal/store/authority_source.go': '33983f674249180250e7c14440e8e1db09b5d40f1d7b04cac51c7eda805f9edf',
}


def compile_sql(root, snapshot):
    """Parse only the exact owner-authored catalog; never use request SQL."""
    try:
        for name, digest in FILES.items():
            raw = (Path(root) / name).read_bytes()
            if snapshot['files'].get(name) != digest or hashlib.sha256(raw).hexdigest() != digest:
                raise Hold('space_catalog_source_unknown')
        text = (Path(root) / next(iter(FILES))).read_text(encoding='utf-8')
        table_section = re.search(r'Tables: \[\]string\{([^}]*)\}', text).group(1)
        tables = re.findall(r'"([a-z0-9_]+)"', table_section)
        triggers = re.findall(r'\{Table: "([a-z0-9_]+)", Name: "([a-z0-9_]+)", Function: "([a-z0-9_]+)", Type: ([0-9]+)(?:, Argument: "([a-z0-9_]+)")?\}', text)
        functions = re.findall(r'\{Name: "([a-z0-9_]+)", Arguments: ([0-9]+), ReturnType: "([a-z0-9_]+)", BodySHA256: "([0-9a-f]{64})"\}', text)
        if len(tables) != 16 or len(set(tables)) != 16 or len(triggers) != 33 or len(functions) != 5:
            raise Hold('space_catalog_source_unknown')
        table_names = ['schema_migrations', *tables]
        locked = ','.join('public."' + name + '"' for name in table_names)
        quoted_tables = ','.join("'" + name + "'" for name in table_names)
        trigger_rows = ','.join("('%s','%s','%s',%s,pg_catalog.decode('%s','hex'))" %
                                (table, name, func, kind, (arg.encode() + b'\0').hex() if arg else '')
                                for table, name, func, kind, arg in triggers)
        function_rows = ','.join("(%s,'%s',%s,'%s','%s')" % (index, *row)
                                 for index, row in enumerate(functions, 1))
        return ("BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY; SET LOCAL row_security=off; "
                "SET LOCAL search_path=public,pg_catalog; LOCK TABLE " + locked + " IN ACCESS SHARE MODE; "
                "SELECT pg_catalog.json_build_object(" + COMMON_SQL + ","
                "'ledger25',(SELECT pg_catalog.count(*)=1 AND coalesce(pg_catalog.bool_and(version=25 AND NOT dirty),false) FROM public.schema_migrations),"
                "'tables_ok',(SELECT pg_catalog.count(*)=17 AND coalesce(pg_catalog.bool_and(c.relkind='r' AND c.relpersistence='p' AND NOT c.relrowsecurity AND NOT c.relforcerowsecurity "
                "AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_policy WHERE polrelid=c.oid) AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_rewrite WHERE ev_class=c.oid) "
                "AND NOT EXISTS(SELECT 1 FROM pg_catalog.pg_inherits WHERE inhrelid=c.oid OR inhparent=c.oid)),false) "
                "FROM pg_catalog.pg_class c JOIN pg_catalog.pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relname=ANY(ARRAY[" + quoted_tables + "])),"
                "'triggers_ok',(SELECT pg_catalog.count(*)=33 FROM (VALUES " + trigger_rows + ") e(table_name,trigger_name,function_name,trigger_type,arguments) "
                "JOIN pg_catalog.pg_trigger t ON t.tgrelid=pg_catalog.to_regclass(pg_catalog.format('public.%I',e.table_name)) AND t.tgname=e.trigger_name::pg_catalog.name "
                "JOIN pg_catalog.pg_proc p ON p.oid=t.tgfoid JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace "
                "WHERE NOT t.tgisinternal AND t.tgenabled='O' AND t.tgtype=e.trigger_type AND n.nspname='public' AND p.proname=e.function_name::pg_catalog.name AND t.tgargs=e.arguments),"
                "'functions_ok',(SELECT pg_catalog.count(*)=5 AND pg_catalog.count(DISTINCT e.ordinal)=5 AND coalesce(pg_catalog.bool_and(pg_catalog.encode(pg_catalog.sha256(pg_catalog.convert_to(p.prosrc,'UTF8')),'hex')=e.body_hash),false) FROM (VALUES " + function_rows + ") e(ordinal,name,arguments,return_type,body_hash) "
                "JOIN pg_catalog.pg_proc p ON p.proname=e.name::pg_catalog.name AND p.pronargs=e.arguments AND p.prorettype=pg_catalog.to_regtype(e.return_type) "
                "JOIN pg_catalog.pg_namespace n ON n.oid=p.pronamespace JOIN pg_catalog.pg_language l ON l.oid=p.prolang "
                "WHERE n.nspname='public' AND NOT p.prosecdef AND p.provolatile='v' AND l.lanname='plpgsql')); COMMIT;")
    except Hold:
        raise
    except Exception:
        raise Hold('space_catalog_source_unknown') from None
