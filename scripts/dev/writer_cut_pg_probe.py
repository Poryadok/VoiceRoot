"""Sample all sessions on an admitted affected-DB set under the DEV fence.

This does not itself select writers, establish ongoing quiescence or prove
unsettled external I/O. WriterCut keeps the actual writer fence in force and
repeats this observer at each relevant boundary.
"""
import copy
import ipaddress
import json
import re
import full_cohort_schema_metadata as metadata

ALLOWED = frozenset(('auth_db','bot_db','chat_db','file_db','game_integration_db','gateway_db',
    'matchmaking_db','messaging_db','moderation_db','notification_db','realtime_db','role_db',
    'search_db','social_db','space_db','story_db','subscription_db','user_db','voice_db'))


class ProbeError(RuntimeError): pass


def inspect_resumed(cut, inventory_phase, token):
    from writer_cut_resumed_probe import inspect_resumed as observe
    return observe(cut, inventory_phase, token)


def _validate_scope(scope, guard):
    if not isinstance(scope,dict) or set(scope)!={'engine','databases','database_oids','server_start'} or not callable(guard):
        raise ProbeError('invalid scope')
    frozen = copy.deepcopy(scope); engine = frozen['engine']; databases = frozen['databases']
    if (not isinstance(engine,dict) or any(not engine.get(k) for k in metadata.ENGINE_IDENTITY if k!='restart_count') or
            type(engine.get('restart_count')) is not int or engine['restart_count']<0 or
            engine.get('namespace')!='voice-dev' or engine.get('pod_name')!='voice-postgres-0' or
            engine.get('container_name')!='postgres' or not isinstance(databases,list) or not databases or
            databases!=sorted(set(databases)) or any(not isinstance(db,str) or not metadata.SAFE.fullmatch(db) for db in databases) or
            not isinstance(frozen['database_oids'],dict) or set(frozen['database_oids'])!=set(databases) or
            any(type(oid) is not int or oid<=0 for oid in frozen['database_oids'].values()) or
            len(set(frozen['database_oids'].values()))!=len(databases) or
            not isinstance(frozen['server_start'],str) or not frozen['server_start']):
        raise ProbeError('invalid pinned engine or database set')
    return frozen


def inspect(scope, guard):
    try:
        frozen = _validate_scope(scope, guard); engine = frozen['engine']; databases = frozen['databases']
        snapshot = metadata.kube_engine_snapshot(metadata.linux_private_bytes,guard)
        metadata._fence(guard)
        if snapshot()!=engine: raise ProbeError('engine changed')
        array = ','.join("'"+db+"'" for db in databases)
        pairs = ','.join("('"+db+"',"+str(frozen['database_oids'][db])+"::oid)" for db in databases)
        sql = ("SELECT json_build_object('server_start',pg_catalog.pg_postmaster_start_time(),"
               "'can_read',pg_catalog.pg_has_role(current_user,'pg_read_all_stats','MEMBER'),"
               "'matched_databases',(SELECT count(*) FROM pg_catalog.pg_database d JOIN (VALUES "+pairs+") AS e(name,oid) ON d.datname=e.name AND d.oid=e.oid),"
               "'prepared',(SELECT count(*) FROM pg_catalog.pg_prepared_xacts WHERE database = ANY(ARRAY["+array+"])),"
               "'sessions',count(*),'transactions',count(*) FILTER (WHERE xact_start IS NOT NULL)) "
               "FROM pg_catalog.pg_stat_activity WHERE datname = ANY(ARRAY["+array+"]) AND pid <> pg_catalog.pg_backend_pid()")
        args = ['kubectl','--kubeconfig','/home/pmd/.kube/config','--context','default','--namespace','voice-dev',
                '--request-timeout=15s','exec','voice-postgres-0','-c','postgres','--','sh','-ec',
                metadata.PSQL_SCRIPT,'--','postgres',sql]
        metadata._fence(guard)
        response = json.loads(metadata.linux_psql(args))
        metadata._fence(guard)
        if (not isinstance(response,dict) or set(response)!={'server_start','can_read','matched_databases','prepared','sessions','transactions'} or
                response['server_start']!=frozen['server_start'] or response['can_read'] is not True or
                type(response['matched_databases']) is not int or response['matched_databases']!=len(databases) or
                type(response['prepared']) is not int or response['prepared']<0 or
                type(response['sessions']) is not int or type(response['transactions']) is not int or
                response['sessions']<0 or not 0<=response['transactions']<=response['sessions']):
            raise ProbeError('session observation invalid')
        if snapshot()!=engine or scope!=frozen: raise ProbeError('engine or scope changed')
        metadata._fence(guard)
        return {'scope':frozen,'active_sessions':response['sessions'],
                'active_transactions':response['transactions']+response['prepared']}
    except BaseException:
        raise ProbeError('writer session observation unavailable') from None


