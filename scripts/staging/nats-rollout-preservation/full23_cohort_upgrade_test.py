"""Old installed cohort must not silently adopt new private history."""
import contextlib
import hashlib
import json
import os
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import installer
import operator_retirement
import story_witness
from resource_claim_test import context


class FutureHistoryTests(unittest.TestCase):
    def target(self):
        result=json.loads((Path(__file__).resolve().parent/'full23_cohort_successor_fixture.json').read_bytes())
        result['nats-rollout-preservation/installer.py']=hashlib.sha256(Path(installer.__file__).read_bytes()).hexdigest()
        return result

    def test_exact_actual_81_to_provisional_94_source_and_binary_pins(self):
        target=self.target();self.assertEqual(len(target),94)
        self.assertEqual(set(target),installer.TT_FENCING_PREDECESSOR_MEMBERS|
            {installer.TT_FENCING_NEW_MEMBER,'nats-rollout-preservation/gis_existing_update.py'} | installer.FULL23_NEW_MEMBERS)
        projection={k:v for k,v in target.items() if k!='nats-rollout-preservation/installer.py'}
        self.assertEqual(hashlib.sha256(json.dumps(projection,sort_keys=True,separators=(',',':')).encode()).hexdigest(),
                         '54aa6eca4a2f7435224fac6e3ac5f4953601122d91fded31eb7dd3953fc62ff4')
        old=json.loads((Path(__file__).resolve().parent/'installed_cohort_predecessor_fixture.json').read_bytes())
        for name in installer.TT_FENCING_BINARY_MEMBERS:self.assertEqual(target[name],old[name])
        with patch.object(installer.root_cli,'code_binding',return_value=target), \
             patch.object(installer,'_upgrade_idle_transition') as transition:
            installer.upgrade_idle_full23_installed_cohort_v1(Path('/capture'))
        self.assertEqual(transition.call_args.args[:4],(Path('/capture'),
            '78f5532cfc06766e5482caf402a6a0d30c7d47255f1fc441a43149132859546b',
            'idle-full23-installed-cohort-v1','installed-78f5532c-full23'))

    def test_every_target_member_drift_missing_extra_and_installer_are_rejected(self):
        valid=self.target()
        for name in valid:
            for kind in ('changed','missing'):
                value=dict(valid)
                if kind=='changed':value[name]='0'*64
                else:del value[name]
                with self.subTest(name=name,kind=kind),patch.object(installer.root_cli,'code_binding',return_value=value), \
                     patch.object(installer,'_upgrade_idle_transition') as transition:
                    with self.assertRaises(installer.Blocked):installer.upgrade_idle_full23_installed_cohort_v1(Path('/capture'))
                    transition.assert_not_called()
        with patch.object(installer.root_cli,'code_binding',return_value=dict(valid,foreign='f'*64)), \
             patch.object(installer,'_upgrade_idle_transition') as transition:
            with self.assertRaises(installer.Blocked):installer.upgrade_idle_full23_installed_cohort_v1(Path('/capture'))
            transition.assert_not_called()
    def test_new_history_paths_must_all_be_absent(self):
        with tempfile.TemporaryDirectory() as folder:
            installed=Path(folder)
            installer.require_future_history_absent(installed)
            for name in ('source-authority','realtime-bootstrap','prerequisites'):
                path=installed/name;path.mkdir()
                with self.assertRaisesRegex(installer.Blocked,'bridge_full23_future_history_present'):
                    installer.require_future_history_absent(installed)
                path.rmdir()
                path.symlink_to(installed/'nonexistent',target_is_directory=True)
                with self.assertRaisesRegex(installer.Blocked,'bridge_full23_future_history_present'):
                    installer.require_future_history_absent(installed)
                path.unlink()

    def test_actual_full23_idle_transition_vetoes_presence_before_replacement(self):
        for name in ('source-authority','realtime-bootstrap','prerequisites'):
            with self.subTest(name=name),tempfile.TemporaryDirectory() as folder:
                root=Path(folder);installed=root/'installed';installed.mkdir(mode=0o750)
                for child,mode in (('inbox',0o1730),('processing',0o700),('journal',0o700),('responses',0o750),('recovery',0o700)):
                    (installed/child).mkdir(mode=mode);(installed/child).chmod(mode)
                (installed/name).mkdir(mode=0o700)
                base=root/('rollout-'+'a'*12);base.mkdir();(base/'checkpoint.json').write_text('{}')
                with patch.object(installer.guard,'ROOT',root),patch.object(installer,'operation_lock',contextlib.nullcontext), \
                     patch.object(installer.root_cli,'code_binding',return_value={}), \
                     patch.object(installer.root_cli,'rollout_directory_kind',return_value='operation'), \
                     patch.object(installer,'checkpoint_json',return_value={'operator_retirement':{}}), \
                     patch.object(operator_retirement,'idle_state'), \
                     patch.object(operator_retirement,'verify_terminal',return_value={'verified':True}), \
                     patch.object(operator_retirement,'private_bytes',return_value=b'fixture'), \
                     patch.object(installer,'Kube') as kube,patch.object(installer,'_upgrade_code') as replace:
                    kube.return_value.get.return_value={'data':{'phase':'active'}}
                    with self.assertRaisesRegex(installer.Blocked,'bridge_full23_future_history_present'):
                        installer._upgrade_idle_transition(root/'new','a'*64,'idle-full23-installed-cohort-v1','old-full23','unused')
                    replace.assert_not_called()
                self.assertEqual((installed/'inbox').stat().st_mode&0o7777,0o1730)

    def test_active_tt_journal_is_rejected_before_swap(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder);installed=root/'installed';installed.mkdir(mode=0o750)
            for child,mode in (('inbox',0o1730),('processing',0o700),('journal',0o700),('responses',0o750),('recovery',0o700)):
                (installed/child).mkdir(mode=mode);(installed/child).chmod(mode)
            base=root/('rollout-'+'a'*12);base.mkdir();(base/'checkpoint.json').write_text('{}')
            journal=installed/'tt-resource-fence.json'
            journal.write_text(json.dumps(dict(schema='voice-staging-resource-fence-v1',context=context(),operation='a'*12,
                helper_sha256='b'*64,target_sha256='d'*64,status='active',phase='executing',actual=context()['images'])))
            journal.chmod(0o440);before=journal.read_bytes()
            with patch.object(installer.guard,'ROOT',root),patch.object(installer,'operation_lock',contextlib.nullcontext), \
                 patch.object(installer.root_cli,'code_binding',return_value={}), \
                 patch.object(installer.root_cli,'rollout_directory_kind',return_value='operation'), \
                 patch.object(installer,'checkpoint_json',return_value={'operator_retirement':{}}), \
                 patch.object(operator_retirement,'idle_state'), \
                 patch.object(operator_retirement,'verify_terminal',return_value={'verified':True}), \
                 patch.object(operator_retirement,'private_bytes',return_value=b'fixture'), \
                 patch.object(installer,'Kube') as kube,patch.object(installer,'_upgrade_code') as replace:
                kube.return_value.get.return_value={'data':{'phase':'active'}}
                with self.assertRaisesRegex(installer.Blocked,'resource_reconciliation_required'):
                    installer._upgrade_idle_transition(root/'new','a'*64,'idle-full23-installed-cohort-v1','old-full23','unused')
                replace.assert_not_called()
            self.assertEqual(journal.read_bytes(),before)
            self.assertEqual((installed/'inbox').stat().st_mode&0o7777,0o1730)

    def test_new_history_created_during_transition_is_detected(self):
        for timing in ('before_swap','after_swap'):
            with self.subTest(timing=timing),tempfile.TemporaryDirectory() as folder:
                root=Path(folder);installed=root/'installed';installed.mkdir(mode=0o750)
                for child,mode in (('inbox',0o1730),('processing',0o700),('journal',0o700),('responses',0o750),('recovery',0o700)):
                    (installed/child).mkdir(mode=mode);(installed/child).chmod(mode)
                base=root/('rollout-'+'a'*12);base.mkdir();(base/'checkpoint.json').write_text('{}')
                def witnesses():
                    if timing=='before_swap':(installed/'source-authority').mkdir(exist_ok=True)
                    return {}
                def replace(*_):
                    if timing=='after_swap':(installed/'realtime-bootstrap').mkdir()
                with patch.object(installer.guard,'ROOT',root),patch.object(installer,'operation_lock',contextlib.nullcontext), \
                     patch.object(installer.root_cli,'code_binding',return_value={}), \
                     patch.object(installer.root_cli,'rollout_directory_kind',return_value='operation'), \
                     patch.object(installer,'checkpoint_json',return_value={'operator_retirement':{}}), \
                     patch.object(operator_retirement,'idle_state'), \
                     patch.object(operator_retirement,'verify_terminal',return_value={'verified':True}), \
                     patch.object(operator_retirement,'private_bytes',return_value=b'fixture'), \
                     patch.object(operator_retirement,'file_sha',return_value=hashlib.sha256(b'fixture').hexdigest()), \
                     patch.object(installer.encrypted_cut,'regular',side_effect=lambda *a,**k:(os.open('/dev/null',os.O_RDONLY),None)), \
                     patch.object(installer,'tt_resource_journal_snapshot',return_value={}), \
                     patch.object(installer.grp,'getgrnam',return_value=SimpleNamespace(gr_gid=0)), \
                     patch.object(story_witness,'registry_snapshot',side_effect=witnesses), \
                     patch.object(installer,'Kube') as kube,patch.object(installer,'_upgrade_code',side_effect=replace) as swap:
                    kube.return_value.get.return_value={'data':{'phase':'active'}}
                    with self.assertRaisesRegex(installer.Blocked,'bridge_full23_future_history_present'):
                        installer._upgrade_idle_transition(root/'new','a'*64,'idle-full23-installed-cohort-v1','old-full23','unused')
                    self.assertEqual(swap.call_count,0 if timing=='before_swap' else 1)
                self.assertEqual((installed/'inbox').stat().st_mode&0o7777,0o1730)

    def test_interrupted_swap_requires_exact_receipt_staged_and_preserved_code(self):
        predecessor='78f5532cfc06766e5482caf402a6a0d30c7d47255f1fc441a43149132859546b'
        for damage in ('none','receipt_missing','receipt_changed','stage_changed','backup_changed','current_present'):
            with self.subTest(damage=damage),tempfile.TemporaryDirectory() as folder:
                root=Path(folder);source=root/'source';source.mkdir();(source/'binding').write_text('new')
                installed=root/'installed';installed.mkdir();(installed/'code').mkdir()
                (installed/'code'/'binding').write_text('old')
                def binding(path):return {'old':predecessor,'new':'d'*64,'bad':'f'*64}[(Path(path)/'binding').read_text()]
                rename=installer.os.rename
                def crash(old,new):
                    if Path(old).name=='code-idle-full23-installed-cohort-v1-staged':raise RuntimeError('interrupted')
                    return rename(old,new)
                args=(source,installed,0,predecessor,'idle-full23-installed-cohort-v1','installed-78f5532c-full23')
                with patch.object(installer,'binding_sha',side_effect=binding),patch.object(installer.os,'chown'), \
                     patch.object(installer,'private_json',side_effect=lambda p:json.loads(Path(p).read_text())):
                    with patch.object(installer.os,'rename',side_effect=crash):
                        with self.assertRaisesRegex(RuntimeError,'interrupted'):installer._upgrade_code(*args)
                    receipt=installed/'upgrade-idle-full23-installed-cohort-v1.json'
                    if damage=='receipt_missing':receipt.unlink()
                    elif damage=='receipt_changed':receipt.write_text('{}')
                    elif damage=='stage_changed':(installed/'code-idle-full23-installed-cohort-v1-staged'/'binding').write_text('bad')
                    elif damage=='backup_changed':(installed/'code-installed-78f5532c-full23-preserved'/'binding').write_text('bad')
                    elif damage=='current_present':
                        (installed/'code').mkdir();(installed/'code'/'binding').write_text('bad')
                    if damage=='none':
                        installer._upgrade_code(*args);installer._upgrade_code(*args)
                        self.assertEqual((installed/'code'/'binding').read_text(),'new')
                        self.assertEqual((installed/'code-installed-78f5532c-full23-preserved'/'binding').read_text(),'old')
                    else:
                        with patch.object(installer.os,'rename') as attempt:
                            with self.assertRaises((installer.Blocked,FileNotFoundError)):installer._upgrade_code(*args)
                            attempt.assert_not_called()


if __name__=='__main__':unittest.main()
