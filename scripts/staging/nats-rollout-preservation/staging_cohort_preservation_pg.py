"""Fixed STAGING catalog/metadata observations and PostgreSQL dump transport."""
import copy
import hashlib
import json
import os
import re
import selectors
import signal
import subprocess
import time

import commands
from controller import Blocked
import staging_cohort_preservation as owner

SAFE=re.compile(r'[A-Za-z_][A-Za-z0-9_]{0,62}')
CATALOG_SQL="""SELECT json_build_object(
 'system_identifier',(SELECT system_identifier::text FROM pg_control_system()),
 'server_start',pg_postmaster_start_time(),'can_read',pg_has_role(current_user,'pg_read_all_stats','MEMBER'),
 'databases',COALESCE(json_agg(json_build_object('name',d.datname,'oid',d.oid::bigint,
 'allowconn',d.datallowconn,'template',d.datistemplate,'encoding',pg_encoding_to_char(d.encoding),
 'collate',d.datcollate,'ctype',d.datctype,'locale_provider',d.datlocprovider,'icu_locale',d.daticulocale,
 'connection_limit',d.datconnlimit,'owner',pg_get_userbyid(d.datdba),'acl',d.datacl::text,
 'connect',has_database_privilege(current_user,d.oid,'CONNECT'),
 'sessions',(SELECT count(*) FROM pg_stat_activity a WHERE a.datid=d.oid AND a.pid<>pg_backend_pid() AND a.backend_type='client backend'),
 'prepared',(SELECT count(*) FROM pg_prepared_xacts x WHERE x.database=d.datname)) ORDER BY d.datname),'[]'::json))
 FROM pg_database d;"""
PSQL='export PGPASSWORD="$POSTGRES_PASSWORD"; exec psql -X -qAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d "$1"'
EXEC=owner.PREFIX+['exec','-i','voice-postgres-0','-c','postgres','--']
IDENTITY_SQL="SELECT json_build_object('system_identifier',(SELECT system_identifier::text FROM pg_catalog.pg_control_system()),'server_start',pg_catalog.pg_postmaster_start_time())"
IDENTITY_ARGS=['exec','voice-postgres-0','-c','postgres','--','sh','-ceu',
    'export PGPASSWORD="$POSTGRES_PASSWORD"; export PGOPTIONS="-c default_transaction_read_only=on -c search_path=pg_catalog,public,pg_temp"; exec psql -h 127.0.0.1 -X -qAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d postgres -c "$1"',
    '--',IDENTITY_SQL]


def guard_identity(session):
    """Same original physical engine, while approved new databases may exist."""
    import realtime_bootstrap
    from stage_runtime import Kube
    plan=owner.load_plan(session);owner._phase_continuity(session,plan);owner._no_pending(session)
    record=owner._read(owner._base(session)/'archive-record.json')
    old=owner._record(session,'catalog.json',record['catalog_ref'],'voice-staging-cohort-catalog-v1')
    kube=Kube();namespace,engine=realtime_bootstrap.engine_snapshot(kube)
    value=kube.run(list(IDENTITY_ARGS))
    current={'namespace_uid':namespace,'pod_uid':engine['pod']['uid'],'pvc_uid':engine['pvc']['uid'],
        'pv_uid':engine['pv']['uid'],'container_id':engine['pod']['container_id'],
        'restart_count':engine['pod']['restart_count'],'system_identifier':value.get('system_identifier')}
    if (set(value)!={'system_identifier','server_start'} or current!=old['engine']
        or value['server_start']!=old['server_start'] or namespace!=plan['namespace_uid']
        or realtime_bootstrap.engine_snapshot(kube)!=(namespace,engine)):
        raise Blocked('cohort_postgres_changed')
    if owner._record(session,'catalog.json',record['catalog_ref'],'voice-staging-cohort-catalog-v1')!=old:
        raise Blocked('cohort_postgres_changed')
    session.check()
    return {'engine':copy.deepcopy(current),'server_start':value['server_start']}


