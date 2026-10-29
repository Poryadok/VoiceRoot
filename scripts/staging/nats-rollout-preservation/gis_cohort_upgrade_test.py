"""Exact installed 81-to-GIS successor transition; no host mutation."""
import contextlib
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import installer
import operator_retirement
from resource_claim_test import context

HERE=Path(__file__).resolve().parent
PREDECESSOR='78f5532cfc06766e5482caf402a6a0d30c7d47255f1fc441a43149132859546b'
SELF='nats-rollout-preservation/installer.py'


def sha(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()


def target():
    value=json.loads((HERE/'gis_cohort_successor_fixture.json').read_bytes())
    value[SELF]=hashlib.sha256(Path(installer.__file__).read_bytes()).hexdigest()
    return value


class GISInstalledCohortTests(unittest.TestCase):
    def test_fixed_source_and_installed_binary_pins(self):
        actual=json.loads((HERE/'installed_cohort_predecessor_fixture.json').read_bytes())
        candidate=target()
        self.assertEqual(sha(actual),PREDECESSOR)
        self.assertEqual(len(candidate),87)
        self.assertIn('nats-rollout-preservation/gis_existing_update.py',candidate)
        self.assertEqual(sha({k:v for k,v in candidate.items() if k!=SELF}),
                         'd64f32b7383e06cd454d2f302ebce656ef7dfcb8b97b3012f20824f2b74615ef')
        for name in installer.TT_FENCING_BINARY_MEMBERS:
            self.assertEqual(candidate[name],actual[name])

    def test_fixed_actual_predecessor_uses_existing_idle_path(self):
        with patch.object(installer.root_cli,'code_binding',return_value=target()), \
             patch.object(installer,'_upgrade_idle_transition') as transition:
            installer.upgrade_idle_gis_installed_cohort_v1(Path('/capture'))
        self.assertEqual(transition.call_args.args[:4],
                         (Path('/capture'),PREDECESSOR,'idle-gis-installed-cohort-v1','installed-78f5532c-gis'))

    def test_each_changed_missing_extra_or_self_member_rejected(self):
        valid=target()
        for name in valid:
            for kind in ('changed','missing'):
                value=dict(valid)
                if kind=='changed':value[name]='0'*64
                else:del value[name]
                with self.subTest(member=name,kind=kind),patch.object(installer.root_cli,'code_binding',return_value=value), \
                     patch.object(installer,'_upgrade_idle_transition') as transition:
                    with self.assertRaises(installer.Blocked):installer.upgrade_idle_gis_installed_cohort_v1(Path('/capture'))
                    transition.assert_not_called()
        with patch.object(installer.root_cli,'code_binding',return_value=dict(valid,foreign='f'*64)), \
             patch.object(installer,'_upgrade_idle_transition') as transition:
            with self.assertRaises(installer.Blocked):installer.upgrade_idle_gis_installed_cohort_v1(Path('/capture'))
            transition.assert_not_called()

    def test_active_resource_journal_blocks_before_replacement(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder);installed=root/'installed';installed.mkdir(mode=0o750)
            for name,mode in (('inbox',0o1730),('processing',0o700),('journal',0o700),('responses',0o750),('recovery',0o700)):
                (installed/name).mkdir(mode=mode);(installed/name).chmod(mode)
            base=root/('rollout-'+'a'*12);base.mkdir();(base/'checkpoint.json').write_text('{}')
            journal=installed/'tt-resource-fence.json'
            journal.write_text(json.dumps(dict(schema='voice-staging-resource-fence-v1',context=context(),operation='a'*12,
                helper_sha256='b'*64,target_sha256='d'*64,status='active',phase='executing',actual=context()['images'])))
            journal.chmod(0o440)
            before=journal.read_bytes()
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
                    installer._upgrade_idle_transition(root/'new',PREDECESSOR,'idle-gis-installed-cohort-v1','installed-78f5532c-gis','unused')
                replace.assert_not_called()
            self.assertEqual(journal.read_bytes(),before)
            self.assertEqual((installed/'inbox').stat().st_mode&0o7777,0o1730)

    def test_interruption_requires_exact_receipt_stage_and_preserved_predecessor(self):
        for corruption in ('none','receipt_missing','receipt_changed','stage_changed','backup_changed','current_present'):
            with self.subTest(corruption=corruption),tempfile.TemporaryDirectory() as folder:
                root=Path(folder);source=root/'source';source.mkdir();(source/'binding').write_text('new')
                installed=root/'installed';installed.mkdir();(installed/'code').mkdir()
                (installed/'code'/'binding').write_text('old')
                def binding(path):return {'old':PREDECESSOR,'new':'d'*64,'bad':'f'*64}[(Path(path)/'binding').read_text()]
                rename=installer.os.rename
                def crash(old,new):
                    if Path(old).name=='code-idle-gis-installed-cohort-v1-staged':raise RuntimeError('interrupted')
                    return rename(old,new)
                args=(source,installed,0,PREDECESSOR,'idle-gis-installed-cohort-v1','installed-78f5532c-gis')
                with patch.object(installer,'binding_sha',side_effect=binding),patch.object(installer.os,'chown'), \
                     patch.object(installer,'private_json',side_effect=lambda p:json.loads(Path(p).read_text())):
                    with patch.object(installer.os,'rename',side_effect=crash):
                        with self.assertRaisesRegex(RuntimeError,'interrupted'):installer._upgrade_code(*args)
                    receipt=installed/'upgrade-idle-gis-installed-cohort-v1.json'
                    if corruption=='receipt_missing':receipt.unlink()
                    elif corruption=='receipt_changed':receipt.write_text('{}')
                    elif corruption=='stage_changed':(installed/'code-idle-gis-installed-cohort-v1-staged'/'binding').write_text('bad')
                    elif corruption=='backup_changed':(installed/'code-installed-78f5532c-gis-preserved'/'binding').write_text('bad')
                    elif corruption=='current_present':
                        (installed/'code').mkdir();(installed/'code'/'binding').write_text('bad')
                    if corruption=='none':
                        installer._upgrade_code(*args);installer._upgrade_code(*args)
                        self.assertEqual((installed/'code'/'binding').read_text(),'new')
                        self.assertEqual((installed/'code-installed-78f5532c-gis-preserved'/'binding').read_text(),'old')
                    else:
                        with patch.object(installer.os,'rename') as attempt:
                            with self.assertRaises((installer.Blocked,FileNotFoundError)):installer._upgrade_code(*args)
                            attempt.assert_not_called()


if __name__=='__main__':unittest.main()
