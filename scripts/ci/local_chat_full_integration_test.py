"""Full Chat diagnostics must reject missing or skipped required tests."""
import json
import os
import shutil
import subprocess
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import local_chat_full_integration as subject


class FullChatTests(unittest.TestCase):
    def events(self, action='pass'):
        package = 'voice/backend/chat/internal/store'
        return [dict(Package=package, Action='start'),
                dict(Package=package, Test='TestRequired', Action='run'),
                dict(Package=package, Test='TestRequired', Action=action),
                dict(Package=package, Action='pass')]

    def test_required_inventory_requires_each_case_to_pass(self):
        package = 'voice/backend/chat/internal/store'
        required = [[package, 'TestRequired']]
        summary = subject.validate_events(self.events(), [package], [], required)
        self.assertEqual(summary['passed'], 1)
        for events in (self.events('skip'), self.events()[:1] + self.events()[-1:]):
            with self.subTest(events=events), self.assertRaises(ValueError):
                subject.validate_events(events, [package], [], required)

    def test_inventory_binds_service_and_discovered_packages(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            path = root / 'scripts/ci/go_integration_required.json'
            path.parent.mkdir(parents=True)
            package = 'voice/backend/chat/internal/store'
            path.write_text(json.dumps({'chat': [[package, 'TestRequired']]}), encoding='utf-8')
            self.assertEqual(subject.read_required(root, [package]), [[package, 'TestRequired']])
            for invalid in ({'chat': [[package, 'TestRequired'], [package, 'TestRequired']]},
                            {'chat': [['voice/backend/voice', 'TestRequired']]},
                            {'chat': [['voice/backend/chat/other', 'TestRequired']]},
                            {'chat': []}):
                path.write_text(json.dumps(invalid), encoding='utf-8')
                with self.subTest(invalid=invalid), self.assertRaises(ValueError):
                    subject.read_required(root, [package])

    def test_failed_setup_closes_owned_run_without_claiming_success(self):
        scheduled = mock.Mock()
        scheduled.run.create_resource.side_effect = RuntimeError('unavailable')
        with mock.patch.object(subject, 'validate_archives'), self.assertRaisesRegex(RuntimeError, 'unavailable'):
            subject.run_owned(scheduled, mock.Mock(), {}, Path('.'), Path('.'), Path('.'),
                              'helper', 'a' * 64, {}, {})
        scheduled.close.assert_called_once_with()

    def test_image_manifest_rejects_missing_or_shared_identity_before_docker(self):
        owner = mock.Mock()
        for identities in ({}, {tag: 'bad' for tag in subject.IMAGES}):
            with self.subTest(identities=identities), self.assertRaises(ValueError):
                subject.verify_images(owner, 'client', identities)
        owner.execute.assert_not_called()

    def test_full_runner_uses_all_packages_without_short_and_validates_required(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'
            root.mkdir()
            copy = Path(directory) / 'copy'
            path = copy / 'scripts/ci/go_integration_required.json'
            path.parent.mkdir(parents=True)
            package = 'voice/backend/chat/internal/store'
            path.write_text(json.dumps({'chat': [[package, 'TestRequired']]}), encoding='utf-8')
            events = b''.join((json.dumps(event) + '\n').encode() for event in self.events())
            owner = mock.Mock()
            def execute(client, command, out, err, timeout):
                self.assertIn('go test -json -count=1 -timeout 20m -p 2 ./...', command[2])
                self.assertNotIn('-short', command[2])
                self.assertEqual(command[-1], 'private-address')
                out.write(events)
                return 0
            owner.execute.side_effect = execute
            source = {'head': 'a' * 40}
            inventory = {'packages_with_tests': [package], 'packages_without_tests': []}
            with mock.patch.object(subject.local_chat_private, 'inspect_pair', return_value='private-address'), \
                 mock.patch.object(subject, 'verify_images'), \
                 mock.patch.object(subject.local_go_execution, 'discover_packages', return_value=inventory), \
                 mock.patch.object(subject.local_go_execution, '_verify_modules'), \
                 mock.patch.object(subject.local_go_execution, '_verify_workspace'), \
                 mock.patch.object(subject.snapshot, 'verify_materialized'), \
                 mock.patch.object(subject.snapshot, 'capture', return_value=source):
                result = subject.run_full(owner, 'daemon', 'client', root, source, copy, 'b' * 64,
                                          {tag: 'sha256:' + 'c' * 64 for tag in subject.IMAGES},
                                          Path(directory) / 'output')
            self.assertEqual(result['status'], 'incomplete')
            self.assertEqual(result['required_count'], 1)
            self.assertEqual(result['tests']['skipped'], [])

    def test_owner_receipt_records_bound_slot_before_execution(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory)
            scheduled = mock.Mock(slot=1)
            scheduled.run.run_id = 'voiceci-' + 'a' * 32
            scheduled.run.directory = output / 'run/tmp/ci-local' / scheduled.run.run_id
            subject.publish_owner(scheduled, output)
            receipt = json.loads((output / 'owner.json').read_text())
            self.assertEqual(receipt['slot'], 1)
            self.assertEqual(receipt['run_id'], scheduled.run.run_id)
            self.assertEqual(receipt['directory'], str(scheduled.run.directory))

    def test_bounded_script_caps_both_streams_and_preserves_failure(self):
        bash = 'C:/Program Files/Git/bin/bash.exe' if os.name == 'nt' else shutil.which('bash')
        if not bash or not Path(bash).is_file():
            self.skipTest('bash unavailable')
        script = subject.bounded_script("printf '12345678901234567890'; printf 'abcdefghijklmno' >&2", 16, 12)
        result = subprocess.run([bash, '-c', script], capture_output=True, timeout=10, check=False)
        self.assertEqual(result.stdout, b'12345678901234567')
        self.assertEqual(result.stderr, b'abcdefghijklm')
        script = subject.bounded_script('exit 7', 16, 12)
        result = subprocess.run([bash, '-c', script], capture_output=True, timeout=10, check=False)
        self.assertEqual(result.returncode, 7)

    def test_failed_or_interrupted_work_still_invokes_scheduled_cleanup(self):
        for failure in (ValueError('output limit exceeded'), KeyboardInterrupt()):
            scheduled = mock.Mock()
            scheduled.run.create_resource.side_effect = failure
            with self.subTest(failure=failure), mock.patch.object(subject, 'validate_archives'), self.assertRaises(type(failure)):
                subject.run_owned(scheduled, mock.Mock(), {}, Path('.'), Path('.'), Path('.'),
                                  'helper', 'a' * 64, {}, {})
            scheduled.close.assert_called_once_with()

    def test_overflow_after_provisioning_does_not_skip_owned_cleanup(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            (root / 'dependencies').mkdir()
            archive = root / 'input.tar'
            archive.write_bytes(b'fixture')
            owner = mock.Mock(directory=root / 'owned')
            owner.directory.mkdir()
            owner.create_resource.side_effect = ['network', 'a', 'b', 'c', 'workspace']
            owner.create_container.return_value = 'client'
            owner.create_rootless_container.return_value = 'daemon'
            owner.execute.return_value = 0
            scheduled = mock.Mock(run=owner, slot=0)
            archives = {tag: {'archive': str(archive), 'archive_sha256': 'a' * 64,
                              'image_id': 'sha256:' + 'b' * 64} for tag in subject.IMAGES}
            with mock.patch.object(subject.orchestration, '_verify_file'), \
                 mock.patch.object(subject.orchestration, '_wait_daemon'), \
                 mock.patch.object(subject.local_go_execution, 'transfer_source', return_value='c' * 64), \
                 mock.patch.object(subject.local_go_execution, 'transfer_dependencies'), \
                 mock.patch.object(subject, 'run_full', side_effect=ValueError('Go diagnostic output limit exceeded')), \
                 self.assertRaisesRegex(ValueError, 'output limit'):
                subject.run_owned(scheduled, mock.Mock(), {}, root, root, root, 'helper', 'd' * 64, archives, {})
            scheduled.close.assert_called_once_with()


if __name__ == '__main__':
    unittest.main()