def _query(session,database,query,label):
    if not isinstance(database,str) or not SAFE.fullmatch(database):raise Blocked('cohort_database_invalid')
    # These are observation queries, never migration DDL. Pin catalog lookup
    # ahead of inherited role/database paths so public overloads cannot replace
    # engine, role or catalog identities in the preservation proof.
    body=('BEGIN READ ONLY;\nSET LOCAL search_path=pg_catalog,public,pg_temp;\n'+query+'\nCOMMIT;\n').encode()
    return owner.private._json(owner._observation(session,label,EXEC+['sh','-ceu',PSQL,'--',database],body,limit=16<<20,timeout=60))


def catalog(session):
    import realtime_bootstrap
    from stage_runtime import Kube
    owner.revalidate_paused(session)
    namespace,engine=realtime_bootstrap.engine_snapshot(Kube())
    value=_query(session,'postgres',CATALOG_SQL,'catalog')
    if realtime_bootstrap.engine_snapshot(Kube())!=(namespace,engine):raise Blocked('cohort_postgres_changed')
    plan=owner.load_plan(session)
    if namespace!=plan['namespace_uid']:raise Blocked('cohort_namespace_changed')
    return validate_catalog(value,engine,namespace,session.operation,owner.digest(plan))


def validate_catalog(value,raw_engine,namespace_uid,operation,plan_sha):
    try:
        if (set(value)!={'system_identifier','server_start','can_read','databases'} or value['can_read'] is not True
            or not re.fullmatch('[0-9]{1,20}',value['system_identifier']) or not isinstance(value['server_start'],str)
            or not value['server_start'] or not isinstance(value['databases'],list) or not 1<=len(value['databases'])<=128):raise ValueError()
        all_db={};backup={};excluded={};properties={}
        fields={'name','oid','allowconn','template','encoding','collate','ctype','locale_provider','icu_locale','connection_limit','owner','acl','connect','sessions','prepared'}
        for row in value['databases']:
            name=row['name']
            if (set(row)!=fields or not isinstance(name,str) or not SAFE.fullmatch(name) or name=='roles' or name in all_db
                or type(row['oid']) is not int or row['oid']<=0 or any(type(row[k]) is not bool for k in ('allowconn','template','connect'))
                or any(not isinstance(row[k],str) or not row[k] for k in ('encoding','collate','ctype','locale_provider','owner'))
                or type(row['connection_limit']) is not int or row['icu_locale'] is not None and not isinstance(row['icu_locale'],str)
                or row['acl'] is not None and not isinstance(row['acl'],str)
                or any(type(row[k]) is not int or row[k]!=0 for k in ('sessions','prepared'))):raise ValueError()
            all_db[name]=row['oid']
            properties[name]={k:row[k] for k in ('template','allowconn','encoding','collate','ctype','locale_provider','icu_locale','connection_limit','owner','acl')}
            if row['allowconn']:
                if row['connect'] is not True:raise ValueError()
                backup[name]=row['oid']
            else:excluded[name]={'oid':row['oid'],'template':row['template'],'reason':'connections_disabled_zero_activity'}
        if not backup or len(set(all_db.values()))!=len(all_db):raise ValueError()
        engine={'namespace_uid':namespace_uid,'pod_uid':raw_engine['pod']['uid'],'pvc_uid':raw_engine['pvc']['uid'],
            'pv_uid':raw_engine['pv']['uid'],'container_id':raw_engine['pod']['container_id'],
            'restart_count':raw_engine['pod']['restart_count'],'system_identifier':value['system_identifier']}
        return {'schema':'voice-staging-cohort-catalog-v1','operation':operation,'plan_sha256':plan_sha,
            'namespace_uid':namespace_uid,'engine':engine,'server_start':value['server_start'],'all_databases':all_db,
            'databases':backup,'excluded':excluded,'properties':properties}
    except (KeyError,TypeError,ValueError):raise Blocked('cohort_catalog_invalid') from None


