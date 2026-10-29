"""Restore-only catalog for actual, separately authorized PostgreSQL databases.

This module does not select databases or confer migration/runtime authority.
Selection and source identity must come from the protected full-catalog phase.
"""
import json
import hashlib
import re


class UnmanagedError(ValueError):
    pass


def verify_catalog_phase(phase, token, binding, cut=None):
    """Bind the full scope to the real guarded, current two-journal receipt."""
    try:
        import full_cohort_affected_plan as affected
        if type(phase) is not affected.FullCatalogPhase or not isinstance(token,str) or not token:
            raise UnmanagedError('full catalog phase unavailable')
        if cut is None:
            receipt=phase.verify_current(token)
        else:
            if (type(cut) is not affected.writer_cut.WriterCut or type(cut.probe) is not affected.CatalogScopedProbe or
                    cut.probe.catalog is not phase or cut.probe._bound_catalog is not phase or
                    cut.probe.token!=token or cut.probe._bound_token!=token):
                raise UnmanagedError('catalog cut identity differs')
            receipt=phase.verify_receipt(token)
            if cut.provision_successor is None:
                if getattr(cut,'space_fresh',None) is None:
                    if phase.verify_current(token)!=receipt: raise UnmanagedError('original catalog changed')
                else:
                    affected.verify_space_catalog(cut.probe,cut)
            elif cut.effective_session is not None:
                affected.verify_effective_catalog(cut.probe,cut)
            else:
                affected.verify_provisioned_catalog(cut.probe,cut)
            if phase.verify_receipt(token)!=receipt: raise UnmanagedError('catalog receipt changed')
        full=binding['full_catalog']; databases=receipt['databases']; properties=receipt['properties']
        digest=hashlib.sha256(json.dumps(receipt,sort_keys=True,separators=(',',':'),
                                         ensure_ascii=True,allow_nan=False).encode('ascii')).hexdigest()
        if (not isinstance(full,dict) or set(full)!={'receipt_sha256','properties'} or
            full['receipt_sha256']!=digest or full['properties']!=properties or
            receipt['scope']!='all-databases-connectable-backup' or
            receipt['operation_id']!=binding['operation_id'] or receipt['source_sha']!=binding['source_sha'] or
            binding['databases']!=sorted(databases) or set(binding['sources'])!=set(databases)|{'roles'} or
            set(properties)!=set(receipt['all_databases']) or
            set(databases)|set(receipt['excluded'])!=set(receipt['all_databases']) or
            set(databases)&set(receipt['excluded'])):
            raise UnmanagedError('full catalog binding differs')
        engine=receipt['engine']; start=receipt['server_start']
        if (engine.get('namespace_uid')!=receipt['namespace_uid'] or
            binding['sources']['roles']!={'engine':engine,'server_start':start} or
            any(type(oid) is not int or oid<=0 or receipt['all_databases'].get(db)!=oid or
                properties[db].get('allowconn') is not True or
                binding['sources'][db]!={'engine':engine,'server_start':start,'database_oid':oid}
                for db,oid in databases.items())):
            raise UnmanagedError('full catalog source differs')
        return receipt
    except Exception:
        raise UnmanagedError('full catalog authority unavailable') from None


