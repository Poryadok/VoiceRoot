import datetime as dt
import unittest
import tempfile
import json
import hashlib
import copy
import ast
import contextlib
import io
import sys
from pathlib import Path
from unittest.mock import patch

from controller import Blocked
import root_main
from controller import archive_closed_store


class ResumeGuardTests(unittest.TestCase):
    def test_operator_error_exposes_only_static_label_or_exception_type(self):
        self.assertEqual(root_main.operator_error(Blocked('pv_storage_path_unsupported')),'pv_storage_path_unsupported')
        self.assertEqual(root_main.operator_error(Blocked('PRIVATE_TEST_SEED_OR_STDERR')),'blocked_unclassified')
        self.assertEqual(root_main.operator_error(KeyError('PRIVATE_TEST_SEED_OR_STDERR')),'exception_KeyError')
        class PrivateCustomError(Exception):pass
        self.assertEqual(root_main.operator_error(PrivateCustomError('PRIVATE_TEST_SEED_OR_STDERR')),'exception_unclassified')

    def test_actual_entrypoint_catch_never_prints_private_exception_values(self):
        tree=ast.parse(Path(root_main.__file__).read_text())
        entry=tree.body[-1]
        for error,expected in ((Blocked('pv_storage_node_unsupported'),'pv_storage_node_unsupported'),(KeyError('PRIVATE_SENTINEL'),'exception_KeyError')):
            def fail(args):raise error
            output=io.StringIO()
            with contextlib.redirect_stderr(output),self.assertRaises(SystemExit):
                exec(compile(ast.Module(body=[entry],type_ignores=[]),'<actual entrypoint>','exec'),{'__name__':'__main__','main':fail,'operator_error':root_main.operator_error,'sys':sys})
            self.assertEqual(output.getvalue(),'KNOWN_NATS_BASELINE=BLOCKED\nKNOWN_NATS_ERROR='+expected+'\n')
            self.assertNotIn('PRIVATE_SENTINEL',output.getvalue())

    def continuation_fixture(self,d):
        root=Path(d);base=root/'known-baseline-0049430b0dbb';base.mkdir()
        inputs=base/'inputs';inputs.mkdir();(base/'server.conf').write_bytes(b'nonsecret-test-public-config')
        (base/'kernel').write_bytes(b'test-kernel')
        code=root/'code';code.mkdir()
        files={'root_main.py':'new-root','stage_runtime.py':'new-stage','kernel':hashlib.sha256(b'test-kernel').hexdigest()}
        current={'schema':'known-nats-code-capture-v1','files':files}
        (code/'capture-manifest.json').write_text(json.dumps(current));(code/'deployed-contract.json').write_text('{"scripts":[]}')
        old=copy.deepcopy(current);old['files'].update({'root_main.py':'5938a72653b973c41ebe0bd138fe4f15f1671ff6dc0895af18f0132c794f3cb0','stage_runtime.py':'0a79beea09cbfe78679054c4b7b661e823b7bea25b0970641d9ef14bed1fbfa3'})
        names=(root_main.HUB,'voice-gateway',*('voice-'+s for s in root_main.LEAVES))
        provenance={}
        for role in ('bootstrap','social','realtime'):
            raw=('test-'+role).encode();(inputs/(role+'.creds')).write_bytes(raw)
            provenance[role+'.creds']={'sha256':hashlib.sha256(raw).hexdigest()}
        store=root/'test-store';store.mkdir();(store/'block').write_bytes(b'controlled-test-records')
        manifest=archive_closed_store(store,base/'fixture.tar');root_main.save(base/'fixture-manifest.json',manifest)
        state={'schema':'known-nats-root-checkpoint-v1','operation':'0049430b0dbb','status':'BLOCKED','phase':'FENCE_STAGE','error':'command_output_limit',
            'created_at':dt.datetime.now(dt.timezone.utc).isoformat(),'code_capture':old,'expected':dict(root_main.CURRENT,deployment_uids=dict.fromkeys(names,'original-uid')),
            'snapshots':{n:{'metadata':{'uid':'original-uid'},'spec':{'replicas':0,'template':{}}} for n in names},
            'journal':[{'kind':'staging_preflight','replicas':dict.fromkeys(names,1)}],'provenance':provenance,'marker':{},'service':{},
            'fixture_backup':{'messages':3,'streams':15,'consumers':42,'restore_verified':True,'node_copy_verified':True,
                'archive':str(base/'fixture.tar'),'manifest':str(base/'fixture-manifest.json'),'archive_sha256':manifest['archive_sha256']}}
        root_main.save(base/'checkpoint.json',state)
        return root,base,code,state

    def test_continuation_rejects_changed_state_code_archive_before_kubernetes(self):
        for mutation in ('wrong-error','changed-kernel-code','tampered-archive','expired','replica-one','changed-private-creds'):
            with self.subTest(mutation=mutation),tempfile.TemporaryDirectory() as d:
                root,base,code,state=self.continuation_fixture(d)
                if mutation=='wrong-error':state['error']='other'
                if mutation=='changed-kernel-code':state['code_capture']['files']['kernel']='different'
                if mutation=='tampered-archive':(base/'fixture.tar').write_bytes(b'tampered')
                if mutation=='expired':state['created_at']=(dt.datetime.now(dt.timezone.utc)-dt.timedelta(hours=5)).isoformat()
                if mutation=='replica-one':state['snapshots'][root_main.HUB]['spec']['replicas']=1
                if mutation=='changed-private-creds':(base/'inputs'/'social.creds').write_bytes(b'changed')
                root_main.save(base/'checkpoint.json',state)
                with patch('root_main.ROOT',root),patch('root_main.Kube') as kube:
                    with self.assertRaises(Blocked):root_main.continue_fence(code,base)
                    kube.assert_not_called()

    def test_continuation_lost_fence_does_not_allocate_or_rewrite_checkpoint(self):
        with tempfile.TemporaryDirectory() as d:
            root,base,code,state=self.continuation_fixture(d)
            checkpoint=(base/'checkpoint.json').read_bytes()
            class Kube:
                def get(self,kind,name):
                    if kind=='pvc':return {'metadata':{'uid':root_main.CURRENT['source_claim_uid']},'spec':{'volumeName':'original-pv'}}
                    return {'metadata':{'uid':root_main.CURRENT['source_pv_uid']},'spec':{'claimRef':{'uid':root_main.CURRENT['source_claim_uid']},'local':{'path':'/var/lib/rancher/k3s/storage/pvc-'+root_main.CURRENT['source_claim_uid']+'_voice-staging_'+root_main.CURRENT['source_claim']},'nodeAffinity':{'required':{'nodeSelectorTerms':[{'matchExpressions':[{'key':'kubernetes.io/hostname','operator':'In','values':['pmdebook']}]}]}}}}
            with patch('root_main.ROOT',root),patch('root_main.Kube',Kube),patch('root_main.revalidate_inputs'),\
                 patch('root_main.Staging') as stage,patch('root_main.DockerRuntime') as runtime:
                stage.return_value.verify_closed.side_effect=Blocked('maintenance_ownership_changed')
                with self.assertRaisesRegex(Blocked,'maintenance_ownership_changed'):root_main.continue_fence(code,base)
                runtime.assert_not_called();stage.return_value.new_claim.assert_not_called()
            self.assertEqual((base/'checkpoint.json').read_bytes(),checkpoint)

    def test_continuation_reuses_fixture_and_stops_at_off_node_checkpoint(self):
        with tempfile.TemporaryDirectory() as d:
            root,base,code,state=self.continuation_fixture(d);calls=[]
            class Kube:
                def get(self,kind,name):
                    if kind=='pvc':return {'metadata':{'uid':root_main.CURRENT['source_claim_uid']},'spec':{'volumeName':'original-pv'}}
                    return {'metadata':{'uid':root_main.CURRENT['source_pv_uid']},'spec':{'claimRef':{'uid':root_main.CURRENT['source_claim_uid']},'local':{'path':'/var/lib/rancher/k3s/storage/pvc-'+root_main.CURRENT['source_claim_uid']+'_voice-staging_'+root_main.CURRENT['source_claim']},'nodeAffinity':{'required':{'nodeSelectorTerms':[{'matchExpressions':[{'key':'kubernetes.io/hostname','operator':'In','values':['pmdebook']}]}]}}}}
            class Stage:
                def __init__(self,kube,operation,expected,save):self.operation=operation;self.save=save
                def verify_closed(self):calls.append('verify-owned-fence')
                def new_claim(self,date):
                    calls.append('new-final-claim');self.final_claim={'uid':'new'};self.final_pv={'uid':'new-pv'};self.final_path=base/'new-store';self.save({'kind':'allocated'});return self.final_path
                def select_claim(self):calls.append('select-final-claim');self.save({'kind':'selected'})
            class Runtime:
                def __init__(self,path,operation):self.owned=[];calls.append(('runtime',operation))
            with patch('root_main.ROOT',root),patch('root_main.Kube',Kube),patch('root_main.Staging',Stage),\
                 patch('root_main.revalidate_inputs'),patch('root_main.DockerRuntime',Runtime),\
                 patch('root_main.staging_baseline',return_value={'messages':0}) as baseline,patch('root_main.fixture') as fixture:
                result_base,result=root_main.continue_fence(code,base)
            self.assertEqual(result_base,base);self.assertEqual(result['operation'],'0049430b0dbb')
            self.assertEqual(result['status'],'AWAITING_OFF_NODE_COPY');self.assertEqual(result['fence_status'],'VERIFIED')
            self.assertEqual(result['fixture_backup'],state['fixture_backup']);self.assertEqual(result['previous_code_capture'],state['code_capture'])
            self.assertEqual(calls,['verify-owned-fence',('runtime','0049430b0dbb'),'new-final-claim','select-final-claim'])
            baseline.assert_called_once();fixture.assert_not_called()

    def test_continue_fence_routes_existing_operation_without_prepare(self):
        with tempfile.TemporaryDirectory() as d,patch('root_main.ROOT',Path(d)):
            base=Path(d)/'known-baseline-0049430b0dbb'
            with patch('root_main.continue_fence',create=True,return_value=(base,{})) as continued,\
                 patch('root_main.share_checkpoint',create=True) as shared,patch('root_main.prepare') as prepared:
                root_main.main_unlocked(['--continue-fence',str(base)])
            continued.assert_called_once()
            shared.assert_called_once_with(base,{})
            prepared.assert_not_called()

    def test_baseline_recovery_routes_same_operation_without_prepare_or_allocation(self):
        with tempfile.TemporaryDirectory() as d,patch('root_main.ROOT',Path(d)):
            base=Path(d)/'known-baseline-0049430b0dbb'
            with patch('root_main.continue_staging_baseline',return_value=(base,{})) as continued,patch('root_main.share_checkpoint') as shared,patch('root_main.prepare') as prepared:
                root_main.main_unlocked(['--continue-staging-baseline',str(base)])
            continued.assert_called_once();shared.assert_called_once_with(base,{})
            prepared.assert_not_called()

    def test_baseline_recovery_container_present_does_not_rewrite_checkpoint(self):
        with tempfile.TemporaryDirectory() as d:
            root,base,code,state=self.continuation_fixture(d)
            before=(base/'checkpoint.json').read_bytes()
            with patch('root_main.continuation_preflight',return_value=(state,{},unittest.mock.Mock())),patch('root_main.DockerRuntime') as runtime:
                runtime.return_value.no_operation_containers.side_effect=Blocked('operation_container_writer_present')
                with self.assertRaisesRegex(Blocked,'operation_container_writer_present'):root_main.continue_staging_baseline(code,base)
                runtime.return_value.allow_existing_store.assert_not_called()
            self.assertEqual((base/'checkpoint.json').read_bytes(),before)

    def test_baseline_recovery_preserves_existing_store_and_creation_cut(self):
        with tempfile.TemporaryDirectory() as d:
            root,base,code,state=self.continuation_fixture(d)
            store=base/'existing-store';store.mkdir();(store/'block').write_bytes(b'existing')
            stage=unittest.mock.Mock(final_path=store,marker=state['marker'],snapshots=state['snapshots'])
            runtime=unittest.mock.Mock(owned=[])
            created=state['created_at'];current={'schema':'known-nats-code-capture-v1','files':{}}
            def recover(rt,path,manifest,guard):
                self.assertEqual(path,store);guard();return {'messages':0,'streams':15,'consumers':42}
            with patch('root_main.continuation_preflight',return_value=(state,current,stage)) as preflight,patch('root_main.DockerRuntime',return_value=runtime),patch('root_main.revalidate_inputs'),patch('root_main.recover_staging_baseline',side_effect=recover):
                result_base,result=root_main.continue_staging_baseline(code,base)
            preflight.assert_called_once_with(code,base,baseline=True)
            self.assertEqual(result_base,base);self.assertEqual(result['created_at'],created)
            self.assertEqual(result['status'],'AWAITING_OFF_NODE_COPY')
            self.assertEqual(result['fixture_backup'],state['fixture_backup'])
            self.assertEqual((store/'block').read_bytes(),b'existing')
            runtime.allow_existing_store.assert_called_once_with(store)
            runtime.no_operation_containers.assert_any_call(running_only=True)
            stage.new_claim.assert_not_called();stage.restart.assert_not_called();stage.select_claim.assert_called_once()
            self.assertTrue(any(r['kind']=='baseline_recovery_custody_revalidated' for r in result['journal']))

    def test_baseline_recovery_invalid_checkpoint_never_opens_kubernetes(self):
        for mutation in ('wrong-phase','wrong-code','expired-recovery-window','wrong-archive'):
            with self.subTest(mutation=mutation),tempfile.TemporaryDirectory() as d:
                root,base,code,state=self.continuation_fixture(d)
                state.update(phase='STAGING_BACKUP_RESTORE',error='root_operation_phase_failed',fence_status='VERIFIED')
                current=json.loads((code/'capture-manifest.json').read_text())
                current['files'].update({'docker_runtime.py':'new-runtime','scenario.py':'new-scenario'})
                (code/'capture-manifest.json').write_text(json.dumps(current))
                state['code_capture']=copy.deepcopy(current)
                state['code_capture']['files'].update({'root_main.py':'28ee5551af7d4a2d9037977f1d6866cf64c682dcbe48932459724d65b507e12f','docker_runtime.py':'c5bd8a4adf22b3dabb7d8d2b7a1c2f88185974e2b532804fb8131cdd10d4e18b','scenario.py':'169a67428404cfb9021415bc4b6c9f9b2da39330a097562b92cf76acec7b75d9'})
                if mutation=='wrong-phase':state['phase']='FENCE_STAGE'
                if mutation=='wrong-code':state['code_capture']['files']['kernel']='wrong'
                if mutation=='expired-recovery-window':state['created_at']=(dt.datetime.now(dt.timezone.utc)-dt.timedelta(hours=25)).isoformat()
                if mutation=='wrong-archive':(base/'fixture.tar').write_bytes(b'tamper')
                root_main.save(base/'checkpoint.json',state)
                with patch('root_main.ROOT',root),patch('root_main.Kube') as kube:
                    with self.assertRaises(Blocked):root_main.continuation_preflight(code,base,baseline=True)
                    kube.assert_not_called()

    def test_second_operator_never_enters_locked_operation(self):
        with tempfile.TemporaryDirectory() as d,patch('root_main.ROOT',Path(d)):
            with root_main.operation_lock():
                with self.assertRaisesRegex(Blocked,'operation_already_running'):
                    with root_main.operation_lock():self.fail('concurrent operation entered')

    def test_failed_refence_never_claims_verified_fence(self):
        class Stage:
            operation='abcd1234'
            marker={'data':{'knownBaselineOperation':'abcd1234'}}
            def refence(self):raise Blocked('ownership_lost')
        state={'stage_fenced':True,'fence_status':'VERIFIED'}
        root_main.observe_refence(state,Stage())
        self.assertFalse(state['stage_fenced'])
        self.assertEqual(state['fence_status'],'UNKNOWN')
        self.assertFalse(state['refence']['verified'])

    def test_changed_captured_code_is_rejected(self):
        with patch('root_main.public_read',return_value=b'{"files":{"kernel":"new"}}'):
            with self.assertRaisesRegex(Blocked,'checkpoint_code_capture_changed'):
                root_main.bind_code(Path('/captured'),{'code_capture':{'files':{'kernel':'old'}}})

    def test_modified_closed_final_store_rejects_before_restart(self):
        with tempfile.TemporaryDirectory() as d:
            base=Path(d);store=base/'store';store.mkdir();(store/'blk').write_bytes(b'old')
            manifest=archive_closed_store(store,base/'closed.tar')
            path=base/'manifest.json';path.write_text(json.dumps(manifest))
            (store/'blk').write_bytes(b'changed')
            class Stage:
                final_path=store
                def verify_final_storage(self):pass
            with self.assertRaisesRegex(Blocked,'closed_final_store_changed'):
                root_main.verify_store(base,{'staging_backup':{'manifest':str(path)}},Stage())
            self.assertEqual(list(base.glob('store-verification-*')),[])

    def state(self):
        return {'status':'AWAITING_OFF_NODE_COPY',
            'expires_at':(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=1)).isoformat(),
            'fixture_backup':{'archive_sha256':'a'*64},
            'staging_backup':{'archive_sha256':'b'*64}}

    def test_wrong_off_node_hash_never_opens_kubernetes(self):
        with patch('root_main.private_json',return_value=self.state()),patch('root_main.Kube') as kube:
            with self.assertRaises(Blocked):
                root_main.resume(Path('/captured-code'),root_main.ROOT/'known-baseline-abcd1234abcd',['c'*64,'b'*64])
            kube.assert_not_called()

    def test_expired_checkpoint_never_restarts(self):
        state=self.state();state['expires_at']=(dt.datetime.now(dt.timezone.utc)-dt.timedelta(seconds=1)).isoformat()
        with patch('root_main.private_json',return_value=state),patch('root_main.Kube') as kube:
            with self.assertRaises(Blocked):
                root_main.resume(Path('/captured-code'),root_main.ROOT/'known-baseline-abcd1234abcd',['a'*64,'b'*64])
            kube.assert_not_called()

    def test_credentials_rotation_rejects_before_script_reuse(self):
        class Kube:
            def get(self,kind,name):return {'metadata':{'uid':'current','resourceVersion':'11'}}
            def secret_meta(self,name):return self.get('secret',name)['metadata']
        with self.assertRaises(Blocked):
            root_main.revalidate_inputs(Kube(),{'scripts':[]},
                {'social.creds':{'resource':'known','uid':'original','resource_version':'10'}})


if __name__=='__main__':unittest.main()