def metadata(session,catalog_record):
    restore=owner.shared_module(session,'story_media_backup_cohort_restore')
    common=owner.shared_module(session,'story_media_backup_restore')
    unmanaged=owner.shared_module(session,'cohort_unmanaged_actual')
    cohort=owner.shared_module(session,'story_media_backup_cohort')
    approved,_,_=owner._source(session,owner.load_plan(session))
    catalogs={};plans={};roles=_query(session,'postgres',common.role_catalog_sql(),'roles');migration_roles={}
    for db in sorted(catalog_record['databases']):
        query=(unmanaged.CATALOG_SQL if db not in cohort.ALLOWED_DATABASES else restore.AUTH_CATALOG if db=='auth_db'
            else restore.GATEWAY_CATALOG if db=='gateway_db' else common.CATALOG)
        result=_query(session,db,query,'metadata');catalogs[db]=result
        if db not in cohort.ALLOWED_DATABASES:
            unmanaged.validate_source_catalog(result,catalog_record['properties'][db])
            plans[db]={'engine':'unmanaged','from':None,'to':None,'files':{}};continue
        prefix=('src/backend/auth/src/main/resources/db/migration/' if db=='auth_db' else 'src/backend/migrations/'+db+'/')
        files={name.removeprefix(prefix):sha for name,sha in approved['source_files'].items() if name.startswith(prefix) and name.endswith('.sql')}
        if db=='gateway_db':
            plans[db]={'engine':'unmanaged','from':None,'to':None,'files':files};continue
        pattern=restore.FLYWAY_SQL if db=='auth_db' else restore.GO_SQL
        if not files or any(pattern.fullmatch(name) is None for name in files):raise Blocked('cohort_migration_source_invalid')
        target=max(int(pattern.fullmatch(name).group(1)) for name in files if db=='auth_db' or name.endswith('.up.sql'))
        history=result['history']
        if db=='auth_db':
            if not history or any(row.get('success') is not True for row in history):raise Blocked('cohort_schema_invalid')
            current=max(int(row['version']) for row in history if row['version'] is not None)
        else:
            if not isinstance(history,list) or len(history)!=1 or history[0].get('dirty') is not False:raise Blocked('cohort_schema_invalid')
            current=history[0]['version']
        plan={'engine':'flyway' if db=='auth_db' else 'golang-migrate','from':current,'to':target,'files':files}
        mode=restore.classify_plan(db,plan,True);plans[db]=plan
        if mode=='migrate':
            role=result['database_owner'];matches=[row for row in roles['roles'] if row['name']==role]
            if (len(matches)!=1 or matches[0]['login'] is not True or matches[0]['super'] is not False
                or result['schema_owner']!=role):raise Blocked('cohort_migration_role_unqualified')
            migration_roles[db]={'role':role,'database_oid':catalog_record['databases'][db],
                'source_files_sha256':owner.digest(files),'catalog_sha256':owner.digest(result)}
    source={'catalog':catalogs,'roles':roles,'migrations':plans}
    restore.migration_inventory(owner.guard.ROOT/'installed'/'sources'/session.operation,sorted(plans),plans,True)
    if catalog(session)!=catalog_record:raise Blocked('cohort_catalog_changed')
    return {'schema':'voice-staging-cohort-metadata-v1','operation':session.operation,
        'plan_sha256':owner.digest(owner.load_plan(session)),'catalog_sha256':owner.digest(catalog_record),
        'source':source,'migration_roles':migration_roles}