# OIDs are deliberately excluded: an isolated restore allocates different OIDs.
# Include every non-system namespace, not only public or a migration ledger.
CATALOG_SQL = r"""SELECT json_build_object(
 'database_owner',(SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname=current_database()),
 'database_properties',(SELECT json_build_object('encoding',pg_encoding_to_char(encoding),'collate',datcollate,
   'ctype',datctype,'locale_provider',datlocprovider::text,'icu_locale',daticulocale,'icu_rules',daticurules,
   'connection_limit',datconnlimit,'acl_raw',datacl::text,'acl',CASE WHEN datacl IS NULL THEN NULL ELSE
   ARRAY(SELECT item::text FROM unnest(datacl) AS item ORDER BY item::text) END,
   'template',datistemplate,'allowconn',datallowconn)
   FROM pg_database WHERE datname=current_database()),
 'schemas',(SELECT coalesce(json_agg(row_to_json(x) ORDER BY x.name),'[]'::json) FROM
   (SELECT nspname AS name,pg_get_userbyid(nspowner) AS owner,CASE WHEN nspacl IS NULL THEN NULL ELSE
     ARRAY(SELECT item::text FROM unnest(nspacl) AS item ORDER BY item::text) END AS acl
    FROM pg_namespace WHERE nspname NOT IN ('pg_catalog','information_schema') AND nspname !~ '^pg_toast' AND nspname !~ '^pg_temp_') x),
 'relations',(SELECT coalesce(json_agg(row_to_json(x) ORDER BY x.schema,x.name,x.kind),'[]'::json) FROM
   (SELECT n.nspname AS schema,c.relname AS name,c.relkind::text AS kind,pg_get_userbyid(c.relowner) AS owner,
     CASE WHEN c.relacl IS NULL THEN NULL ELSE ARRAY(SELECT item::text FROM unnest(c.relacl) AS item ORDER BY item::text) END AS acl,
     c.relrowsecurity AS row_security,c.relforcerowsecurity AS force_row_security
    FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace
    WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_toast' AND n.nspname !~ '^pg_temp_'
    AND c.relkind IN ('r','p','S','v','m','f')) x),
 'columns',(SELECT coalesce(json_agg(row_to_json(x) ORDER BY x.schema,x.relation,x.position),'[]'::json) FROM
   (SELECT n.nspname AS schema,c.relname AS relation,a.attnum AS position,a.attname AS name,
     format_type(a.atttypid,a.atttypmod) AS type,a.attnotnull AS not_null,a.attidentity::text AS identity,
     a.attgenerated::text AS generated,pg_get_expr(d.adbin,d.adrelid) AS default_value,
     CASE WHEN a.attacl IS NULL THEN NULL ELSE ARRAY(SELECT item::text FROM unnest(a.attacl) AS item ORDER BY item::text) END AS acl
    FROM pg_attribute a JOIN pg_class c ON c.oid=a.attrelid JOIN pg_namespace n ON n.oid=c.relnamespace
    LEFT JOIN pg_attrdef d ON d.adrelid=c.oid AND d.adnum=a.attnum
    WHERE a.attnum>0 AND NOT a.attisdropped AND c.relkind IN ('r','p','v','m','f')
    AND n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_toast' AND n.nspname !~ '^pg_temp_') x),
 'constraints',(SELECT coalesce(json_agg(row_to_json(x) ORDER BY x.schema,x.relation,x.name),'[]'::json) FROM
   (SELECT n.nspname AS schema,c.relname AS relation,k.conname AS name,pg_get_constraintdef(k.oid) AS definition
    FROM pg_constraint k JOIN pg_class c ON c.oid=k.conrelid JOIN pg_namespace n ON n.oid=c.relnamespace
    WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_toast' AND n.nspname !~ '^pg_temp_') x),
 'indexes',(SELECT coalesce(json_agg(row_to_json(x) ORDER BY x.schema,x.name),'[]'::json) FROM
   (SELECT n.nspname AS schema,c.relname AS name,pg_get_indexdef(c.oid) AS definition
    FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE c.relkind IN ('i','I')
    AND n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_toast' AND n.nspname !~ '^pg_temp_') x),
 'routines',(SELECT coalesce(json_agg(row_to_json(x) ORDER BY x.schema,x.name,x.arguments),'[]'::json) FROM
   (SELECT n.nspname AS schema,p.proname AS name,pg_get_function_identity_arguments(p.oid) AS arguments,
     p.prokind::text AS kind,pg_get_userbyid(p.proowner) AS owner,
     CASE WHEN p.proacl IS NULL THEN NULL ELSE ARRAY(SELECT item::text FROM unnest(p.proacl) AS item ORDER BY item::text) END AS acl,
     CASE WHEN p.prokind<>'a' THEN pg_get_functiondef(p.oid) ELSE NULL END AS definition
    FROM pg_proc p JOIN pg_namespace n ON n.oid=p.pronamespace
    WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_toast' AND n.nspname !~ '^pg_temp_') x),
 'types',(SELECT coalesce(json_agg(row_to_json(x) ORDER BY x.schema,x.name),'[]'::json) FROM
   (SELECT n.nspname AS schema,t.typname AS name,t.typtype::text AS kind,pg_get_userbyid(t.typowner) AS owner,
     CASE WHEN t.typacl IS NULL THEN NULL ELSE ARRAY(SELECT item::text FROM unnest(t.typacl) AS item ORDER BY item::text) END AS acl,
     (SELECT coalesce(json_agg(e.enumlabel ORDER BY e.enumsortorder),'[]'::json) FROM pg_enum e WHERE e.enumtypid=t.oid) AS labels
    FROM pg_type t JOIN pg_namespace n ON n.oid=t.typnamespace WHERE t.typtype IN ('e','d','r','m')
    AND n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_toast' AND n.nspname !~ '^pg_temp_') x),
 'extensions',(SELECT coalesce(json_agg(row_to_json(x) ORDER BY x.name),'[]'::json) FROM
   (SELECT extname AS name,extversion AS version,pg_get_userbyid(extowner) AS owner FROM pg_extension) x),
 'sequences',(SELECT coalesce(json_agg(row_to_json(x) ORDER BY x.schema,x.name),'[]'::json) FROM
   (SELECT schemaname AS schema,sequencename AS name,sequenceowner AS owner,data_type::text AS type,
     start_value,min_value,max_value,increment_by,cycle,cache_size,last_value FROM pg_sequences
    WHERE schemaname NOT IN ('pg_catalog','information_schema') AND schemaname !~ '^pg_toast' AND schemaname !~ '^pg_temp_') x),
 'defaults',(SELECT coalesce(json_agg(row_to_json(x) ORDER BY x.owner,x.schema,x.kind),'[]'::json) FROM
   (SELECT pg_get_userbyid(d.defaclrole) AS owner,n.nspname AS schema,d.defaclobjtype::text AS kind,
     CASE WHEN d.defaclacl IS NULL THEN NULL ELSE ARRAY(SELECT item::text FROM unnest(d.defaclacl) AS item ORDER BY item::text) END AS acl
    FROM pg_default_acl d LEFT JOIN pg_namespace n ON n.oid=d.defaclnamespace) x),
 'policies',(SELECT coalesce(json_agg(row_to_json(x) ORDER BY x.schema,x.relation,x.name),'[]'::json) FROM
   (SELECT n.nspname AS schema,c.relname AS relation,p.polname AS name,p.polcmd::text AS command,p.polpermissive AS permissive,
     (SELECT array_agg(CASE WHEN r.oid=0 THEN 'PUBLIC' ELSE pg_get_userbyid(r.oid) END
                       ORDER BY CASE WHEN r.oid=0 THEN 'PUBLIC' ELSE pg_get_userbyid(r.oid) END)
      FROM unnest(p.polroles) AS r(oid)) AS roles,
     pg_get_expr(p.polqual,p.polrelid) AS using_expr,pg_get_expr(p.polwithcheck,p.polrelid) AS check_expr
    FROM pg_policy p JOIN pg_class c ON c.oid=p.polrelid JOIN pg_namespace n ON n.oid=c.relnamespace
    WHERE n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_toast' AND n.nspname !~ '^pg_temp_') x),
 'triggers',(SELECT coalesce(json_agg(row_to_json(x) ORDER BY x.schema,x.relation,x.name),'[]'::json) FROM
   (SELECT n.nspname AS schema,c.relname AS relation,t.tgname AS name,pg_get_triggerdef(t.oid) AS definition,t.tgenabled::text AS enabled
    FROM pg_trigger t JOIN pg_class c ON c.oid=t.tgrelid JOIN pg_namespace n ON n.oid=c.relnamespace
    WHERE NOT t.tgisinternal AND n.nspname NOT IN ('pg_catalog','information_schema') AND n.nspname !~ '^pg_toast' AND n.nspname !~ '^pg_temp_') x));"""

