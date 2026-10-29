import copy
from pathlib import Path
import unittest
from unittest.mock import patch, Mock

import media_operation_journal as journal
import realtime_db_provision as provision
import realtime_db_provision_journal_test as fixture
import realtime_secret_phase_test as secret_fixture
import realtime_secret_provision as bridge


class BridgeTest(unittest.TestCase):
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
