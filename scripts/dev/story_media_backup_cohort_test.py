import copy
import io
import tempfile
import unittest
import sys
from types import SimpleNamespace
from unittest.mock import Mock, patch
from pathlib import Path

import story_media_backup_cohort as cohort


class CohortTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name); self.key = bytes(range(32))
        self.databases = ['auth_db', 'voice_db']
        self.binding = {'operation_id': 'media-c1-' + 'a' * 32, 'snapshot_id': 'capture-' + 'b' * 32,
                        'source_sha': 'a' * 40, 'databases': self.databases,
                        'sources': {'auth_db': {'engine': {'fixture': 'same'}, 'server_start': 'fixture', 'database_oid': 1},
                                    'voice_db': {'engine': {'fixture': 'same'}, 'server_start': 'fixture', 'database_oid': 2},
                                    'roles': {'engine': {'fixture': 'same'}, 'server_start': 'fixture'}}}

    def test_exact_approved_members_are_streamed_and_restored(self):
        called = []
        def producer(member, output):
            called.append(member); output.write(('fixture-' + member).encode())
        manifest = cohort.export(self.root, self.key, self.binding, self.databases, producer)
        self.assertEqual(set(called), {'auth_db', 'voice_db', 'roles'})
        self.assertEqual(cohort.verify(self.root, self.key, self.binding, self.databases, manifest), set(called))
        for member in called:
            output = io.BytesIO()
            cohort.decrypt(self.root, self.key, self.binding, self.databases, manifest, member, output)
            self.assertEqual(output.getvalue(), ('fixture-' + member).encode())

    def test_staging_fixed_archive_flow_uses_private_single_attempt_destination(self):
        import guard
        binding=copy.deepcopy(self.binding);binding['operation_id']='stg-run-17'
        session=SimpleNamespace(operation='a'*12);members={'auth_db','voice_db','roles'}
        adapter=Mock();adapter.validate_backup_binding.return_value=members
        adapter.backup_custody.return_value=(self.root,self.key)
        adapter.produce_dump.side_effect=lambda _s,name,out:out.write(('fixture-'+name).encode())
        adapter.restore_attempt.return_value='c'*32
        private=self.root/'installed'/'cohort-preservation'/session.operation/'restore-input'
        private.mkdir(parents=True,mode=0o700)
        with patch.object(guard,'ROOT',self.root),patch.dict(sys.modules,{'staging_cohort_preservation':adapter}):
            manifest=cohort.export_staging(session,binding,self.databases,{}, {})
            self.assertEqual(cohort.verify_staging(session,binding,self.databases,manifest,{}, {}),members)
            adapter.load_backup_archive.return_value={'binding':binding,'databases':self.databases,'manifest':manifest,
                                                       'catalog_ref':{},'metadata_ref':{}}
            path=cohort.decrypt_staging(session,'voice_db')
            self.assertEqual(path.read_bytes(),b'fixture-voice_db')
            self.assertEqual(path.stat().st_mode & 0o777,0o600)
            with self.assertRaises(cohort.CohortError):cohort.decrypt_staging(session,'voice_db')
        with self.assertRaises(cohort.CohortError):
            cohort.verify(self.root,self.key,binding,self.databases,manifest)

    def test_second_cycle_is_distinct_and_other_cycles_are_rejected(self):
        binding = copy.deepcopy(self.binding); binding['operation_id'] = 'media-c2-' + 'a' * 32
        manifest = cohort.export(self.root, self.key, binding, self.databases,
                                 lambda member, output: output.write(b'fixture'))
        self.assertEqual(cohort.verify(self.root, self.key, binding, self.databases, manifest),
                         {'auth_db', 'voice_db', 'roles'})
        invalid = copy.deepcopy(binding); invalid['operation_id'] = 'media-c3-' + 'a' * 32
        with self.assertRaises(cohort.CohortError):
            cohort.verify(self.root, self.key, invalid, self.databases, manifest)

    def test_changed_or_incomplete_selection_and_ciphertext_fail_closed(self):
        manifest = cohort.export(self.root, self.key, self.binding, self.databases,
                                 lambda name, output: output.write(b'fixture'))
        for changed in (['auth_db'], ['auth_db', 'voice_db', 'gateway_db'], ['voice_db', 'auth_db']):
            with self.subTest(changed=changed), self.assertRaises(cohort.CohortError):
                cohort.verify(self.root, self.key, self.binding, changed, manifest)
        broken = copy.deepcopy(manifest); del broken['files']['roles']
        with self.assertRaises(cohort.CohortError):
            cohort.verify(self.root, self.key, self.binding, self.databases, broken)
        path = self.root / 'voice_db.enc'; raw = path.read_bytes(); path.write_bytes(raw[:-1])
        with self.assertRaises(cohort.CohortError):
            cohort.decrypt(self.root, self.key, self.binding, self.databases, manifest, 'voice_db', io.BytesIO())

    def test_multiple_or_unbound_engines_never_start_dump(self):
        for fault in ('different', 'missing', 'server_start', 'duplicate_oid', 'bad_operation', 'bad_snapshot', 'engine_type'):
            with self.subTest(fault=fault):
                self.setUp()
                binding = copy.deepcopy(self.binding); calls = []
                if fault == 'different': binding['sources']['voice_db']['engine'] = {'fixture': 'other'}
                if fault == 'missing': del binding['sources']['roles']
                if fault == 'server_start': binding['sources']['voice_db']['server_start'] = 'other'
                if fault == 'duplicate_oid': binding['sources']['voice_db']['database_oid'] = 1
                if fault == 'bad_operation': binding['operation_id'] = 'wrong'
                if fault == 'bad_snapshot': binding['snapshot_id'] = 'wrong'
                if fault == 'engine_type':
                    for source in binding['sources'].values(): source['engine'] = {'generation': 1}
                    binding['sources']['voice_db']['engine'] = {'generation': True}
                with self.assertRaises(cohort.CohortError):
                    cohort.export(self.root, self.key, binding, self.databases, lambda n, o: calls.append(n))
                self.assertEqual(calls, [])


if __name__ == '__main__': unittest.main()