FIELDS = frozenset(('schemas','relations','columns','constraints','indexes','routines',
                    'types','extensions','defaults','policies','triggers','sequences'))


def restore_result(source, restored):
    """Compare frozen source/isolated restore; ledger rows grant no qualification."""
    try:
        source,restored=json.loads(json.dumps([source,restored],sort_keys=True,allow_nan=False))
        if any(not isinstance(value,dict) or set(value)!=FIELDS|{'database_owner','database_properties'} or
               not isinstance(value['database_owner'],str) or not value['database_owner'] or
               not isinstance(value['database_properties'],dict) or not value['database_properties'] or
               any(not isinstance(value[field],list) for field in FIELDS) for value in (source,restored)):
            raise UnmanagedError('unmanaged catalog shape')
        for value in (source,restored):
            value['database_properties'].pop('acl_raw',None)
        if source != restored:
            raise UnmanagedError('unmanaged restore catalog differs')
        return {'mode':'restore-only','from':None,'to':None}
    except (TypeError,ValueError):
        raise UnmanagedError('unmanaged source or restored catalog invalid') from None


def validate_source_catalog(source, bound):
    """Check source-captured database properties against the guarded receipt."""
    restore_result(source,source)
    try:
        props=source['database_properties']
        expected={'template':props['template'],'allowconn':props['allowconn'],'encoding':props['encoding'],
                  'collate':props['collate'],'ctype':props['ctype'],'locale_provider':props['locale_provider'],
                  'icu_locale':props['icu_locale'],'connection_limit':props['connection_limit'],
                  'owner':source['database_owner'],'acl':props['acl_raw']}
        if expected!=bound or expected['allowconn'] is not True or type(expected['template']) is not bool:
            raise UnmanagedError('source database properties differ')
    except (KeyError,TypeError):
        raise UnmanagedError('source database properties missing') from None


