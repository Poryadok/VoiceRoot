"""Real fixture materialization regression; never a F13 guard/producer RED."""
import hashlib
import contextlib
import io
import json
import os
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest import mock
import zipfile

import f13_exact_j_historical_red_test as harness


class CoarseStatusContract(unittest.TestCase):
    """Optional observation status, never guard/admission evidence."""
    def fixture(self, public=False):
        actual = {'publictool': '/usr/bin/publictool'} if public else {'PRIVATE_SECRET': '/private/PRIVATE_SECRET'}
        identity = {'commit': 'a' * 40, 'files': {'driver': 'b' * 64}}
        context = harness.public_diagnostic_context(('/usr/bin/publictool',), actual, identity)
        observation = harness.opaque_utility_observation(Path('/absent'), actual, {}, 1, tuple(actual), identity)
        receipt = harness.resolve_owned_utility_output(b'', harness.encode_opaque_observation(observation), context)[2]
        self.assertIsNotNone(receipt)
        mismatch = harness.diagnostic_json(harness.utility_view_diagnostic(Path('/absent'), actual, {}, 1, tuple(actual))) + b'\n'
        return mismatch, context, receipt

    def test_context_unavailable(self):
        mismatch, _, _ = self.fixture()
        self.assertEqual(harness.coarse_public_diagnostic_status(b'', mismatch, None, None), 'context_unavailable')

    def test_candidate_rejected_or_not_selected(self):
        mismatch, context, _ = self.fixture()
        self.assertEqual(harness.coarse_public_diagnostic_status(mismatch, b'', context, None), 'candidate_rejected_or_not_selected')

    def test_no_catalog_match(self):
        mismatch, context, receipt = self.fixture()
        self.assertEqual(harness.coarse_public_diagnostic_status(b'', mismatch, context, receipt), 'catalog_match_failed')

    def test_qualified_match_stays_quiet(self):
        mismatch, context, receipt = self.fixture(public=True)
        self.assertIsNone(harness.coarse_public_diagnostic_status(b'', mismatch, context, receipt))

    def test_no_original_mismatch_stays_quiet(self):
        for raw in (b'', b'F13_HISTORICAL_SETUP_FAILURE\n', b'{"kind":"F13_UTILITY_VIEW_DIAGNOSTIC_v1","status":"diagnostic_unavailable"}\n'):
            self.assertIsNone(harness.coarse_public_diagnostic_status(raw, b'', None, None))

    def test_malformed_and_overbudget_stay_quiet(self):
        for raw in (b'{PRIVATE_SECRET\n', b'\xff\n', b'x' * 16385,
                    json.dumps({'kind': 'PRIVATE_SECRET', 'status': 'mismatch'}).encode()):
            self.assertIsNone(harness.coarse_public_diagnostic_status(b'', raw, None, None))

    def test_qualified_closure_match_and_malformed_receipt(self):
        mismatch, context, receipt = self.fixture()
        for invalid in ({}, {'kind': receipt['kind'], 'entries': 'PRIVATE_SECRET'}):
            self.assertIsNone(harness.coarse_public_diagnostic_status(b'', mismatch, context, invalid))
        token = receipt['entries'][0]['actual']['target']
        receipt['entries'][0]['actual']['closure']['edges'] = [dict(token, path='/usr/bin/publictool',
            qualification='OUTER_CATALOG_RESOLUTION_OF_CHILD_TOKEN')]
        self.assertIsNone(harness.coarse_public_diagnostic_status(b'', mismatch, context, receipt))

    def test_fixed_emission_only_and_output_failure_contained(self):
        import types
        output = io.BytesIO()
        with mock.patch.object(harness.sys, 'stderr', types.SimpleNamespace(buffer=output)):
            for value in ('context_unavailable', 'candidate_rejected_or_not_selected', 'catalog_match_failed',
                          None, 'PRIVATE_SECRET', {'path': '/PRIVATE_SECRET'}):
                harness.emit_coarse_public_diagnostic_status(value)
        records = [json.loads(line) for line in output.getvalue().splitlines()]
        self.assertEqual([item['status'] for item in records],
                         ['context_unavailable', 'candidate_rejected_or_not_selected', 'catalog_match_failed'])
        self.assertTrue(all(set(item) == {'kind', 'status'} for item in records))
        self.assertNotIn(b'PRIVATE_SECRET', output.getvalue())
        broken = mock.Mock()
        broken.write.side_effect = OSError('PRIVATE_SECRET')
        with mock.patch.object(harness.sys, 'stderr', types.SimpleNamespace(buffer=broken)):
            harness.emit_coarse_public_diagnostic_status('context_unavailable')

    def test_namespace_setup_failure_preserved_with_coarse_context_status(self):
        import types
        mismatch, _, _ = self.fixture()
        original_out = b'original stdout\n'
        original_err = mismatch + b'F13_HISTORICAL_SETUP_FAILURE: pinned_Git_private_utility_view\n'
        outputs = (io.BytesIO(), io.BytesIO())
        def command(args):
            if args[-2:] == ['rev-parse', 'HEAD']:
                return b'a' * 40 + b'\n'
            return (harness.ROOT / args[-1].split(':', 1)[1]).read_bytes()
        def materialize(root):
            (root / 'historical-git').mkdir()
        with mock.patch.object(harness.os, 'geteuid', return_value=1000), \
             mock.patch.object(harness.shutil, 'which', return_value='/trusted/tool'), \
             mock.patch.object(harness, 'run', side_effect=command), \
             mock.patch.object(harness, 'materialize', side_effect=materialize), \
             mock.patch.object(harness, 'prepare_public_diagnostic_context', return_value=None), \
             mock.patch.object(harness.subprocess, 'run', return_value=subprocess.CompletedProcess([], 2, original_out, original_err)) as spawn, \
             mock.patch.object(harness.sys, 'stdout', types.SimpleNamespace(buffer=outputs[0])), \
             mock.patch.object(harness.sys, 'stderr', types.SimpleNamespace(buffer=outputs[1])):
            with self.assertRaisesRegex(harness.SetupFailure, '^namespace_must_return_actual_guard_receipt_not_setup_exit$'):
                harness.namespace_main()
        self.assertEqual(spawn.call_count, 1)
        self.assertEqual(spawn.call_args.kwargs, {'capture_output': True}, 'stdin/env/invocation unchanged')
        self.assertEqual(outputs[0].getvalue(), original_out)
        raw = outputs[1].getvalue()
        self.assertTrue(raw.startswith(original_err))
        self.assertEqual(json.loads(raw[len(original_err):]),
                         dict(kind='F13_PUBLIC_UTILITY_DIAGNOSTIC_STATUS_v1', status='context_unavailable'))


