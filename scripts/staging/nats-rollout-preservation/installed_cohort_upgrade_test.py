"""The public installed cohort must have one finite, binary-preserving target."""
import copy
import contextlib
import hashlib
import json
import tempfile
from pathlib import Path
import unittest
from unittest.mock import patch

import installer
import operator_retirement

HERE = Path(__file__).resolve().parent
INSTALLED = '78f5532cfc06766e5482caf402a6a0d30c7d47255f1fc441a43149132859546b'
SELF = 'nats-rollout-preservation/installer.py'


def digest(binding):
    return hashlib.sha256(json.dumps(binding, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def target():
    binding = json.loads((HERE / 'installed_cohort_successor_fixture.json').read_bytes())
    binding[SELF] = hashlib.sha256(Path(installer.__file__).read_bytes()).hexdigest()
    return binding


class InstalledCohortUpgradeTests(unittest.TestCase):
    def test_public_manifest_and_reviewed_historical_target_are_exact(self):
        old = json.loads((HERE / 'installed_cohort_predecessor_fixture.json').read_bytes())
        successor = target()
        self.assertEqual(len(old), 81)
        self.assertEqual(digest(old), INSTALLED)
        self.assertEqual(len(successor), 86)
        self.assertEqual(digest({k:v for k,v in successor.items() if k != SELF}),
                         'd4c2c9f639975f66c6cc2caeac713d3e447d76cebf7f7d3b1edb5673f5da884a')
        for binary in installer.TT_FENCING_BINARY_MEMBERS:
            self.assertEqual(successor[binary], old[binary])
        self.assertEqual(digest(installer.tt_fencing_predecessor(successor)),
                         '0a81983170a46924ed21194e005087a999748a7e13e6762d6eefa76ed526fc21')

    def test_exact_target_uses_fixed_observed_predecessor_and_guarded_idle_path(self):
        with patch.object(installer.root_cli, 'code_binding', return_value=target()), \
             patch.object(installer, '_upgrade_idle_transition') as transition:
            installer.upgrade_idle_installed_cohort_v1(Path('/captured'))
        args = transition.call_args.args
        self.assertEqual(args[0], Path('/captured'))
        self.assertEqual(args[1], INSTALLED)
        self.assertEqual(args[2], 'idle-installed-cohort-v1')

    def test_every_target_member_drift_missing_extra_and_self_rejected(self):
        valid = target()
        mutations = []
        for key in valid:
            changed = copy.copy(valid)
            changed[key] = '0' * 64
            mutations.append((key + ':changed', changed))
            missing = copy.copy(valid)
            del missing[key]
            mutations.append((key + ':missing', missing))
        mutations.append(('extra', dict(valid, foreign='f' * 64)))
        for label, changed in mutations:
            with self.subTest(case=label), \
                 patch.object(installer.root_cli, 'code_binding', return_value=changed), \
                 patch.object(installer, '_upgrade_idle_transition') as transition:
                with self.assertRaises(installer.Blocked):
                    installer.upgrade_idle_installed_cohort_v1(Path('/captured'))
                transition.assert_not_called()

    def test_active_resource_journal_blocks_before_replacement(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            installed = root / 'installed'
            installed.mkdir(mode=0o750)
            for name,mode in (('inbox',0o1730),('processing',0o700),('journal',0o700),('responses',0o750),('recovery',0o700)):
                (installed/name).mkdir(mode=mode)
                (installed/name).chmod(mode)
            base = root / ('rollout-' + 'a' * 12)
            base.mkdir()
            (base/'checkpoint.json').write_text('{}')
            journal = installed/'tt-resource-fence.json'
            journal.write_text(json.dumps({'status':'active'}))
            journal.chmod(0o440)
            before = journal.read_bytes()
            with patch.object(installer.guard,'ROOT',root), patch.object(installer,'operation_lock',contextlib.nullcontext), \
                 patch.object(installer.root_cli,'code_binding',return_value={}), \
                 patch.object(installer.root_cli,'rollout_directory_kind',return_value='operation'), \
                 patch.object(installer,'checkpoint_json',return_value={'operator_retirement':{}}), \
                 patch.object(operator_retirement,'idle_state'), \
                 patch.object(operator_retirement,'verify_terminal',return_value={'verified':True}), \
                 patch.object(operator_retirement,'private_bytes',return_value=b'fixture'), \
                 patch.object(installer,'Kube') as kube, patch.object(installer,'_upgrade_code') as replace:
                kube.return_value.get.return_value = {'data':{'phase':'active'}}
                with self.assertRaisesRegex(installer.Blocked,'resource_reconciliation_required'):
                    installer._upgrade_idle_transition(root/'new',INSTALLED,'idle-installed-cohort-v1','installed-78f5532c','unused')
                replace.assert_not_called()
            self.assertEqual(journal.read_bytes(),before)
            self.assertEqual((installed/'inbox').stat().st_mode & 0o7777,0o1730)

    def test_abrupt_death_leaving_closed_inbox_blocks_full_route_retry(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder); installed=root/'installed'; installed.mkdir(mode=0o750)
            for name in ('inbox','processing','journal','responses','recovery'):
                (installed/name).mkdir(mode=0o700)
            base=root/('rollout-'+'a'*12); base.mkdir(); (base/'checkpoint.json').write_text('{}')
            with patch.object(installer.guard,'ROOT',root), patch.object(installer,'operation_lock',contextlib.nullcontext), \
                 patch.object(installer.root_cli,'code_binding',return_value={}), \
                 patch.object(installer.root_cli,'rollout_directory_kind',return_value='operation'), \
                 patch.object(installer,'checkpoint_json',return_value={'operator_retirement':{}}), \
                 patch.object(operator_retirement,'idle_state'), \
                 patch.object(operator_retirement,'verify_terminal',return_value={'verified':True}), \
                 patch.object(operator_retirement,'private_bytes',return_value=b'fixture'), \
                 patch.object(installer,'Kube') as kube, patch.object(installer,'_upgrade_code') as replace:
                kube.return_value.get.return_value={'data':{'phase':'active'}}
                with self.assertRaisesRegex(installer.Blocked,'directory_untrusted'):
                    installer._upgrade_idle_transition(root/'new',INSTALLED,'idle-installed-cohort-v1','installed-78f5532c','unused')
                replace.assert_not_called()
            self.assertEqual((installed/'inbox').stat().st_mode & 0o7777,0o700)

    def test_interrupted_swap_resumes_only_with_exact_receipt_and_backup(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            source = root/'source'; source.mkdir(); (source/'binding').write_text('new')
            installed = root/'installed'; installed.mkdir(); (installed/'code').mkdir()
            (installed/'code'/'binding').write_text('old')
            def binding(path):
                return INSTALLED if (Path(path)/'binding').read_text() == 'old' else 'd'*64
            rename = installer.os.rename
            def crash(old,new):
                if Path(old).name == 'code-idle-installed-cohort-v1-staged':
                    raise RuntimeError('interrupted')
                return rename(old,new)
            args = (source,installed,0,INSTALLED,'idle-installed-cohort-v1','installed-78f5532c')
            with patch.object(installer,'binding_sha',side_effect=binding), \
                 patch.object(installer.os,'chown'), \
                 patch.object(installer,'private_json',side_effect=lambda p:json.loads(Path(p).read_text())):
                with patch.object(installer.os,'rename',side_effect=crash):
                    with self.assertRaisesRegex(RuntimeError,'interrupted'):
                        installer._upgrade_code(*args)
                self.assertFalse((installed/'code').exists())
                self.assertEqual((installed/'code-installed-78f5532c-preserved'/'binding').read_text(),'old')
                installer._upgrade_code(*args)
                installer._upgrade_code(*args)
            self.assertEqual((installed/'code'/'binding').read_text(),'new')

    def test_interrupted_swap_rejects_missing_or_changed_receipt_stage_backup_and_current(self):
        for corruption in ('receipt_missing','receipt_changed','stage_changed','backup_changed','current_present'):
            with self.subTest(corruption=corruption), tempfile.TemporaryDirectory() as folder:
                root = Path(folder)
                source = root/'source'; source.mkdir(); (source/'binding').write_text('new')
                installed = root/'installed'; installed.mkdir(); (installed/'code').mkdir()
                (installed/'code'/'binding').write_text('old')
                def binding(path):
                    return {'old':INSTALLED,'new':'d'*64,'bad':'f'*64}[(Path(path)/'binding').read_text()]
                rename = installer.os.rename
                def crash(old,new):
                    if Path(old).name == 'code-idle-installed-cohort-v1-staged':
                        raise RuntimeError('interrupted')
                    return rename(old,new)
                args = (source,installed,0,INSTALLED,'idle-installed-cohort-v1','installed-78f5532c')
                with patch.object(installer,'binding_sha',side_effect=binding), \
                     patch.object(installer.os,'chown'), \
                     patch.object(installer,'private_json',side_effect=lambda p:json.loads(Path(p).read_text())):
                    with patch.object(installer.os,'rename',side_effect=crash):
                        with self.assertRaisesRegex(RuntimeError,'interrupted'):
                            installer._upgrade_code(*args)
                    receipt=installed/'upgrade-idle-installed-cohort-v1.json'
                    if corruption=='receipt_missing': receipt.unlink()
                    elif corruption=='receipt_changed': receipt.write_text('{}')
                    elif corruption=='stage_changed': (installed/'code-idle-installed-cohort-v1-staged'/'binding').write_text('bad')
                    elif corruption=='backup_changed': (installed/'code-installed-78f5532c-preserved'/'binding').write_text('bad')
                    else:
                        (installed/'code').mkdir(); (installed/'code'/'binding').write_text('bad')
                    with patch.object(installer.os,'rename') as attempt:
                        with self.assertRaises((installer.Blocked,FileNotFoundError)): installer._upgrade_code(*args)
                        attempt.assert_not_called()


if __name__ == '__main__':
    unittest.main()