def verify_restored_properties(psql, database, bound):
    """Verify protected database-level attributes in the disposable restore."""
    try:
        actual=json.loads(psql(database,CATALOG_SQL))
        value=actual['database_properties']
        expected={key:value[key] for key in ('template','allowconn','encoding','collate','ctype',
                    'locale_provider','icu_locale','connection_limit')}
        expected['owner']=actual['database_owner']
        source_acl=bound['acl']
        if source_acl is None:
            acl=None
        else:
            if not isinstance(source_acl,str):raise ValueError()
            literal="'"+source_acl.replace("'","''")+"'"
            sql=("SELECT coalesce(json_agg(entry ORDER BY entry),'[]'::json) FROM "
                 "(SELECT item::text AS entry FROM unnest("+literal+"::pg_catalog.aclitem[]) AS item) AS source_acl")
            acl=json.loads(psql(database,sql))
        if value['acl']!=acl or any(bound.get(key)!=value for key,value in expected.items()) or set(bound)!=set(expected)|{'acl'}:
            raise ValueError()
    except Exception:
        raise UnmanagedError('restored database properties differ') from None


def prepare_disposable_template(psql, source, maintenance_db='postgres'):
    """Prepare template0 only inside a verified disposable isolated cluster.

    The caller owns container identity and cleanup; this helper provides no live
    transport or source authorization. Restore compares the complete catalog.
    """
    if not callable(psql) or not isinstance(source,dict) or not isinstance(source.get('extensions'),list):
        raise UnmanagedError('template source missing')
    extensions=[x for x in source['extensions'] if isinstance(x,dict) and x.get('name')=='plpgsql']
    if len(extensions)>1:
        raise UnmanagedError('ambiguous template extension')
    sql='DROP EXTENSION IF EXISTS plpgsql'
    if extensions:
        owner=extensions[0].get('owner')
        if not isinstance(owner,str) or not owner or '\0' in owner or len(owner.encode('utf-8'))>63:
            raise UnmanagedError('template extension owner invalid')
        sql+='; SET ROLE "'+owner.replace('"','""')+'"; CREATE EXTENSION plpgsql'
    psql(maintenance_db,'ALTER DATABASE template0 WITH ALLOW_CONNECTIONS true')
    try:
        psql('template0',sql)
    finally:
        psql(maintenance_db,'ALTER DATABASE template0 WITH ALLOW_CONNECTIONS false')


def prepare_disposable_database(psql, database, maintenance_db):
    """Remove an existing default database only in the owned isolated target."""
    if (not callable(psql) or database not in ('postgres','template1') or
            not isinstance(maintenance_db,str) or not re.fullmatch(r'voice_cohort_private_[a-z0-9]+',maintenance_db)):
        raise UnmanagedError('invalid disposable builtin restore')
    if database=='template1':
        psql(maintenance_db,'ALTER DATABASE "template1" IS_TEMPLATE false')
    psql(maintenance_db,'DROP DATABASE "'+database+'"')
