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


if __name__=='__main__': unittest.main()
