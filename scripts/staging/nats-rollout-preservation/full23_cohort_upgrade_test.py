"""Old installed cohort must not silently adopt new private history."""
import contextlib
import hashlib
import os
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import installer
import operator_retirement
import story_witness


class FutureHistoryTests(unittest.TestCase):
    def test_new_history_paths_must_both_be_absent(self):
        with tempfile.TemporaryDirectory() as folder:
            installed=Path(folder)
            installer.require_future_history_absent(installed)
            for name in ('source-authority','realtime-bootstrap'):
                path=installed/name;path.mkdir()
                with self.assertRaisesRegex(installer.Blocked,'bridge_full23_future_history_present'):
                    installer.require_future_history_absent(installed)
                path.rmdir()
                path.symlink_to(installed/'nonexistent',target_is_directory=True)
                with self.assertRaisesRegex(installer.Blocked,'bridge_full23_future_history_present'):
                    installer.require_future_history_absent(installed)
                path.unlink()

    def test_actual_full23_idle_transition_vetoes_presence_before_replacement(self):
        for name in ('source-authority','realtime-bootstrap'):
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


if __name__=='__main__':unittest.main()
