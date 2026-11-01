import unittest
import tempfile
import json
import subprocess
from pathlib import Path
from unittest import mock

import local_auth_full_orchestration as subject


class FullAuthInputsTests(unittest.TestCase):
    def inputs(self):
        return {tag: {'archive': str(Path(tempfile.gettempdir()) / str(index)), 'sha256': 'a' * 64,
                      'image_id': 'sha256:' + ('%x' % (index + 1)) * 64}
                for index, tag in enumerate(subject.IMAGE_TAGS)}

    def test_exact_required_image_inventory_and_identities(self):
        items = self.inputs()
        with mock.patch.object(subject.common, '_verify_file') as verify:
            subject.validate_images(items)
        self.assertEqual(verify.call_count, len(subject.IMAGE_TAGS))
        for mutation in ('missing', 'extra', 'relative', 'digest', 'identity', 'unknown', 'duplicate-id'):
            bad = self.inputs(); first = next(iter(bad))
            if mutation == 'missing': bad.pop(first)
            if mutation == 'extra': bad['foreign:latest'] = bad[first]
            if mutation == 'relative': bad[first]['archive'] = 'relative'
            if mutation == 'digest': bad[first]['sha256'] = 'bad'
            if mutation == 'identity': bad[first]['image_id'] = 'sha256:bad'
            if mutation == 'unknown': bad[first]['extra'] = 'unexpected'
            if mutation == 'duplicate-id': bad[list(bad)[1]]['image_id'] = bad[first]['image_id']
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                subject.validate_images(bad)

    def test_public_run_uses_broker_and_rejects_worker_failure(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); output = root / 'out'; images = self.inputs()
            report = {'status': 'incomplete'}
            def supervised(command, **kwargs):
                request = json.loads(kwargs['input'])
                self.assertEqual(request['images'], images)
                self.assertIn('--worker', command)
                output.mkdir(); (output / 'diagnostic.json').write_text(json.dumps(report), encoding='utf-8')
                return subprocess.CompletedProcess(command, 0, stdout=json.dumps(report).encode(), stderr=b'')
            with mock.patch.object(subject.supervision, 'run_diagnostic', side_effect=supervised) as broker, \
                 mock.patch.object(subject, '_run_owned') as direct:
                result = subject.run(root, 'a' * 40, output, root / 'helper', 'b' * 64,
                                     'sha256:' + 'c' * 64, root / 'cache', 'd' * 64, images)
            direct.assert_not_called(); broker.assert_called_once()
            self.assertEqual(result, report)
            with mock.patch.object(subject.supervision, 'run_diagnostic', return_value=subprocess.CompletedProcess([], 1, b'', b'')), \
                 self.assertRaisesRegex(ValueError, 'worker failed'):
                subject.run(root, 'a' * 40, root / 'failed', root / 'helper', 'b' * 64,
                            'sha256:' + 'c' * 64, root / 'cache', 'd' * 64, images)

    def test_worker_entry_refuses_unfenced_invocation_before_owned_work(self):
        with mock.patch.object(subject.sys, 'argv', ['worker', '--worker']), \
             mock.patch.object(subject, '_require_worker_job', side_effect=ValueError('unfenced')), \
             mock.patch.object(subject, '_run_owned') as owned, self.assertRaisesRegex(ValueError, 'unfenced'):
            subject.main()
        owned.assert_not_called()

    def test_isolated_worker_allowlist_includes_its_transitive_modules(self):
        command = subject._worker_command()
        self.assertEqual(command[1:4], ['-I', '-S', '-B'])
        for name in ('local_auth_full', 'local_auth_private', 'local_nats_hosted_diagnostic'):
            self.assertIn(repr(name), command[5])

    def test_owned_setup_failure_closes_scheduled_run(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'; root.mkdir(); output = Path(directory) / 'out'
            engine = mock.Mock(); engine._call.return_value = 'sha256:' + 'c' * 64
            scheduled = mock.Mock(); scheduled.run.create_resource.side_effect = ValueError('fixture failure')
            with mock.patch.object(subject, '_require_worker_job'), \
                 mock.patch.object(subject.common, '_verify_file'), \
                 mock.patch.object(subject.local_auth_private, 'cache_digest', return_value='d' * 64), \
                 mock.patch.object(subject, 'validate_images'), \
                 mock.patch.object(subject.snapshot, 'capture', return_value={'head': 'a' * 40}), \
                 mock.patch.object(subject.snapshot, 'materialize', return_value=Path(directory) / 'copy'), \
                 mock.patch.object(subject, 'DockerCLI', return_value=engine), \
                 mock.patch.object(subject.ScheduledRun, 'start_host', return_value=scheduled), \
                 self.assertRaisesRegex(ValueError, 'fixture failure'):
                subject._run_owned(root, 'a' * 40, output, root / 'helper', 'b' * 64,
                                   'sha256:' + 'c' * 64, root / 'cache', 'd' * 64, self.inputs())
            scheduled.close.assert_called_once()


if __name__ == '__main__':
    unittest.main()
