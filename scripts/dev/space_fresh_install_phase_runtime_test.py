import copy
import unittest
import tempfile
import sys
from pathlib import Path
from types import SimpleNamespace
from unittest import mock

import space_fresh_install_phase_runtime as runtime
from space_fresh_install_phase_roles_test import FactsTest


class RouteTest(unittest.TestCase):
    def setUp(self):
        self.observed = FactsTest().fixture()
        self.secret = {'uid':'u','resource_version':'1',
                       'value':'postgres://space:private@voice-postgres/space_db?sslmode=disable'}
        ref = {'kind':'secret','uid':'u','resource_version':'1'}
        for name in ('space','chat','messaging'):
            result = self.observed['workloads'][name][0]['result']
            binding = result['binding'] if name == 'space' else next(x['binding'] for x in result['secondary'] if x['binding']['database']=='space_db')
            binding.update(host='voice-postgres',role='space',references=[ref],deployment_uid='uid-'+name)
        self.controllers=[]
        for name in ('space','chat','messaging'):
            variable='DATABASE_URL' if name=='space' else 'SPACE_DATABASE_URL'
            env = [{'name':variable,'valueFrom':{'secretKeyRef':{'name':'voice-app-secrets','key':'SPACE_DATABASE_URL'}}}]
            pod = {'containers':[{'name':name,'env':env}]}
            self.controllers.append({'kind':'Deployment','metadata':{'name':'voice-'+name,'uid':'uid-'+name},
                                     'spec':{'template':{'spec':pod}}})
    def read(self, kind, name, key): return self.secret
    def test_exact_shared_key_and_cut_consumers(self):
        self.assertEqual(runtime._source_routes(self.observed,self.controllers,self.read),['chat','messaging','space'])
    def test_missing_restart_or_different_key_holds(self):
        with self.assertRaises(ValueError): runtime._source_routes(self.observed,self.controllers[:-1],self.read)
        other=copy.deepcopy(self.controllers); other[0]['spec']['template']['spec']['containers'][0]['env'][0]['valueFrom']['secretKeyRef']['key']='OTHER'
        with self.assertRaises((ValueError, runtime.schema.Hold)): runtime._source_routes(self.observed,other,self.read)

    def test_secret_patch_preserves_unrelated_keys_and_binds_uid_rv(self):
        secret={'apiVersion':'v1','kind':'Secret','metadata':{'name':'voice-app-secrets','namespace':'voice-dev',
                'uid':'u','resourceVersion':'4'},'data':{'SPACE_DATABASE_URL':'b2xk','OTHER':'c2VudGluZWw='}}
        patch, safe, target = runtime._secret_patch(secret,'postgres://space_runtime:new@voice-postgres/space_db?sslmode=disable',b'k'*32)
        self.assertEqual([x['op'] for x in patch],['test','test','replace'])
        self.assertEqual(target['OTHER'], secret['data']['OTHER'])
        self.assertEqual(safe['secret_uid'], 'u')
        self.assertNotIn('space_runtime:new', str(safe))
        admitted=copy.deepcopy(secret); admitted['data']=target; admitted['metadata']['resourceVersion']='5'
        runtime._secret_unchanged(secret,admitted,target)
        admitted['metadata']['ownerReferences']=[{'uid':'foreign'}]
        with self.assertRaises(ValueError): runtime._secret_unchanged(secret,admitted,target)
        secret['immutable'] = True
        with self.assertRaises(ValueError): runtime._secret_patch(secret,'anything',b'k'*32)

    def test_ledger_identity_mismatch_is_guard_loss(self):
        phase=runtime.SpaceRuntimeRolePhase.__new__(runtime.SpaceRuntimeRolePhase)
        row={'status':'active'}
        def fence(rv):
            return SimpleNamespace(ledger=SimpleNamespace(read=lambda:(row,('uid',rv))),owned=lambda value:value==row)
        phase.fence=fence('1'); phase.catalog=SimpleNamespace(fence=fence('1')); phase.cut=SimpleNamespace(fence=fence('1'))
        self.assertEqual(phase._ledger(),(row,('uid','1')))
        phase.cut.fence=fence('2')
        with self.assertRaises(runtime.schema.GuardLost): phase._ledger()

    def test_role_check_fails_closed_on_unsafe_or_invalid_oid(self):
        pg=SimpleNamespace(_run=mock.Mock(return_value=b'dedicated\n'))
        self.assertEqual(runtime._role_check(pg,42,require_zero_sessions=True),'dedicated')
        sql=pg._run.call_args.args[0]
        self.assertIn("usename='space_runtime'",sql)
        self.assertIn('d.dbid<>42::oid',sql)
        pg._run.return_value=b'unsafe\n'
        with self.assertRaises(ValueError): runtime._role_check(pg,42,require_zero_sessions=True)
        with self.assertRaises(ValueError): runtime._role_check(pg,True,require_zero_sessions=True)

    def test_changed_sql_tail_fails_even_if_source_snapshot_matches(self):
        import publish_candidate
        raw=b'\\getenv space_runtime_password SPACE_DB_PASSWORD\nSELECT 1;\n'
        with tempfile.TemporaryDirectory() as directory:
            path=Path(directory)/'docker/postgres/provision-space-runtime-role.sql'
            path.parent.mkdir(parents=True); path.write_bytes(raw)
            source='a'*40
            def snapshot(root, sha):
                return {'source_sha':sha,'files':{'docker/postgres/provision-space-runtime-role.sql':runtime.hashlib.sha256(path.read_bytes()).hexdigest()}}
            with mock.patch.object(publish_candidate,'source_snapshot',side_effect=snapshot), \
                 mock.patch.object(runtime,'RUNTIME_SQL_SHA256',runtime.hashlib.sha256(raw).hexdigest()):
                self.assertEqual(runtime._runtime_sql(directory,source),'SELECT 1;\n')
                path.write_bytes(raw+b'DROP TABLE important;\n')
                with self.assertRaises(ValueError): runtime._runtime_sql(directory,source)

    def test_pre_effect_oid_or_role_drift_prevents_sql_or_secret_patch(self):
        import full_cohort_go_migration as go
        import space_fresh_install_phase as fresh
        import space_fresh_install_phase_pg as pgmodule
        for fault in ('oid','role','grants'):
            phase=runtime.SpaceRuntimeRolePhase.__new__(runtime.SpaceRuntimeRolePhase)
            cut=SimpleNamespace(verify_space_running=mock.Mock()); phase.cut=cut
            phase.fence=SimpleNamespace(uncertain=mock.Mock()); phase.namespace_uid='ns'; phase.token='token'; phase.source_root='root'
            phase.poisoned=False; phase._guard=mock.Mock(); phase._advance=mock.Mock()
            phase.catalog=SimpleNamespace(verify_receipt=mock.Mock(return_value={'engine':{'id':'engine'},'server_start':'start'}))
            source='SELECT 1;\n'; before={'apiVersion':'v1','kind':'Secret','metadata':{'name':'voice-app-secrets',
                'namespace':'voice-dev','uid':'u','resourceVersion':'1'},'data':{'SPACE_DATABASE_URL':'b2xk'}}
            key=b'k'*32; _,safe,target=runtime._secret_patch(before,'postgres://space_runtime:test@voice-postgres:5432/space_db?sslmode=disable',key)
            facts={'operation_id':'op','source_sha':'a'*40,'namespace_uid':'ns','original_space_oid':2,'role_state':'absent','admin_role':'voice',
                   'runtime_sql_sha256':runtime.hashlib.sha256(source.encode()).hexdigest(),'secret_uid':'u','old_rv':'1',
                   'old_data_hmac_sha256':safe['old_data_hmac_sha256'],
                   'unchanged_other_keys_hmac_sha256':safe['unchanged_other_keys_hmac_sha256'],'consumers':['space']}
            marker={'status':'prepared','nonce':'n','facts_sha256':'x'}
            phase._local=mock.Mock(return_value=({'facts':facts},marker,key))
            native={'operation_id':'op','source_sha':'a'*40,'namespace_uid':'ns','new_oid':9,'native_receipt_sha256':'h','installer_role':'voice'}
            phase.journal=object(); owner=SimpleNamespace(cut=cut,runtime=phase,journal=phase.journal,fence=phase.fence)
            pg=SimpleNamespace(snapshot=mock.Mock(return_value={'id':'engine'}),space_oid=mock.Mock(side_effect=[9,10] if fault=='oid' else [9,9,9,9]),_run=mock.Mock(side_effect=[b'',b'ok\n',b'unsafe\n']))
            calls=[]; patch_calls=[]
            def kube(args, body=None):
                calls.append(args)
                return copy.deepcopy(before)
            def private_patch(body, *, dry_run):
                patch_calls.append(dry_run)
                result=copy.deepcopy(before); result['data']=copy.deepcopy(target); return result
            role_calls=(['absent','absent','dedicated','unsafe'] if fault=='role' else
                        ['absent','absent','dedicated','dedicated'] if fault=='grants' else ['absent'])
            with mock.patch.object(fresh,'verify_native_completed_before_runtime',return_value=native,create=True), \
                 mock.patch.object(pgmodule,'KubeSpacePostgres',return_value=pg), \
                 mock.patch.object(runtime,'_runtime_sql',return_value=source), \
                 mock.patch.object(runtime,'_role_check',side_effect=role_calls), \
                 mock.patch.object(runtime,'_admin_check'), \
                 mock.patch.object(runtime.secrets,'token_hex',return_value='test'), \
                 mock.patch.object(runtime,'_private_patch',side_effect=private_patch), \
                 mock.patch.object(go,'_private_kube',side_effect=kube):
                with self.subTest(fault=fault), self.assertRaises(runtime.SpaceRuntimeError): phase.run(owner)
            if fault=='oid': pg._run.assert_not_called()
            self.assertEqual(patch_calls,[True])
            self.assertEqual(phase.fence.uncertain.call_count,1, (fault, phase._advance.call_args_list, calls))

    def test_successor_checks_current_full_secret_hmac_and_rv(self):
        import full_cohort_go_migration as go
        key=b'k'*32; data={'SPACE_DATABASE_URL':'bmV3','OTHER':'eA=='}; other={'OTHER':'eA=='}
        facts={'operation_id':'op','source_sha':'a'*40,'namespace_uid':'ns','secret_uid':'u','old_rv':'1',
               'old_data_hmac_sha256':'old','unchanged_other_keys_hmac_sha256':runtime._mac(key,other),
               'consumers':['space'],'original_space_oid':2}
        receipt=facts | {'role':'space_runtime','new_rv':'2','database_oid':9,'new_data_hmac_sha256':runtime._mac(key,data)}
        marker={'status':'done','new_rv':'2','receipt_sha256':runtime._digest_public(receipt)}
        local={'facts':facts,'receipt':receipt}; row={'space_runtime_role':marker}; rev=('ledger','2')
        phase=runtime.SpaceRuntimeRolePhase.__new__(runtime.SpaceRuntimeRolePhase); phase.token='token'; phase._guard=mock.Mock()
        phase._local=mock.Mock(return_value=(local,marker,key)); phase._ledger=mock.Mock(return_value=(row,rev))
        phase.journal=SimpleNamespace(state={'steps':{phase.STEP:local}})
        actual={'kind':'Secret','apiVersion':'v1','metadata':{'name':'voice-app-secrets','namespace':'voice-dev',
                'uid':'u','resourceVersion':'2'},'data':data}
        with mock.patch.object(go,'_private_kube') as private:
            self.assertEqual(runtime.verify_receipt(phase,'token'),receipt)
            private.assert_not_called()
        with mock.patch.object(go,'_private_kube',return_value=actual):
            self.assertEqual(runtime.verify_secret_successor(phase,'token')['new_rv'],'2')
        for field in ('rv','data'):
            altered=copy.deepcopy(actual)
            if field=='rv': altered['metadata']['resourceVersion']='3'
            else: altered['data']['OTHER']='eQ=='
            with self.subTest(field=field), mock.patch.object(go,'_private_kube',return_value=altered), self.assertRaises(runtime.SpaceRuntimeError):
                runtime.verify_secret_successor(phase,'token')

    def test_transient_guard_loss_during_grants_is_not_masked(self):
        import space_fresh_install_phase_pg as pgmodule
        phase=runtime.SpaceRuntimeRolePhase.__new__(runtime.SpaceRuntimeRolePhase); phase._guard=mock.Mock()
        receipt={'database_oid':9}; marker={'status':'done','receipt_sha256':runtime._digest_public(receipt)}
        phase._local=mock.Mock(return_value=({'receipt':receipt},marker,b'k'*32))
        phase.catalog=SimpleNamespace(verify_receipt=mock.Mock(return_value={'engine':{'id':'engine'}}))
        pg=SimpleNamespace(snapshot=mock.Mock(return_value={'id':'engine'}),space_oid=mock.Mock(return_value=9))
        with mock.patch.object(runtime,'verify_secret_successor'), mock.patch.object(runtime,'verify_receipt',return_value=receipt), \
             mock.patch.object(pgmodule,'KubeSpacePostgres',return_value=pg), \
             mock.patch.object(runtime,'_role_check',return_value='dedicated'), \
             mock.patch.object(runtime,'_grants_check',side_effect=runtime.schema.GuardLost('lost')):
            with self.assertRaises(runtime.schema.GuardLost): runtime.verify_completed(phase,'token')

    def test_wrong_runtime_instance_is_rejected_before_effect(self):
        import space_fresh_install_phase as fresh
        import full_cohort_go_migration as go
        phase=runtime.SpaceRuntimeRolePhase.__new__(runtime.SpaceRuntimeRolePhase)
        phase.cut=object(); phase.journal=object(); phase.fence=SimpleNamespace(uncertain=mock.Mock())
        phase.token='token'; phase.namespace_uid='ns'
        facts={'operation_id':'op','source_sha':'a'*40,'admin_role':'voice','original_space_oid':2}
        phase._local=mock.Mock(return_value=({'facts':facts},{'status':'prepared'},b'k'*32))
        other=SimpleNamespace(cut=phase.cut,runtime=object(),journal=phase.journal,fence=phase.fence)
        native={'operation_id':'op','source_sha':'a'*40,'namespace_uid':'ns','new_oid':9,'installer_role':'voice'}
        with mock.patch.object(fresh,'verify_native_completed_before_runtime',return_value=native), \
             mock.patch.object(go,'_private_kube') as private:
            with self.assertRaises(runtime.SpaceRuntimeError): phase.run(other)
        private.assert_not_called(); phase.fence.uncertain.assert_not_called()

    def test_private_hmac_key_replacement_fails_historical_custody(self):
        phase=runtime.SpaceRuntimeRolePhase.__new__(runtime.SpaceRuntimeRolePhase); phase._guard=mock.Mock()
        phase.namespace_uid='ns'; phase.fence=SimpleNamespace(context={'operation_id':'op','commit':'a'*40})
        facts={'operation_id':'op','source_sha':'a'*40,'namespace_uid':'ns'}; key=b'k'*32
        marker={'status':'prepared','nonce':'n','facts_sha256':runtime._digest_public(facts),
                'hmac_key_sha256':runtime.hashlib.sha256(key).hexdigest()}
        local=marker | {'facts':facts,'private_hmac_key':key.hex()}
        phase.journal=SimpleNamespace(state={'steps':{phase.STEP:local}})
        phase._ledger=mock.Mock(return_value=({phase.LEDGER:marker},('u','1')))
        self.assertEqual(phase._local()[2],key)
        local['private_hmac_key']=(b'x'*32).hex()
        with self.assertRaises(ValueError): phase._local()

    def test_patch_payload_is_sealed_fd_not_argv_or_stdin(self):
        import frontend_runtime_executor
        program=("import sys,os,json,fcntl; "
                 "p=next(x.split('=',1)[1] for x in sys.argv if x.startswith('--patch-file=')); "
                 "fd=int(p.rsplit('/',1)[1]); seals=fcntl.fcntl(fd,fcntl.F_GET_SEALS); "
                 "data=open(p,'rb').read(); "
                 "assert seals & 15 == 15 and b'private-sentinel' in data; "
                 "assert all('private-sentinel' not in x for x in sys.argv); "
                 "assert sys.stdin.read()==''; print(json.dumps({'status':'ok'}))")
        with mock.patch.object(frontend_runtime_executor.KubectlTransport,'PREFIX',[sys.executable,'-c',program]):
            self.assertEqual(runtime._private_patch([{'op':'replace','value':'private-sentinel'}],dry_run=True),{'status':'ok'})


if __name__ == '__main__': unittest.main()