def inspect_space(scope,identity,guard):
    """Observe the exact owned native installer; all other client sessions remain blockers."""
    try:
        frozen=_validate_scope(scope,guard); item=copy.deepcopy(identity)
        keys={'old_oid','new_oid','role','pod_uid','pod_ip','run_id','job_kind'}
        if (not isinstance(item,dict) or set(item)!=keys or type(item['old_oid']) is not int or item['old_oid']<=0 or
                (item['new_oid'] is not None and (type(item['new_oid']) is not int or item['new_oid']<=0 or item['new_oid']==item['old_oid'])) or
                not isinstance(item['role'],str) or not metadata.SAFE.fullmatch(item['role']) or
                item['job_kind'] not in ('provision','verify')):raise ValueError()
        uuid=r'[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}'
        if item['run_id'] is not None and (not isinstance(item['run_id'],str) or not re.fullmatch(uuid,item['run_id'])):raise ValueError()
        has_pod=item['pod_uid'] is not None or item['pod_ip'] is not None
        if has_pod and (not isinstance(item['pod_uid'],str) or not item['pod_uid'] or
                not isinstance(item['pod_ip'],str) or str(ipaddress.ip_address(item['pod_ip']))!=item['pod_ip']):raise ValueError()
        if 'postgres' not in frozen['database_oids']:raise ValueError()
        if item['new_oid'] is not None and frozen['database_oids'].get('space_db')!=item['new_oid']:raise ValueError()
        names=frozen['databases'];pairs=','.join("('"+db+"',"+str(frozen['database_oids'][db])+"::oid)" for db in names)
        name_check=("application_name='native-installer-"+item['run_id']+"'" if item['job_kind']=='provision' and item['run_id'] is not None
                    else "application_name ~ '^native-installer-[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'")
        match=("COALESCE(datname IN ('postgres','space_db') AND usename='"+item['role']+"' AND "+name_check+
               " AND client_addr='"+item['pod_ip']+"'::inet,false)") if has_pod else 'false'
        sql=("SELECT json_build_object('server_start',pg_catalog.pg_postmaster_start_time(),"
             "'can_read',pg_catalog.pg_has_role(current_user,'pg_read_all_stats','MEMBER'),"
             "'matched_databases',(SELECT count(*) FROM pg_catalog.pg_database d JOIN (VALUES "+pairs+
             ") AS e(name,oid) ON d.datname=e.name AND d.oid=e.oid),"
             "'prepared',(SELECT count(*) FROM pg_catalog.pg_prepared_xacts),"
             "'sessions',count(*) FILTER (WHERE NOT "+match+"),"
             "'transactions',count(*) FILTER (WHERE NOT "+match+" AND xact_start IS NOT NULL),"
             "'attributed',COALESCE(json_agg(json_build_object('pid',pid,'backend_start',backend_start,"
             "'database',datname,'role',usename,'application_name',application_name,'client_addr',pg_catalog.host(client_addr))) "
             "FILTER (WHERE "+match+"),'[]'::json)) FROM pg_catalog.pg_stat_activity "
             "WHERE backend_type='client backend' AND pid <> pg_catalog.pg_backend_pid()")
        args=['kubectl','--kubeconfig','/home/pmd/.kube/config','--context','default','--namespace','voice-dev',
              '--request-timeout=15s','exec','voice-postgres-0','-c','postgres','--','sh','-ec',metadata.PSQL_SCRIPT,'--','postgres',sql]
        snapshot=metadata.kube_engine_snapshot(metadata.linux_private_bytes,guard)
        metadata._fence(guard)
        if snapshot()!=frozen['engine']:raise ValueError()
        response=json.loads(metadata.linux_psql(args));metadata._fence(guard)
        if (not isinstance(response,dict) or set(response)!={'server_start','can_read','matched_databases','prepared','sessions','transactions','attributed'} or
                response['server_start']!=frozen['server_start'] or response['can_read'] is not True or
                response['matched_databases']!=len(names) or type(response['matched_databases']) is not int or
                any(type(response[k]) is not int or response[k]<0 for k in ('prepared','sessions','transactions')) or
                response['transactions']>response['sessions'] or not isinstance(response['attributed'],list) or
                len(response['attributed'])>64 or (not has_pod and response['attributed'])):raise ValueError()
        seen=set();apps=set()
        for session in response['attributed']:
            if (not isinstance(session,dict) or set(session)!={'pid','backend_start','database','role','application_name','client_addr'} or
                    type(session['pid']) is not int or session['pid']<=0 or session['pid'] in seen or
                    not isinstance(session['backend_start'],str) or not session['backend_start'] or
                    session['database'] not in ('postgres','space_db') or session['role']!=item['role'] or
                    session['client_addr']!=item['pod_ip'] or not isinstance(session['application_name'],str) or
                    not re.fullmatch('native-installer-'+uuid,session['application_name'])):raise ValueError()
            if item['job_kind']=='provision' and item['run_id'] is not None and session['application_name']!='native-installer-'+item['run_id']:raise ValueError()
            seen.add(session['pid']);apps.add(session['application_name'])
        if len(apps)>1 or snapshot()!=frozen['engine'] or scope!=frozen or identity!=item:raise ValueError()
        metadata._fence(guard)
        return {'scope':frozen,'active_sessions':response['sessions'],
                'active_transactions':response['transactions']+response['prepared'],'attributed_sessions':response['attributed']}
    except BaseException:raise ProbeError('Space session observation unavailable') from None


