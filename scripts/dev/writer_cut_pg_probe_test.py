import json
import unittest
from unittest.mock import patch
import writer_cut_pg_probe as probe


class ProbeTest(unittest.TestCase):
    def setUp(self):
        engine = {key:'fixture' for key in probe.metadata.ENGINE_IDENTITY}
        engine.update(namespace='voice-dev',pod_name='voice-postgres-0',container_name='postgres',restart_count=0)
        self.scope = {'engine':engine,'databases':['chat_db','file_db'],'database_oids':{'chat_db':11,'file_db':12},
                      'server_start':'2026-10-29T00:00:00+00:00'}
        self.result = {'server_start':self.scope['server_start'],'can_read':True,'matched_databases':2,
                       'prepared':0,'sessions':0,'transactions':0}

    def test_exact_scope_and_guarded_zero(self):
        calls = []
        def guard(): calls.append(1)
        with patch.object(probe.metadata,'kube_engine_snapshot',return_value=lambda:self.scope['engine']), \
             patch.object(probe.metadata,'linux_psql',return_value=json.dumps(self.result)) as run:
            receipt = probe.inspect(self.scope,guard)
        self.assertEqual(receipt,{'scope':self.scope,'active_sessions':0,'active_transactions':0})
        self.assertGreaterEqual(len(calls),4)
        self.assertIn('chat_db',run.call_args.args[0][-1])

    def test_catalog_qualified_extra_and_postgres_scope(self):
        scope=self.scope|{'databases':['postgres','custom_db'], 'database_oids':{'postgres':1,'custom_db':2}}
        scope['databases'].sort()
        with patch.object(probe.metadata,'kube_engine_snapshot',return_value=lambda:scope['engine']), \
             patch.object(probe.metadata,'linux_psql',return_value=json.dumps(self.result)):
            self.assertEqual(probe.inspect(scope,lambda:None)['active_sessions'],0)
        scope['databases']=['bad-name']; scope['database_oids']={'bad-name':1}
        with self.assertRaises(probe.ProbeError): probe.inspect(scope,lambda:None)

    def test_scope_privilege_and_engine_fail_closed(self):
        for mutation in ('unknown-db','duplicate','privilege','engine','restart'):
            scope = dict(self.scope); result = dict(self.result)
            if mutation=='unknown-db': scope['databases']=['other_db']
            if mutation=='duplicate': scope['databases']=['file_db','file_db']
            if mutation=='privilege': result['can_read']=False
            if mutation=='restart': result['server_start']='different'
            engine = {'pod_uid':'different'} if mutation=='engine' else scope['engine']
            with self.subTest(mutation=mutation), \
                 patch.object(probe.metadata,'kube_engine_snapshot',return_value=lambda:engine), \
                 patch.object(probe.metadata,'linux_psql',return_value=json.dumps(result)):
                with self.assertRaises(probe.ProbeError): probe.inspect(scope,lambda:None)

    def test_nonzero_counts_are_observed_not_faked(self):
        result = self.result | {'sessions':2,'transactions':1}
        with patch.object(probe.metadata,'kube_engine_snapshot',return_value=lambda:self.scope['engine']), \
             patch.object(probe.metadata,'linux_psql',return_value=json.dumps(result)):
            receipt = probe.inspect(self.scope,lambda:None)
        self.assertEqual((receipt['active_sessions'],receipt['active_transactions']),(2,1))

    def test_prepared_transaction_is_not_hidden(self):
        result = self.result | {'prepared':1}
        with patch.object(probe.metadata,'kube_engine_snapshot',return_value=lambda:self.scope['engine']), \
             patch.object(probe.metadata,'linux_psql',return_value=json.dumps(result)):
            receipt = probe.inspect(self.scope,lambda:None)
        self.assertEqual((receipt['active_sessions'],receipt['active_transactions']),(0,1))

    def test_realtime_migration_session_is_separate_from_other_sessions(self):
        identity = {'database':'realtime_db','database_oid':99,'role':'realtime',
                    'application_name':'voice-realtime-migrate-'+'a'*32,'pod_ip':'10.1.2.3','pod_uid':'pod-uid'}
        result = self.result | {'matched_databases':3, 'sessions':1, 'transactions':1,
            'attributed':[{'pid':42,'backend_start':'2026-10-29T00:01:00+00:00',
                'database':'realtime_db','role':'realtime','application_name':identity['application_name'],
                'client_addr':'10.1.2.3'}]}
        with patch.object(probe.metadata,'kube_engine_snapshot',return_value=lambda:self.scope['engine']), \
             patch.object(probe.metadata,'linux_psql',return_value=json.dumps(result)):
            receipt = probe.inspect_migration(self.scope,identity,lambda:None)
        self.assertEqual((receipt['active_sessions'],receipt['active_transactions']),(1,1))
        self.assertEqual(receipt['attributed_sessions'],result['attributed'])

    def test_go_attribution_rejects_invalid_role_or_unowned_database(self):
        identity={'database':'chat_db','database_oid':11,'role':'chat_login',
                  'application_name':'voice-go-migrate-chat-'+'a'*32,'pod_ip':'10.1.2.3','pod_uid':'pod-uid'}
        result=self.result | {'attributed':[]}
        with patch.object(probe.metadata,'kube_engine_snapshot',return_value=lambda:self.scope['engine']), \
             patch.object(probe.metadata,'linux_psql',return_value=json.dumps(result)):
            self.assertEqual(probe.inspect_migration(self.scope,identity,lambda:None)['active_sessions'],0)
            for changes in ({'role':"x' OR true--"},{'database':'other_db'},
                            {'application_name':'voice-go-migrate-file-'+'a'*32}):
                with self.assertRaises(probe.ProbeError): probe.inspect_migration(self.scope,identity|changes,lambda:None)

    def test_space_native_window_counts_all_other_clients_and_prepared(self):
        scope=self.scope | {'databases':['chat_db','postgres','space_db'],
                           'database_oids':{'chat_db':11,'postgres':1,'space_db':88}}
        run='12345678-1234-4234-8234-123456789abc'
        identity={'old_oid':77,'new_oid':88,'role':'voice','pod_uid':'pod','pod_ip':'10.1.2.3',
                  'run_id':run,'job_kind':'provision'}
        session={'pid':5,'backend_start':'start','database':'space_db','role':'voice',
                 'application_name':'native-installer-'+run,'client_addr':'10.1.2.3'}
        result=self.result | {'matched_databases':3,'prepared':1,'sessions':1,'transactions':0,'attributed':[session]}
        with patch.object(probe.metadata,'kube_engine_snapshot',return_value=lambda:scope['engine']), \
             patch.object(probe.metadata,'linux_psql',return_value=json.dumps(result)) as run_sql:
            receipt=probe.inspect_space(scope,identity,lambda:None)
            self.assertEqual((receipt['active_sessions'],receipt['active_transactions']),(1,1))
            self.assertIn("backend_type='client backend'",run_sql.call_args.args[0][-1])
            for fault in ({'role':"x' OR true--"},{'new_oid':77},{'run_id':'bad'},{'pod_ip':'10.1.2.999'}):
                with self.assertRaises(probe.ProbeError):probe.inspect_space(scope,identity|fault,lambda:None)
            result['attributed'][0]['application_name']='native-installer-aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'
            run_sql.return_value=json.dumps(result)
            with self.assertRaises(probe.ProbeError):probe.inspect_space(scope,identity,lambda:None)

    def test_migration_rejects_attribution_drift_or_invalid_identity(self):
        identity = {'database':'realtime_db','database_oid':99,'role':'realtime',
                    'application_name':'voice-realtime-migrate-'+'a'*32,'pod_ip':'10.1.2.3','pod_uid':'pod-uid'}
        result = self.result | {'matched_databases':3, 'attributed':[{'pid':42,
            'backend_start':'2026-10-29T00:01:00+00:00','database':'realtime_db','role':'realtime',
            'application_name':identity['application_name'],'client_addr':'10.1.2.4'}]}
        with patch.object(probe.metadata,'kube_engine_snapshot',return_value=lambda:self.scope['engine']), \
             patch.object(probe.metadata,'linux_psql',return_value=json.dumps(result)):
            with self.assertRaises(probe.ProbeError): probe.inspect_migration(self.scope,identity,lambda:None)
            with self.assertRaises(probe.ProbeError): probe.inspect_migration(self.scope,identity | {'role':'postgres'},lambda:None)

    def test_auth_and_prestart_count_every_session_and_prepared(self):
        identity = {'database':'auth_db','database_oid':99,'role':'voice',
                    'application_name':'voice-au-au-'+'a'*28+'-migrate-auth','pod_ip':None,'pod_uid':None}
        result = self.result | {'matched_databases':3,'sessions':1,'transactions':1,'prepared':1,'attributed':[]}
        with patch.object(probe.metadata,'kube_engine_snapshot',return_value=lambda:self.scope['engine']), \
             patch.object(probe.metadata,'linux_psql',return_value=json.dumps(result)) as run:
            receipt = probe.inspect_migration(self.scope,identity,lambda:None)
        self.assertEqual((receipt['active_sessions'],receipt['active_transactions']),(1,2))
        self.assertEqual(receipt['attributed_sessions'],[])
        self.assertIn("datname = ANY",run.call_args.args[0][-1])


if __name__=='__main__': unittest.main()
