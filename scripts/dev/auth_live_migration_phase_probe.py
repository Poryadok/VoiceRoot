"""Fixed guarded read-only Auth ledger/session observer; no credentials returned."""
import copy
import json
import os
import selectors
import subprocess
import tempfile
import time
import base64
import hmac

import full_cohort_schema_metadata as metadata
import writer_cut_pg_probe as cut_probe


class ProbeError(RuntimeError): pass


def _login(password):
    script=('IFS= read -r encoded; PGPASSWORD="$(printf %s "$encoded" | base64 -d)"; '
            'export PGPASSWORD; export PGAPPNAME=voice-auth-credential-proof; '
            'export PGOPTIONS="-c default_transaction_read_only=on -c statement_timeout=5000 -c lock_timeout=3000"; '
            "exec psql -h voice-postgres -p 5432 -X -qAt -v ON_ERROR_STOP=1 -U voice -d auth_db -c \"SELECT json_build_object('database',current_database(),'database_oid',(SELECT oid::bigint FROM pg_database WHERE datname=current_database()),'server_start',pg_postmaster_start_time(),'role',current_user)\"")
    args=['kubectl','--kubeconfig','/home/pmd/.kube/config','--context','default','--namespace','voice-dev',
          '--request-timeout=15s','exec','-i','voice-postgres-0','-c','postgres','--',
          'env','-i','PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin','sh','-ec',script]
    result=subprocess.run(args,input=base64.b64encode(password)+b'\n',stdout=subprocess.PIPE,
                          stderr=subprocess.DEVNULL,timeout=20,check=True)
    if len(result.stdout)>4096: raise ValueError()
    return json.loads(result.stdout)


def verify_login(client, scope, guard):
    """Verify actual TCP login with the private Job Secret and fixed engine identity."""
    try:
        frozen=copy.deepcopy(scope)
        if (not isinstance(frozen,dict) or set(frozen)!={'engine','database_oid','server_start'}
                or type(frozen['database_oid']) is not int or frozen['database_oid']<=0
                or not isinstance(frozen['server_start'],str) or not frozen['server_start']): raise ValueError()
        snapshot=metadata.kube_engine_snapshot(metadata.linux_private_bytes,guard)
        def read():
            metadata._fence(guard)
            value=client.call('get','secret','voice-app-secrets','-o','json')
            metadata._fence(guard)
            info=value['metadata']
            if (info.get('name')!='voice-app-secrets' or info.get('namespace')!='voice-dev'
                    or any(not isinstance(info.get(k),str) or not info[k] for k in ('uid','resourceVersion'))): raise ValueError()
            raw=base64.b64decode(value['data']['POSTGRES_PASSWORD'],validate=True)
            if not raw or len(raw)>4096 or any(ch in raw for ch in (b'\x00',b'\n',b'\r')): raise ValueError()
            raw.decode('utf-8')
            return {'uid':info['uid'],'resourceVersion':info['resourceVersion']},raw
        if snapshot()!=frozen['engine']: raise ValueError()
        identity,password=read(); metadata._fence(guard); result=_login(password); metadata._fence(guard)
        second,password2=read()
        if (identity!=second or not hmac.compare_digest(password,password2) or snapshot()!=frozen['engine']
                or scope!=frozen or result!={'database':'auth_db','database_oid':frozen['database_oid'],
                                            'server_start':frozen['server_start'],'role':'voice'}): raise ValueError()
        metadata._fence(guard)
        return identity
    except BaseException:
        raise ProbeError('Auth Job credential proof unavailable') from None


def _query(sql):
    script=('export PGPASSWORD="$POSTGRES_PASSWORD"; export PGAPPNAME=voice-auth-migration-observe; '
            'export PGOPTIONS="-c default_transaction_read_only=on -c statement_timeout=5000 -c lock_timeout=3000"; '
            'exec psql -h 127.0.0.1 -X -qAt -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d auth_db')
    args=['kubectl','--kubeconfig','/home/pmd/.kube/config','--context','default','--namespace','voice-dev',
          '--request-timeout=15s','exec','-i','voice-postgres-0','-c','postgres','--','sh','-ec',script]
    with tempfile.TemporaryFile() as private_input:
        private_input.write(sql.encode('utf-8')); private_input.seek(0)
        process=subprocess.Popen(args,stdin=private_input,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL)
        selector=selectors.DefaultSelector(); chunks=[]; total=0; deadline=time.monotonic()+20
        try:
            selector.register(process.stdout,selectors.EVENT_READ)
            while selector.get_map():
                left=deadline-time.monotonic()
                if left<=0 or not selector.select(left): raise ValueError()
                chunk=os.read(process.stdout.fileno(),min(8192,262145-total))
                if not chunk: selector.unregister(process.stdout); break
                total+=len(chunk)
                if total>262144: raise ValueError()
                chunks.append(chunk)
            if process.wait(timeout=max(.01,deadline-time.monotonic())): raise ValueError()
            return json.loads(b''.join(chunks))
        finally:
            selector.close()
            if process.poll() is None: process.kill(); process.wait()
            process.stdout.close()


