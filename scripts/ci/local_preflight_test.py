"""Behavioral tests for the planner-driven local preflight foundation."""
import tempfile
import subprocess
import hashlib
import json
import os
import shutil
import unittest
from pathlib import Path
from unittest.mock import patch

import local_preflight


class HandoffGateTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / 'repo'
        self.root.mkdir()
        self.output = Path(self.temp.name) / 'proof'
        self.base, self.head = 'a' * 40, 'b' * 40
        self.source = {'head': self.head, 'tree': 'c' * 40, 'changed_paths': [], 'raw_mismatch_paths': []}
        self.plan = {'source': self.source, 'checks': {'current-diff-contract': ['every pull request']}}

    def run_gate(self, plans=None, refs=None):
        contract = {'base': self.base, 'event_base': self.base, 'pr_head': self.head}
        with patch.object(local_preflight.local_current_diff, '_windows', return_value=True), \
             patch.object(local_preflight, '_handoff_refs', side_effect=refs or [None, None]), \
             patch.object(local_preflight.affected_check_plan, 'build_local_plan', side_effect=plans or [self.plan, self.plan]), \
             patch.object(local_preflight.local_current_diff, 'run_merge_candidate', return_value={'source': self.source, 'contract': contract}) as run:
            result = local_preflight.run_handoff(self.root, self.base, self.head, 'feature/docs', self.output)
            return result, run

    def test_exact_single_check_is_executed_before_acceptance(self):
        result, run = self.run_gate()
        self.assertEqual(result['status'], 'complete')
        run.assert_called_once()
        self.assertEqual(json.loads((self.output / 'handoff.json').read_text())['source'], self.source)

    def test_extra_selected_check_fails_closed(self):
        plan = dict(self.plan, checks={'current-diff-contract': [], 'backend-auth': []})
        with self.assertRaisesRegex(ValueError, 'unsupported selected checks'):
            self.run_gate(plans=[plan])

    def test_changed_plan_or_refs_are_rejected(self):
        with self.assertRaisesRegex(ValueError, 'changed during'):
            self.run_gate(plans=[self.plan, dict(self.plan, checks={'current-diff-contract': [], 'admin': []})])

    def test_dirty_source_rejected(self):
        source = dict(self.source, changed_paths=['uncommitted.md'])
        with self.assertRaisesRegex(ValueError, 'clean committed source'):
            self.run_gate(plans=[dict(self.plan, source=source)])

    def test_overlay_plan_executes_and_verifies_owned_evidence(self):
        plan = dict(self.plan, checks={'current-diff-contract': [], 'match-squad-overlay': []})
        contract = {'base': self.base, 'event_base': self.base, 'pr_head': self.head}
        image = local_preflight.local_overlay_execution.PYTHON_IMAGE
        with patch.object(local_preflight.local_current_diff, '_windows', return_value=True), \
             patch.object(local_preflight, '_handoff_refs'), \
             patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_current_diff, 'run_merge_candidate', return_value={'source': self.source, 'contract': contract}), \
             patch.object(local_preflight.local_overlay_execution, 'run_supervised', return_value={'diagnostic': 'value'}) as run, \
             patch.object(local_preflight, '_verify_overlay_evidence') as verify:
            report = local_preflight.run_handoff(self.root, self.base, self.head, 'feature/overlay', self.output,
                                                  overlay_image_id=image)
        run.assert_called_once()
        verify.assert_called_once_with(self.output / 'overlay', self.source, image, {'diagnostic': 'value'})
        self.assertEqual(set(report['checks']), {'current-diff-contract', 'match-squad-overlay'})
        self.assertEqual(report['status'], 'complete')

    def test_overlay_plan_requires_exact_image_before_execution(self):
        plan = dict(self.plan, checks={'current-diff-contract': [], 'match-squad-overlay': []})
        with patch.object(local_preflight.local_current_diff, '_windows', return_value=True), \
             patch.object(local_preflight, '_handoff_refs'), \
             patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_current_diff, 'run_merge_candidate') as current, \
             self.assertRaisesRegex(ValueError, 'pinned overlay image'):
            local_preflight.run_handoff(self.root, self.base, self.head, 'feature/overlay', self.output)
        current.assert_not_called()


class OverlayHandoffEvidenceTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.output = Path(self.temp.name) / 'overlay'; self.output.mkdir()
        self.slot_dir = Path(self.temp.name) / 'slots'; self.slot_dir.mkdir()
        (self.slot_dir / 'heavy-0.lock').write_bytes(b'\0idle\n')
        self.source = {'head': 'a' * 40}
        self.image = local_preflight.local_overlay_execution.PYTHON_IMAGE
        self.stdout, self.stderr = b'', b'Ran 2 tests in 0.100s\n\nOK\n'
        (self.output / 'tests.stdout').write_bytes(self.stdout)
        (self.output / 'tests.stderr').write_bytes(self.stderr)
        (self.output / 'discovery.stdout').write_bytes(b'VOICE_OVERLAY_DISCOVERED=2\n')
        (self.output / 'source.tar').write_bytes(b'source')
        directory = self.output / 'run/tmp/ci-local' / ('voiceci-' + 'f' * 32)
        directory.mkdir(parents=True)
        self.report = {'schema': 'voice-local-overlay-diagnostic-v1', 'status': 'incomplete',
                       'source': self.source, 'image_id': self.image, 'version': 'Python 3.12.1',
                       'modules': list(local_preflight.local_overlay_execution.MODULES), 'slot': 0,
                       'tests': {'discovered': 2, 'passed': 2, 'skipped': []},
                       'stdout_sha256': hashlib.sha256(self.stdout).hexdigest(),
                       'stderr_sha256': hashlib.sha256(self.stderr).hexdigest(),
                       'source_archive_sha256': hashlib.sha256(b'source').hexdigest(),
                       'ownership_journal': str(Path('run/tmp/ci-local') / directory.name / 'ownership.json')}
        self.owner = unittest.mock.Mock()
        self.owner.state = {'resources': [{'kind': kind, 'state': 'removed', 'id': str(i)}
                                           for i, kind in enumerate(('network', 'volume', 'container', 'container'))]}
        self.engine = unittest.mock.Mock(); self.engine.exists.return_value = False

    def verify(self):
        with patch.object(local_preflight, 'DockerCLI', return_value=self.engine), \
             patch.object(local_preflight.OwnedRun, 'load', return_value=self.owner), \
             patch.object(local_preflight.local_heavy_slots, 'host_slot_directory', return_value=self.slot_dir):
            return local_preflight._verify_overlay_evidence(self.output, self.source, self.image, self.report)

    def test_exact_logs_archive_and_absent_owned_resources(self):
        self.verify()
        self.assertEqual(self.engine.exists.call_count, 4)
        self.owner._check_daemon.assert_called_once()
        self.owner.close.assert_called_once()

    def test_tampered_logs_resources_or_slot_fail_closed(self):
        (self.output / 'tests.stderr').write_bytes(b'Ran 2 tests in 0.100s\n\nOK (skipped=1)\n')
        with self.assertRaises(ValueError): self.verify()
        (self.output / 'tests.stderr').write_bytes(self.stderr)
        self.engine.exists.return_value = True
        with self.assertRaisesRegex(ValueError, 'resources'): self.verify()
        self.engine.exists.return_value = False
        (self.slot_dir / 'heavy-0.lock').write_bytes(b'\0busy:stale\n')
        with self.assertRaisesRegex(ValueError, 'slot'): self.verify()

    def test_boolean_pass_count_is_not_integer_evidence(self):
        self.report['tests'] = {'discovered': 1, 'passed': True, 'skipped': []}
        with self.assertRaisesRegex(ValueError, 'inventory'): self.verify()


