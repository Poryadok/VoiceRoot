"""Sample all sessions on an admitted affected-DB set under the DEV fence.

This does not itself select writers, establish ongoing quiescence or prove
unsettled external I/O. WriterCut keeps the actual writer fence in force and
repeats this observer at each relevant boundary.
"""
import copy
import json
import full_cohort_schema_metadata as metadata

ALLOWED = frozenset(('auth_db','bot_db','chat_db','file_db','game_integration_db','gateway_db',
    'matchmaking_db','messaging_db','moderation_db','notification_db','realtime_db','role_db',
    'search_db','social_db','space_db','story_db','subscription_db','user_db','voice_db'))


class ProbeError(RuntimeError): pass


def inspect(scope, guard):
    try:
        if (not isinstance(scope,dict) or set(scope)!={'engine','databases','database_oids','server_start'} or not callable(guard)):
            raise ProbeError('invalid scope')
        frozen = copy.deepcopy(scope); engine = frozen['engine']; databases = frozen['databases']
        if (not isinstance(engine,dict) or any(not engine.get(k) for k in metadata.ENGINE_IDENTITY if k!='restart_count') or
                type(engine.get('restart_count')) is not int or engine['restart_count']<0 or
                engine.get('namespace')!='voice-dev' or engine.get('pod_name')!='voice-postgres-0' or
                engine.get('container_name')!='postgres' or not isinstance(databases,list) or not databases or
                databases!=sorted(set(databases)) or any(not isinstance(db,str) or db not in ALLOWED for db in databases) or
                not isinstance(frozen['database_oids'],dict) or set(frozen['database_oids'])!=set(databases) or
                any(type(oid) is not int or oid<=0 for oid in frozen['database_oids'].values()) or
                len(set(frozen['database_oids'].values()))!=len(databases) or
                not isinstance(frozen['server_start'],str) or not frozen['server_start']):
            raise ProbeError('invalid pinned engine or database set')
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
