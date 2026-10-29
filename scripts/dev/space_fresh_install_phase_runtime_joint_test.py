"""Real Space phases and Journal/Fence; Kubernetes, PG and CUT boundaries synthetic."""
import base64
import copy
import unittest
from types import SimpleNamespace
from unittest import mock

import full_cohort_go_migration as go
import full_cohort_schema_metadata as schema
import space_fresh_install_phase_pg as pgmodule
import space_fresh_install_phase_runtime as runtime
from space_fresh_install_phase_roles_test import FactsTest
from space_fresh_install_phase_runtime_test import RouteTest
from space_fresh_install_phase_test import PhaseTest


class RuntimeJointTest(unittest.TestCase):
    def exercise(self, fault):
        f=PhaseTest('test_one_shot_success_retains_original_and_checked_cleanup'); f.setUp(); self.addCleanup(f.doCleanups)
        f.patches[0].stop()  # restore the actual runtime producer, not the main fixture's synthetic one
        routes=RouteTest(); routes.setUp(); observed=routes.observed
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

    def test_native_to_runtime_secret_cas_and_terminal_receipt(self): self.exercise(None)
    def test_admission_injection_fails_before_drop(self): self.exercise('admission')
    def test_lost_secret_patch_is_uncertain_without_replay(self): self.exercise('lost')
    def test_database_password_drift_blocks_terminal_receipt(self): self.exercise('login_drift')


if __name__ == '__main__': unittest.main()
