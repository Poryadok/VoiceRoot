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
        kinds = ['network'] + ['volume'] * 4 + ['container'] * 5
        if self.request['shard'] == 'runtime':
            kinds += ['network', 'volume', 'container', 'container']
        for index, kind in enumerate(kinds):
            nonce = f'{index:032x}'
            name = f'{run}-{kind}-{nonce}'
            resources.append({'kind': kind, 'name': name, 'nonce': nonce,
                              'id': name if kind == 'volume' else f'{index + 1:064x}', 'state': 'removed'})
        if self.request['shard'] == 'runtime':
            network = resources[0]['id']
            rootless_mounts = {'/home/rootless/.local/share/docker': resources[1]['name'],
                               '/var/lib/docker': resources[2]['name'], '/run/user/1000': resources[3]['name']}
            image_ids = [staging.CLI_IMAGE, staging.PYTHON_CLIENT, staging.GO_IMAGE, staging.NODE_IMAGE]
            for row, image in zip(resources[5:9], image_ids):
                row.update(network=network, image=image, ports=[], mounts={'/run/user/1000': resources[3]['name']},
                           command=['sleep', 'infinity'])
            resources[6]['mounts']['/workspace'] = resources[4]['name']
            resources[9].update(profile='rootless-unix-v1', network=network, image=staging.rootless_profile.IMAGE,
                                ports=[], mounts=rootless_mounts)
            resources[12].update(network=resources[10]['id'], image=staging.PYTHON_CLIENT, ports=[],
                                 mounts={'/phase0-source': resources[11]['name']}, command=['sleep', 'infinity'])
            resources[13].update(profile='phase0-isolated-v1', network='none',
                                 image=staging.PYTHON_CLIENT, ports=[],
                                 mounts={'/workspace/repo': resources[11]['name']},
                                 readonly_mounts=['/workspace/repo'], command=['sleep', 'infinity'])
        journal.write_text(json.dumps({'version': 1, 'run_id': run, 'daemon': 'daemon-1', 'resources': resources}))
        (directory / 'identity.json').write_text(json.dumps({'source': 'a' * 40, 'daemon': 'daemon-1', 'journal': str(journal), 'slot': 0}))
        (directory / 'suite-counts.json').write_text(json.dumps({'shard': self.request['shard'],
            'recipes': {'staging': 34, 'proofs': 15, 'runtime': 23, 'policy': 76}[self.request['shard']],
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
        self.request['assets']['kubectl'] = str(Path(self.tmp.name) / 'kubectl')
        with patch.object(staging.supervision, 'run_diagnostic', side_effect=lambda *a, **k: self.completed()):
            report = staging.run(self.request)
        self.assertEqual((report['recipes'], report['hosted_policy_tests']), (76, 1))
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
        self.request['assets'].update({name: str(Path(self.request['output']).parent / name)
                                       for name in ('phase0_jdk', 'phase0_compose')})
        parent = Path(self.tmp.name)
        images = {key: {'archive': str(parent / key), 'sha256': 'd' * 64, 'image_id': 'sha256:' + 'e' * 64}
                  for key in ('postgres', 'docker_cli', 'python')}
        images['postgres']['image_id'] = 'sha256:75f5a96988cdf694a215073c3e9c001b706b371e2f94df3967f2efdec2787f6b'
        self.request['runtime_images'] = images
        with patch.object(staging.supervision, 'run_diagnostic', side_effect=lambda *a, **k: self.completed()):
            report = staging.run(self.request)
        self.assertEqual((report['shard'], report['recipes'], report['hosted_go_tests']), ('runtime', 23, 0))

    def test_runtime_journal_rejects_missing_extra_wrong_kind_or_phase0_drift(self):
        self.request['shard'] = 'runtime'
        self.request['assets'].update({name: str(Path(self.tmp.name) / name) for name in ('phase0_jdk', 'phase0_compose')})
        images = {key: {'archive': str(Path(self.tmp.name) / key), 'sha256': 'd' * 64, 'image_id': 'sha256:' + 'e' * 64}
                  for key in ('postgres', 'docker_cli', 'python')}
        images['postgres']['image_id'] = 'sha256:75f5a96988cdf694a215073c3e9c001b706b371e2f94df3967f2efdec2787f6b'
        self.request['runtime_images'] = images
        for mode in ('missing', 'extra', 'wrong-kind', 'wrong-profile', 'wrong-volume', 'wrong-network', 'writable',
                     'unremoved', 'rootless-empty', 'rootless-foreign', 'ordinary-foreign', 'ordinary-image', 'ordinary-command'):
            with self.subTest(mode=mode):
                self.request['output'] = str(Path(self.tmp.name) / mode)
                def finish(*args, **kwargs):
                    result = self.completed()
                    identity = json.loads((Path(self.request['output']) / 'identity.json').read_text())
                    path = Path(identity['journal']); journal = json.loads(path.read_text()); rows = journal['resources']
                    if mode == 'missing': rows.pop()
                    elif mode == 'extra': rows.append(dict(rows[-1]))
                    elif mode == 'wrong-kind': rows[10]['kind'] = 'volume'
                    elif mode == 'wrong-profile': rows[13]['profile'] = 'rootless-unix-v1'
                    elif mode == 'wrong-volume': rows[13]['mounts']['/workspace/repo'] = rows[1]['name']
                    elif mode == 'wrong-network': rows[12]['network'] = rows[0]['id']
                    elif mode == 'writable': rows[13]['readonly_mounts'] = []
                    elif mode == 'rootless-empty': rows[9]['mounts'] = {}
                    elif mode == 'rootless-foreign': rows[9]['mounts']['/var/lib/docker'] = 'foreign-volume'
                    elif mode == 'ordinary-foreign': rows[5]['mounts']['/run/user/1000'] = 'foreign-volume'
                    elif mode == 'ordinary-image': rows[5]['image'] = 'sha256:' + 'f' * 64
                    elif mode == 'ordinary-command': rows[5]['command'] = ['sh']
                    else: rows[13]['state'] = 'ready'
                    path.write_text(json.dumps(journal))
                    return result
                with patch.object(staging.supervision, 'run_diagnostic', side_effect=finish), self.assertRaises(staging.ExecutionError):
                    staging.run(self.request)

    def test_runtime_inventory_is_not_accepted_for_other_shards(self):
        self.request['shard'] = 'runtime'
        self.completed()
        output = Path(self.request['output']).resolve()
        identity = json.loads((output / 'identity.json').read_text())
        for shard in ('staging', 'proofs', 'policy', 'unknown'):
            with self.subTest(shard=shard), self.assertRaises(ValueError):
                staging._validate_journal(output, identity, shard)

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
