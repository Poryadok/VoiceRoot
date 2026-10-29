import copy
import json
from pathlib import Path
import unittest
from unittest.mock import patch, Mock

import media_operation_journal as journal
import realtime_db_provision as provision
import realtime_db_provision_journal_test as fixture
import realtime_secret_phase_test as secret_fixture
import realtime_secret_provision as bridge
import resource_fencing


class BridgeTest(unittest.TestCase):
    def setUp(self):
        # This suite isolates bridge/credential behavior; C1 CUT is tested jointly.
        self.enterContext(patch.object(provision.Provision,'_before_effect'))
        self.enterContext(patch.object(provision.Provision,'_cut_local'))
        self.enterContext(patch.object(provision.Provision,'_effect_local'))
    def test_secret_ledger_drift_after_role_prevents_database_sql(self):
        base, client, secret = secret_fixture.SecretPhaseTest().setup_phase()
        self.addCleanup(base.doCleanups)
        password=b'a'*32; receipt=secret.run(password)
        state=fixture.JournalIntegration(); state.setUp(); self.addCleanup(state.doCleanups)
        child=journal.Journal(Path(base.temp.name)/'owned-provision',
            {'resource_context':base.context,'namespace_uid':'ns','postgres':state.pg.snapshot(),
             'realtime_credential':receipt}); self.addCleanup(child.close)
        separate=copy.copy(client); separate.journal=child
        class LedgerView:
            def __init__(inner): inner.client=separate
            def read(inner): return base.ledger.read()
            def write(inner,revision,row): return base.ledger.write(revision,row)
        fence=resource_fencing.Fence(base.context,base.fence.check,LedgerView())
        sent=[]
        def transport(command, **kwargs):
            sql=kwargs['input'].decode('ascii'); sent.append(sql)
            if sql==provision.CATALOG_SQL:
                return Mock(returncode=0,stdout=json.dumps(state.pg.catalog()).encode())
            state.pg.execute(sql)
            base.ledger.row['realtime_secret']['nonce']='f'*32
            secret.journal.state['steps']['realtime-secret']['nonce']='f'*32
            return Mock(returncode=0,stdout=b'')
        with patch.object(bridge.frontend_runtime_executor,'KubectlTransport',type(separate)), \
             patch.object(bridge.resource_fencing,'KubeLedger',LedgerView), \
             patch.object(provision.metadata,'kube_engine_snapshot',return_value=lambda:state.pg.snapshot()), \
             patch.object(provision.subprocess,'run',side_effect=transport):
            with self.assertRaises(bridge.RecoveryRequired):
                bridge.run_owned(secret,child,separate,fence,base.lock,'ns',password)
        self.assertTrue(any('CREATE ROLE realtime' in sql for sql in sent))
        self.assertNotIn(provision.database_sql(),sent)
        self.assertEqual(base.ledger.row['status'],'uncertain')

    def test_owned_worker_survives_for_cut_and_binds_two_journals(self):
        base, client, secret = secret_fixture.SecretPhaseTest().setup_phase()
        self.addCleanup(base.doCleanups)
        password=b'a'*32; receipt=secret.run(password)
        state=fixture.JournalIntegration(); state.setUp(); self.addCleanup(state.doCleanups)
        child=journal.Journal(Path(base.temp.name)/'owned-provision',
            {'resource_context':base.context,'namespace_uid':'ns','postgres':state.pg.snapshot(),
             'realtime_credential':receipt}); self.addCleanup(child.close)
        separate=copy.copy(client); separate.journal=child
        class LedgerView:
            def __init__(inner): inner.client=separate
            def read(inner): return base.ledger.read()
            def write(inner,revision,row): return base.ledger.write(revision,row)
        fence=resource_fencing.Fence(base.context,base.fence.check,LedgerView())
        class PG:
            def __init__(inner,guard): inner.guard=guard
            def snapshot(inner): inner.guard(); return state.pg.snapshot()
            def catalog(inner): inner.guard(); return state.pg.catalog()
            def execute(inner,sql,worker=None): inner.guard(); state.pg.execute(sql,worker)
        with patch.object(bridge.frontend_runtime_executor,'KubectlTransport',type(separate)), \
             patch.object(bridge.resource_fencing,'KubeLedger',LedgerView), \
             patch.object(provision,'KubePostgres',PG):
            worker,result=bridge.run_owned(secret,child,separate,fence,base.lock,'ns',password)
            self.assertEqual(result['database_oid'],13)
            self.assertEqual(provision.verify_completed(worker)['database_oid'],13)
            base.ledger.row['realtime_secret']['receipt']['uid']='foreign'
            with self.assertRaises(bridge.RecoveryRequired):
                bridge.run_owned(secret,child,separate,fence,base.lock,'ns',password)

    def test_create_receipt_binds_actual_fenced_provision(self):
        base, client, secret = secret_fixture.SecretPhaseTest().setup_phase()
        self.addCleanup(base.doCleanups)
        password = b'a'*32
        receipt = secret.run(password)
        state = fixture.JournalIntegration(); state.setUp(); self.addCleanup(state.doCleanups)
        intent = {'resource_context':base.context, 'namespace_uid':'ns',
                  'postgres':state.pg.snapshot(), 'realtime_credential':receipt}
        child = journal.Journal(Path(base.temp.name)/'provision', intent); self.addCleanup(child.close)
        with patch.object(bridge.provision, 'KubePostgres', return_value=state.pg):
            result = bridge.run(secret,child,password)
        self.assertEqual((result['role_oid'],result['database_oid']),(12,13))
        self.assertFalse(result['migration_pass'])

    def test_changed_ledger_receipt_refuses_before_pg(self):
        base, client, secret = secret_fixture.SecretPhaseTest().setup_phase()
        self.addCleanup(base.doCleanups)
        password = b'a'*32; receipt = secret.run(password)
        state = fixture.JournalIntegration(); state.setUp(); self.addCleanup(state.doCleanups)
        child = journal.Journal(Path(base.temp.name)/'provision',
            {'resource_context':base.context,'namespace_uid':'ns','postgres':state.pg.snapshot(),
             'realtime_credential':receipt}); self.addCleanup(child.close)
        base.ledger.row['realtime_secret']['receipt']['uid']='foreign'
        with patch.object(bridge.provision, 'KubePostgres', return_value=state.pg):
            with self.assertRaises(bridge.RecoveryRequired): bridge.run(secret,child,password)
        self.assertEqual(state.pg.state, {'role':None,'database':None})

    def test_actual_kube_transport_guards_each_psql(self):
        calls = []
        def guard():
            calls.append('guard')
            if len(calls)>2: raise RuntimeError('authority changed')
        with patch.object(provision.metadata, 'kube_engine_snapshot', return_value=lambda: {}), \
             patch.object(provision.subprocess, 'run', return_value=Mock(returncode=0,stdout=b'{}')) as proc:
            pg = provision.KubePostgres(guard)
            pg.catalog()
            with self.assertRaises(provision.RecoveryRequired): pg.execute(provision.database_sql())
        self.assertEqual(proc.call_count,1)


if __name__ == '__main__': unittest.main()