def stream_dump(session,member,stream,record):
    """One fixed kubectl/pg_dump process, bounded and never replayed."""
    catalog_record=owner.load_catalog(session,record['catalog_ref'])
    if member!='roles' and member not in catalog_record['databases']:raise Blocked('cohort_dump_member_invalid')
    script=('export PGPASSWORD="$POSTGRES_PASSWORD"; exec pg_dumpall --roles-only --username="$POSTGRES_USER"'
        if member=='roles' else 'export PGPASSWORD="$POSTGRES_PASSWORD"; exec pg_dump --format=custom --create --username="$POSTGRES_USER" --dbname="$1"')
    argv=EXEC+['sh','-ceu',script]+([] if member=='roles' else ['--',member])
    path=owner._base(session)/'effects'/('dump-'+member+'.json')
    owner.private._private_directory(path.parent);owner._no_pending(session)
    intent={'schema':'voice-staging-cohort-effect-v1','operation':session.operation,'effect_id':'dump-'+member,
        'binding':session.binding(),'argv':argv,'body_sha256':hashlib.sha256(b'').hexdigest(),'status':'PENDING',
        'catalog_sha256':owner.digest(catalog_record),'attempt_nonce':owner.secrets.token_hex(16)}
    owner._write(path,intent)
    permit={'effect_id':intent['effect_id'],'argv':argv,'body_sha256':intent['body_sha256']}
    if getattr(session,'_cohort_permit',None) is not None:raise Blocked('cohort_permit_busy')
    owner._guard(session)
    if owner._read(path)!=intent:raise Blocked('cohort_effect_changed')
    session._cohort_permit=permit
    child=None;selector=selectors.DefaultSelector();count=0;sha=hashlib.sha256();deadline=time.monotonic()+900
    sink=stream.fileno();blocking=os.get_blocking(sink);pending=b''
    try:
        commands.authority_guard(argv)
        if session._cohort_permit is not None:raise Blocked('cohort_permit_not_consumed')
        child=subprocess.Popen(argv,stdin=subprocess.DEVNULL,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,
            env={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'},start_new_session=True)
        os.set_blocking(child.stdout.fileno(),False);os.set_blocking(sink,False)
        selector.register(child.stdout,selectors.EVENT_READ,'source')
        while selector.get_map():
            if time.monotonic()>=deadline:raise Blocked('cohort_dump_timeout')
            session.check()
            for key,_ in selector.select(.1):
                if key.data=='sink':
                    size=os.write(sink,pending);pending=pending[size:]
                    if not pending:
                        selector.unregister(sink);selector.register(child.stdout,selectors.EVENT_READ,'source')
                else:
                    block=os.read(key.fd,1<<20)
                    if not block:selector.unregister(key.fileobj);break
                    count+=len(block)
                    if count>64<<30:raise Blocked('cohort_dump_limit')
                    sha.update(block);pending=block
                    selector.unregister(child.stdout);selector.register(sink,selectors.EVENT_WRITE,'sink')
        if child.wait(timeout=max(.01,deadline-time.monotonic()))!=0 or not count:raise Blocked('cohort_dump_failed')
    finally:
        session._cohort_permit=None;selector.close();os.set_blocking(sink,blocking)
        if child is not None:
            if child.poll() is None:os.killpg(child.pid,signal.SIGKILL);child.wait(timeout=2)
            child.stdout.close()
    # No read-based adoption: only this process's terminal success can reach DONE.
    if owner._read(path)!=intent:raise Blocked('cohort_effect_changed')
    owner._terminal_reads[session]=copy.deepcopy(intent)
    try:after=catalog(session)
    finally:owner._terminal_reads.pop(session,None)
    if after!=catalog_record:raise Blocked('cohort_catalog_changed')
    receipt={'operation_id':session.context['operation_id'],'snapshot_id':record['binding']['snapshot_id'],
        'member':member,**{key:catalog_record['engine'][key] for key in ('pod_uid','pvc_uid','pv_uid','container_id','restart_count')},
        'server_start':catalog_record['server_start'],'database_oid':catalog_record['databases'].get(member),
        'attempt_nonce':intent['attempt_nonce'],'exit':0,'scope':'dump-terminal-only'}
    owner.save(path,{**intent,'status':'DONE','receipt':receipt,'bytes':count,'plaintext_sha256':sha.hexdigest()})
    return receipt