class HandoffRefsTest(unittest.TestCase):
    def test_actual_tracking_refs_and_checkout_must_match(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            def git(*args):
                return subprocess.check_output(['git', '-C', str(root), *args], stderr=subprocess.PIPE).decode().strip()
            git('init', '-q')
            git('config', 'user.name', 'Test')
            git('config', 'user.email', 'test@example.invalid')
            (root / 'file').write_text('base')
            git('add', 'file')
            git('commit', '-qm', 'base')
            base = git('rev-parse', 'HEAD')
            (root / 'file').write_text('head')
            git('add', 'file')
            git('commit', '-qm', 'head')
            head = git('rev-parse', 'HEAD')
            git('update-ref', 'refs/remotes/origin/master', base)
            git('update-ref', 'refs/remotes/origin/feature/docs', head)
            local_preflight._handoff_refs(root, base, head, 'feature/docs')
            git('update-ref', 'refs/remotes/origin/master', head)
            with self.assertRaisesRegex(ValueError, 'refs differ'):
                local_preflight._handoff_refs(root, base, head, 'feature/docs')

    def test_docs_only_plan_executes_real_merge_contract(self):
        if os.name != 'nt':
            with tempfile.TemporaryDirectory() as directory:
                with self.assertRaisesRegex(ValueError, 'Windows descendant supervision'):
                    local_preflight.run_handoff(directory, 'a' * 40, 'b' * 40, 'feature/docs', Path(directory).parent / 'unused-proof')
            return
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'repo'
            root.mkdir()
            project = Path(__file__).resolve().parents[2]
            for name in ('.github/ci/path-filters.yml', 'scripts/ci/resolve-go-matrix.sh',
                         'scripts/ci/resolve-go-docker-matrix.py', 'scripts/ci/current-diff-contract.py'):
                target = root / name
                target.parent.mkdir(parents=True, exist_ok=True)
                shutil.copyfile(project / name, target)
            def git(*args):
                return subprocess.check_output(['git', '-C', str(root), *args], stderr=subprocess.PIPE).decode().strip()
            git('init', '-q')
            git('config', 'user.name', 'Test')
            git('config', 'user.email', 'test@example.invalid')
            git('config', 'core.autocrlf', 'false')
            docs = root / 'docs/DATA_MODEL.md'
            docs.parent.mkdir()
            docs.write_bytes(b'original\n')
            git('add', '.')
            git('commit', '-qm', 'base')
            base = git('rev-parse', 'HEAD')
            docs.write_bytes(b'updated\n')
            git('add', '.')
            git('commit', '-qm', 'documentation change')
            head = git('rev-parse', 'HEAD')
            git('update-ref', 'refs/remotes/origin/master', base)
            git('update-ref', 'refs/remotes/origin/feature/docs', head)
            report = local_preflight.run_handoff(root, base, head, 'feature/docs', Path(directory) / 'proof')
            self.assertEqual(report['status'], 'complete')
            self.assertEqual(set(report['checks']), {'current-diff-contract'})


class LocalPreflightTests(unittest.TestCase):
    def test_overlay_opt_in_preserves_incomplete_and_hosted_check(self):
        plan = {'source': self.source, 'checks': {'match-squad-overlay': ['overlay']}}
        inputs = {'image_id': local_preflight.local_overlay_execution.PYTHON_IMAGE}
        diagnostic = {'schema': 'voice-local-overlay-diagnostic-v1', 'status': 'incomplete',
                      'source': self.source, 'image_id': inputs['image_id'], 'version': 'Python 3.12.1',
                      'slot': 0, 'ownership_journal': 'run/tmp/ci-local/voiceci-test/ownership.json',
                      'modules': list(local_preflight.local_overlay_execution.MODULES),
                      'tests': {'discovered': 2, 'passed': 2, 'skipped': []},
                      'stdout_sha256': 'b' * 64, 'stderr_sha256': 'c' * 64,
                      'source_archive_sha256': 'd' * 64, 'unbound_inputs': ['image provenance']}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_overlay_execution, 'run_supervised', return_value=diagnostic):
            result = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter',
                                         head_ref='codex/test', overlay_inputs=inputs)
        self.assertEqual(result['status'], 'incomplete')
        self.assertEqual(result['checks']['match-squad-overlay']['status'], 'not_run')
        self.assertEqual(result['overlay_report'], 'overlay/diagnostic.json')

    def test_overlay_unselected_or_partial_refuses_before_worker(self):
        plan = {'source': self.source, 'checks': {}}
        inputs = {'image_id': local_preflight.local_overlay_execution.PYTHON_IMAGE}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_overlay_execution, 'run_supervised') as run:
            result = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter',
                                         overlay_inputs=inputs)
        self.assertEqual(result['status'], 'failed')
        run.assert_not_called()

    def test_lint_opt_in_preserves_diagnostic_status_and_inventory(self):
        plan = {'source': self.source, 'checks': {'golangci': ['go']},
                'components': {'go': ['chat']}}
        inputs = {'go': str(self.root / 'go'), 'go_sha256': 'a' * 64,
                  'lint': str(self.root / 'lint'), 'lint_sha256': 'b' * 64}
        commands = [{'action': action, 'module': module, 'exit_code': 0,
                     'stdout_sha256': 'c' * 64, 'stderr_sha256': 'd' * 64}
                    for action in ('resolve', 'lint') for module in ('pkg', 'chat')]
        diagnostic = {'schema': 'voice-local-golangci-diagnostic-v1', 'status': 'incomplete',
                      'source': self.source, 'modules': ['pkg', 'chat'], 'commands': commands,
                      'tools': {'go_sha256': 'a' * 64, 'golangci_sha256': 'b' * 64},
                      'platform': {'goos': 'windows', 'goarch': 'amd64'}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_chat_orchestration, '_verify_file'), \
             patch.object(local_preflight.local_golangci_execution, 'run', return_value=diagnostic):
            result = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter',
                                         lint_inputs=inputs)
        self.assertEqual(result['status'], 'incomplete')
        self.assertEqual(result['checks']['golangci']['status'], 'not_run')
        self.assertEqual(result['golangci_report'], 'golangci/diagnostic.json')

    def test_lint_opt_in_refuses_unselected_before_execution(self):
        plan = {'source': self.source, 'checks': {}, 'components': {'go': ['chat']}}
        inputs = {'go': str(self.root / 'go'), 'go_sha256': 'a' * 64,
                  'lint': str(self.root / 'lint'), 'lint_sha256': 'b' * 64}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_golangci_execution, 'run') as run:
            result = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter',
                                         lint_inputs=inputs)
        self.assertEqual(result['status'], 'failed')
        run.assert_not_called()

    def full_go_inputs(self, services):
        return {name: {'helper': str(self.root / 'helper'), 'helper_sha256': 'a' * 64,
                       'archives': {tag: {'archive': str(self.root / (name + str(i))),
                                           'archive_sha256': 'b' * 64, 'image_id': 'sha256:' + str(i + 1) * 64}
                                    for i, tag in enumerate(local_preflight.local_go_full_integration.profile(name)['images'])}}
                for name in services}

    def test_full_go_preflight_rejects_mixed_selection_before_workers(self):
        plan = {'base': 'c' * 40, 'source': self.source, 'checks': {'backend-go-integration-pr': ['bot', 'file']},
                'components': {'go_integration': ['bot', 'file']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_go_full_integration, 'run_supervised') as run:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter',
                                         head_ref='codex/test', full_go_inputs=self.full_go_inputs(['bot']))
        self.assertEqual(report['status'], 'failed')
        run.assert_not_called()

    def test_full_go_preflight_preserves_skips_and_incomplete_status(self):
        plan = {'base': 'c' * 40, 'source': self.source, 'checks': {'backend-go-integration-pr': ['bot']},
                'components': {'go_integration': ['bot']}}
        result = {'schema': 'voice-local-bot-full-integration-v1', 'status': 'incomplete', 'source': self.source,
                  'exit_code': 0, 'tests': {'passed': 3, 'skipped': [['voice/backend/bot', 'TestOptional']],
                                             'packages': ['voice/backend/bot']},
                  'inventory': {'packages_with_tests': ['voice/backend/bot'], 'packages_without_tests': []},
                  'images': {tag: item['image_id'] for tag, item in self.full_go_inputs(['bot'])['bot']['archives'].items()},
                  'input_identity': {'helper_sha256': 'a' * 64,
                      'archives': {tag: {'archive_sha256': item['archive_sha256'], 'image_id': item['image_id']}
                                   for tag, item in self.full_go_inputs(['bot'])['bot']['archives'].items()}},
                  'events_sha256': 'b' * 64, 'source_archive_sha256': 'c' * 64, 'required_inventory_sha256': 'd' * 64,
                  'required_count': 1, 'go_image_id': 'sha256:' + 'e' * 64, 'slot': 0,
                  'ownership_journal': str(self.output / 'go-full' / 'bot' / 'run' / 'tmp' / 'ci-local' / ('voiceci-' + 'f' * 32) / 'ownership.json'),
                  'unbound_inputs': ['external provenance']}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_chat_orchestration, '_verify_file'), \
             patch.object(local_preflight.local_go_full_integration, 'run_supervised', return_value=result) as run:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter',
                                         head_ref='codex/test', full_go_inputs=self.full_go_inputs(['bot']))
        run.assert_called_once()
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['checks']['backend-go-integration-pr']['status'], 'not_run')
        self.assertEqual(report['go_full_reports']['bot']['skipped'], result['tests']['skipped'])
        for bad in (dict(result, status='pass'), dict(result, source={'head': 'wrong'}),
                    dict(result, images={}), dict(result, tests={**result['tests'], 'skipped': 'hidden'})):
            with self.subTest(bad=bad), self.assertRaises(ValueError):
                local_preflight._validate_full_go_report(bad, 'bot', self.source, self.full_go_inputs(['bot'])['bot'],
                                                         self.output / 'go-full' / 'bot')
    def full_auth_inputs(self):
        return {'helper': str(self.root / 'helper'), 'helper_sha256': 'a' * 64,
                'client_image_id': 'sha256:' + 'b' * 64, 'maven_cache': str(self.root / 'cache'),
                'maven_cache_sha256': 'c' * 64, 'images': str(self.root / 'images.json'), 'images_sha256': 'd' * 64}

    def full_auth_images(self):
        return {tag: {'archive': str(self.root / str(index)), 'sha256': 'e' * 64,
                      'image_id': 'sha256:' + str(index + 1) * 64}
                for index, tag in enumerate(local_preflight.local_auth_full_orchestration.IMAGE_TAGS)}

    def full_auth_result(self):
        values, images = self.full_auth_inputs(), self.full_auth_images()
        return {'schema': 'voice-local-auth-full-orchestration-v1', 'status': 'incomplete', 'slot': 0,
                'ownership_journal': 'run/tmp/ci-local/voiceci-fixture/ownership.json',
                'inputs': {'creator_sha256': values['helper_sha256'], 'client_image_id': values['client_image_id'],
                           'maven_cache_sha256': values['maven_cache_sha256'],
                           'make_tool': {'image_id': local_preflight.local_auth_full_orchestration.MAKE_IMAGE,
                                         'sha256': 'a' * 64, 'version': 'GNU Make 4.3'},
                           'images': {tag: {'image_id': item['image_id'], 'archive_sha256': item['sha256']}
                                      for tag, item in images.items()}},
                'diagnostic': {'schema': 'voice-local-auth-full-diagnostic-v1', 'status': 'incomplete',
                               'source': self.source, 'exit_code': 0, 'source_archive_sha256': '1' * 64,
                               'build_archive_sha256': '2' * 64, 'maven_cache_sha256': values['maven_cache_sha256'],
                               'reports': {'tests': 100, 'suite_count': 30, 'required_suite_count': 28,
                                           'skipped': [], 'reports_sha256': '3' * 64}}}

    def test_full_auth_opt_in_never_promotes_preflight(self):
        plan = {'source': self.source, 'checks': {'backend-auth': ['auth']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_chat_orchestration, '_verify_file'), \
             patch.object(local_preflight.local_auth_full_orchestration, 'read_images', return_value=self.full_auth_images()), \
             patch.object(local_preflight.local_auth_full_orchestration, 'run', return_value=self.full_auth_result()) as run:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter',
                                         full_auth_inputs=self.full_auth_inputs())
        run.assert_called_once()
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['checks']['backend-auth']['status'], 'not_run')
        self.assertEqual(report['auth_full_report'], 'auth-full/diagnostic.json')

    def test_full_auth_rejects_missing_or_mismatched_make_identity(self):
        for mutation in ('missing', 'image', 'digest', 'version'):
            report = self.full_auth_result()
            tool = report['inputs']['make_tool']
            if mutation == 'missing': report['inputs'].pop('make_tool')
            if mutation == 'image': tool['image_id'] = 'sha256:' + '0' * 64
            if mutation == 'digest': tool['sha256'] = 'wrong'
            if mutation == 'version': tool['version'] = 'unknown'
            with self.subTest(mutation=mutation), self.assertRaises(ValueError):
                local_preflight._validate_full_auth_report(report, self.source, self.full_auth_inputs(), self.full_auth_images())

    def test_full_auth_invalid_or_unselected_refuses_execution(self):
        selected = {'source': self.source, 'checks': {'backend-auth': ['auth']}}
        for kind in ('partial', 'unselected', 'source', 'image', 'skip-format'):
            with self.subTest(kind=kind), tempfile.TemporaryDirectory() as directory:
                result, inputs = self.full_auth_result(), self.full_auth_inputs()
                plan = selected if kind != 'unselected' else {'source': self.source, 'checks': {}}
                if kind == 'partial': inputs.pop('images_sha256')
                if kind == 'source': result['diagnostic']['source'] = {'head': 'other'}
                if kind == 'image': result['inputs']['client_image_id'] = 'sha256:' + 'f' * 64
                if kind == 'skip-format': result['diagnostic']['reports']['skipped'] = 'unknown'
                with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
                     patch.object(local_preflight.local_chat_orchestration, '_verify_file'), \
                     patch.object(local_preflight.local_auth_full_orchestration, 'read_images', return_value=self.full_auth_images()), \
                     patch.object(local_preflight.local_auth_full_orchestration, 'run', return_value=result) as run:
                    report = local_preflight.run(self.root, 'c' * 40, self.source['head'], Path(directory) / 'out',
                                                 'flutter', full_auth_inputs=inputs)
                self.assertEqual(report['status'], 'failed')
                if kind in ('partial', 'unselected'): run.assert_not_called()

    def test_compose_opt_in_preserves_planned_not_run_checks(self):
        plan = {'source': self.source, 'components': {}, 'checks': {'compose-e2e': ['test.dart'],
                                                                    'compose-config': ['docker-compose.yml']}}
        diagnostic = {'schema': 'voice-local-compose-bridge-v1', 'status': 'incomplete',
                      'selected_checks': ['compose-config', 'compose-e2e'], 'fixture_report': 'fixture/diagnostic.json'}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_compose_bridge, 'run', return_value=diagnostic) as bridge:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter',
                                         compose_inputs=self.chat_inputs())
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['compose_private_report'], 'compose-private/bridge.json')
        self.assertEqual(report['checks'], {'compose-e2e': {'status': 'not_run'}, 'compose-config': {'status': 'not_run'}})
        bridge.assert_called_once_with(plan, self.chat_inputs(), self.output / 'compose-private')

    def test_compose_opt_out_or_failure_never_qualifies_checks(self):
        plan = {'source': self.source, 'components': {}, 'checks': {'compose-config': ['docker-compose.yml']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_compose_bridge, 'run') as bridge:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        bridge.assert_not_called()
        self.assertEqual(report['checks']['compose-config']['status'], 'not_run')
        for result in ({'status': 'incomplete'}, {'schema': 'voice-local-compose-bridge-v1', 'status': 'pass',
                       'selected_checks': ['compose-config'], 'fixture_report': 'fixture/diagnostic.json'}):
            with self.subTest(result=result), tempfile.TemporaryDirectory() as temp, \
                 patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
                 patch.object(local_preflight.local_compose_bridge, 'run', return_value=result):
                report = local_preflight.run(self.root, 'c' * 40, self.source['head'], Path(temp) / 'out', 'flutter',
                                             compose_inputs=self.chat_inputs())
            self.assertEqual(report['status'], 'failed')
            self.assertEqual(report['checks']['compose-config']['status'], 'not_run')

    def test_compose_unselected_and_partial_input_fail_before_launch(self):
        for checks, inputs in (({'backend-auth': ['auth']}, self.chat_inputs()),
                               ({'compose-e2e': ['test.dart']}, {'helper': 'partial'})):
            with self.subTest(checks=checks), tempfile.TemporaryDirectory() as temp, \
                 patch.object(local_preflight.affected_check_plan, 'build_local_plan',
                              return_value={'source': self.source, 'components': {}, 'checks': checks}), \
                 patch.object(local_preflight.local_compose_bridge.supervision, 'run_diagnostic') as broker:
                report = local_preflight.run(self.root, 'c' * 40, self.source['head'], Path(temp) / 'out', 'flutter',
                                             compose_inputs=inputs)
            self.assertEqual(report['status'], 'failed')
            broker.assert_not_called()

    def auth_inputs(self):
        return {'helper': str(self.root / 'creator.exe'), 'helper_sha256': 'a' * 64,
                'postgres_archive': str(self.root / 'postgres.tar'), 'postgres_archive_sha256': 'b' * 64,
                'postgres_image_id': 'sha256:' + 'c' * 64, 'maven_image_id': 'sha256:' + 'd' * 64,
                'maven_cache': str(self.root / 'maven.tar'), 'maven_cache_sha256': 'e' * 64}

    def auth_result(self):
        values = self.auth_inputs()
        return {'schema': 'voice-local-auth-private-diagnostic-v1', 'status': 'incomplete',
                'diagnostic': {'status': 'incomplete', 'source': self.source, 'report_sha256': 'f' * 64,
                               'maven_cache_sha256': values['maven_cache_sha256'],
                               'source_archive_sha256': '1' * 64, 'build_archive_sha256': '2' * 64},
                'inputs': {'creator_sha256': values['helper_sha256'], 'postgres_image_id': values['postgres_image_id'],
                           'postgres_archive_sha256': values['postgres_archive_sha256'],
                           'maven_image_id': values['maven_image_id']},
                'ownership_journal': 'run/tmp/ci-local/voiceci-fixture/ownership.json', 'slot': 0}

    def test_auth_bridge_is_opt_in_selected_and_never_satisfies_check(self):
        plan = {'source': self.source, 'checks': {'backend-auth': ['auth changed']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_chat_orchestration, '_verify_file'), \
             patch.object(local_preflight.local_auth_orchestration, 'run', return_value=self.auth_result()) as run:
            result = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output,
                                         'flutter', auth_inputs=self.auth_inputs())
        self.assertEqual(result['status'], 'incomplete')
        self.assertEqual(result['checks']['backend-auth']['status'], 'not_run')
        self.assertEqual(result['auth_private_report'], 'auth-private/diagnostic.json')
        values = self.auth_inputs()
        run.assert_called_once_with(self.root, self.source['head'], self.output / 'auth-private',
            values['helper'], values['helper_sha256'], values['postgres_archive'], values['postgres_archive_sha256'],
            values['postgres_image_id'], values['maven_image_id'], values['maven_cache'])

    def test_auth_unselected_partial_or_mismatched_never_qualifies(self):
        selected = {'source': self.source, 'checks': {'backend-auth': ['auth']}}
        cases = [('not-selected', {'source': self.source, 'checks': {}}, self.auth_inputs()),
                 ('partial', selected, {'helper': 'partial'}), ('source', selected, self.auth_inputs()),
                 ('cache', selected, self.auth_inputs()), ('journal', selected, self.auth_inputs())]
        for mutation, plan, inputs in cases:
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as tmp:
                result = self.auth_result()
                if mutation == 'source': result['diagnostic']['source'] = {'head': 'different'}
                if mutation == 'cache': result['diagnostic']['maven_cache_sha256'] = 'f' * 64
                if mutation == 'journal': result['ownership_journal'] = '../outside/ownership.json'
                with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
                     patch.object(local_preflight.local_chat_orchestration, '_verify_file'), \
                     patch.object(local_preflight.local_auth_orchestration, 'run', return_value=result) as run:
                    report = local_preflight.run(self.root, 'c' * 40, self.source['head'], Path(tmp) / 'out',
                                                 'flutter', auth_inputs=inputs)
                self.assertEqual(report['status'], 'failed')
                if mutation in ('not-selected', 'partial'): run.assert_not_called()

    def test_selected_auth_without_opt_in_never_runs(self):
        plan = {'source': self.source, 'checks': {'backend-auth': ['auth']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_auth_orchestration, 'run') as run:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        run.assert_not_called()
        self.assertEqual(report['checks']['backend-auth']['status'], 'not_run')

    def test_auth_cache_change_or_cleanup_uncertainty_fails_closed(self):
        plan = {'source': self.source, 'checks': {'backend-auth': ['auth']}}
        for mutation in ('cache', 'cleanup'):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as tmp:
                verify = [None, ValueError('cache changed')] if mutation == 'cache' else None
                failure = local_preflight.OwnershipError('cleanup uncertain') if mutation == 'cleanup' else None
                with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
                     patch.object(local_preflight.local_chat_orchestration, '_verify_file', side_effect=verify), \
                     patch.object(local_preflight.local_auth_orchestration, 'run', return_value=self.auth_result(),
                                  side_effect=failure):
                    report = local_preflight.run(self.root, 'c' * 40, self.source['head'], Path(tmp) / 'out',
                                                 'flutter', auth_inputs=self.auth_inputs())
                self.assertEqual(report['status'], 'failed')
                self.assertEqual(report['checks']['backend-auth']['status'], 'not_run')
                self.assertNotIn('auth_private_report', report)

    def chat_inputs(self):
        return {'helper': str(self.root / 'creator.exe'), 'helper_sha256': 'a' * 64,
                'postgres_archive': str(self.root / 'postgres.tar'), 'postgres_archive_sha256': 'b' * 64,
                'postgres_image_id': 'sha256:' + 'c' * 64}

    def chat_result(self):
        inputs = self.chat_inputs()
        return {'schema': 'voice-local-chat-private-diagnostic-v1', 'status': 'incomplete', 'source': self.source,
                'diagnostic': {'status': 'incomplete', 'source': self.source, 'exit_code': 0,
                               'events_sha256': 'd' * 64,
                               'tests': {'packages': ['voice/backend/chat/internal/store'], 'passed': 1, 'skipped': []}},
                'inputs': {'creator_sha256': inputs['helper_sha256'], 'postgres_archive_sha256': inputs['postgres_archive_sha256'],
                           'postgres_image_id': inputs['postgres_image_id']},
                'ownership_journal': 'run/tmp/ci-local/voiceci-fixture/ownership.json', 'slot': 0}

    def test_opt_in_chat_runs_only_when_affected_and_cannot_satisfy_matrix(self):
        plan = {'source': self.source, 'components': {'go_integration': ['chat']},
                'checks': {'backend-go-integration-pr': ['chat']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_chat_orchestration, 'run', return_value=self.chat_result()) as run:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter',
                                         chat_inputs=self.chat_inputs())
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['checks']['backend-go-integration-pr']['status'], 'not_run')
        self.assertEqual(report['chat_private_report'], 'chat-private/diagnostic.json')
        values = self.chat_inputs()
        run.assert_called_once_with(self.root, self.source['head'], self.output / 'chat-private',
                                    values['helper'], values['helper_sha256'], values['postgres_archive'],
                                    values['postgres_archive_sha256'], values['postgres_image_id'], 'go')

    def test_selected_chat_does_not_run_without_explicit_inputs(self):
        plan = {'source': self.source, 'components': {'go_integration': ['chat']},
                'checks': {'backend-go-integration-pr': ['chat']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_chat_orchestration, 'run') as run:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        run.assert_not_called()
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['checks']['backend-go-integration-pr']['status'], 'not_run')

    def test_chat_inputs_refuse_unselected_or_malformed_before_execution(self):
        for plan, inputs in (({'source': self.source, 'components': {'go_integration': ['bot']},
                               'checks': {'backend-go-integration-pr': ['bot']}}, self.chat_inputs()),
                             ({'source': self.source, 'components': {'go_integration': ['chat']},
                               'checks': {'backend-go-integration-pr': ['chat']}}, {'helper': 'partial'})):
            with self.subTest(inputs=inputs), tempfile.TemporaryDirectory() as tmp:
                output = Path(tmp) / 'result'
                with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
                     patch.object(local_preflight.local_chat_orchestration, 'run') as run:
                    report = local_preflight.run(self.root, 'c' * 40, self.source['head'], output, 'flutter', chat_inputs=inputs)
                self.assertEqual(report['status'], 'failed')
                run.assert_not_called()

    def test_chat_mismatch_skip_or_cleanup_uncertainty_cannot_qualify(self):
        plan = {'source': self.source, 'components': {'go_integration': ['chat']},
                'checks': {'backend-go-integration-pr': ['chat']}}
        for mutation in ('source', 'skip', 'image', 'cleanup'):
            with self.subTest(mutation=mutation), tempfile.TemporaryDirectory() as tmp:
                result = self.chat_result()
                if mutation == 'source': result['source'] = {'head': 'different'}
                if mutation == 'skip': result['diagnostic']['tests']['skipped'] = ['required']
                if mutation == 'image': result['inputs']['postgres_image_id'] = 'sha256:' + 'd' * 64
                failure = local_preflight.OwnershipError('cleanup uncertain') if mutation == 'cleanup' else None
                with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
                     patch.object(local_preflight.local_chat_orchestration, 'run', side_effect=failure, return_value=result):
                    report = local_preflight.run(self.root, 'c' * 40, self.source['head'], Path(tmp) / 'result',
                                                 'flutter', chat_inputs=self.chat_inputs())
                self.assertEqual(report['status'], 'failed')
                self.assertEqual(report['checks']['backend-go-integration-pr']['status'], 'not_run')

    def test_chat_combined_execution_and_cleanup_failure_is_not_swallowed(self):
        plan = {'source': self.source, 'components': {'go_integration': ['chat']},
                'checks': {'backend-go-integration-pr': ['chat']}}
        failure = BaseExceptionGroup('uncertain', [KeyboardInterrupt(), local_preflight.OwnershipError('cleanup')])
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_chat_orchestration, 'run', side_effect=failure):
            with self.assertRaises(BaseExceptionGroup):
                local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter',
                                    chat_inputs=self.chat_inputs())
        self.assertFalse((self.output / 'preflight.json').exists())

    def test_ci_routing_subset_does_not_satisfy_script_matrix(self):
        plan = {'source': self.source, 'checks': {'ci-script-tests': ['CI change']}}
        diagnostic = {'schema': 'voice-local-ci-routing-diagnostic-v1', 'status': 'incomplete',
                      'source': self.source, 'commands': [{'name': 'gate-shell', 'tests': None},
                                                          {'name': 'gate', 'tests': 8}, {'name': 'router', 'tests': 4}]}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_ci_routing, 'run', return_value=diagnostic) as route:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        route.assert_called_once_with(self.root, self.source, self.output / 'ci-routing')
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['checks']['ci-script-tests']['status'], 'not_run')
        self.assertEqual(report['ci_routing_report'], 'ci-routing/diagnostic.json')

    def test_ci_routing_supervision_refusal_fails_closed(self):
        plan = {'source': self.source, 'checks': {'ci-script-tests': ['CI change']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_ci_routing, 'run', side_effect=local_preflight.SupervisionError('unsupported')):
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        self.assertEqual(report['status'], 'failed')
        self.assertEqual(report['checks']['ci-script-tests']['status'], 'not_run')

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / 'checkout'
        self.root.mkdir()
        self.output = Path(self.temp.name) / 'report'
        self.source = {'head': 'a' * 40, 'tree': 'b' * 40}
        self.plan = {'source': self.source, 'checks': {'flutter': ['changed frontend'],
                     'flutter-unit-tests': ['changed frontend'], 'backend-auth': ['shared contract']}}

    def run_with(self, plan, flutter_result=None, final_source=None):
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=final_source or self.source), \
             patch.object(local_preflight.local_flutter_execution, 'run', return_value=flutter_result) as flutter:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        return report, flutter

    def test_selected_flutter_runs_but_unimplemented_checks_never_pass(self):
        result = {'status': 'incomplete', 'source': self.source, 'full_inventory': True,
                  'tests': {'skipped': [], 'passed': 1}}
        report, flutter = self.run_with(self.plan, result)
        flutter.assert_called_once()
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['checks']['backend-auth']['status'], 'not_run')
        self.assertEqual(report['checks']['flutter-unit-tests']['status'], 'diagnostic')
        self.assertTrue((self.output / 'preflight.json').is_file())

    def test_unselected_flutter_does_not_run(self):
        report, flutter = self.run_with({'source': self.source, 'checks': {'backend-auth': ['auth']}})
        flutter.assert_not_called()
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['checks']['backend-auth']['status'], 'not_run')

    def test_selected_current_diff_is_source_bound_and_stays_diagnostic(self):
        plan = {'source': self.source, 'checks': {'current-diff-contract': ['every pull request']}}
        diagnostic = {'status': 'diagnostic', 'source': self.source, 'base': 'c' * 40}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_current_diff, 'run', return_value=diagnostic) as current:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        current.assert_called_once_with(self.root, self.source, 'c' * 40, self.output / 'current-diff')
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['checks']['current-diff-contract']['status'], 'diagnostic')

    def test_current_diff_failure_or_wrong_base_fails_preflight(self):
        plan = {'source': self.source, 'checks': {'current-diff-contract': ['every pull request']}}
        diagnostic = {'status': 'diagnostic', 'source': self.source, 'base': 'd' * 40}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_current_diff, 'run', return_value=diagnostic):
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        self.assertEqual(report['status'], 'failed')
        self.assertEqual(report['checks']['current-diff-contract']['status'], 'not_run')

    def test_current_diff_refusal_fails_preflight(self):
        plan = {'source': self.source, 'checks': {'current-diff-contract': ['every pull request']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_current_diff, 'run', side_effect=ValueError('contract refused')):
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        self.assertEqual(report['status'], 'failed')
        self.assertIn('contract refused', report['error'])

    def test_selected_protobuf_runs_but_cannot_pass_preflight(self):
        plan = {'source': self.source, 'checks': {'protobuf': ['contract changed']}}
        diagnostic = {'status': 'incomplete', 'source': self.source, 'base': 'c' * 40,
                      'commands': [{'name': name, 'exit_code': 0} for name in
                                   ('buf-lint', 'buf-format', 'buf-breaking', 'go-codegen-selftest',
                                    'go-codegen', 'auth-proto-sync', 'auth-compile', 'r23-contract')],
                      'r23': {'passed': 1, 'skipped': []}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_proto_execution, 'run', return_value=diagnostic) as proto:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output,
                                         'flutter', 'feature/example', 'go-executable', 'buf-executable')
        proto.assert_called_once_with(self.root, self.source, 'c' * 40, self.output / 'protobuf',
                                      'go-executable', 'buf-executable')
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['checks']['protobuf']['status'], 'diagnostic')

    def test_failed_or_mismatched_protobuf_diagnostic_fails_preflight(self):
        plan = {'source': self.source, 'checks': {'protobuf': ['contract changed']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_proto_execution, 'run', return_value={'status': 'failed'}):
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        self.assertEqual(report['status'], 'failed')
        self.assertEqual(report['checks']['protobuf']['status'], 'not_run')

    def test_selected_go_services_run_without_claiming_pkg_or_preflight_pass(self):
        plan = {'source': self.source, 'components': {'go': ['chat', 'voice']},
                'checks': {'backend-go': ['affected service'], 'backend-go-pkg': ['shared package']}}
        def service_result(name):
            return {'acquisition': {'status': 'incomplete', 'source': self.source,
                                    'module_dir': 'src/backend/' + name, 'archive_sha256': 'f' * 64},
                    'transfer': {'status': 'incomplete', 'archive_sha256': 'f' * 64},
                    'source_archive_sha256': 'e' * 64, 'slot': 0,
                    'ownership_journal': name + '/tmp/ci-local/voiceci-fixture/ownership.json',
                    'inventory': {'status': 'incomplete', 'packages_with_tests': ['voice/backend/' + name],
                                  'packages_without_tests': []},
                    'diagnostic': {'status': 'incomplete', 'exit_code': 0,
                                   'tests': {'packages': ['voice/backend/' + name], 'passed': 1, 'skipped': []}}}
        diagnostic = {'status': 'incomplete', 'source': self.source,
                      'image': {'reference': local_preflight.local_go_plan.GO_IMAGE, 'id': 'sha256:' + 'a' * 64},
                      'services': {'chat': service_result('chat'), 'voice': service_result('voice')}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source), \
             patch.object(local_preflight.local_go_plan, 'run', return_value=diagnostic) as go:
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output,
                                         'flutter', 'feature/example', 'go-executable')
        go.assert_called_once_with(self.root, self.source, ['chat', 'voice'], self.output / 'go', 'go-executable')
        self.assertEqual(report['status'], 'incomplete')
        self.assertEqual(report['checks']['backend-go']['status'], 'diagnostic')
        self.assertEqual(report['checks']['backend-go-pkg']['status'], 'not_run')

    def test_go_diagnostic_must_cover_every_planned_service(self):
        plan = {'source': self.source, 'components': {'go': ['chat', 'voice']},
                'checks': {'backend-go': ['affected service']}}
        diagnostic = {'status': 'incomplete', 'source': self.source, 'services': {'chat': {}}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_go_plan, 'run', return_value=diagnostic):
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        self.assertEqual(report['status'], 'failed')
        self.assertIn('did not match', report['error'])

    def test_missing_docker_fails_selected_go_preflight(self):
        plan = {'source': self.source, 'components': {'go': ['chat']},
                'checks': {'backend-go': ['affected service']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan), \
             patch.object(local_preflight.local_go_plan, 'run',
                          side_effect=local_preflight.OwnershipError('Docker unavailable')):
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        self.assertEqual(report['status'], 'failed')
        self.assertEqual(report['checks']['backend-go']['status'], 'not_run')
        self.assertIn('Docker unavailable', report['error'])

    def test_source_change_invalidates_run(self):
        result = {'status': 'incomplete', 'source': self.source, 'full_inventory': True,
                  'tests': {'skipped': [], 'passed': 1}}
        report, _ = self.run_with(self.plan, result, {'head': 'a' * 40, 'tree': 'd' * 40})
        self.assertEqual(report['status'], 'failed')
        self.assertIn('source changed', report['error'])

    def test_failed_flutter_diagnostic_cannot_succeed(self):
        result = {'status': 'failed', 'source': self.source, 'error': 'required test skipped'}
        report, _ = self.run_with(self.plan, result)
        self.assertEqual(report['status'], 'failed')

    def test_flutter_diagnostic_must_match_planned_source(self):
        result = {'status': 'incomplete', 'source': {'head': 'different'}, 'full_inventory': True,
                  'tests': {'skipped': [], 'passed': 1}}
        report, _ = self.run_with(self.plan, result)
        self.assertEqual(report['status'], 'failed')
        self.assertIn('does not match', report['error'])

    def test_flutter_skips_cannot_be_diagnostic_success(self):
        result = {'status': 'incomplete', 'source': self.source, 'full_inventory': True,
                  'tests': {'skipped': ['example'], 'passed': 1}}
        report, _ = self.run_with(self.plan, result)
        self.assertEqual(report['status'], 'failed')

    def test_missing_test_counts_are_not_a_completed_diagnostic(self):
        result = {'status': 'incomplete', 'source': self.source, 'full_inventory': True,
                  'tests': {'skipped': []}}
        report, _ = self.run_with(self.plan, result)
        self.assertEqual(report['status'], 'failed')

    def test_output_in_checkout_is_rejected(self):
        with self.assertRaises(ValueError):
            local_preflight.run(self.root, 'c' * 40, self.source['head'], self.root / 'report', 'flutter')

    def test_planner_failure_is_recorded_as_failed_preflight(self):
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan',
                          side_effect=subprocess.CalledProcessError(1, ['bash'], stderr='jq is required\n')):
            report = local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter')
        self.assertEqual(report['status'], 'failed')
        self.assertIsNone(report['plan'])
        self.assertIn('jq is required', report['error'])
        self.assertEqual(json.loads((self.output / 'preflight.json').read_text(encoding='utf-8')), report)

    def test_head_ref_is_forwarded_to_shared_planner(self):
        plan = {'source': self.source, 'checks': {'voice-history-audit': ['release branch']}}
        with patch.object(local_preflight.affected_check_plan, 'build_local_plan', return_value=plan) as planner, \
             patch.object(local_preflight.local_source_snapshot, 'capture', return_value=self.source):
            local_preflight.run(self.root, 'c' * 40, self.source['head'], self.output, 'flutter', 'codex/voice-master-release-x')
        planner.assert_called_once_with(self.root, 'c' * 40, self.source['head'], 'codex/voice-master-release-x')

    def test_real_git_source_and_planner_preserve_dirty_identity(self):
        def git(*args):
            return subprocess.check_output(['git', '-C', str(self.root), *args], stderr=subprocess.PIPE)
        git('init', '-q')
        git('config', 'user.name', 'Test')
        git('config', 'user.email', 'test@example.invalid')
        (self.root / '.github/ci').mkdir(parents=True)
        (self.root / 'scripts/ci').mkdir(parents=True)
        (self.root / 'scripts/ci/current-diff-contract.py').write_bytes(
            (Path(__file__).parent / 'current-diff-contract.py').read_bytes())
        (self.root / '.github/ci/path-filters.yml').write_text('code:\n  - "src/**"\ndocs:\n  - "docs/**"\n')
        (self.root / 'docs').mkdir()
        (self.root / 'docs/readme.md').write_text('before\n')
        git('add', '.')
        git('commit', '-qm', 'base')
        head = git('rev-parse', 'HEAD').decode().strip()
        (self.root / 'docs/readme.md').write_text('after\n')
        matrix = {'go_services': [], 'docker_go_services': [], 'integration_go_services': [],
                  'run_go': False, 'run_pkg': False}
        with patch.object(local_preflight.affected_check_plan, 'resolve_go_matrix', return_value=matrix), \
             patch.object(local_preflight.local_flutter_execution, 'run') as flutter:
            report = local_preflight.run(self.root, head, head, self.output, 'flutter')
        flutter.assert_not_called()
        self.assertEqual(report['status'], 'incomplete')
        self.assertIn('docs/readme.md', report['plan']['changed_paths'])
        self.assertNotEqual(report['plan']['source']['tree'], git('rev-parse', 'HEAD^{tree}').decode().strip())

    def test_planner_and_flutter_runner_share_real_git_snapshot(self):
        def git(*args):
            return subprocess.check_output(['git', '-C', str(self.root), *args], stderr=subprocess.PIPE)
        git('init', '-q')
        git('config', 'user.name', 'Test')
        git('config', 'user.email', 'test@example.invalid')
        (self.root / '.github/ci').mkdir(parents=True)
        (self.root / 'scripts/ci').mkdir(parents=True)
        (self.root / 'scripts/ci/current-diff-contract.py').write_bytes(
            (Path(__file__).parent / 'current-diff-contract.py').read_bytes())
        (self.root / '.github/ci/path-filters.yml').write_text('code:\n  - "src/**"\nfrontend:\n  - "src/frontend/**"\n')
        test_dir = self.root / 'src/frontend/test'
        test_dir.mkdir(parents=True)
        (test_dir / 'example_test.dart').write_text('before\n')
        git('add', '.')
        git('commit', '-qm', 'base')
        head = git('rev-parse', 'HEAD').decode().strip()
        (test_dir / 'example_test.dart').write_text('after\n')
        matrix = {'go_services': [], 'docker_go_services': [], 'integration_go_services': [],
                  'run_go': False, 'run_pkg': False}
        def invoke(command, cwd=None, stdout=None, **kwargs):
            if '--version' in command:
                return subprocess.CompletedProcess(command, 0, stdout=b'{"frameworkRevision":"fixture"}')
            if 'test' in command:
                path = str(Path(cwd) / 'test/example_test.dart')
                events = [{'type': 'start'}, {'type': 'suite', 'suite': {'id': 1, 'path': path}},
                          {'type': 'allSuites', 'count': 1},
                          {'type': 'testStart', 'test': {'id': 2, 'suiteID': 1, 'name': 'example', 'metadata': {'skip': False}}},
                          {'type': 'testDone', 'testID': 2, 'result': 'success', 'skipped': False, 'hidden': False},
                          {'type': 'done', 'success': True}]
                stdout.write(('\n'.join(json.dumps(event) for event in events) + '\n').encode())
            return subprocess.CompletedProcess(command, 0)
        with patch.object(local_preflight.affected_check_plan, 'resolve_go_matrix', return_value=matrix), \
             patch.object(local_preflight.local_flutter_execution, '_invoke', side_effect=invoke):
            report = local_preflight.run(self.root, head, head, self.output, 'flutter')
        diagnostic = json.loads((self.output / 'flutter/diagnostic.json').read_text(encoding='utf-8'))
        self.assertEqual(diagnostic['source'], report['plan']['source'])
        self.assertEqual(diagnostic['tests']['passed'], 1)
        self.assertEqual(report['checks']['flutter-unit-tests']['status'], 'diagnostic')
        self.assertEqual(report['status'], 'incomplete')


if __name__ == '__main__':
    unittest.main()
