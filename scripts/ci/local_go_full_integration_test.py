"""Only reviewed service profiles may run full private Go diagnostics."""
import json
import io
from types import SimpleNamespace
from pathlib import Path
import subprocess
import tempfile
import sys
import unittest
from unittest import mock

import local_go_full_integration as subject


class FullGoTests(unittest.TestCase):
    def setUp(self):
        patcher = mock.patch.object(subject.auth_broker, '_require_worker_job')
        patcher.start()
        self.addCleanup(patcher.stop)
    def test_worker_refuses_before_reading_request_when_job_is_unavailable(self):
        with mock.patch.object(sys, 'argv', ['worker', '--worker']), \
             mock.patch.object(subject.auth_broker, '_require_worker_job', side_effect=ValueError('no job')), \
             mock.patch.object(subject, 'run_selected') as run, self.assertRaisesRegex(ValueError, 'no job'):
            subject.main()
        run.assert_not_called()
        with mock.patch.object(subject.auth_broker, '_require_worker_job', side_effect=ValueError('no job')), \
             mock.patch.object(subject.chat, 'run_full') as full, self.assertRaisesRegex(ValueError, 'no job'):
            subject.run_profile('bot', mock.Mock(), 'daemon', 'client', Path('.'), {}, Path('.'), 'x', {}, Path('out'))
        full.assert_not_called()
        with mock.patch.object(subject.auth_broker, '_require_worker_job', side_effect=ValueError('no job')), \
             mock.patch.object(subject.chat.local_chat_private, 'inspect_pair') as inspect, self.assertRaisesRegex(ValueError, 'no job'):
            subject.chat.run_full(mock.Mock(), 'daemon', 'client', Path('.'), {}, Path('.'), 'x', {}, Path('out'))
        inspect.assert_not_called()
        scheduled = mock.Mock()
        with mock.patch.object(subject.auth_broker, '_require_worker_job', side_effect=ValueError('no job')), \
             self.assertRaisesRegex(ValueError, 'no job'):
            subject.chat.run_owned(scheduled, mock.Mock(), {}, Path('.'), Path('.'), Path('out'), 'helper', 'x', {}, {})
        scheduled.run.create_resource.assert_not_called()

    def test_worker_binds_explicit_helper_and_archive_identities(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory)
            request = {'plan': {}, 'service': 'bot', 'root': str(output), 'output': str(output),
                       'helper': str(output / 'helper'), 'helper_digest': 'a' * 64,
                       'archives': {'postgres:16-bookworm': {'archive': str(output / 'image'),
                                    'archive_sha256': 'b' * 64, 'image_id': 'sha256:' + 'c' * 64}},
                       'go_binary': 'go', 'base': 'd' * 40, 'head_ref': 'codex/test'}
            with mock.patch.object(sys, 'argv', ['worker', '--worker']), \
                 mock.patch.object(sys, 'stdin', SimpleNamespace(buffer=io.BytesIO(json.dumps(request).encode()))), \
                 mock.patch.object(sys, 'stdout', new_callable=io.StringIO), \
                 mock.patch.object(subject.auth_broker, '_require_worker_job'), \
                 mock.patch.object(subject, 'run_selected', return_value={'status': 'incomplete'}):
                self.assertEqual(subject.main(), 0)
            report = json.loads((output / 'diagnostic.json').read_text(encoding='utf-8'))
            self.assertEqual(report['input_identity'], {'helper_sha256': 'a' * 64, 'archives': {
                'postgres:16-bookworm': {'archive_sha256': 'b' * 64, 'image_id': 'sha256:' + 'c' * 64}}})

    def test_supervised_worker_accepts_only_matching_saved_incomplete_report(self):
        plan = {'source': {'head': 'a' * 40}}
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'out'; output.mkdir()
            report = {'status': 'incomplete', 'source': plan['source']}
            (output / 'diagnostic.json').write_text(json.dumps(report), encoding='utf-8')
            fake = subprocess.CompletedProcess([], 0, stdout=json.dumps(report).encode(), stderr=b'')
            with mock.patch.object(subject.supervision, 'run_diagnostic', return_value=fake):
                self.assertEqual(subject.run_supervised(plan, 'bot', Path(directory), output, 'helper', 'c' * 64, {},
                                                        base='b' * 40, head_ref='codex/test'), report)
                (output / 'diagnostic.json').write_text('{}', encoding='utf-8')
                with self.assertRaises(ValueError):
                    subject.run_supervised(plan, 'bot', Path(directory), output, 'helper', 'c' * 64, {},
                                           base='b' * 40, head_ref='codex/test')
    def test_supervised_worker_cannot_run_inline_or_accept_forged_receipt(self):
        plan = {'base': 'b' * 40, 'source': {'head': 'a' * 40},
                'checks': {'backend-go-integration-pr': ['bot']}, 'components': {'go_integration': ['bot']}}
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'out'
            fake = subprocess.CompletedProcess([], 0, stdout=b'{"status":"pass"}', stderr=b'')
            with mock.patch.object(subject.supervision, 'run_diagnostic', return_value=fake) as broker, \
                 mock.patch.object(subject, 'run_selected') as inline, self.assertRaises(ValueError):
                subject.run_supervised(plan, 'bot', Path(directory), output, 'helper', 'c' * 64, {},
                                       base='b' * 40, head_ref='codex/test')
            broker.assert_called_once()
            inline.assert_not_called()
            request = json.loads(broker.call_args.kwargs['input'])
            self.assertEqual(request['plan'], plan)
            self.assertEqual(request['service'], 'bot')
    def test_profiles_are_explicit_and_unsupported_services_refuse(self):
        self.assertEqual(subject.profile('bot')['images'], ('postgres:16-bookworm', 'nats:2.10-alpine'))
        self.assertEqual(subject.profile('search')['images'], ('postgres:16-bookworm',))
        self.assertEqual(subject.profile('chat')['images'], subject.chat.IMAGES)
        for name in ('gateway', 'file', 'voice', '../bot', 'unknown'):
            with self.subTest(name=name), self.assertRaisesRegex(ValueError, 'unsupported'):
                subject.profile(name)

    def test_inventory_is_source_bound_to_selected_service(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            path = root / 'scripts/ci/go_integration_required.json'
            path.parent.mkdir(parents=True)
            package = 'voice/backend/bot/internal/store'
            path.write_text(json.dumps({'bot': [[package, 'TestRequired']]}), encoding='utf-8')
            self.assertEqual(subject.read_required(root, 'bot', [package]), [[package, 'TestRequired']])
            for inventory in ({'bot': []}, {'bot': [[package, 'TestRequired'], [package, 'TestRequired']]},
                              {'bot': [['voice/backend/chat/internal/store', 'TestRequired']]}):
                path.write_text(json.dumps(inventory), encoding='utf-8')
                with self.subTest(inventory=inventory), self.assertRaises(ValueError):
                    subject.read_required(root, 'bot', [package])

    def test_chat_dispatch_keeps_existing_executor(self):
        with mock.patch.object(subject.auth_broker, '_require_worker_job'), \
             mock.patch.object(subject.chat, 'run', return_value={'status': 'incomplete'}) as run:
            result = subject.run('chat', Path('.'), 'a' * 40, Path('out'), 'helper', 'b' * 64, {}, 'go')
        self.assertEqual(result['status'], 'incomplete')
        run.assert_called_once_with(Path('.'), 'a' * 40, Path('out'), 'helper', 'b' * 64, {}, 'go')

    def test_unsupported_selection_refuses_before_any_docker(self):
        with mock.patch.object(subject, 'DockerCLI') as engine, self.assertRaisesRegex(ValueError, 'unsupported'):
            subject.run('file', Path('.'), 'a' * 40, Path('out'), 'helper', 'b' * 64, {}, 'go')
        engine.assert_not_called()

    def test_bot_full_command_uses_canonical_package_set_and_serial_limit(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'
            root.mkdir()
            copy = Path(directory) / 'copy'
            inventory_path = copy / 'scripts/ci/go_integration_required.json'
            inventory_path.parent.mkdir(parents=True)
            package = 'voice/backend/bot/internal/store'
            inventory_path.write_text(json.dumps({'bot': [[package, 'TestRequired']]}), encoding='utf-8')
            events = [dict(Package=package, Action='start'), dict(Package=package, Test='TestRequired', Action='run'),
                      dict(Package=package, Test='TestRequired', Action='pass'), dict(Package=package, Action='pass')]
            owner = mock.Mock()
            def execute(client, command, out, err, timeout):
                self.assertIn('cd /workspace/source/src/backend/bot', command[2])
                self.assertIn('go test -json -count=1 -timeout 20m -p 1 -parallel 1 ./...', command[2])
                self.assertNotIn('-short', command[2])
                out.write(b''.join((json.dumps(event) + '\n').encode() for event in events))
                return 0
            owner.execute.side_effect = execute
            source = {'head': 'a' * 40}
            inventory = {'packages_with_tests': [package], 'packages_without_tests': []}
            with mock.patch.object(subject.chat.local_chat_private, 'inspect_pair', return_value='private'), \
                 mock.patch.object(subject.chat, 'verify_images') as images, \
                 mock.patch.object(subject.chat.local_go_execution, 'discover_packages', return_value=inventory), \
                 mock.patch.object(subject.chat.local_go_execution, '_verify_modules'), \
                 mock.patch.object(subject.chat.local_go_execution, '_verify_workspace'), \
                 mock.patch.object(subject.chat.snapshot, 'verify_materialized'), \
                 mock.patch.object(subject.chat.snapshot, 'capture', return_value=source):
                result = subject.run_profile('bot', owner, 'daemon', 'client', root, source, copy, 'b' * 64,
                                         {tag: 'sha256:' + 'c' * 64 for tag in subject.BOT_IMAGES},
                                         Path(directory) / 'output')
            self.assertEqual(result['schema'], 'voice-local-bot-full-integration-v1')
            self.assertEqual(result['status'], 'incomplete')
            self.assertEqual(result['required_count'], 1)
            self.assertEqual(images.call_args.args[3], subject.BOT_IMAGES)

    def test_shared_provisioner_rejects_unreviewed_service_before_resources(self):
        scheduled = mock.Mock()
        with self.assertRaisesRegex(ValueError, 'unsupported'):
            subject.chat.run_owned(scheduled, mock.Mock(), {}, Path('.'), Path('.'), Path('.'),
                                   'helper', 'a' * 64, {}, {}, service='file')
        scheduled.run.create_resource.assert_not_called()
        scheduled.close.assert_called_once_with()

    def test_search_profile_uses_only_search_required_inventory(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            path = root / 'scripts/ci/go_integration_required.json'
            path.parent.mkdir(parents=True)
            package = 'voice/backend/search/internal/store'
            path.write_text(json.dumps({'search': [[package, 'TestSearch']]}), encoding='utf-8')
            self.assertEqual(subject.read_required(root, 'search', [package]), [[package, 'TestSearch']])
            with mock.patch.object(subject.chat, 'run_full', return_value={'status': 'incomplete'}) as run:
                subject.run_profile('search', 'owner', 'daemon', 'client', root, {}, root, 'a' * 64, {}, root / 'out')
            self.assertEqual(run.call_args.kwargs, {'service': 'search'})

    def test_repository_profile_inventories_are_nonempty_and_service_scoped(self):
        root = Path(__file__).resolve().parents[2]
        inventory = json.loads((root / 'scripts/ci/go_integration_required.json').read_text(encoding='utf-8'))
        for service in ('bot', 'search'):
            packages = sorted({item[0] for item in inventory[service]})
            with self.subTest(service=service):
                required = subject.read_required(root, service, packages)
                self.assertEqual(required, inventory[service])
                self.assertTrue(required)

    def test_planner_selection_is_required_before_execution(self):
        source = {'head': 'a' * 40}
        plan = {'source': source, 'checks': {'backend-go-integration-pr': ['bot']},
                'components': {'go_integration': ['bot']}, 'base': 'c' * 40}
        with mock.patch.object(subject.snapshot, 'capture', return_value=source), \
             mock.patch.object(subject.affected_check_plan, 'build_local_plan', return_value=plan) as planner, \
             mock.patch.object(subject.auth_broker, '_require_worker_job'), \
             mock.patch.object(subject, 'run', return_value={'status': 'incomplete', 'source': source}) as run:
            self.assertEqual(subject.run_selected(plan, 'bot', Path('.'), Path('out'), 'helper', 'b' * 64, {},
                                                  base='c' * 40, head_ref='codex/example'),
                             {'status': 'incomplete', 'source': source})
        planner.assert_called_once_with(Path('.').resolve(), 'c' * 40, source['head'], 'codex/example')
        run.assert_called_once_with('bot', Path('.'), source['head'], Path('out'), 'helper', 'b' * 64, {}, 'go')
        for invalid in ({**plan, 'checks': {}}, {**plan, 'components': {'go_integration': ['chat']}},
                        {**plan, 'source': {'head': 'bad'}}):
            with self.subTest(invalid=invalid), mock.patch.object(subject, 'run') as run, self.assertRaises(ValueError):
                subject.run_selected(invalid, 'bot', Path('.'), Path('out'), 'helper', 'b' * 64, {},
                                     base='c' * 40, head_ref='codex/example')
            run.assert_not_called()
        with mock.patch.object(subject.affected_check_plan, 'build_local_plan', return_value=plan), \
             mock.patch.object(subject.snapshot, 'capture', return_value={'head': 'different'}), \
             mock.patch.object(subject, 'run') as run, self.assertRaisesRegex(ValueError, 'source'):
            subject.run_selected(plan, 'bot', Path('.'), Path('out'), 'helper', 'b' * 64, {},
                                 base='c' * 40, head_ref='codex/example')
        run.assert_not_called()

        with mock.patch.object(subject.affected_check_plan, 'build_local_plan', return_value={**plan, 'checks': {}}), \
             mock.patch.object(subject, 'run') as run, self.assertRaisesRegex(ValueError, 'plan differs'):
            subject.run_selected(plan, 'bot', Path('.'), Path('out'), 'helper', 'b' * 64, {},
                                 base='c' * 40, head_ref='codex/example')
        run.assert_not_called()

    def test_direct_execution_refuses_unfenced_before_acquisition(self):
        source = {'head': 'a' * 40}
        plan = {'base': 'c' * 40, 'source': source, 'checks': {'backend-go-integration-pr': ['chat']},
                'components': {'go_integration': ['chat']}}
        with mock.patch.object(subject.auth_broker, '_require_worker_job', side_effect=ValueError('no job')), \
             mock.patch.object(subject.chat, 'run') as chat_run, \
             mock.patch.object(subject, 'DockerCLI') as docker, self.assertRaisesRegex(ValueError, 'no job'):
            subject.run('chat', Path('.'), source['head'], Path('out'), 'helper', 'b' * 64, {})
        chat_run.assert_not_called()
        docker.assert_not_called()
        with mock.patch.object(subject.auth_broker, '_require_worker_job', side_effect=ValueError('no job')), \
             mock.patch.object(subject.affected_check_plan, 'build_local_plan', return_value=plan), \
             mock.patch.object(subject.snapshot, 'capture', return_value=source), \
             mock.patch.object(subject, 'run') as run, self.assertRaisesRegex(ValueError, 'no job'):
            subject.run_selected(plan, 'chat', Path('.'), Path('out'), 'helper', 'b' * 64, {},
                                 base='c' * 40, head_ref='codex/test')
        run.assert_not_called()

    def test_actual_local_planner_refuses_forged_selection(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            def git(*args):
                return subprocess.check_output(['git', '-C', str(root), *args], text=True).strip()
            git('init', '-q')
            git('config', 'user.name', 'Test')
            git('config', 'user.email', 'test@example.invalid')
            (root / '.github/ci').mkdir(parents=True)
            (root / '.github/ci/path-filters.yml').write_text('code:\n  - "**"\nsvc_bot:\n  - src/backend/bot/**\n')
            git('add', '.')
            git('commit', '-qm', 'base')
            base = git('rev-parse', 'HEAD')
            (root / 'src/backend/bot').mkdir(parents=True)
            (root / 'src/backend/bot/main.go').write_text('package main\n')
            git('add', '.')
            git('commit', '-qm', 'head')
            head = git('rev-parse', 'HEAD')
            matrix = {'go_services': ['bot'], 'docker_go_services': [],
                      'integration_go_services': ['bot'], 'run_go': True, 'run_pkg': False}
            with mock.patch.object(subject.affected_check_plan, 'resolve_go_matrix', return_value=matrix):
                plan = subject.affected_check_plan.build_local_plan(root, base, head, 'codex/example')
                forged = dict(plan, checks=dict(plan['checks'], forged=['x']))
                with mock.patch.object(subject, 'run') as run, self.assertRaisesRegex(ValueError, 'plan differs'):
                    subject.run_selected(forged, 'bot', root, root.parent / 'out', 'helper', 'b' * 64, {},
                                         base=base, head_ref='codex/example')
                run.assert_not_called()


if __name__ == '__main__':
    unittest.main()
