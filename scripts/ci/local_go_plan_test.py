"""Planner-selected Go services must execute under owned host slots."""
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import local_go_plan as runner


class GoPlanTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.base = Path(self.temp.name)
        self.root = self.base / 'root'
        self.root.mkdir()
        self.source = {'head': 'a' * 40}
        self.scheduled = []

    def test_selected_services_run_full_discovered_inventory_under_host_slots(self):
        class Owner:
            def __init__(self, directory):
                self.directory = directory / 'tmp/ci-local/voiceci-fixture'
            def create_resource(self, kind):
                return kind
            def create_container(self, *args, **kwargs):
                return 'container'
            def start_container(self, name):
                pass
        class Scheduled:
            def __init__(self, directory):
                self.run, self.closed, self.slot = Owner(directory), False, 0
            def close(self):
                self.closed = True
        class Engine:
            def _call(self, *args):
                return 'sha256:' + 'a' * 64
        def start(*args):
            result = Scheduled(args[0])
            self.scheduled.append(result)
            return result
        def acquire(root, source, copy, module, output, go_binary):
            return {'module_dir': module, 'status': 'incomplete'}
        inventory = {'packages_with_tests': ['example.test/module'],
                     'packages_without_tests': ['example.test/module/empty']}
        diagnostic = {'status': 'incomplete', 'tests': {'passed': 1, 'skipped': [['example.test/module', 'TestOptIn']]}}
        with patch.object(runner.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(runner.local_source_snapshot, 'materialize', return_value=self.base / 'copy'), \
             patch.object(runner, 'DockerCLI', return_value=Engine()), \
             patch.object(runner, '_check_go_toolchain', return_value='go-executable'), \
             patch.object(runner.ScheduledRun, 'start_host', side_effect=start), \
             patch.object(runner.local_go_dependencies, 'acquire', side_effect=acquire) as acquisition, \
             patch.object(runner.local_go_execution, 'transfer_source', return_value='a' * 64), \
             patch.object(runner.local_go_execution, 'transfer_dependencies', return_value={'status': 'incomplete'}), \
             patch.object(runner.local_go_execution, 'discover_packages', return_value=inventory), \
             patch.object(runner.local_go_execution, 'run_package_diagnostic', return_value=diagnostic) as tests:
            result = runner.run(self.root, self.source, ['chat', 'voice'], self.base / 'out', 'go-executable')
        self.assertEqual(result['status'], 'incomplete')
        self.assertEqual(set(result['services']), {'chat', 'voice'})
        self.assertTrue(all(item.closed for item in self.scheduled))
        self.assertEqual([call.args[3] for call in acquisition.call_args_list], ['src/backend/chat', 'src/backend/voice'])
        self.assertEqual(tests.call_count, 2)
        self.assertEqual(tests.call_args.args[7], ['example.test/module', 'example.test/module/empty'])
        self.assertEqual(result['services']['chat']['diagnostic']['tests']['skipped'], [['example.test/module', 'TestOptIn']])
        self.assertTrue((self.base / 'out/diagnostic.json').is_file())

    def test_invalid_service_rejected_before_start(self):
        with patch.object(runner.ScheduledRun, 'start_host') as start:
            with self.assertRaisesRegex(ValueError, 'services'):
                runner.run(self.root, self.source, ['../escape'], self.base / 'out')
        start.assert_not_called()

    def test_missing_docker_reports_the_required_image(self):
        with patch.object(runner.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(runner.local_source_snapshot, 'materialize', return_value=self.base / 'copy'), \
             patch.object(runner, 'DockerCLI', side_effect=runner.OwnershipError('unavailable')):
            with self.assertRaisesRegex(runner.OwnershipError, 'ensure Docker is reachable and pull') as caught:
                runner.run(self.root, self.source, ['chat'], self.base / 'out')
        self.assertIn(runner.GO_IMAGE, str(caught.exception))

    def test_mismatched_go_toolchain_fails_before_acquisition(self):
        with patch.object(runner.shutil, 'which', return_value='go'), \
             patch.object(runner.subprocess, 'run', return_value=SimpleNamespace(
                 returncode=0, stdout=b'go version go1.26.2 windows/amd64\n')):
            with self.assertRaisesRegex(ValueError, 'go1.26.10'):
                runner._check_go_toolchain('go')

    def test_acquisition_failure_closes_the_owned_run(self):
        class Owner:
            def create_resource(self, kind): return kind
            def create_container(self, *args, **kwargs): return 'container'
            def start_container(self, name): pass
        class Scheduled:
            run = Owner()
            closed = False
            def close(self): self.closed = True
        scheduled = Scheduled()
        with patch.object(runner.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(runner.local_source_snapshot, 'materialize', return_value=self.base / 'copy'), \
             patch.object(runner, 'DockerCLI') as engine, \
             patch.object(runner, '_check_go_toolchain', return_value='go'), \
             patch.object(runner.ScheduledRun, 'start_host', return_value=scheduled), \
             patch.object(runner.local_go_dependencies, 'acquire', side_effect=ValueError('download failed')):
            engine.return_value._call.return_value = 'sha256:' + 'a' * 64
            with self.assertRaisesRegex(ValueError, 'download failed'):
                runner.run(self.root, self.source, ['chat'], self.base / 'out')
        self.assertTrue(scheduled.closed)


if __name__ == '__main__':
    unittest.main()
