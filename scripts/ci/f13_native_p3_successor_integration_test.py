"""Real-Git successor qualification fixtures, explicitly not producer admission.

Run against an isolated clone with the candidate carrier committed above R.
Detached commit-tree objects are fixture events and never alter shared refs.
These fixtures explicitly exercise historical v2 qualification; public current
qualification must reject their restored historical context on an H9 checkout.
"""
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
import importlib.util

import f13_native_p3_authority as authority
import f13_native_p3_source as legacy
import f13_native_p3_successor as successor

ROOT = Path(__file__).resolve().parents[2]
BASE = '288e6a4fb963b3d5775f885e14ecfda1b603a4ff'
# Catalog documentation merged after the v3 documentation authority record.
# This pin is used only to construct the private historical test fixture.
CATALOG_DOCUMENT_COMMIT = 'eaa4c1a30921cea1f030ee53de89c71695cbfc87'
DATA_STORES_COMMIT = 'f3a0ac5dc372379b4d175655fb612b917a408948'
# The reviewed catalog/message-view documentation carried by the A1 aggregate.
# This remains a test-fixture source tuple, never production authority.
CATALOG_DATA_STORES_COMMIT = '04f1390e94c2285b20f2a208a55b1ee4b35facab'
# D1's independently reviewed terminal-routing documentation union (BATCH-D1
# c6/c7). Admit only this exact fixture source entry and its reviewed ancestry.
D1_DATA_STORES_COMMIT = '27dc90d3ac34aeda2dfe297932542d1aeacd5162'
D1_DATA_STORES_ENTRY = {'mode': '100644', 'type': 'blob',
                        'oid': 'f5c2326acba40feceffcb2dff5b3dd92f6d4581c'}
CATALOG_VOICE_PATHS = frozenset(
    'src/backend/voice/pb/voice/' + service + '/v1/catalog_assets' + suffix + '.pb.go'
    for service in ('chat', 'file') for suffix in ('', '_grpc'))


def git(*args, data=None, env=None):
    return subprocess.check_output(['git', '-C', str(ROOT), *args], input=data, env=env).decode().strip()


def reviewed_stores_source(source):
    stores = 'docs/DATA_STORES.md'
    actual = legacy.entry(ROOT, source, stores)
    for reviewed in (DATA_STORES_COMMIT, CATALOG_DATA_STORES_COMMIT):
        ancestor = subprocess.run(
            ['git', '-C', str(ROOT), 'merge-base', '--is-ancestor', reviewed, source],
            capture_output=True).returncode == 0
        if ancestor and actual == legacy.entry(ROOT, reviewed, stores):
            return True
    if actual == D1_DATA_STORES_ENTRY and legacy.entry(
            ROOT, D1_DATA_STORES_COMMIT, stores) == D1_DATA_STORES_ENTRY:
        return subprocess.run(
            ['git', '-C', str(ROOT), 'merge-base', '--is-ancestor',
             D1_DATA_STORES_COMMIT, source], capture_output=True).returncode == 0
    return False


def reviewed_catalog_voice_additions(source, record):
    prefix = 'src/backend/voice/'
    expected = {name for name in legacy.tree(str(ROOT), record['source']) if name.startswith(prefix)}
    actual = {name for name in legacy.tree(str(ROOT), source) if name.startswith(prefix)}
    added = actual - expected
    legacy.require(not expected - actual, 'v2_fixture_unreviewed_voice_source')
    if not added:
        return added
    ancestor = subprocess.run(
        ['git', '-C', str(ROOT), 'merge-base', '--is-ancestor', CATALOG_DATA_STORES_COMMIT, source],
        capture_output=True).returncode == 0
    legacy.require(added == CATALOG_VOICE_PATHS and ancestor and all(
        legacy.entry(ROOT, source, name) == legacy.entry(ROOT, CATALOG_DATA_STORES_COMMIT, name)
        for name in added), 'v2_fixture_unreviewed_voice_source')
    return added


class RealGitSuccessorTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.source = git('rev-parse', 'HEAD')
        cls.record = successor.load_record(ROOT)
        # This suite preserves the v2 unit's unchanged predicates after a
        # separately reviewed v3 documentation source exists. Restore only the
        # exact v2 document in a private Git tree; never alter the checkout or
        # treat the resulting fixture as current production source authority.
        document = 'docs/microservices/space-service.md'
        expected = next(row['after'] for row in cls.record['projection'] if row['path'] == document)
        if legacy.entry(ROOT, cls.source, document) != expected:
            legacy.require(authority.documentation is not None, 'v2_fixture_unknown_document_source')
            current = authority.documentation.load_record(ROOT)
            reviewed = next(row['after'] for row in current['projection'] if row['path'] == document)
            actual = legacy.entry(ROOT, cls.source, document)
            catalog = legacy.entry(ROOT, CATALOG_DOCUMENT_COMMIT, document)
            catalog_ancestor = subprocess.run(
                ['git', '-C', str(ROOT), 'merge-base', '--is-ancestor',
                 CATALOG_DOCUMENT_COMMIT, cls.source], capture_output=True).returncode == 0
            legacy.require(actual == reviewed or (catalog_ancestor and actual == catalog),
                           'v2_fixture_unreviewed_document_source')
            with tempfile.TemporaryDirectory() as directory:
                env = dict(os.environ, GIT_INDEX_FILE=str(Path(directory) / 'index'))
                git('read-tree', cls.source, env=env)
                git('update-index', '--add', '--cacheinfo',
                    expected['mode'] + ',' + expected['oid'] + ',' + document, env=env)
                cls.source = cls.commit(git('write-tree', env=env), [cls.source])
        stores = 'docs/DATA_STORES.md'
        stores_expected = next(row['after'] for row in cls.record['projection'] if row['path'] == stores)
        if legacy.entry(ROOT, cls.source, stores) != stores_expected:
            legacy.require(reviewed_stores_source(cls.source),
                           'v2_fixture_unreviewed_stores_source')
            with tempfile.TemporaryDirectory() as directory:
                env = dict(os.environ, GIT_INDEX_FILE=str(Path(directory) / 'index'))
                git('read-tree', cls.source, env=env)
                git('update-index', '--add', '--cacheinfo',
                    stores_expected['mode'] + ',' + stores_expected['oid'] + ',' + stores, env=env)
                cls.source = cls.commit(git('write-tree', env=env), [cls.source])
        additions = reviewed_catalog_voice_additions(cls.source, cls.record)
        if additions:
            with tempfile.TemporaryDirectory() as directory:
                env = dict(os.environ, GIT_INDEX_FILE=str(Path(directory) / 'index'))
                git('read-tree', cls.source, env=env)
                git('update-index', '--force-remove', '--', *sorted(additions), env=env)
                cls.source = cls.commit(git('write-tree', env=env), [cls.source])
        # Current generated dependencies can advance independently of v2. Pin
        # every historical context entry in this private fixture, including its
        # mode; never relax the verifier or use current bytes as v2 authority.
        with tempfile.TemporaryDirectory() as directory:
            env = dict(os.environ, GIT_INDEX_FILE=str(Path(directory) / 'index'))
            git('read-tree', cls.source, env=env)
            entries = ''.join(row['entry']['mode'] + ' ' + row['entry']['oid']
                              + '\t' + row['path'] + '\n' for row in cls.record['context'])
            git('update-index', '--index-info', data=entries.encode(), env=env)
            cls.source = cls.commit(git('write-tree', env=env), [cls.source])
        cls.context_path = next(row['path'] for row in cls.record['context']
                                if row['path'].startswith('src/backend/voice/internal/store/')
                                and row['path'].endswith('.go'))
        cls.target = cls.commit(git('rev-parse', cls.source + '^{tree}'), [BASE, cls.source])

    @staticmethod
    def commit(tree, parents):
        args = ['commit-tree', tree]
        for parent in parents:
            args += ['-p', parent]
        env=dict(os.environ,GIT_AUTHOR_NAME='F13 fixture',GIT_COMMITTER_NAME='F13 fixture',
                 GIT_AUTHOR_EMAIL='fixture@example.invalid',GIT_COMMITTER_EMAIL='fixture@example.invalid')
        return git(*args, data=b'Synthetic successor validation fixture; no admission\n',env=env)

    def qualify(self, source=None, target=None):
        source, target = source or self.source, target or self.target
        return successor.qualify(ROOT, BASE, source, target, 'pull_request',
                                 'refs/pull/875/merge', target, '', 'master')

    def test_public_current_authority_rejects_historical_fixture(self):
        with self.assertRaisesRegex(legacy.Rejected, '^context_bootstrap_actual_head$'):
            authority.qualify(ROOT, BASE, self.source, self.target, 'pull_request',
                              'refs/pull/875/merge', self.target, '', 'master')

    def changed(self, name, content=b'changed fixture input\n', mode='100644'):
        with tempfile.TemporaryDirectory() as directory:
            env = dict(os.environ, GIT_INDEX_FILE=str(Path(directory)/'index'))
            git('read-tree', self.source, env=env)
            blob = git('hash-object', '-w', '--stdin', data=content)
            git('update-index', '--add', '--cacheinfo', mode + ',' + blob + ',' + name, env=env)
            source = self.commit(git('write-tree', env=env), [self.source])
        return source, self.commit(git('rev-parse', source+'^{tree}'), [BASE, source])

    def test_exact_whole_unit_and_event(self):
        binding = self.qualify()
        self.assertEqual(binding['version'], successor.VERSION)
        self.assertEqual(len(binding['paths']), 99)
        verifier, record = authority.verified_record(ROOT, binding)
        self.assertIs(verifier, successor)
        self.assertEqual(record, self.record)

    def test_historical_fixture_has_complete_pinned_context(self):
        for row in self.record['context']:
            with self.subTest(path=row['path']):
                self.assertEqual(legacy.entry(ROOT, self.source, row['path']), row['entry'])

    def test_stores_fixture_restores_exact_historical_projection(self):
        stores = 'docs/DATA_STORES.md'
        expected = next(row['after'] for row in self.record['projection'] if row['path'] == stores)
        self.assertEqual(legacy.entry(ROOT, self.source, stores), expected)
        self.assertNotEqual(legacy.entry(ROOT, git('rev-parse', 'HEAD'), stores), expected)

    def test_stores_source_guard_accepts_only_exact_reviewed_entries(self):
        self.assertTrue(reviewed_stores_source(git('rev-parse', 'HEAD')))
        for reviewed in (DATA_STORES_COMMIT, CATALOG_DATA_STORES_COMMIT,
                         D1_DATA_STORES_COMMIT):
            with self.subTest(source=reviewed):
                self.assertTrue(reviewed_stores_source(reviewed))
        stores = 'docs/DATA_STORES.md'
        raw = (ROOT / stores).read_bytes()
        for content, mode in ((raw + b'\nunreviewed fixture change\n', '100644'),
                              (raw, '100755')):
            source, _ = self.changed(stores, content, mode)
            with self.subTest(mode=mode):
                self.assertFalse(reviewed_stores_source(source))

    def test_d1_stores_source_matches_reviewed_full_entry(self):
        self.assertEqual(legacy.entry(ROOT, D1_DATA_STORES_COMMIT,
                                      'docs/DATA_STORES.md'), D1_DATA_STORES_ENTRY)
        self.assertEqual(legacy.entry(ROOT, git('rev-parse', 'HEAD'),
                                      'docs/DATA_STORES.md'), D1_DATA_STORES_ENTRY)

    def test_stores_source_guard_rejects_matching_bytes_without_reviewed_ancestry(self):
        stores = 'docs/DATA_STORES.md'
        source, _ = self.changed(stores, (ROOT / stores).read_bytes())
        self.assertTrue(reviewed_stores_source(source))
        unrelated = self.commit(git('rev-parse', source + '^{tree}'), [BASE])
        self.assertEqual(legacy.entry(ROOT, unrelated, stores), legacy.entry(ROOT, source, stores))
        self.assertFalse(reviewed_stores_source(unrelated))

    def test_catalog_fixture_has_exact_historical_voice_inventory(self):
        prefix = 'src/backend/voice/'
        expected = {name for name in legacy.tree(str(ROOT), self.record['source']) if name.startswith(prefix)}
        actual = {name for name in legacy.tree(str(ROOT), self.source) if name.startswith(prefix)}
        self.assertEqual(actual, expected)
        self.assertEqual(reviewed_catalog_voice_additions(git('rev-parse', 'HEAD'), self.record),
                         CATALOG_VOICE_PATHS)
        unknown, _ = self.changed('src/backend/voice/unreviewed-fixture.go')
        with self.assertRaisesRegex(legacy.Rejected, 'v2_fixture_unreviewed_voice_source'):
            reviewed_catalog_voice_additions(unknown, self.record)

    def test_catalog_source_guard_rejects_content_mode_missing_paths_and_unrelated_ancestry(self):
        current = git('rev-parse', 'HEAD')
        name = sorted(CATALOG_VOICE_PATHS)[0]
        for variant in ('content', 'mode', 'missing_catalog', 'missing_historical'):
            with tempfile.TemporaryDirectory() as directory:
                env = dict(os.environ, GIT_INDEX_FILE=str(Path(directory) / 'index'))
                git('read-tree', current, env=env)
                if variant.startswith('missing_'):
                    removed = self.context_path if variant == 'missing_historical' else name
                    git('update-index', '--force-remove', '--', removed, env=env)
                else:
                    entry = legacy.entry(ROOT, current, name)
                    mode = '100755' if variant == 'mode' else entry['mode']
                    oid = entry['oid'] if variant == 'mode' else git(
                        'hash-object', '-w', '--stdin', data=b'unreviewed generated fixture\n')
                    git('update-index', '--cacheinfo', mode + ',' + oid + ',' + name, env=env)
                changed = self.commit(git('write-tree', env=env), [current])
            with self.subTest(variant=variant), self.assertRaisesRegex(
                    legacy.Rejected, 'v2_fixture_unreviewed_voice_source'):
                reviewed_catalog_voice_additions(changed, self.record)
        unrelated = self.commit(git('rev-parse', current + '^{tree}'), [BASE])
        with self.assertRaisesRegex(legacy.Rejected, 'v2_fixture_unreviewed_voice_source'):
            reviewed_catalog_voice_additions(unrelated, self.record)

    def test_projection_drift(self):
        with self.assertRaisesRegex(legacy.Rejected, 'projection_drift'):
            self.qualify(*self.changed('src/backend/voice/main.go'))

    def test_context_drift(self):
        with self.assertRaisesRegex(legacy.Rejected, 'context_drift'):
            self.qualify(*self.changed(self.context_path))

    def test_context_mode_drift(self):
        name=self.context_path
        with self.assertRaisesRegex(legacy.Rejected,'context_drift'):
            self.qualify(*self.changed(name,(ROOT/name).read_bytes(),'100755'))

    def test_dynamic_spec_outside_script_directory(self):
        code = '''import importlib.util,sys
spec=importlib.util.spec_from_file_location('extracted_native',sys.argv[1])
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
b=m.successor.qualify(*sys.argv[2:]);assert b['version']=='NATIVE-P3-SOURCE-v2'
'''
        with tempfile.TemporaryDirectory() as directory:
            subprocess.run([sys.executable,'-c',code,str(ROOT/'scripts/ci/f13_native_p3_authority.py'),
                            str(ROOT),BASE,self.source,self.target,'pull_request','refs/pull/875/merge',
                            self.target,'','master'],cwd=directory,check=True,capture_output=True)

    def bridge(self, historical_mutation=None):
        spec=importlib.util.spec_from_file_location('successor_f54',ROOT/'scripts/ci/f13_f54_history.py')
        module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)
        binding=self.qualify(); previous=legacy.load_record(ROOT)
        historical=dict(binding,version=previous['version'],snapshot=previous['source'],
                        record=legacy.PINNED_RECORD_COMMIT,record_sha256=legacy.PINNED_RECORD_SHA256,
                        paths=[r['path'] for r in previous['projection']])
        if historical_mutation: historical_mutation(historical)
        return module.current_author_J_native_applicability(ROOT,authority,historical,binding)

    def test_exact_predecessor_bridge(self):
        proof=self.bridge()
        self.assertEqual(proof['predecessor']['record'],legacy.PINNED_RECORD_COMMIT)
        self.assertEqual(proof['successor_lineage']['record'],legacy.PINNED_RECORD_COMMIT)
        self.assertFalse(proof['admission'])

    def test_historical_projection_tamper(self):
        with self.assertRaisesRegex(ValueError,'current_author_J_complete_historical_membership'):
            self.bridge(lambda b:b['paths'].pop())

    def test_historical_record_tamper(self):
        with self.assertRaisesRegex(ValueError,'current_author_J_historical_record_hash'):
            self.bridge(lambda b:b.update(record_sha256='0'*64))

    def test_new_voice_path(self):
        with self.assertRaisesRegex(legacy.Rejected, 'voice_tree_added_or_removed'):
            self.qualify(*self.changed('src/backend/voice/unreviewed.go'))

    def test_record_drift(self):
        with self.assertRaisesRegex(legacy.Rejected, 'successor_record_drift'):
            self.qualify(*self.changed(successor.RECORD_PATH))

    def test_evidence_drift(self):
        with self.assertRaisesRegex(legacy.Rejected, 'successor_evidence_drift'):
            self.qualify(*self.changed(self.record['evidence'][0]['name']))

    def test_old_verifier_immutable(self):
        with self.assertRaisesRegex(legacy.Rejected, 'successor_carrier_'):
            self.qualify(*self.changed(legacy.VERIFIER_PATH))

    def test_carrier_mode(self):
        with self.assertRaisesRegex(legacy.Rejected, 'successor_carrier_mode'):
            self.qualify(*self.changed(successor.VERIFIER_PATH,
                (ROOT/successor.VERIFIER_PATH).read_bytes(), '100755'))

    def test_record_cannot_be_its_own_carrier(self):
        source = successor.PINNED_RECORD_COMMIT
        target = self.commit(git('rev-parse',source+'^{tree}'),[BASE,source])
        with self.assertRaisesRegex(legacy.Rejected, 'missing_carrier'):
            self.qualify(source,target)

    def test_wrong_event_parent(self):
        target = self.commit(git('rev-parse',self.source+'^{tree}'),[self.source,BASE])
        with self.assertRaisesRegex(legacy.Rejected,'master_shape_or_before'):
            self.qualify(target=target)

    def test_result_drift(self):
        changed,_ = self.changed('outside-unit-fixture.txt')
        target = self.commit(git('rev-parse',changed+'^{tree}'),[BASE,self.source])
        with self.assertRaisesRegex(legacy.Rejected,'master_result'):
            self.qualify(target=target)

    def test_caller_version_cannot_select_authority(self):
        binding = self.qualify()
        binding['version'] = 'NATIVE-P3-SOURCE-v1'
        with self.assertRaisesRegex(legacy.Rejected,'native_unknown_authority_tuple'):
            authority.verified_record(ROOT,binding)

    def test_empty_map_cannot_claim_complete_authority(self):
        binding = self.qualify()
        with tempfile.TemporaryDirectory() as directory:
            map_path, history_path = Path(directory)/'map', Path(directory)/'history'
            map_path.write_bytes(b'');history_path.write_bytes(b'')
            with self.assertRaisesRegex(legacy.Rejected,'map_incomplete'):
                authority.audit_map(ROOT,binding,map_path,history_path)
            self.assertEqual(map_path.read_bytes(),b'')
            self.assertEqual(history_path.read_bytes(),b'')

    def test_prior_runtime_taint_cannot_be_cleared(self):
        binding=self.qualify(); name='docs/DATA_STORES.md'
        expected=legacy.entry(ROOT,self.record['source'],name)
        row=None
        for owner in git('log','--format=%H','--no-merges',self.record['source'],'--',name).splitlines():
            parents=git('rev-list','--parents','-n','1',owner).split()
            if len(parents)!=2 or legacy.entry(ROOT,owner,name)!=expected: continue
            parent=parents[1]
            delta=git('diff','--name-only',parent,owner).splitlines()
            if legacy.runtime_changed(delta): continue
            row='\t'.join([name,'false',owner,parent,parent,owner,parent,parent]); break
        self.assertIsNotNone(row,'fixture requires an actual nonruntime direct source edge')
        with tempfile.TemporaryDirectory() as directory:
            map_path,history_path=Path(directory)/'map',Path(directory)/'history'
            map_path.write_text(row+'\n')
            history_path.write_text(row.replace('\tfalse\t','\ttrue\t',1)+'\n'+row+'\n')
            with self.assertRaisesRegex(legacy.Rejected,'map_prior_runtime_taint'):
                authority.audit_map(ROOT,binding,map_path,history_path)


if __name__ == '__main__':
    unittest.main()
