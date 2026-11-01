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

    def test_owned_make_copy_is_verified_inside_actual_client(self):
        owner = mock.Mock()
        owner.create_container.return_value = 'tools'
        engine = mock.Mock()
        engine._call.return_value = subject.MAKE_IMAGE + ' linux/amd64'
        expected = ('a' * 64 + '  /workspace/tools/make\nGNU Make 4.3\n').encode()
        def execute(client, command, out, err, timeout):
            out.write(expected)
            return 0
        owner.execute.side_effect = execute
        result = subject.provision_make(owner, engine, 'net', 'volume', 'client')
        self.assertEqual(result, {'image_id': subject.MAKE_IMAGE, 'sha256': 'a' * 64, 'version': 'GNU Make 4.3'})
        self.assertEqual([call.args[0] for call in owner.execute.call_args_list], ['tools', 'client'])
        self.assertIn('cmp /usr/bin/make /workspace/tools/make', owner.execute.call_args_list[0].args[1][2])

    def test_make_mismatch_or_incompatible_client_fails_closed(self):
        for second in (b'b' * 64 + b'  /workspace/tools/make\nGNU Make 4.3\n', b''):
            owner = mock.Mock(); owner.create_container.return_value = 'tools'
            engine = mock.Mock(); engine._call.return_value = subject.MAKE_IMAGE + ' linux/amd64'
            def execute(client, command, out, err, timeout):
                out.write((b'a' * 64 + b'  /workspace/tools/make\nGNU Make 4.3\n') if client == 'tools' else second)
                return 0 if client == 'tools' or second else 1
            owner.execute.side_effect = execute
            with self.subTest(second=second), self.assertRaises(ValueError):
                subject.provision_make(owner, engine, 'net', 'volume', 'client')

    def test_exact_required_image_inventory_and_identities(self):
        items = self.inputs()
        with mock.patch.object(subject.common, '_verify_file') as verify:
            subject.validate_images(items)
        self.assertEqual(verify.call_count, len(subject.IMAGE_TAGS))
        for mutation in ('missing', 'extra', 'relative', 'digest', 'identity', 'unknown'):
            bad = self.inputs(); first = next(iter(bad))
            if mutation == 'missing': bad.pop(first)
            if mutation == 'extra': bad['foreign:latest'] = bad[first]
            if mutation == 'relative': bad[first]['archive'] = 'relative'
            if mutation == 'digest': bad[first]['sha256'] = 'bad'
            if mutation == 'identity': bad[first]['image_id'] = 'sha256:bad'
            if mutation == 'unknown': bad[first]['extra'] = 'unexpected'
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                subject.validate_images(bad)

    def test_duplicate_image_id_uses_journaled_alias_load(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory); owner = mock.Mock(); owner.directory = root / 'owner'; owner.directory.mkdir()
            images = self.inputs()
            first, second = subject.IMAGE_TAGS[:2]
            images[second]['image_id'] = images[first]['image_id']
            for index, tag in enumerate(subject.IMAGE_TAGS):
                path = root / ('source-' + str(index)); path.write_bytes(b'archive')
                images[tag]['archive'] = str(path)
            output = root / 'out'; output.mkdir()
            with mock.patch.object(subject.common, '_verify_file'):
                subject.validate_images(images)
                result = subject.load_images(owner, 'daemon', images, output)
            self.assertEqual(owner.load_canonical_rootless_image.call_count, 4)
            self.assertEqual(owner.load_canonical_rootless_alias.call_count, 1)
            args = owner.load_canonical_rootless_alias.call_args.args
            self.assertEqual((args[4], args[5]), (first, second))
            self.assertEqual(result[second]['image_id'], result[first]['image_id'])

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

    def test_missing_make_fails_before_private_daemon_or_image_load(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'; root.mkdir(); output = Path(directory) / 'out'
            engine = mock.Mock(); engine._call.return_value = 'sha256:' + 'c' * 64
            scheduled = mock.Mock(); scheduled.run.create_resource.return_value = 'owned-volume'
            with mock.patch.object(subject, '_require_worker_job'), \
                 mock.patch.object(subject.common, '_verify_file'), \
                 mock.patch.object(subject.local_auth_private, 'cache_digest', return_value='d' * 64), \
                 mock.patch.object(subject, 'validate_images'), \
                 mock.patch.object(subject.snapshot, 'capture', return_value={'head': 'a' * 40}), \
                 mock.patch.object(subject.snapshot, 'materialize', return_value=Path(directory) / 'copy'), \
                 mock.patch.object(subject, 'DockerCLI', return_value=engine), \
                 mock.patch.object(subject.ScheduledRun, 'start_host', return_value=scheduled), \
                 mock.patch.object(subject, 'provision_make', side_effect=ValueError('missing make')), \
                 mock.patch.object(subject, 'load_images') as load, \
                 self.assertRaisesRegex(ValueError, 'missing make'):
                subject._run_owned(root, 'a' * 40, output, root / 'helper', 'b' * 64,
                                   'sha256:' + 'c' * 64, root / 'cache', 'd' * 64, self.inputs())
            scheduled.run.create_rootless_container.assert_not_called()
            load.assert_not_called()
            scheduled.close.assert_called_once()


if __name__ == '__main__':
    unittest.main()
