"""Real Space phases and Journal/Fence; Kubernetes, PG and CUT boundaries synthetic."""
import base64
import copy
import hashlib
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest import mock

import full_cohort_go_migration as go
import full_cohort_affected_plan as affected
import full_cohort_schema_metadata as schema
import space_fresh_install_phase_pg as pgmodule
import space_fresh_install_phase as fresh
import space_fresh_install_phase_runtime as runtime
import space_fresh_install_phase_runtime_test as route_fixture
import space_fresh_install_phase_test as phase_fixture
import writer_cut_space_test as cut_fixture
import frontend_runtime_executor as transport
import media_operation_journal as journals
import realtime_db_provision as provision
import resource_fencing


def _cleanup_fixture(fixture):
    if not fixture.doCleanups(): raise AssertionError('nested fixture cleanup failed')


class RuntimeJointTest(unittest.TestCase):
    def exercise(self, fault, with_real_cut=False):
        if with_real_cut:
            wrapper=cut_fixture.SpaceCutTest('test_two_jobs_and_durable_finished_binding')
            self.addCleanup(_cleanup_fixture,wrapper);wrapper.setUp();f=wrapper.f
        else:
            f=phase_fixture.PhaseTest('test_one_shot_success_retains_original_and_checked_cleanup')
            self.addCleanup(_cleanup_fixture,f); f.setUp()
        f.patches[0].stop()  # restore the actual runtime producer, not the main fixture's synthetic one
        routes=route_fixture.RouteTest(); routes.setUp(); observed=routes.observed
        if with_real_cut:
            controllers=[]
            for source in routes.controllers:
                name=source['metadata']['name'];uid=source['metadata']['uid']
                obj=cut_fixture.cut_fixture.workload('Deployment',name,uid,1,'source-image')
                container=copy.deepcopy(source['spec']['template']['spec']['containers'][0]);container['image']='source-image'
                obj['spec']['template']['spec']['containers']=[container];controllers.append(obj)
            f.cut.admission['controllers']=controllers
            f.cut.intent['writer_cut']=copy.deepcopy(f.cut.admission)
            f.cut.journal.intent=copy.deepcopy(f.cut.intent)
            f.cut.journal.state['intent']=copy.deepcopy(f.cut.intent)
            f.cut.probe=f.cut._initial_probe=affected.CatalogScopedProbe(f.catalog,'token')
        else:
            f.cut.admission={'controllers':routes.controllers}; f.cut._guard=mock.Mock()
        inv=SimpleNamespace(verify_current=mock.Mock(return_value={'database_observations':observed}),
                            verify_receipt=mock.Mock(return_value={'database_observations':observed}),_admin_role=lambda:'voice')
        f.catalog.inventory=inv
        secret={'apiVersion':'v1','kind':'Secret','type':'Opaque','metadata':{'name':'voice-app-secrets','namespace':'voice-dev',
                'uid':'u','resourceVersion':'1'},'data':{'SPACE_DATABASE_URL':base64.b64encode(routes.secret['value'].encode()).decode(),
                'OTHER':'c2VudGluZWw='}}
        state={'secret':secret,'role':'absent','patches':0}
        def key_reader(*args,**kwargs):
            def read(kind,name,key):
                obj=state['secret']; return {'uid':obj['metadata']['uid'],'resource_version':obj['metadata']['resourceVersion'],
                    'value':base64.b64decode(obj['data'][key]).decode()}
            return read
        def kube(args,body=None):
            return copy.deepcopy(state['secret'])
        def private_patch(body, *, dry_run):
            result=copy.deepcopy(state['secret']); result['data']['SPACE_DATABASE_URL']=body[-1]['value']
            if dry_run and fault=='admission': result['metadata']['ownerReferences']=[{'uid':'foreign'}]
            if not dry_run:
                state['patches']+=1; result['metadata']['resourceVersion']='2'; state['secret']=copy.deepcopy(result)
                if fault=='lost': raise TimeoutError('private sentinel')
            return result
        class PG:
            def __init__(self,guard): self.guard=guard
            def snapshot(self): return copy.deepcopy(f.engine)
            def space_oid(self): self.guard(); return f.oid
            def drop_exact(self,oid): self.guard(); f.drops.append(oid); f.oid=0; self.guard()
            def _run(self,sql):
                self.guard()
                if sql.startswith('\\connect space_db\n\\set '): state['role']='dedicated'
                return b'ok\n'
        login=mock.Mock(side_effect=[{}, ValueError('private sentinel')] if fault=='login_drift' else None)
        patches=[mock.patch.object(pgmodule,'KubeSpacePostgres',PG),
                 mock.patch.object(schema,'kube_key_reader',side_effect=key_reader),
                 mock.patch.object(go,'_private_kube',side_effect=kube),
                 mock.patch.object(runtime,'_private_patch',side_effect=private_patch),
                 mock.patch.object(go,'_login',login),
                 mock.patch.object(runtime,'_runtime_sql',return_value='SELECT 1;\n'),
                 mock.patch.object(runtime,'_role_check',side_effect=lambda *a,**k:state['role'])]
        for patcher in patches: patcher.start(); self.addCleanup(patcher.stop)
        if fault=='admission':
            with self.assertRaises(Exception): f.phase()
            self.assertEqual(f.drops,[]); self.assertEqual(state['patches'],0); return
        phase=f.phase()
        if fault in ('lost','login_drift'):
            with self.assertRaises(Exception): phase.run()
            self.assertEqual(f.drops,[11]); self.assertEqual(state['patches'],1)
            self.assertEqual(f.backing.row['status'],'uncertain')
            with self.assertRaises(Exception): phase.run()
            self.assertEqual(state['patches'],1); return
        receipt=phase.run()
        self.assertEqual(receipt['new_oid'],42); self.assertEqual(receipt['runtime']['role'],'space_runtime')
        self.assertEqual(state['patches'],1); self.assertEqual(state['secret']['data']['OTHER'],'c2VudGluZWw=')
        self.assertNotIn('postgres://',str(receipt)); self.assertEqual(runtime.verify_secret_successor(phase.runtime,'token')['new_rv'],'2')
        if with_real_cut:
            scope=f.cut.current_base_session_scope()
            self.assertEqual(scope['database_oids']['space_db'],42)
            self.assertEqual(affected.verify_space_catalog(f.cut.probe,f.cut)['all_databases']['space_db'],42)
            f.oid=43
            with self.assertRaises(affected.AffectedError):affected.verify_space_catalog(f.cut.probe,f.cut)
            f.oid=42
            self._realtime_successor(f)
        with mock.patch.object(runtime,'verify_receipt',wraps=runtime.verify_receipt) as historical:
            self.assertEqual(fresh.verify_historical(phase,'token')['new_oid'],receipt['new_oid'])
            historical.assert_called_once_with(phase.runtime,'token')

    def _realtime_successor(self,f):
        password=b'fixtureCredential_123456789012345678901234'
        ref={'name':'voice-realtime-db-fixture','uid':'secret','resourceVersion':'10',
             'password_sha256':hashlib.sha256(password).hexdigest()}
        j=journals.Journal(Path(f.temp.name)/'realtime-provision',{'resource_context':copy.deepcopy(f.context),
            'namespace_uid':'ns','postgres':copy.deepcopy(f.engine),'realtime_credential':ref})
        self.addCleanup(j.close)
        client=transport.KubectlTransport(j);ledger=resource_fencing.KubeLedger(client)
        ledger.read=f.backing.read;ledger.write=f.backing.write
        fence=resource_fencing.Fence(copy.deepcopy(f.context),lambda:copy.deepcopy(f.card),ledger)
        url='postgres://realtime:'+password.decode()+'@voice-postgres:5432/realtime_db?sslmode=disable'
        def call(*args,**kwargs):
            if args[:2]==('get','secret'):
                return {'metadata':{'name':ref['name'],'namespace':'voice-dev','uid':ref['uid'],
                    'resourceVersion':ref['resourceVersion']},'immutable':True,'type':'Opaque',
                    'data':{'DATABASE_URL':base64.b64encode(url.encode()).decode()}}
            return f.call(*args,**kwargs)
        client.call=call
        class PG:
            def __init__(self):self.state={'role':None,'database':None}
            def snapshot(self):return copy.deepcopy(f.engine)
            def catalog(self):return copy.deepcopy(self.state)
            def execute(self,sql):
                if sql==provision.database_sql():self.state['database']={'oid':55,'owner_oid':54}
                else:self.state['role']={'oid':54,'login':True,'superuser':False,'createdb':False,
                    'createrole':False,'inherit':False,'replication':False,'bypassrls':False,'memberships':0,
                    'connection_limit':-1,'valid_until':None,'config':None}
        pg=PG()
        def current(*args):
            result=f.current_catalog()
            if pg.state['database']:
                result['all_databases']['realtime_db']=pg.state['database']['oid']
                result['databases']['realtime_db']=pg.state['database']['oid']
            return result
        with mock.patch.object(provision,'KubePostgres',PG),mock.patch.object(affected,'_full_catalog',side_effect=current), \
             mock.patch.object(affected.pg,'inspect',side_effect=lambda scope,guard:{'scope':copy.deepcopy(scope),
                 'active_sessions':0,'active_transactions':0}):
            phase=provision.Provision(j,fence,client,f.lock,'ns',pg,password);phase.run()
            f.cut.bind_provision_scope(phase)
            actual=f.cut.current_provisioned_session_scope()
            self.assertEqual(actual['database_oids']['space_db'],42)
            self.assertEqual(actual['database_oids']['realtime_db'],55)
            self.assertEqual(affected.verify_provisioned_catalog(f.cut.probe,f.cut)['all_databases'],actual['database_oids'])
            pg.state['database']['oid']=56
            with self.assertRaises(Exception):f.cut.current_provisioned_session_scope()

    def test_native_to_runtime_secret_cas_and_terminal_receipt(self): self.exercise(None)
    def test_native_runtime_with_real_cut_separate_facades(self): self.exercise(None,with_real_cut=True)
    def test_admission_injection_fails_before_drop(self): self.exercise('admission')
    def test_lost_secret_patch_is_uncertain_without_replay(self): self.exercise('lost')
    def test_database_password_drift_blocks_terminal_receipt(self): self.exercise('login_drift')

    def test_nested_cleanup_failure_is_not_silently_accepted(self):
        with self.assertRaises(AssertionError): _cleanup_fixture(SimpleNamespace(doCleanups=lambda:False))


if __name__ == '__main__': unittest.main()
