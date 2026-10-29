"""Bridge preservation admission negatives with a real private root journal.

The TT card is synthetic; no Kubernetes, remote service or live host is used.
These tests establish a hold boundary, not a successful preservation or rollout.
"""
import copy
import os
import shutil
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

    def test_captured_helpers_check_protected_source_receipt(self):
        source = Path(__file__).resolve().parents[3]
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root = Path(directory)
            installed = root / 'installed'
            installed.mkdir(mode=0o700)
            code = installed / 'code'
            pins = (staging_prerequisites.BINARY_PINS if os.environ.get('VOICE_TEST_PINNED_ARCHIVE')
                    else root_composition_fixture.SYNTHETIC_PINS)
            with patch.object(staging_prerequisites, 'BINARY_PINS', pins):
                binding = root_composition_fixture.helper_binding(source, code)
            workspace = installed / 'sources' / ('a' * 12)
            workspace.mkdir(parents=True, mode=0o700)
            for relative in binding:
                if relative in pins:
                    continue
                dest = workspace / 'scripts' / 'staging' / relative
                dest.parent.mkdir(parents=True, exist_ok=True, mode=0o700)
                shutil.copyfile(source / 'scripts' / 'staging' / relative, dest)
                dest.chmod(0o600)
            script = r'''
import copy, json, sys
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch
import bridge_root, commands, guard, resource_claim, stage_runtime, staging_prerequisites as parent
import staging_cohort_preservation as producer, staging_gis_material as gis
root=Path(sys.argv[1]); code=root/'installed'/'code'; workspace=root/'installed'/'sources'/('a'*12)
binding=json.loads((code/'capture-manifest.json').read_bytes()); pins=json.loads(sys.argv[2])
c=dict(project='voice',resource='STAGING',actor='operator',claim='11111111-1111-4111-8111-111111111111',generation=7,
 window='22222222-2222-4222-8222-222222222222',revision=12,operation_id='stg-run-17',commit='c'*40,
 images={'voice-user/user':'registry/user@sha256:'+'c'*64})
card=dict(code='STAGING',environment='stg',state='held',phase='deploy',owner=c['actor'],claim=c['claim'],
 generation=c['generation'],window=c['window'],revision=c['revision'],operation=2,
 candidate=dict(state='known',commit=c['commit'],images=copy.deepcopy(c['images'])),
 observed=dict(state='known',claim=c['claim'],window=c['window'],operation=1,
 images={'voice-user/user':'registry/user@sha256:'+'b'*64}))
approved={'source_sha':c['commit'],'source_files':{'scripts/staging/'+k:v for k,v in binding.items() if k not in pins}}
execution={'head_sha':c['commit']}; session=resource_claim.Session(c,'a'*12,resource_claim.digest(binding),
 lambda:copy.deepcopy(card),resource_claim.RootJournal(root))
actions=object.__new__(bridge_root.Actions);actions.code=code;actions.binding=binding;actions.tt_session=session
with patch.object(guard,'ROOT',root),patch.object(bridge_root,'INSTALLED',root/'installed'), \
 patch.object(parent,'BINARY_PINS',pins),patch('grp.getgrnam',return_value=SimpleNamespace(gr_gid=0)):
 session.admit(start=True)
 with resource_claim.command_scope(session.check):
  bridge_root.record_source_authority(session,approved,execution)
  wrong=copy.deepcopy(approved); wrong['source_sha']='d'*40
  try: parent._capture_authority(actions,session,wrong,execution)
  except parent.Blocked as exc:
   assert str(exc)=='staging_prerequisites_source_changed', str(exc)
  else: raise AssertionError('counterfeit source receipt accepted')
  assert not (root/'installed'/'prerequisites').exists()
  # The external namespace is synthetic. The actual producer must perform
  # source custody first, then reject this deliberately incomplete source
  # inventory before it can bind or cut writers.
  with patch.object(stage_runtime.Kube,'get',return_value={'metadata':{'uid':'fixture-namespace'}}):
   try: producer.prepare(actions,{'mode':'app-only'},approved,execution,workspace)
   except gis.MaterialError as exc:
    assert str(exc)=='staging_gis_source_contract_invalid', str(exc)
   else: raise AssertionError('incomplete source admitted')
  assert not (root/'installed'/'cohort-preservation').exists()
  p=workspace/'scripts/staging/nats-rollout-preservation/staging_cohort_preservation.py'
  p.write_bytes(p.read_bytes()+b'\n# fixture drift\n')
  try: parent._verify_helper_source(session,approved,workspace)
  except parent.Blocked as exc:
   assert str(exc)=='staging_prerequisites_source_changed', str(exc)
  else: raise AssertionError('changed source accepted')
  assert session.check()['phase']=='prepare'
'''
            import json
            env = dict(os.environ, PYTHONPATH=os.pathsep.join((str(code / 'nats-rollout-preservation'),
                                                               str(code / 'nats-known-baseline'))))
            subprocess.run([sys.executable, '-c', script, str(root), json.dumps(pins)], cwd=directory,
                           env=env, check=True, capture_output=True, text=True)

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