class OuterPublicResolution(unittest.TestCase):
    """Synthetic diagnostic/privacy proof, never actual F13 guard RED."""
    def setUp(self):
        self.identity = {'commit': 'a' * 40, 'files': {'driver': 'b' * 64}}
        self.actual = {'publictool': '/usr/bin/publictool', 'private': '/private/SECRET'}
        self.catalog = ('/usr/bin/publictool',)
        self.context = harness.public_diagnostic_context(self.catalog, self.actual, self.identity)

    def observation(self):
        return harness.opaque_utility_observation(Path('/nonexistent'), self.actual, {}, 2,
                                                 tuple(self.actual), self.identity)

    def test_public_basename_never_qualifies_private_target(self):
        self.actual = {'publictool': '/private/SECRET'}
        context = harness.public_diagnostic_context(self.catalog, self.actual, self.identity)
        observation = self.observation()
        raw = harness.encode_opaque_observation(observation)
        out, err, receipt = harness.resolve_owned_utility_output(b'original\n', raw, context)
        self.assertEqual((out, err), (b'original\n', b''))
        self.assertIsNotNone(receipt)
        self.assertEqual(receipt['entries'][0]['tool']['basename'], 'publictool')
        self.assertNotIn('path', receipt['entries'][0]['actual']['target'])
        self.assertNotIn('SECRET', json.dumps(receipt))

    def test_cached_map_source_and_union_duplicate_reject(self):
        raw = harness.encode_opaque_observation(self.observation())
        for stdout, stderr in ((raw, raw), (b'', raw + raw)):
            out, err, receipt = harness.resolve_owned_utility_output(stdout, stderr, self.context)
            self.assertEqual((out, err, receipt), (b'', b'', None))
        for field in ('actual_sha256', 'source_sha256'):
            candidate = self.observation()
            candidate[field] = '0' * 64
            _, _, receipt = harness.resolve_owned_utility_output(b'', harness.encode_opaque_observation(candidate), self.context)
            self.assertIsNone(receipt)

    def test_both_stream_malformed_prefix_suppression_preserves_unrelated_bytes(self):
        raw = b'F13_OPAQUE_UTILITY_BROKEN SECRET\r\n'
        out, err, receipt = harness.resolve_owned_utility_output(b'guard\r\n' + raw, b'err\n' + raw[:-2], self.context)
        self.assertEqual((out, err, receipt), (b'guard\r\n', b'err\n', None))

    def test_opaque_child_never_emits_new_raw_names_or_targets(self):
        raw = harness.encode_opaque_observation(self.observation())
        self.assertNotIn(b'publictool', raw)
        self.assertNotIn(b'SECRET', raw)
        self.assertLessEqual(len(raw), 16384)

    def test_namespace_owner_integrates_resolution_without_stdin_or_exec_growth(self):
        import types
        outputs = (io.BytesIO(), io.BytesIO())
        calls = []
        def command(args):
            if args[-2:] == ['rev-parse', 'HEAD']:
                return b'a' * 40 + b'\n'
            return (harness.ROOT / args[-1].split(':', 1)[1]).read_bytes()
        def materialize(root):
            (root / 'historical-git/util').mkdir(parents=True)
            (root / 'historical-git/util/publictool').symlink_to('/usr/bin/publictool')
        def spawn(args, **kwargs):
            calls.append((args, kwargs))
            self.assertEqual(set(kwargs), {'capture_output'})
            self.assertTrue(kwargs['capture_output'])
            identity = dict(commit='a' * 40, files={name: harness.sha((harness.ROOT / name).read_bytes())
                                                  for name in harness.HARNESS_SOURCE_PATHS})
            observation = harness.opaque_utility_observation(Path('/absent'), {'publictool': '/usr/bin/publictool'}, {}, 1,
                                                            ('publictool',), identity)
            guard = b'{"kind":"FAITHFUL_HISTORICAL_SHARED_GUARD","guard_observed":1,"contract_expected":0}\n'
            return subprocess.CompletedProcess(args, 1, guard, harness.encode_opaque_observation(observation))
        with mock.patch.object(harness.os, 'geteuid', return_value=1000), \
             mock.patch.object(harness.shutil, 'which', return_value='/trusted/tool'), \
             mock.patch.object(harness, 'run', side_effect=command), \
             mock.patch.object(harness, 'materialize', side_effect=materialize), \
             mock.patch.object(harness, 'read_public_utility_catalog', return_value=self.catalog), \
             mock.patch.object(harness.subprocess, 'run', side_effect=spawn), \
             mock.patch.object(harness.sys, 'stdout', types.SimpleNamespace(buffer=outputs[0])), \
             mock.patch.object(harness.sys, 'stderr', types.SimpleNamespace(buffer=outputs[1])):
            self.assertEqual(harness.namespace_main(), 1)
        self.assertEqual(len(calls), 1)
        self.assertNotIn('F13_OPAQUE', calls[0][0][-1])
        self.assertNotIn('F13_PUBLIC', calls[0][0][-1])
        self.assertIn(b'F13_PUBLIC_UTILITY_OUTER_RESOLUTION_RECEIPT_v1', outputs[1].getvalue())
        self.assertNotIn(b'F13_OPAQUE_UTILITY_', outputs[1].getvalue())

    def test_catalog_parser_canonical_lexical_and_bounds(self):
        self.assertEqual(harness.public_catalog_from_lists([b'/usr/bin/z\n/bin/a\n/usr/bin/../private\n/usr/local/bin/secret\n']),
                         ('/bin/a', '/usr/bin/z'))
        for raw in (b'/usr/bin/x', b'/usr/bin/x\0\n', b'x' * 1048577):
            with self.assertRaises((ValueError, UnicodeError)):
                harness.public_catalog_from_lists([raw])
        with self.assertRaises(ValueError):
            harness.public_catalog_from_lists([b''] * 4097)

    def test_schema_cleartext_duplicate_keys_types_and_relative_edges_reject(self):
        value = self.observation()
        for mutate in (lambda x: x.update(entry_count=True), lambda x: x.update(private='/SECRET'),
                       lambda x: x['entries'][0]['tool'].update(basename='SECRET')):
            candidate = json.loads(json.dumps(value))
            mutate(candidate)
            self.assertIsNone(harness.resolve_owned_utility_output(b'', harness.encode_opaque_observation(candidate), self.context)[2])
        raw = harness.encode_opaque_observation(value).replace(b'{', b'{"actual_count":2,', 1)
        self.assertIsNone(harness.resolve_owned_utility_output(b'', raw, self.context)[2])

    def test_real_installed_catalog_or_closed_unavailability_and_acl_errors(self):
        catalog = harness.read_public_utility_catalog()
        if catalog is not None:
            self.assertIn('/usr/bin/dpkg', catalog)
            self.assertEqual(catalog, tuple(sorted(set(catalog))))
        with mock.patch.object(harness.os, 'getxattr', return_value=b'extended_acl'):
            self.assertIsNone(harness.read_public_utility_catalog())
        with mock.patch.object(harness.os, 'getxattr', side_effect=OSError(13, 'SECRET')):
            self.assertIsNone(harness.read_public_utility_catalog())

    def test_each_closure_edge_relative_unknown_and_surrogate_tokens(self):
        with tempfile.TemporaryDirectory() as folder:
            private = Path(folder) / 'PRIVATE_SECRET'
            private.write_bytes(b'contents not read')
            alias = Path(folder) / 'alias'
            alias.symlink_to(private)
            relative = Path(folder) / 'relative'
            relative.symlink_to('PRIVATE_SECRET')
            self.assertEqual(harness.opaque_utility_closure(str(alias))['edges'], [harness.opaque_utility_token(str(private))])
            self.assertEqual(harness.opaque_utility_closure(str(relative))['edges'], [harness.opaque_utility_token('PRIVATE_SECRET')])
            self.assertNotIn('PRIVATE_SECRET', json.dumps(harness.opaque_utility_closure(str(relative))))
        candidate = self.observation()
        candidate['entries'][0]['actual']['closure'] = dict(edges=[harness.opaque_utility_token('publictool'),
            harness.opaque_utility_token('/usr/bin/publictool')], status='complete')
        receipt = harness.resolve_owned_utility_output(b'', harness.encode_opaque_observation(candidate), self.context)[2]
        edges = receipt['entries'][0]['actual']['closure']['edges']
        self.assertNotIn('path', edges[0], 'basename must never qualify a relative edge')
        self.assertEqual(edges[1]['path'], '/usr/bin/publictool')
        token = harness.opaque_utility_token('\udcff')
        self.assertEqual(token['utf8_bytes'], 3)
        self.assertEqual(token, harness.utility_diagnostic_token('\udcff'))

    def test_catalog_canonical_max_and_plus_one_and_snapshot_bounds(self):
        # 8192 direct-child public paths with <=131072 canonical bytes.
        raw = b''.join(('/bin/a%04x\n' % i).encode() for i in range(8192))
        self.assertEqual(len(harness.public_catalog_from_lists([raw])), 8192)
        with self.assertRaises(ValueError):
            harness.public_catalog_from_lists([raw + b'/bin/new\n'])
        self.assertEqual(harness.public_catalog_from_lists([b'/usr/bin/publictool\r\n']), ())
        with self.assertRaises(ValueError):
            harness.public_diagnostic_context(self.catalog, {str(i): 'x' for i in range(8193)}, self.identity)
        with self.assertRaises(ValueError):
            harness.public_diagnostic_context(self.catalog, {'x': 'x' * 2097153}, self.identity)
        exact = b''.join(('/bin/a%09x\n' % i).encode() for i in range(8192))
        self.assertEqual(len(exact), 131072)
        self.assertEqual(len(harness.public_catalog_from_lists([exact])), 8192)
        with self.assertRaises(ValueError):
            harness.public_catalog_from_lists([exact.replace(b'/bin/a000000000\n', b'/bin/aa000000000\n')])
        file_max = b'x' * 1048575 + b'\n'
        self.assertEqual(harness.public_catalog_from_lists([file_max] * 32), ())
        with self.assertRaises(ValueError):
            harness.public_catalog_from_lists([file_max] * 32 + [b'\n'])

    def test_candidate_and_expanded_receipt_token_budgets_and_loop(self):
        with tempfile.TemporaryDirectory() as folder:
            links = [Path(folder) / str(i) for i in range(10)]
            for index in range(9):
                links[index].symlink_to(links[index + 1])
            self.assertEqual(harness.opaque_utility_closure(str(links[0]))['status'], 'limit')
            links[9].symlink_to(links[8])
            self.assertEqual(harness.opaque_utility_closure(str(links[8]))['status'], 'loop')
        actual = {'unknown' + str(i): '/private/' + str(i) for i in range(40)}
        candidate = harness.opaque_utility_observation(Path('/absent'), actual, {}, 40, tuple(actual), self.identity)
        self.assertEqual(len(candidate['entries']) + candidate['omitted'], 40)
        self.assertGreater(candidate['omitted'], 0)
        raw = harness.encode_opaque_observation(candidate)
        context = harness.public_diagnostic_context(self.catalog, actual, self.identity)
        receipt = harness.resolve_owned_utility_output(b'', raw, context)[2]
        self.assertIsNotNone(receipt)
        self.assertLessEqual(len(harness.diagnostic_json(receipt)) + 1, 16384)
        # Structural metadata now also consumes global32; force the independent
        # byte bound regardless of the smaller admitted original row count.
        candidate['entries'] *= 100
        with self.assertRaises(ValueError):
            harness.encode_opaque_observation(candidate)

    def test_catalog_unavailability_context_failure_and_token_ambiguity_omit(self):
        with mock.patch.object(harness, 'read_public_utility_catalog', return_value=None):
            self.assertIsNone(harness.prepare_public_diagnostic_context(Path('/absent'), self.identity))
        value = self.observation()
        key = harness.opaque_utility_token('/usr/bin/publictool')
        self.context['paths'][(key['sha256'], key['utf8_bytes'])] = {'/usr/bin/publictool', '/bin/another'}
        receipt = harness.resolve_owned_utility_output(b'', harness.encode_opaque_observation(value), self.context)[2]
        self.assertNotIn('path', receipt['entries'][1]['actual']['target'] if receipt['entries'][1]['tool'].get('basename') == 'publictool'
                         else receipt['entries'][0]['actual']['target'])

    def test_expanded_receipt_recounts_bytes_and_extra_tokens_reject(self):
        paths = tuple('/usr/bin/a' + str(i).zfill(3) + 'x' * 80 for i in range(60))
        actual = {path.rsplit('/', 1)[1]: path for path in paths}
        context = harness.public_diagnostic_context(paths, actual, self.identity)
        observation = harness.opaque_utility_observation(Path('/absent'), actual, {}, len(actual), tuple(actual), self.identity)
        raw = harness.encode_opaque_observation(observation)
        receipt = harness.resolve_owned_utility_output(b'', raw, context)[2]
        self.assertIsNotNone(receipt)
        self.assertLessEqual(len(harness.diagnostic_json(receipt)) + 1, 16384)
        self.assertEqual(len(receipt['entries']) + receipt['omitted'], 60)
        many = self.observation()
        many['entries'] = [many['entries'][0]] * 17  # 2 tokens per entry exceeds global32.
        self.assertIsNone(harness.resolve_owned_utility_output(b'', harness.encode_opaque_observation(many), self.context)[2])

    def test_original_mismatch_seam_emits_opaque_cached_short_circuit_once(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            (base / 'util').chmod(0o755)
            (base / 'util/publictool').write_bytes(b'content must not be read')
            (base / 'util').chmod(0o555)
            actual = {p.name: str(p.readlink()) for p in (base / 'util').iterdir() if p.is_symlink()}
            context = harness.public_diagnostic_context(self.catalog, actual, self.identity)
            output = io.StringIO()
            with mock.patch.dict(os.environ, {'F13_HARNESS_IDENTITY': json.dumps(self.identity)}), \
                 mock.patch.object(harness, 'expected_utilities', side_effect=RuntimeError('must not evaluate')) as expected, \
                 contextlib.redirect_stderr(output):
                with self.assertRaisesRegex(harness.SetupFailure, '^pinned_Git_private_utility_view$'):
                    harness.verify_pinned_git(root)
                self.assertEqual(expected.call_count, 0)
            _, stderr, receipt = harness.resolve_owned_utility_output(b'', output.getvalue().encode(), context)
            self.assertIsNotNone(receipt)
            self.assertEqual(receipt['expected_status'], 'not_evaluated_short_circuit')
            self.assertEqual(receipt['entries'][0]['tool']['basename'], 'publictool')
            self.assertNotIn('actual', receipt['entries'][0], 'regular slot is not part of the cached symlink map')
            self.assertNotIn(b'F13_OPAQUE_UTILITY_', stderr)
            self.assertIn(b'F13_UTILITY_VIEW_DIAGNOSTIC_v1', stderr)

    def test_catalog_handles_closed_before_existing_spawn_even_on_acl_error(self):
        before = set(os.listdir('/proc/self/fd'))
        harness.read_public_utility_catalog()
        self.assertEqual(set(os.listdir('/proc/self/fd')), before)
        with mock.patch.object(harness.os, 'getxattr', side_effect=OSError(95, 'unknown_acl_semantics')):
            self.assertIsNone(harness.read_public_utility_catalog())
        self.assertEqual(set(os.listdir('/proc/self/fd')), before)


class HistoricalJTransport(unittest.TestCase):
    def test_utility_mismatch_emits_redacted_diagnostic_without_changing_failure(self):
        # Synthetic output/invariance regression, NOT the actual hosted cause.
        with tempfile.TemporaryDirectory(prefix='f13-diag-') as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            link = base / 'util/sha256sum'
            (base / 'util').chmod(0o755)
            link.unlink()
            secret = '/home/private/HOST_SECRET_SENTINEL'
            link.symlink_to(secret)
            (base / 'util').chmod(0o555)
            before = {p.name: str(p.readlink()) for p in (base / 'util').iterdir()}
            output = io.StringIO()
            with contextlib.redirect_stderr(output):
                with self.assertRaisesRegex(harness.SetupFailure, '^pinned_Git_private_utility_view$'):
                    harness.verify_pinned_git(root)
            receipts = [json.loads(line) for line in output.getvalue().splitlines()]
            self.assertEqual(len(receipts), 1, 'original mismatch must emit a bounded diagnostic')
            self.assertEqual(receipts[0]['kind'], 'F13_UTILITY_VIEW_DIAGNOSTIC_v1')
            self.assertNotIn(secret, output.getvalue())
            self.assertEqual(before, {p.name: str(p.readlink()) for p in (base / 'util').iterdir()})

    def test_utility_success_is_silent_and_empty_metadata_is_bounded(self):
        with tempfile.TemporaryDirectory(prefix='f13-success-') as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            output = io.StringIO()
            with mock.patch.object(harness, 'expected_utilities', wraps=harness.expected_utilities) as expected:
                with contextlib.redirect_stderr(output):
                    harness.verify_pinned_git(root)
                self.assertEqual(expected.call_count, 1)
            self.assertEqual(output.getvalue(), '')
            empty = harness.utility_view_diagnostic(base, {}, {}, 0)
            self.assertEqual(empty['counts'], dict(expected=0, actual=0, entries=0, non_symlink=0,
                                                  missing=0, extra=0, changed=0))
            self.assertEqual(empty['entries'], [])
            self.assertFalse(empty['truncated'])

    def test_non_symlink_short_circuit_and_diagnostic_errors_preserve_failure(self):
        with tempfile.TemporaryDirectory(prefix='f13-short-') as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            (base / 'util').chmod(0o755)
            regular = base / 'util/UNKNOWN_SECRET_NAME'
            regular.write_bytes(b'synthetic private content must not be read')
            (base / 'util').chmod(0o555)
            output = io.StringIO()
            with mock.patch.object(harness, 'expected_utilities', side_effect=RuntimeError('must remain unevaluated')) as expected:
                with contextlib.redirect_stderr(output):
                    with self.assertRaisesRegex(harness.SetupFailure, '^pinned_Git_private_utility_view$'):
                        harness.verify_pinned_git(root)
                self.assertEqual(expected.call_count, 0)
            value = json.loads(output.getvalue())
            self.assertEqual(value['expected_status'], 'not_evaluated_short_circuit')
            self.assertIsNone(value['counts']['expected'])
            self.assertEqual(value['counts']['non_symlink'], 1)
            self.assertEqual(value['entries'][0]['difference'], 'not_in_actual_symlink_snapshot')
            self.assertFalse(value['entries'][0]['slot']['is_symlink'])
            self.assertNotIn('UNKNOWN_SECRET_NAME', output.getvalue())
            output = io.StringIO()
            with mock.patch.object(harness, 'utility_view_diagnostic', side_effect=OSError('DO_NOT_LEAK_EXCEPTION_SECRET')):
                with contextlib.redirect_stderr(output):
                    with self.assertRaisesRegex(harness.SetupFailure, '^pinned_Git_private_utility_view$'):
                        harness.verify_pinned_git(root)
            self.assertEqual(json.loads(output.getvalue())['status'], 'diagnostic_unavailable')
            self.assertNotIn('DO_NOT_LEAK', output.getvalue())
            self.assertEqual(regular.read_bytes(), b'synthetic private content must not be read')

    def test_utility_metadata_redacts_unknown_names_paths_and_environment(self):
        # Deliberately hostile synthetic logging inputs, not hosted mappings.
        secret = 'SECRET_SENTINEL_token_123'
        actual = {'python3': '/home/' + secret, secret + '\n\0': '/usr/bin/python3/../' + secret}
        expected = {'python3': '/usr/bin/python3', 'sha256sum': '/usr/bin/sha256sum'}
        before = json.dumps([actual, expected], sort_keys=True)
        output = io.StringIO()
        with mock.patch.dict(os.environ, {'AWS_SECRET_ACCESS_KEY': secret}):
            with mock.patch.object(os, 'getgroups', return_value=[0, 1000, secret]):
                with contextlib.redirect_stderr(output):
                    harness.emit_utility_view_diagnostic(Path('/nonexistent'), actual, expected, 2)
        raw = output.getvalue()
        self.assertNotIn(secret, raw)
        self.assertNotIn('AWS_', raw)
        value = json.loads(raw)
        self.assertEqual(value['groups'], [0, 1000])
        self.assertEqual(value['groups_invalid'], 1)
        self.assertEqual(value['counts']['missing'], 1)
        self.assertEqual(value['counts']['extra'], 1)
        self.assertEqual(value['counts']['changed'], 1)
        self.assertEqual(harness.utility_diagnostic_token('/usr/bin/python3'), {'root': '/usr/bin', 'relative': 'python3'})
        for path in ('/usr/bin/../' + secret, '/usr/bin/python3/' + secret, '/opt/' + secret, secret, ''):
            self.assertEqual(set(harness.utility_diagnostic_token(path)), {'sha256', 'utf8_bytes'})
        self.assertEqual(json.dumps([actual, expected], sort_keys=True), before)

    def test_utility_diagnostic_truncation_entry_and_byte_bounds(self):
        secret = 'MUST_NOT_APPEAR'
        actual = {secret + str(i): '/private/' + secret + ('x' * 5000) for i in range(1000)}
        expected = {secret + str(i): '/other/' + secret + ('y' * 5000) for i in range(1000)}
        output = io.StringIO()
        with contextlib.redirect_stderr(output):
            harness.emit_utility_view_diagnostic(Path('/absent'), actual, expected, 1000)
        self.assertLessEqual(len(output.getvalue().encode()), harness.UTILITY_DIAGNOSTIC_BYTES)
        self.assertNotIn(secret, output.getvalue())
        value = json.loads(output.getvalue())
        self.assertLessEqual(len(value['entries']), harness.UTILITY_DIAGNOSTIC_ENTRIES)
        self.assertTrue(value['truncated'])
        self.assertEqual(value['omitted'] + len(value['entries']), 1000)

    def test_utility_access_metadata_does_not_expose_paths_or_contents(self):
        with tempfile.TemporaryDirectory(prefix='f13-access-') as folder:
            path = Path(folder) / 'SECRET_FILENAME'
            content = b'SECRET_FILE_CONTENT'
            path.write_bytes(content)
            path.chmod(0o600)
            link = Path(folder) / 'link'
            link.symlink_to(path)
            value = harness.utility_diagnostic_access(str(link))
            self.assertTrue(value['is_symlink'])
            self.assertTrue(value['is_file'])
            self.assertFalse(value['executable'])
            self.assertNotIn('SECRET', json.dumps(value))
            self.assertEqual(path.read_bytes(), content)
            self.assertEqual(harness.utility_diagnostic_access('relative/secret'), {'status': 'not_inspected'})

    def test_utility_predicate_exception_and_output_error_order_unchanged(self):
        with tempfile.TemporaryDirectory(prefix='f13-order-') as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            output = io.StringIO()
            with mock.patch.object(harness, 'expected_utilities', side_effect=RuntimeError('original_error')):
                with contextlib.redirect_stderr(output):
                    with self.assertRaisesRegex(RuntimeError, '^original_error$'):
                        harness.verify_pinned_git(root)
            self.assertEqual(output.getvalue(), '')
            (base / 'util').chmod(0o755)
            (base / 'util/regular').write_bytes(b'no mutation')
            (base / 'util').chmod(0o555)
            class BrokenOutput(io.StringIO):
                def write(self, text):
                    raise OSError('private output error')
            with contextlib.redirect_stderr(BrokenOutput()):
                with self.assertRaisesRegex(harness.SetupFailure, '^pinned_Git_private_utility_view$'):
                    harness.verify_pinned_git(root)

    def test_actual_materializer_installs_pinned_Git(self):
        with tempfile.TemporaryDirectory(prefix='f13-pinned-') as folder:
            root = Path(folder)
            harness.materialize(root)
            binary = root / 'historical-git/bin/git'
            self.assertTrue(binary.is_file(), 'actual materializer did not install pinned historical Git')
            self.assertEqual(hashlib.sha256(binary.read_bytes()).hexdigest(),
                             '2540879925a6881e3877ff7e3330746ba3027b04edf16a3a12dccd1644c4f32d')

    def test_real_loader_argv0_stdout_stderr_and_exit(self):
        self.assertNotEqual(os.getuid(), 0, 'component execution must be unprivileged Linux')
        with tempfile.TemporaryDirectory(prefix='f13-loader-') as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            env = harness.pinned_git_environment(root)
            result = subprocess.run(['git', '--version'], env=env, capture_output=True)
            self.assertEqual((result.returncode, result.stdout, result.stderr), (0, b'git version 2.39.5\n', b''))
            for name, args in [('git', ['-C', str(root), 'show', 'no-such-object']),
                               ('git-index-pack', ['--no-such-option']), ('git-rev-list', ['--no-such-option'])]:
                entry = base / ('launch' if name == 'git' else 'exec') / name
                actual = subprocess.run([str(entry), *args], env=env, capture_output=True)
                direct = subprocess.run([str(base / 'lib/ld-linux-x86-64.so.2'), '--library-path', str(base / 'lib'),
                                         '--argv0', name, str(base / 'bin/git-elf'), *args], env=env, capture_output=True)
                self.assertEqual((actual.returncode, actual.stdout, actual.stderr),
                                 (direct.returncode, direct.stdout, direct.stderr), name)
                self.assertNotEqual(actual.returncode, 0)

    def test_unknown_git_routes_cannot_fall_back_to_host(self):
        with tempfile.TemporaryDirectory(prefix='f13-fallback-') as folder:
            root = Path(folder) / 'private'
            root.mkdir()
            host = Path(folder) / 'fake-host'
            host.mkdir()
            marker = Path(folder) / 'forbidden-host-executed'
            for name in ('git', 'git-unknown-child', 'git-upload-pack'):
                file = host / name
                file.write_text('#!/bin/sh\ntouch ' + str(marker) + '\nexit 0\n')
                file.chmod(0o755)
            with mock.patch.object(harness, 'UTILITY_DIRS', (str(host), *harness.UTILITY_DIRS)):
                base = harness.materialize_pinned_git(root)
                env = harness.pinned_git_environment(root)
                self.assertFalse(any(p.name == 'git' or p.name.startswith('git-') for p in (base / 'util').iterdir()))
                for args in (['unknown-child'], ['--exec-path=' + str(host), '--version'], ['clone', 'https://invalid.example', 'bad']):
                    result = subprocess.run(['git', *args], env=env, capture_output=True)
                    self.assertEqual(result.returncode, 125)
                    self.assertIn(b'F13_PINNED_GIT_SETUP_FAILURE', result.stderr)
                for change in ({'GIT_EXEC_PATH': str(host)}, {'PATH': env['PATH'] + ':' + str(host)},
                               {'GIT_CONFIG_COUNT': '1', 'GIT_CONFIG_KEY_0': 'alias.version', 'GIT_CONFIG_VALUE_0': '!false'},
                               {'LD_LIBRARY_PATH': str(host)}):
                    result = subprocess.run([str(base / 'launch/git'), '--version'], env=dict(env, **change), capture_output=True)
                    self.assertEqual(result.returncode, 125)
                self.assertFalse(marker.exists())

    def test_same_size_binary_and_all_library_corruption_reject_before_launch(self):
        with tempfile.TemporaryDirectory(prefix='f13-forged-') as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            for name in harness.pinned_manifest():
                with self.subTest(name=name):
                    path = base / name
                    raw = path.read_bytes()
                    path.chmod(0o755)
                    forged = bytearray(raw)
                    forged[-1] ^= 1
                    path.write_bytes(forged)
                    path.chmod(0o555)
                    with self.assertRaisesRegex(harness.SetupFailure, 'pinned_Git_bytes'):
                        harness.pinned_git_environment(root)
                    path.chmod(0o755)
                    path.write_bytes(raw)
                    path.chmod(0o555)
            harness.pinned_git_environment(root)

    def test_forged_path_mode_and_inherited_context_reject(self):
        with tempfile.TemporaryDirectory(prefix='f13-path-') as folder:
            root = Path(folder)
            base = harness.materialize_pinned_git(root)
            binary = base / 'bin/git'
            binary.chmod(0o755)
            with self.assertRaisesRegex(harness.SetupFailure, 'pinned_Git_mode'):
                harness.pinned_git_environment(root)
            binary.chmod(0o555)
            (base / 'bin').chmod(0o755)
            binary.rename(base / 'bin/saved-git')
            binary.symlink_to(base / 'bin/saved-git')
            with self.assertRaisesRegex(harness.SetupFailure, 'pinned_Git_path'):
                harness.pinned_git_environment(root)
            binary.unlink()
            (base / 'bin/saved-git').rename(binary)
            (base / 'bin').chmod(0o555)
            for change in ({'GIT_CONFIG_COUNT': '1'}, {'LD_PRELOAD': '/wrong.so'}, {'GIT_EXEC_PATH': '/host/git'}):
                with mock.patch.dict(os.environ, change):
                    with self.assertRaises(harness.SetupFailure):
                        harness.pinned_git_environment(root)

    def test_archive_manifest_forgery_rejects_without_extraction(self):
        with tempfile.TemporaryDirectory(prefix='f13-archive-') as folder:
            fixture = Path(folder) / 'fixture'
            fixture.mkdir()
            for name in ('git-2.39.5-data.tar.gz', 'git-2.39.5-data-manifest.json'):
                shutil.copyfile(harness.FIXTURE / name, fixture / name)
            for name, reason in [('git-2.39.5-data.tar.gz', 'pinned_Git_archive'),
                                 ('git-2.39.5-data-manifest.json', 'pinned_Git_manifest')]:
                original = (fixture / name).read_bytes()
                forged = bytearray(original)
                forged[-1] ^= 1
                (fixture / name).write_bytes(forged)
                root = Path(folder) / ('private-' + reason)
                root.mkdir()
                with mock.patch.object(harness, 'FIXTURE', fixture):
                    with self.assertRaisesRegex(harness.SetupFailure, reason):
                        harness.materialize_pinned_git(root)
                self.assertFalse((root / 'historical-git').exists())
                (fixture / name).write_bytes(original)

    def test_captured_bundle_clone_child_routing_and_precheckout_rejections(self):
        with tempfile.TemporaryDirectory(prefix='f13-child-') as folder:
            root = Path(folder)
            harness.materialize(root)
            env = harness.pinned_git_environment(root)
            repo = root / 'repo'
            result = subprocess.run(['git', 'clone', '--no-checkout', str(root / 'out/prelaunch/private-target.bundle'), str(repo)],
                                    env=env, capture_output=True)
            self.assertEqual(result.returncode, 0, result.stderr.decode())
            with mock.patch.dict(os.environ, env, clear=True):
                harness.verify_checkout_inputs(repo, harness.TARGET)
                parents = harness.run(['git', '-C', str(repo), 'rev-list', '--parents', '-n', '1', harness.TARGET]).decode().split()
                self.assertEqual(parents, [harness.TARGET, harness.BASE, harness.SOURCE])
                config = repo / '.git/config'
                original = config.read_bytes()
                config.write_bytes(original + b'\n[alias]\nversion = !false\n')
                with self.assertRaisesRegex(harness.SetupFailure, 'pinned_Git_unsafe_config:alias.version'):
                    harness.verify_checkout_inputs(repo, harness.TARGET)
                config.write_bytes(original)
                hooks = repo / '.git/hooks'
                hooks.mkdir(exist_ok=True)
                (hooks / 'post-checkout').write_text('#!/bin/sh\nexit 0\n')
                with self.assertRaisesRegex(harness.SetupFailure, 'pinned_Git_hooks_absent'):
                    harness.verify_checkout_inputs(repo, harness.TARGET)
            self.assertFalse((repo / 'scripts').exists(), 'component tests never checkout or run the historical helper')

    def test_capability_diagnostic_preserves_every_field_and_raw_inputs(self):
        captured = {'git_version': 'git version 2.39.5', 'native': {'paths': ['a'], 'admission': False}, 'ledger': {'sha': 'abc'}}
        before = json.dumps(captured, sort_keys=True)
        self.assertEqual(harness.capability_differences(captured, captured), [])
        changed = json.loads(before)
        changed['native']['paths'][0] = 'foreign'
        changed['ledger']['sha'] = 'wrong'
        changed['git_version'] = 'git version 2.55.0'
        self.assertEqual(harness.capability_differences(captured, changed), ['$.git_version', '$.ledger.sha', '$.native.paths[0]'])
        self.assertEqual(json.dumps(captured, sort_keys=True), before)

    def test_actual_archive_restores_original_J_record_path(self):
        with tempfile.TemporaryDirectory(prefix='f13-transport-') as folder:
            root = Path(folder)
            harness.materialize(root)
            captured = root / 'out/actual/observed/fixed3071-J-ledger.json'
            original = root / 'tmp/F13-author-547/fixed3071-J-ledger.json'
            self.assertTrue(original.is_file(), 'original literal J-ledger consumer path was not materialized')
            raw = original.read_bytes()
            self.assertEqual(len(raw), 109549632)
            self.assertEqual(hashlib.sha256(raw).hexdigest(), harness.LEDGER_SHA)
            self.assertEqual(raw, captured.read_bytes())

    def test_record_alias_rejects_wrong_declared_binding(self):
        with tempfile.TemporaryDirectory(prefix='f13-transport-') as folder:
            root = Path(folder)
            for binding in (None, ('0' * 64, 109549632), (harness.LEDGER_SHA, 0)):
                with self.subTest(binding=binding):
                    logical = {} if binding is None else {harness.J_CAPTURE: binding}
                    with self.assertRaisesRegex(harness.SetupFailure, 'J_record_declared_binding'):
                        harness.materialize_J_record_alias(root, logical)
                    self.assertEqual(list(root.iterdir()), [])

    def test_record_alias_rejects_forged_captured_bytes(self):
        with tempfile.TemporaryDirectory(prefix='f13-transport-') as folder:
            root = Path(folder)
            source = root / harness.J_CAPTURE.lstrip('/')
            source.parent.mkdir(parents=True)
            with zipfile.ZipFile(harness.FIXTURE / 'historical-inputs.zip') as archive:
                forged = bytearray(archive.read('objects/' + harness.LEDGER_SHA))
            self.assertEqual(len(forged), harness.J_RECORD_BYTES)
            forged[-1] ^= 1  # Same-length actual record mutation must hit SHA, not size.
            self.assertNotEqual(hashlib.sha256(forged).hexdigest(), harness.LEDGER_SHA)
            source.write_bytes(forged)
            with self.assertRaisesRegex(harness.SetupFailure, 'J_record_raw_binding'):
                harness.materialize_J_record_alias(root, {harness.J_CAPTURE: (harness.LEDGER_SHA, 109549632)})
            self.assertEqual(source.read_bytes(), forged)
            self.assertFalse((root / harness.J_ORIGINAL_RECORD.lstrip('/')).exists())

    def test_record_alias_never_overwrites_existing_target(self):
        with tempfile.TemporaryDirectory(prefix='f13-transport-') as folder:
            root = Path(folder)
            source, target = root / harness.J_CAPTURE.lstrip('/'), root / harness.J_ORIGINAL_RECORD.lstrip('/')
            source.parent.mkdir(parents=True)
            source.write_bytes(b'rejection-only source control')
            target.parent.mkdir(parents=True)
            target.write_bytes(b'prior target must stay byte-exact')
            with self.assertRaisesRegex(harness.SetupFailure, 'J_record_no_overwrite'):
                harness.materialize_J_record_alias(root, {harness.J_CAPTURE: (harness.LEDGER_SHA, 109549632)})
            self.assertEqual(target.read_bytes(), b'prior target must stay byte-exact')


if __name__ == '__main__':
    unittest.main()
