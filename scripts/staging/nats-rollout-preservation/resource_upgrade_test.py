"""Exact predecessor reconstruction and fail-closed upgrade custody tests."""
import copy
import hashlib
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import installer
import operator_retirement
from resource_claim_test import context


class UpgradeTests(unittest.TestCase):
    def binding(self):
        root=Path(__file__).resolve().parents[1]
        return {p:hashlib.sha256(p.encode() if p in installer.TT_FENCING_BINARY_MEMBERS else (root/p).read_bytes()).hexdigest()
            for p in installer.TT_FENCING_PREDECESSOR_MEMBERS|{installer.TT_FENCING_NEW_MEMBER}}

    def historical_binding(self):
        # The reviewed 9aa2 successor inventory is independent of this checkout;
        # production still validates it against the fixed f862 transition.
        return json.loads((Path(__file__).resolve().parent/'installed_cohort_successor_fixture.json').read_bytes())

    def test_closed_predecessor_preserves_all_unchanged_binary_pins(self):
        current=self.historical_binding();old=installer.tt_fencing_predecessor(current)
        self.assertEqual(len(old),85);self.assertNotIn(installer.TT_FENCING_NEW_MEMBER,old)
        self.assertEqual(set(old),installer.TT_FENCING_PREDECESSOR_MEMBERS)
        self.assertEqual(hashlib.sha256(json.dumps(old,sort_keys=True,separators=(',',':')).encode()).hexdigest(),
                         '0a81983170a46924ed21194e005087a999748a7e13e6762d6eefa76ed526fc21')
        for key in old:
            self.assertEqual(old[key],installer.TT_FENCING_OLD_HASHES.get(key,current[key]))
        for key in ('nats-known-baseline/kernel','nats-rollout-preservation/bootstrap-renewer','nats-rollout-preservation/story-current4-probe'):
            self.assertEqual(old[key],current[key])

    def test_missing_extra_swapped_unsafe_or_bad_hash_member_is_refused(self):
        valid=self.historical_binding()
        cases=[{k:v for k,v in valid.items() if k!='configure-kubectl-ci.sh'},
               dict(valid,foreign='a'*64),dict(valid,**{'../escape':'a'*64}),
               dict(valid,**{'nats-known-baseline/kernel':'not-a-hash'}),
               dict(valid,**{'nats-known-baseline/controller.py':'f'*64})]
        swapped=copy.deepcopy(valid);del swapped['configure-kubectl-ci.sh'];swapped['foreign']='a'*64;cases.append(swapped)
        for row in cases:
            with self.assertRaises(installer.Blocked):installer.tt_fencing_predecessor(row)

    def test_current_bot9_source_is_not_an_authorized_tt_successor(self):
        with self.assertRaisesRegex(installer.Blocked, 'bridge_tt_upgrade_predecessor_source_drift'):
            installer.tt_fencing_predecessor(self.binding())

    def test_new_custody_source_is_not_a_historical_successor(self):
        changed=self.historical_binding()
        changed['nats-rollout-preservation/github_custody.py']='f'*64
        with self.assertRaisesRegex(installer.Blocked,'bridge_tt_upgrade_predecessor_source_drift'):
            installer.tt_fencing_predecessor(changed)

    def test_upgrade_uses_only_fixed_source_transition_and_exact_full_predecessor(self):
        current=self.historical_binding();old=installer.tt_fencing_predecessor(current)
        expected=hashlib.sha256(json.dumps(old,sort_keys=True,separators=(',',':')).encode()).hexdigest()
        with patch.object(installer.root_cli,'code_binding',return_value=current),patch.object(installer,'_upgrade_idle_transition') as apply:
            installer.upgrade_idle_tt_resource_fencing_v1(Path('/reviewed-code'))
        args=apply.call_args.args
        self.assertEqual(args[1:4],(expected,'idle-tt-resource-fencing-v1','tt-predecessor-f8624086'))

    def test_resource_journal_is_preserved_and_unknown_or_active_state_rejects(self):
        with tempfile.TemporaryDirectory() as folder:
            installed=Path(folder);path=installed/'tt-resource-fence.json'
            self.assertEqual(installer.tt_resource_journal_snapshot(installed),{str(path):None})
            row=dict(schema='voice-staging-resource-fence-v1',context=context(),operation='a'*12,
                helper_sha256='b'*64,target_sha256='d'*64,status='complete',phase='finished',actual=context()['images'])
            def write(value):path.write_text(json.dumps(value));path.chmod(0o440)
            write(row);before=path.read_bytes()
            self.assertEqual(installer.tt_resource_journal_snapshot(installed),{str(path):hashlib.sha256(before).hexdigest()})
            self.assertEqual(path.read_bytes(),before)
            for changed in (dict(row,status='active'),dict(row,status='uncertain'),dict(row,phase='unknown'),dict(row,actual={}),dict(row,foreign=True)):
                write(changed)
                with self.assertRaises(installer.Blocked):installer.tt_resource_journal_snapshot(installed)
            import resource_claim
            scoped=dict(row,schema='voice-staging-resource-fence-v2',installation_sha256=resource_claim.production_installation().sha256,
                        cohort_preservation_sha256='e'*64)
            write(scoped);before=path.read_bytes()
            self.assertEqual(installer.tt_resource_journal_snapshot(installed),{str(path):hashlib.sha256(before).hexdigest()})
            for changed in (dict(scoped,installation_sha256='f'*64),
                            {k:v for k,v in scoped.items() if k!='installation_sha256'},
                            dict(scoped,schema='voice-staging-resource-fence-v1'),
                            dict(scoped,cohort_preservation_sha256='invalid'),dict(scoped,actual={}),
                            dict(scoped,phase='unknown'),dict(scoped,foreign=True)):
                write(changed)
                with self.assertRaises(installer.Blocked):installer.tt_resource_journal_snapshot(installed)

    def test_inventory_rejects_unlisted_source_hardlinks_and_symlinks(self):
        with tempfile.TemporaryDirectory() as folder:
            code=Path(folder);source=code/'safe.py';source.write_text('SAFE=True');source.chmod(0o440)
            manifest={'safe.py':hashlib.sha256(source.read_bytes()).hexdigest()}
            (code/'capture-manifest.json').write_text(json.dumps(manifest));(code/'capture-manifest.json').chmod(0o440)
            installer.tt_code_inventory(code,manifest)
            extra=code/'foreign.py';extra.write_text('UNLISTED=True');extra.chmod(0o440)
            with self.assertRaises(installer.Blocked):installer.tt_code_inventory(code,manifest)
            extra.unlink();extra.symlink_to(source)
            with self.assertRaises(installer.Blocked):installer.tt_code_inventory(code,manifest)
            extra.unlink();extra.hardlink_to(source)
            with self.assertRaises(installer.Blocked):installer.tt_code_inventory(code,manifest)

    def test_real_idle_transition_rejects_active_resource_before_code_replacement(self):
        with tempfile.TemporaryDirectory() as folder:
            root=Path(folder);installed=root/'installed';installed.mkdir(mode=0o750)
            for name,mode in (('inbox',0o1730),('processing',0o700),('journal',0o700),('responses',0o750),('recovery',0o700)):
                (installed/name).mkdir(mode=mode);(installed/name).chmod(mode)
            base=root/('rollout-'+'a'*12);base.mkdir();(base/'checkpoint.json').write_text('{}')
            path=installed/'tt-resource-fence.json';path.write_text(json.dumps(dict(status='active')));path.chmod(0o440)
            before=path.read_bytes()
            with patch.object(installer.guard,'ROOT',root),patch.object(installer,'operation_lock',__import__('contextlib').nullcontext),\
                patch.object(installer.root_cli,'code_binding',return_value={}),patch.object(installer.root_cli,'rollout_directory_kind',return_value='operation'),\
                patch.object(installer,'checkpoint_json',return_value={'operator_retirement':{}}),\
                patch.object(operator_retirement,'idle_state'),patch.object(operator_retirement,'verify_terminal',return_value={'verified':True}),\
                patch.object(operator_retirement,'private_bytes',return_value=b'fixture'),patch.object(installer,'Kube') as kube,\
                patch.object(installer,'_upgrade_code') as replace:
                kube.return_value.get.return_value={'data':{'phase':'active'}}
                with self.assertRaisesRegex(installer.Blocked,'resource_reconciliation_required'):
                    installer._upgrade_idle_transition(root/'new','b'*64,'idle-tt-resource-fencing-v1','tt-predecessor-f8624086','unused')
                replace.assert_not_called()
            self.assertEqual(path.read_bytes(),before);self.assertEqual((installed/'inbox').stat().st_mode&0o7777,0o1730)

    def test_duplicate_manifest_member_is_not_silently_collapsed(self):
        with tempfile.TemporaryDirectory() as folder:
            code=Path(folder);source=code/'safe.py';source.write_text('SAFE=True');source.chmod(0o440)
            value=hashlib.sha256(source.read_bytes()).hexdigest()
            (code/'capture-manifest.json').write_text('{"safe.py":"'+value+'","safe.py":"'+value+'"}')
            (code/'capture-manifest.json').chmod(0o440)
            with self.assertRaises(installer.Blocked):installer.root_cli.code_binding(code)


if __name__=='__main__':unittest.main()
