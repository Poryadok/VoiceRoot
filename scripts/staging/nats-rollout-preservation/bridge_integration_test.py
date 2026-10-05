import contextlib
import json
import os
from pathlib import Path
import subprocess
import tempfile
import unittest
from unittest.mock import patch
import bridge
import bridge_client
import installer
import root_cli
import bridge_root

class IntegrationTests(unittest.TestCase):
    def test_workflows_use_supported_yaml_aliases_without_merge_keys(self):
        import yaml
        root = Path(__file__).resolve().parents[3]
        def validate(node):
            self.assertNotEqual(node.tag, 'tag:yaml.org,2002:merge')
            if isinstance(node, yaml.MappingNode):
                for key, value in node.value:
                    self.assertNotEqual(key.value, '<<')
                    validate(key); validate(value)
            elif isinstance(node, yaml.SequenceNode):
                for value in node.value: validate(value)
        for name in ('ci.yml', 'staging-deploy.yml'):
            validate(yaml.compose((root / '.github/workflows' / name).read_text()))
        workflow = yaml.safe_load((root / '.github/workflows/staging-deploy.yml').read_text())
        steps = workflow['jobs']['deploy']['steps']
        prepare = next(s for s in steps if s.get('id') == 'preservation')
        self.assertEqual(prepare['env']['VOICE_NATS_ROLLOUT_OPERATION'], '')
        self.assertEqual(prepare['env']['VOICE_IMAGE_TAG'], '${{ inputs.image_tag }}')
        self.assertEqual(prepare['env']['ROLLOUT_ARTIFACT_ID'], '')
        apply = next(s for s in steps if s.get('id') == 'paused_apply')
        self.assertEqual(apply['env']['ROLLOUT_CLAIM_RV'], '')
        finish = next(s for s in steps if s.get('env', {}).get('ROLLOUT_ACTION') == '--bridge-finish')
        self.assertEqual(finish['env']['ROLLOUT_CLAIM_RV'], '${{ steps.paused_apply.outputs.claim_rv }}')
        self.assertEqual(finish['env']['ROLLOUT_ARTIFACT_ID'], '${{ steps.encrypted_backup.outputs.artifact-id }}')

    def test_idle_distinguishes_verified_launcher_capture_from_operation(self):
        import hashlib,guard
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);capture=root/('rollout-code-'+'a'*32);capture.mkdir(mode=0o700)
            sentinel=capture/'sentinel.py';sentinel.write_text('SAFE=True');sentinel.chmod(0o600)
            manifest=capture/'capture-manifest.json';manifest.write_text(json.dumps({'sentinel.py':hashlib.sha256(sentinel.read_bytes()).hexdigest()}));manifest.chmod(0o600)
            actions=object.__new__(bridge_root.Actions)
            with patch.object(guard,'ROOT',root):
                actions._idle()
                sentinel.write_text('CHANGED=True')
                with self.assertRaises(Exception):actions._idle()
                sentinel.write_text('SAFE=True')
                manifest.unlink()
                with self.assertRaisesRegex(Exception,'rollout_capture_incomplete'):actions._idle()
                manifest.write_text(json.dumps({'sentinel.py':hashlib.sha256(sentinel.read_bytes()).hexdigest()}));manifest.chmod(0o600)
                capture.chmod(0o777)
                with self.assertRaises(Exception):actions._idle()
                capture.chmod(0o700);os.chown(sentinel,1000,1000)
                with self.assertRaises(Exception):actions._idle()
                os.chown(sentinel,0,0)
                moved=root/'captured';capture.rename(moved);capture.symlink_to(moved)
                with self.assertRaises(Exception):actions._idle()
                capture.unlink();moved.rename(capture)
                operation=root/('rollout-'+'b'*12);operation.mkdir(mode=0o700)
                with self.assertRaises(Exception):actions._idle()
                checkpoint=operation/'checkpoint.json';checkpoint.write_text('{"status":"PASS"}');checkpoint.chmod(0o600)
                actions._idle()
                (root/'rollout-code-unrecognized').mkdir(mode=0o700)
                with self.assertRaises(Exception):actions._idle()

    def run_fixture(self,workflow,event,run_id=9002,head='a'*40):
        path='.github/workflows/ci.yml' if workflow==263230665 else '.github/workflows/staging-deploy.yml'
        return {'id':run_id,'workflow_id':workflow,'event':event,'head_sha':head,'head_branch':'master','status':'in_progress','run_attempt':2,'path':path,'repository':{'full_name':'Poryadok/VoiceRoot'},'head_repository':{'full_name':'Poryadok/VoiceRoot'}}

    def test_standalone_rollback_separates_approved_ci_and_custody_dispatcher(self):
        actions=object.__new__(bridge_root.Actions)
        dispatcher=self.run_fixture(263689731,'workflow_dispatch')
        approved=self.run_fixture(263230665,'push',9001)
        with patch.object(bridge_root.source_authority,'_get',side_effect=[dispatcher,{'workflow_runs':[approved]}]):
            source_run,execution=actions._dispatcher({'run_id':9002,'token':'private'})
        self.assertEqual(source_run,9001);self.assertEqual(execution['dispatcher_run_id'],9002)
        self.assertEqual(execution['head_sha'],'a'*40)
        # A called reusable workflow is the original CI push run, not a
        # synthetic staging run; source and execution IDs coincide here.
        with patch.object(bridge_root.source_authority,'_get',return_value=approved):
            source_run,execution=actions._dispatcher({'run_id':9001,'source_sha':'a'*40,'token':'private'})
        self.assertEqual(source_run,execution['dispatcher_run_id'])

    def test_dispatcher_foreign_head_workflow_and_unapproved_source_reject(self):
        actions=object.__new__(bridge_root.Actions)
        for altered in (dict(self.run_fixture(263689731,'workflow_dispatch'),head_branch='other'),dict(self.run_fixture(263689731,'workflow_dispatch'),workflow_id=7),dict(self.run_fixture(263689731,'workflow_dispatch'),head_repository={'full_name':'other/VoiceRoot'})):
            with patch.object(bridge_root.source_authority,'_get',return_value=altered),self.assertRaises(Exception):actions._dispatcher({'run_id':9002,'token':'private'})
        with patch.object(bridge_root.source_authority,'_get',side_effect=[self.run_fixture(263689731,'workflow_dispatch'),{'workflow_runs':[self.run_fixture(263230665,'push',9001,'b'*40)]}]),self.assertRaises(Exception):actions._dispatcher({'run_id':9002,'token':'private'})

    def test_nonlatest_pass_rollback_never_prepares_or_fences(self):
        actions=object.__new__(bridge_root.Actions)
        previous={'operation':'a'*12,'target':{'manifest_sha256':'c'*64}}
        with patch.object(actions,'state',return_value=(Path('/owned'),previous)),patch.object(bridge_root,'private_json',return_value={'operation':'b'*12}),patch.object(actions,'_prepare') as prepare,self.assertRaises(Exception):actions.execute({'action':'prepare-rollback','operation':'a'*12,'run_id':9002,'token':'private','nonce':'d'*64})
        prepare.assert_not_called()

    def test_actual_reusable_parent_run_and_source_request_binding(self):
        environment={'GITHUB_RUN_ID':'9001','GITHUB_RUN_ATTEMPT':'2','GITHUB_SHA':'a'*40,'GITHUB_TOKEN':'private-token','GITHUB_JOB':'deploy'}
        row=bridge_client.request(['prepare','images-only','web','a'*40,'9001'],environment)
        self.assertEqual(row['run_id'],9001);self.assertEqual(row['source_sha'],environment['GITHUB_SHA'])
        with self.assertRaises(bridge.BridgeError):bridge_client.request(['prepare','images-only','web','a'*40,'9002'],environment)
        with self.assertRaises(bridge.BridgeError):bridge_client.request(['prepare','images-only','web','b'*40,'9001'],environment)

    def test_actual_push_required_jobs_script_rejects_missing_and_accepts_success(self):
        script=Path(__file__).resolve().parents[3]/'.github/ci/verify-required-jobs.sh'
        environment=dict(os.environ,RUN_AUTH='true',JOB_BACKEND_AUTH='skipped')
        missing=subprocess.run(['bash',str(script),'push','true','auto'],env=environment,capture_output=True,text=True)
        self.assertNotEqual(missing.returncode,0);self.assertIn('backend-auth',missing.stderr)
        environment['JOB_BACKEND_AUTH']='success'
        ready=subprocess.run(['bash',str(script),'push','true','auto'],env=environment,capture_output=True,text=True)
        self.assertEqual(ready.returncode,0,ready.stderr);self.assertIn('all required jobs present',ready.stdout)

    def test_actual_workflow_bridge_order_and_ci_dependency(self):
        import yaml
        root=Path(__file__).resolve().parents[3]
        workflow=yaml.safe_load((root/'.github/workflows/staging-deploy.yml').read_text())
        steps=workflow['jobs']['deploy']['steps']
        names=[s['name'] for s in steps if 'name' in s]
        expected=['Prepare cold backup and isolated proof using installed root bridge','Upload encrypted backup to off-node GitHub custody','Verify complete off-node ciphertext readback and authorize paused apply','Configure kubectl using protected captured code','Verify root-established preservation authorization','Apply exact frozen target while all consumers remain paused','Prove unchanged store and resume through installed root bridge']
        self.assertEqual([n for n in names if n in expected],expected)
        artifact=next(s for s in steps if s.get('id')=='encrypted_backup')
        self.assertEqual(artifact['with']['path'],'${{ steps.preservation.outputs.cipher_path }}')
        self.assertNotIn('rollout-before',json.dumps(artifact))
        ci=yaml.safe_load((root/'.github/workflows/ci.yml').read_text())
        self.assertIn('ci-gate',ci['jobs']['deploy-staging']['needs'])
        self.assertIn("needs.ci-gate.result == 'success'",ci['jobs']['deploy-staging']['if'])
        self.assertIn("github.event_name == 'push'",ci['jobs']['ci-gate']['if'])
        self.assertEqual(workflow['permissions']['actions'],'read')

    @unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'disposable root Linux fixture')
    def test_installer_fixed_units_private_key_dirs_and_no_replacement(self):
        import guard
        with tempfile.TemporaryDirectory() as directory:
            root=Path(directory);code=root/'captured';code.mkdir();(code/'sentinel.py').write_text('SAFE=True')
            policy=root/'policy.json';policy.write_text(json.dumps({'s3_signing_endpoint':'https://example.invalid'}));policy.chmod(0o600)
            class FakeKube:
                def get(self,kind,name):return {'data':{'phase':'active'}}
            calls=[]
            with patch.object(guard,'ROOT',root),patch.object(root_cli,'code_binding',return_value={'sentinel.py':'a'*64}),patch.object(installer,'Kube',FakeKube),patch.object(installer,'operation_lock',contextlib.nullcontext),patch.object(installer.encrypted_cut,'directory',side_effect=lambda path:Path(path)),patch.object(installer.encrypted_cut,'initialize_recovery_key') as key,patch('grp.getgrnam',return_value=type('G',(),{'gr_gid':65532})()),patch.object(installer.subprocess,'run',side_effect=lambda args,**kwargs:calls.append(args)):
                installer.install(code,policy)
                key.assert_called_once_with(root/'installed'/'recovery')
                self.assertEqual((root/'installed/recovery').stat().st_mode&0o777,0o700)
                self.assertEqual((root/'installed/inbox').stat().st_mode&0o7777,0o1730)
                with self.assertRaises(Exception):installer.install(code,policy)
            for suffix in ('service','path','timer'):
                path=Path('/etc/systemd/system/voice-nats-preservation.'+suffix)
                text=path.read_text();self.assertNotIn('sudo',text)
                if suffix=='service':self.assertIn('/installed/code/nats-rollout-preservation/bridge.py',text)
                path.unlink()
            self.assertEqual(calls[-1],['/usr/bin/systemctl','enable','--now','voice-nats-preservation.path','voice-nats-preservation.timer'])