def inspect(scope, guard):
    try:
        frozen=copy.deepcopy(scope)
        if (not isinstance(frozen,dict) or set(frozen)!={'engine','databases','database_oids','server_start'}
                or not callable(guard) or not isinstance(frozen['databases'],list)
                or frozen['databases']!=sorted(set(frozen['databases'])) or 'auth_db' not in frozen['databases']
                or any(db not in cut_probe.ALLOWED for db in frozen['databases'])
                or set(frozen['database_oids'])!=set(frozen['databases'])
                or any(type(v) is not int or v<=0 for v in frozen['database_oids'].values())
                or len(set(frozen['database_oids'].values()))!=len(frozen['databases'])):
            raise ValueError()
        engine=frozen['engine']
        if (not isinstance(engine,dict) or set(engine)!=set(metadata.ENGINE_IDENTITY)
                or engine['namespace']!='voice-dev' or engine['pod_name']!='voice-postgres-0'
                or engine['container_name']!='postgres' or type(engine['restart_count']) is not int
                or engine['restart_count']<0 or not isinstance(frozen['server_start'],str) or not frozen['server_start']):
            raise ValueError()
        snapshot=metadata.kube_engine_snapshot(metadata.linux_private_bytes,guard)
        metadata._fence(guard)
        if snapshot()!=engine: raise ValueError()
        databases=','.join("'"+db+"'" for db in frozen['databases'])
        pairs=','.join("('"+db+"',"+str(frozen['database_oids'][db])+"::oid)" for db in frozen['databases'])
        sql=("SELECT json_build_object('database',current_database(),'database_oid',(SELECT oid::bigint FROM pg_database WHERE datname=current_database()),"
             "'server_start',pg_postmaster_start_time(),'role',current_user,'read_only',current_setting('transaction_read_only'),"
             "'can_read',pg_has_role(current_user,'pg_read_all_stats','MEMBER'),"
             "'matched_databases',(SELECT count(*) FROM pg_database d JOIN (VALUES "+pairs+") e(name,oid) ON d.datname=e.name AND d.oid=e.oid),"
             "'prepared',(SELECT count(*) FROM pg_prepared_xacts WHERE database=ANY(ARRAY["+databases+"])),"
             "'other_ledger',to_regclass('public.schema_migrations') IS NOT NULL,"
             "'history',(SELECT coalesce(json_agg(row_to_json(h) ORDER BY installed_rank),'[]'::json) FROM public.flyway_schema_history h),"
             "'sessions',(SELECT coalesce(json_agg(row_to_json(s) ORDER BY s.pid),'[]'::json) FROM (SELECT pid,backend_start,usename,datname,application_name,client_addr::text,xact_start FROM pg_stat_activity WHERE datname=ANY(ARRAY["+databases+"]) AND pid<>pg_backend_pid() ORDER BY pid LIMIT 101) s))")
        metadata._fence(guard); value=_query(sql); metadata._fence(guard)
        if (not isinstance(value,dict) or set(value)!={'database','database_oid','server_start','role','read_only',
                'can_read','matched_databases','prepared','other_ledger','history','sessions'}
                or value['database']!='auth_db' or value['database_oid']!=frozen['database_oids']['auth_db']
                or value['server_start']!=frozen['server_start'] or value['role']!='voice'
                or value['read_only']!='on' or value['can_read'] is not True
                or value['matched_databases']!=len(frozen['databases']) or type(value['prepared']) is not int
                or value['prepared']<0 or value['other_ledger'] is not False
                or not isinstance(value['history'],list) or not isinstance(value['sessions'],list)
                or len(value['sessions'])>100 or snapshot()!=engine or scope!=frozen):
            raise ValueError()
        metadata._fence(guard)
        return value
    except BaseException:
        raise ProbeError('guarded Auth observation unavailable') from None