def inspect_migration(scope, identity, guard):
    """Count non-job sessions while observing an exact owned migration Job.

    The caller must authenticate the Job/Pod and derive this identity from
    its protected phase; this query alone cannot establish Pod ownership.
    """
    try:
        frozen = _validate_scope(scope, guard); item = copy.deepcopy(identity)
        if (not isinstance(item,dict) or set(item)!={'database','database_oid','role','application_name','pod_ip','pod_uid'} or
                type(item['database_oid']) is not int or item['database_oid']<=0 or
                not isinstance(item['application_name'],str)):
            raise ProbeError('migration identity invalid')
        realtime = (item['database']=='realtime_db' and item['role']=='realtime' and
                    re.fullmatch(r'voice-realtime-migrate-[0-9a-f]{32}',item['application_name']))
        auth = (item['database']=='auth_db' and item['role']=='voice' and
                re.fullmatch(r'voice-au-au-[0-9a-f]{28}-migrate-auth',item['application_name']))
        import full_cohort_go_migration_plan
        go = (item['database'] in full_cohort_go_migration_plan.DATABASES and
              isinstance(item['role'],str) and re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]{0,62}',item['role']) and
              re.fullmatch(r'voice-go-migrate-'+re.escape(item['database'][:-3])+r'-[0-9a-f]{32}',item['application_name']))
        if not (realtime or auth or go): raise ProbeError('migration identity invalid')
        has_pod = item['pod_uid'] is not None or item['pod_ip'] is not None
        if has_pod and (not isinstance(item['pod_uid'],str) or not item['pod_uid'] or
                not isinstance(item['pod_ip'],str) or str(ipaddress.ip_address(item['pod_ip']))!=item['pod_ip']):
            raise ProbeError('migration pod invalid')
        oids = copy.deepcopy(frozen['database_oids'])
        if item['database'] in oids and oids[item['database']] != item['database_oid']:
            raise ProbeError('migration database changed')
        oids[item['database']] = item['database_oid']
        if len(set(oids.values())) != len(oids): raise ProbeError('database identity collision')
        names = sorted(oids)
        snapshot = metadata.kube_engine_snapshot(metadata.linux_private_bytes,guard)
        metadata._fence(guard)
        if snapshot()!=frozen['engine']: raise ProbeError('engine changed')
        array = ','.join("'"+db+"'" for db in names)
        pairs = ','.join("('"+db+"',"+str(oids[db])+"::oid)" for db in names)
        match = ("COALESCE(datname='"+item['database']+"' AND usename='"+item['role']+
                 "' AND application_name='"+item['application_name']+"' AND client_addr='"+
                 item['pod_ip']+"'::inet,false)") if has_pod else 'false'
        sql = ("SELECT json_build_object('server_start',pg_catalog.pg_postmaster_start_time(),"
               "'can_read',pg_catalog.pg_has_role(current_user,'pg_read_all_stats','MEMBER'),"
               "'matched_databases',(SELECT count(*) FROM pg_catalog.pg_database d JOIN (VALUES "+pairs+
               ") AS e(name,oid) ON d.datname=e.name AND d.oid=e.oid),"
               "'prepared',(SELECT count(*) FROM pg_catalog.pg_prepared_xacts WHERE database = ANY(ARRAY["+array+
               "])), 'sessions',count(*) FILTER (WHERE NOT "+match+"),"
               "'transactions',count(*) FILTER (WHERE NOT "+match+" AND xact_start IS NOT NULL),"
               "'attributed',COALESCE(json_agg(json_build_object('pid',pid,'backend_start',backend_start,"
               "'database',datname,'role',usename,'application_name',application_name,'client_addr',pg_catalog.host(client_addr))) "
               "FILTER (WHERE "+match+"),'[]'::json)) FROM pg_catalog.pg_stat_activity WHERE datname = ANY(ARRAY["+
               array+"]) AND pid <> pg_catalog.pg_backend_pid()")
        args = ['kubectl','--kubeconfig','/home/pmd/.kube/config','--context','default','--namespace','voice-dev',
                '--request-timeout=15s','exec','voice-postgres-0','-c','postgres','--','sh','-ec',
                metadata.PSQL_SCRIPT,'--','postgres',sql]
        metadata._fence(guard)
        response = json.loads(metadata.linux_psql(args))
        metadata._fence(guard)
        if (not isinstance(response,dict) or set(response)!={'server_start','can_read','matched_databases','prepared','sessions','transactions','attributed'} or
                response['server_start']!=frozen['server_start'] or response['can_read'] is not True or
                type(response['matched_databases']) is not int or response['matched_databases']!=len(names) or
                type(response['prepared']) is not int or response['prepared']<0 or
                type(response['sessions']) is not int or type(response['transactions']) is not int or
                response['sessions']<0 or not 0<=response['transactions']<=response['sessions'] or
                not isinstance(response['attributed'],list) or len(response['attributed'])>64 or
                (not has_pod and response['attributed'])):
            raise ProbeError('session observation invalid')
        seen = set()
        for session in response['attributed']:
            if (not isinstance(session,dict) or set(session)!={'pid','backend_start','database','role','application_name','client_addr'} or
                    type(session['pid']) is not int or session['pid']<=0 or session['pid'] in seen or
                    not isinstance(session['backend_start'],str) or not session['backend_start'] or
                    any(session[k] != item[k] for k in ('database','role','application_name')) or
                    session['client_addr'] != item['pod_ip']):
                raise ProbeError('migration session attribution changed')
            seen.add(session['pid'])
        if snapshot()!=frozen['engine'] or scope!=frozen or identity!=item: raise ProbeError('engine or input changed')
        metadata._fence(guard)
        return {'scope':frozen,'active_sessions':response['sessions'],
                'active_transactions':response['transactions']+response['prepared'],
                'attributed_sessions':response['attributed']}
    except BaseException:
        raise ProbeError('migration session observation unavailable') from None
