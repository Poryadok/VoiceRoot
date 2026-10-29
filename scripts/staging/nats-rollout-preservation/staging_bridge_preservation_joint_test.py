"""Bridge preservation admission negatives with a real private root journal.

The TT card is synthetic; no Kubernetes, remote service or live host is used.
These tests establish a hold boundary, not a successful preservation or rollout.
"""
import copy
import os
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import bridge_root
import commands
import guard
import migrations
import resource_claim
from resource_claim_test import card, context
import staging_prerequisites


@unittest.skipUnless(os.name == 'posix' and getattr(os, 'geteuid', lambda: -1)() == 0, 'root Linux fixture required')
class BridgePreservationAdmission(unittest.TestCase):
    def test_missing_preservation_holds_before_parent_or_cluster(self):
        for mode in ('full', 'app-only'):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory(dir='/root') as directory:
                root = Path(directory)
                (root / 'installed').mkdir(mode=0o700)
                candidate = context()
                tt_card = card(candidate)
                journal = resource_claim.RootJournal(root)
                session = resource_claim.Session(candidate, 'a' * 12, 'b' * 64,
                    lambda: copy.deepcopy(tt_card), journal)
                actions = object.__new__(bridge_root.Actions)
                actions.tt_session = session
                with patch.object(guard, 'ROOT', root), \
                     patch('grp.getgrnam', return_value=SimpleNamespace(gr_gid=0)):
                    session.admit(start=True)
                    with resource_claim.command_scope(session.check), \
                         patch.object(commands, 'capture', side_effect=AssertionError('unexpected cluster command')), \
                         patch.object(staging_prerequisites, 'run_prepare', wraps=staging_prerequisites.run_prepare) as parent:
                        with self.assertRaisesRegex(migrations.Blocked, 'staging_migration_preservation_authority_missing'):
                            actions._prepare({'mode': mode})
                        parent.assert_not_called()
                    self.assertEqual(session.check()['phase'], 'prepare')
                    self.assertFalse((root / 'installed' / 'cohort-preservation').exists())
                    self.assertFalse((root / 'installed' / 'prerequisites').exists())


if __name__ == '__main__':
    unittest.main()
