import json
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import local_ci_staging_execution as staging
import local_ci_staging_request as contract


class ExecutionTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        parent = Path(self.tmp.name)
        root = parent / 'root'
        root.mkdir()
        self.request = {'root': str(root), 'output': str(parent / 'output'),
                        'commit': 'a' * 40, 'tree': 'b' * 40, 'base': 'c' * 40, 'shard': 'staging',
                        'assets': {name: str(parent / name) for name in contract.ASSET_NAMES}}

    def completed(self, output=b'STAGING_SHARD_DIAGNOSTIC_OK\nOWNED_RESOURCES_CLEANED\n', rc=0):
        directory = Path(self.request['output'])
        directory.mkdir()
        run = 'voiceci-' + 'a' * 32
        journal = directory / 'owner' / 'tmp' / 'ci-local' / run / 'ownership.json'
        journal.parent.mkdir(parents=True)
        resources = []
        for index, kind in enumerate(['network'] + ['volume'] * 4 + ['container'] * 5):
            nonce = f'{index:032x}'
            name = f'{run}-{kind}-{nonce}'
            resources.append({'kind': kind, 'name': name, 'nonce': nonce,
                              'id': name if kind == 'volume' else f'{index + 1:064x}', 'state': 'removed'})
        journal.write_text(json.dumps({'version': 1, 'run_id': run, 'daemon': 'daemon-1', 'resources': resources}))
        (directory / 'identity.json').write_text(json.dumps({'source': 'a' * 40, 'daemon': 'daemon-1', 'journal': str(journal), 'slot': 0}))
        (directory / 'suite-counts.json').write_text(json.dumps({'shard': self.request['shard'],
            'recipes': {'staging': 34, 'proofs': 15, 'runtime': 23, 'policy': 69}[self.request['shard']],
            'hosted_go_tests': 1 if self.request['shard'] == 'staging' else 0,
            'hosted_policy_tests': 1 if self.request['shard'] == 'policy' else 0}))
        return SimpleNamespace(returncode=rc, stdout=output, stderr=b'')

    def test_supervised_result_remains_incomplete(self):
        with patch.object(staging.supervision, 'run_diagnostic', side_effect=lambda *a, **k: self.completed()) as run:
            report = staging.run(self.request)
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['source'], self.request['commit'])
        self.assertEqual(report['recipes'], 34)
        command = run.call_args.args[0]
        self.assertEqual(command[1:4], ['-I', '-S', '-B'])

    def test_failure_and_missing_cleanup_cannot_produce_report(self):
        with patch.object(staging.supervision, 'run_diagnostic', side_effect=lambda *a, **k: self.completed(output=b'STAGING_SHARD_DIAGNOSTIC_OK\n')), self.assertRaises(staging.ExecutionError):
            staging.run(self.request)

    def test_proofs_receipt_is_bound_to_proofs_inventory(self):
        self.request['shard'] = 'proofs'
        with patch.object(staging.supervision, 'run_diagnostic', side_effect=lambda *a, **k: self.completed()):
            report = staging.run(self.request)
        self.assertEqual((report['shard'], report['recipes'], report['hosted_go_tests']), ('proofs', 15, 0))

    def test_policy_receipt_requires_hosted_tests(self):
        self.request['shard'] = 'policy'
        with patch.object(staging.supervision, 'run_diagnostic', side_effect=lambda *a, **k: self.completed()):
            report = staging.run(self.request)
        self.assertEqual((report['recipes'], report['hosted_policy_tests']), (69, 1))
        self.request['output'] = str(Path(self.tmp.name) / 'policy-missing')
        def missing(*args, **kwargs):
            result = self.completed()
            path = Path(self.request['output']) / 'suite-counts.json'
            counts = json.loads(path.read_text()); counts['hosted_policy_tests'] = 0
            path.write_text(json.dumps(counts))
            return result
        with patch.object(staging.supervision, 'run_diagnostic', side_effect=missing), self.assertRaises(staging.ExecutionError):
            staging.run(self.request)

    def test_proofs_cannot_accept_staging_counts(self):
        self.request['shard'] = 'proofs'
        def wrong(*args, **kwargs):
            self.request['shard'] = 'staging'
            result = self.completed()
            self.request['shard'] = 'proofs'
            return result
        with patch.object(staging.supervision, 'run_diagnostic', side_effect=wrong), self.assertRaises(staging.ExecutionError):
            staging.run(self.request)

    def test_runtime_receipt_binds_all_23_recipes(self):
        self.request['shard'] = 'runtime'
        parent = Path(self.tmp.name)
        images = {key: {'archive': str(parent / key), 'sha256': 'd' * 64, 'image_id': 'sha256:' + 'e' * 64}
                  for key in ('postgres', 'docker_cli', 'python')}
        images['postgres']['image_id'] = 'sha256:75f5a96988cdf694a215073c3e9c001b706b371e2f94df3967f2efdec2787f6b'
        self.request['runtime_images'] = images
        with patch.object(staging.supervision, 'run_diagnostic', side_effect=lambda *a, **k: self.completed()):
            report = staging.run(self.request)
        self.assertEqual((report['shard'], report['recipes'], report['hosted_go_tests']), ('runtime', 23, 0))

    def test_failed_exit_and_bad_receipt_fail_closed(self):
        for mode in ('exit', 'source', 'zero-tests', 'missing-recipe', 'wrong-path', 'slot', 'missing-journal', 'unremoved', 'daemon'):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory() as directory:
                parent = Path(directory)
                root = parent / 'root'
                root.mkdir()
                request = dict(self.request, root=str(root), output=str(parent / 'output'))
                def finish(*args, **kwargs):
                    output = Path(request['output'])
                    output.mkdir()
                    run = 'voiceci-' + 'a' * 32
                    journal = output / 'owner' / 'tmp' / 'ci-local' / run / 'ownership.json'
                    journal.parent.mkdir(parents=True)
                    resources = []
                    for index, kind in enumerate(['network'] + ['volume'] * 4 + ['container'] * 5):
                        nonce = f'{index:032x}'; name = f'{run}-{kind}-{nonce}'
                        resources.append({'kind': kind, 'name': name, 'nonce': nonce,
                            'id': name if kind == 'volume' else f'{index + 1:064x}',
                            'state': 'ready' if mode == 'unremoved' and index == 0 else 'removed'})
                    if mode != 'missing-journal':
                        journal.write_text(json.dumps({'version': 1, 'run_id': run, 'daemon': 'daemon-1', 'resources': resources}))
                    identity = {'source': 'd' * 40 if mode == 'source' else request['commit'],
                        'journal': str(parent / 'forged.json') if mode == 'wrong-path' else str(journal),
                        'daemon': 'daemon-2' if mode == 'daemon' else 'daemon-1', 'slot': 2 if mode == 'slot' else 0}
                    counts = {'shard': 'staging', 'recipes': 33 if mode == 'missing-recipe' else 34,
                              'hosted_go_tests': 0 if mode == 'zero-tests' else 1, 'hosted_policy_tests': 0}
                    (output / 'identity.json').write_text(json.dumps(identity))
                    (output / 'suite-counts.json').write_text(json.dumps(counts))
                    return SimpleNamespace(returncode=1 if mode == 'exit' else 0,
                        stdout=b'STAGING_SHARD_DIAGNOSTIC_OK\nOWNED_RESOURCES_CLEANED\n', stderr=b'')
                with patch.object(staging.supervision, 'run_diagnostic', side_effect=finish), self.assertRaises(staging.ExecutionError):
                    staging.run(request)


if __name__ == '__main__':
    unittest.main()
