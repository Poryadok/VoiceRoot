"""Bridge preservation admission negatives with a real private root journal.

The TT card is synthetic; no Kubernetes, remote service or live host is used.
These tests establish a hold boundary, not a successful preservation or rollout.
"""
import copy
import os
import subprocess
import sys
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
import root_composition_fixture


@unittest.skipUnless(os.name == 'posix' and getattr(os, 'geteuid', lambda: -1)() == 0, 'root Linux fixture required')
class BridgePreservationAdmission(unittest.TestCase):
    def test_producer_imports_from_captured_helper_directory(self):
        source = Path(__file__).resolve().parents[3]
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            code = Path(directory) / 'code'
            # The optional pinned archive exercises production binary identities;
            # otherwise this test uses synthetic binary bytes only.
            pins = (staging_prerequisites.BINARY_PINS if os.environ.get('VOICE_TEST_PINNED_ARCHIVE')
                    else root_composition_fixture.SYNTHETIC_PINS)
            with patch.object(staging_prerequisites, 'BINARY_PINS', pins):
                binding = root_composition_fixture.helper_binding(source, code)
            modules = ('staging_cohort_preservation', 'staging_cohort_preservation_runtime',
                       'staging_cohort_preservation_pg', 'staging_cohort_preservation_auxiliary',
                       'staging_cohort_preservation_auth')
            for name in modules:
                self.assertIn('nats-rollout-preservation/' + name + '.py', binding)
            script = ('import importlib,json; names=' + repr(modules) + '; '
                      'print(json.dumps([importlib.import_module(n).__file__ for n in names]))')
            env = dict(os.environ, PYTHONPATH=os.pathsep.join((str(code / 'nats-rollout-preservation'),
                                                               str(code / 'nats-known-baseline'))))
            result = subprocess.run([sys.executable, '-c', script], cwd=directory, env=env,
                                    check=True, capture_output=True, text=True)
            import json
            self.assertEqual(json.loads(result.stdout),
                             [str(code / 'nats-rollout-preservation' / (name + '.py')) for name in modules])

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
