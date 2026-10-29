"""Bounded actual Realtime role login and read-only lifecycle schema check.

Caller must authenticate source Store.Ready identity, created DB OID, Secret,
and Job result; this observation does not prove rollout or peer readiness.
"""
import re
import subprocess


class RecoveryRequired(RuntimeError):
    pass


SQL = b'''SELECT id,runtime_identity,state,opened_at,closed_at FROM realtime_incarnations LIMIT 0;
SELECT chat_id,space_id,scope_known,generation,state,operation_id,updated_at FROM realtime_chat_gates LIMIT 0;
SELECT id,incarnation_id,chat_id,generation,state,admitted_at,settled_at FROM realtime_admissions LIMIT 0;
SELECT space_id,operation_id,generation,kind,request_bytes,request_sha256,response_bytes,completed_at FROM realtime_lifecycle_operations LIMIT 0;
SELECT space_id,operation_id,schedule_generation,generation,state,manifest_id,manifest_sha256,item_count,received_pages,received_items,sealed,freeze_complete FROM realtime_manifest_heads LIMIT 0;
SELECT space_id,operation_id,chat_id FROM realtime_manifest_items LIMIT 0;
SELECT version,dirty,(SELECT oid FROM pg_catalog.pg_database WHERE datname=current_database()),current_user FROM schema_migrations LIMIT 2;
'''


def check(password, database_oid, expected_engine, guard, snapshot):
    try:
        if (not isinstance(password,bytes) or not re.fullmatch(rb'[A-Za-z0-9_-]{32,128}',password) or
                type(database_oid) is not int or database_oid<=0 or not isinstance(expected_engine,dict) or not expected_engine):
            raise RecoveryRequired('invalid pinned input')
        guard()
        if snapshot()!=expected_engine: raise RecoveryRequired('engine changed')
        guard()
        script = ('IFS= read -r PGPASSWORD; export PGPASSWORD; '
                  'export PGOPTIONS="-c default_transaction_read_only=on -c statement_timeout=5000 -c lock_timeout=3000"; '
                  'exec psql -h voice-postgres -X -qAt -v ON_ERROR_STOP=1 -U realtime -d realtime_db')
        args = ['kubectl','--kubeconfig','/home/pmd/.kube/config','--context','default','--namespace','voice-dev',
                '--request-timeout=15s','exec','-i','voice-postgres-0','-c','postgres','--','sh','-ec',script]
        result = subprocess.run(args,input=password+b'\n'+SQL,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,timeout=25)
        guard()
        if (result.returncode or result.stdout != ('1|f|'+str(database_oid)+'|realtime\n').encode() or
                snapshot()!=expected_engine):
            raise RecoveryRequired('login or schema observation failed')
        guard()
        return {'database_oid':database_oid,'version':1,'dirty':False,'scope':'realtime-login-schema-readiness'}
    except BaseException:
        raise RecoveryRequired('Realtime schema not verified') from None
