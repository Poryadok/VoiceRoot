"""Exercise the public shell CLI; fixtures never execute Go or Flutter."""
from pathlib import Path
import os
import json
import subprocess
import tempfile
import unittest

import yaml


ROOT = Path(__file__).resolve().parents[2]
SCRIPT = ROOT / 'scripts/ci/e2e-manifest.sh'


class ManifestTest(unittest.TestCase):
    def test_compose_flutter_json_is_not_contaminated_by_dependency_resolution(self):
        script = (ROOT / 'scripts/ci/compose-e2e-smoke.sh').read_text(encoding='utf-8')
        self.assertLess(script.index('flutter pub get'), script.index('flutter test --no-pub --reporter json'))

    def test_changed_live_flutter_suites_are_selected_in_manifest_order(self):
        import e2e_manifest
        with tempfile.TemporaryDirectory() as directory:
            manifest = Path(directory) / 'features.yml'
            manifest.write_text('full_flutter: [test/a.dart, test/b.dart, test/c.dart]\na1_flutter_profile_handoff: [test/a1_live_test.dart]\n', encoding='utf-8')
            self.assertEqual(e2e_manifest.affected_flutter(manifest, [
                'src/frontend/test/c.dart', 'src/frontend/test/a.dart',
                'src/frontend/test/unrelated.dart', 'src/frontend/test/c.dart',
            ]), ['test/a.dart', 'test/c.dart'])
            self.assertEqual(e2e_manifest.affected_flutter(manifest, ['docs/README.md']), [])
            self.assertEqual(e2e_manifest.affected_flutter(manifest, ['src/frontend/test/a1_live_test.dart']), [])
            root = Path(directory)
            new_suite = root / 'src/frontend/test/new_live_test.dart'
            new_suite.parent.mkdir(parents=True)
            new_suite.write_text('void main() {}', encoding='utf-8')
            with self.assertRaises(ValueError):
                e2e_manifest.affected_flutter(manifest, ['src/frontend/test/new_live_test.dart'], root)
            new_suite.unlink()
            self.assertEqual(e2e_manifest.affected_flutter(manifest, ['src/frontend/test/new_live_test.dart'], root), [])
            self.assertEqual(e2e_manifest.affected_flutter(manifest, [
                'src/frontend/test/new_live_test.dart', 'src/frontend/test/c.dart'], root), ['test/c.dart'])

    def test_affected_flutter_cli_rejects_invalid_inputs_without_partial_output(self):
        with tempfile.TemporaryDirectory() as directory:
            manifest = Path(directory) / 'features.yml'
            manifest.write_text('full_flutter: [test/a.dart, test/b.dart]\na1_flutter_profile_handoff: [test/a1_live_test.dart]\n', encoding='utf-8')
            for value, expected, output in [
                (json.dumps(['src/frontend/test/b.dart']), 0, 'test/b.dart\n'),
                ('[]', 0, ''), ('[1]', 1, ''), ('{', 1, ''), ('{}', 1, ''),
                (json.dumps(['src/frontend/test/accessibility_settings_capture_test.dart']), 0, ''),
                (json.dumps(['src/frontend/test/search_timeout_e2e_live_test.dart']), 1, ''),
            ]:
                env = dict(os.environ, VOICE_AFFECTED_PATHS_JSON=value)
                result = subprocess.run(['bash', str(SCRIPT), str(manifest), 'affected_flutter'],
                                        env=env, text=True, capture_output=True, check=False)
                with self.subTest(value=value):
                    self.assertEqual(result.returncode, expected, result.stderr)
                    self.assertEqual(result.stdout, output)

    def parse(self, text, section='smoke_gateway'):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'features.yml'
            path.write_text(text, encoding='utf-8')
            return subprocess.run(['bash', str(SCRIPT), str(path), section],
                                  text=True, capture_output=True, check=False)

    def test_quoted_yaml_and_comments_decode_to_exact_names(self):
        result = self.parse('smoke_gateway:\n  - "TestFirst" # comment\n  - \'TestSecond\'\n')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, 'TestFirst\nTestSecond\n')

    def test_flow_yaml_preserves_order(self):
        result = self.parse('smoke_gateway: [TestSecond, TestFirst]\n')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, 'TestSecond\nTestFirst\n')

    def test_full_gateway_scalar_is_decoded_by_same_entrypoint(self):
        result = self.parse("full_gateway_run: 'TestCompose.*_live'\n", 'full_gateway_run')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout, 'TestCompose.*_live\n')

    def test_invalid_selection_never_emits_partial_stdout(self):
        for text in [
            'smoke_gateway: [TestFirst, 42]\n',
            'smoke_gateway:\n  - TestFirst\n  - 42\n',
            'smoke_gateway:\n  - TestFirst\n  - {name: TestSecond}\n',
            'smoke_gateway: TestFirst\n',
            'smoke_gateway: []\n',
            'smoke_gateway: null\n',
            'smoke_gateway: [TestFirst, TestFirst]\n',
            'smoke_gateway: [TestFirst]\nsmoke_gateway: [TestSecond]\n',
            'smoke_gateway:\n  - TestFirst\nsmoke_gateway:\n  - TestSecond\n',
            'smoke_gateway: [TestFirst,\n',
            'smoke_flutter: [test/first.dart]\n',
            'smoke_gateway: ["TestFirst\\nTestSecond"]\n',
        ]:
            with self.subTest(text=text):
                result = self.parse(text)
                self.assertNotEqual(result.returncode, 0)
                self.assertEqual(result.stdout, '')

    def test_current_catalog_sections_still_have_expected_order(self):
        manifest = (ROOT / '.github/ci/e2e-features.yml').read_text(encoding='utf-8')
        result = self.parse(manifest, 'a1_flutter_profile_handoff')
        self.assertEqual(result.returncode, 0, result.stderr)
        self.assertEqual(result.stdout.splitlines(), [
            'test/t055_profile_switch_reconnect_inbox_e2e_live_test.dart',
            'test/t106_account_soft_delete_e2e_live_test.dart',
            'test/t107_folders_quick_access_e2e_live_test.dart',
            'test/a1_pending_email_reentry_e2e_live_test.dart',
        ])

    def test_every_ci_catalog_consumer_installs_pinned_dependency(self):
        expected = {
            'ci.yml': ['compose-e2e', 'a1-e2e', 'a1-flutter-profile-handoff', 'a1-attachment-restart-proof'],
            'compose-e2e-live.yml': ['compose-e2e-live'],
        }
        requirement = (ROOT / 'scripts/ci/requirements-e2e.txt').read_text().strip()
        self.assertEqual(requirement, 'PyYAML==6.0.3')
        for workflow, jobs in expected.items():
            document = yaml.safe_load((ROOT / '.github/workflows' / workflow).read_text())
            for job in jobs:
                with self.subTest(workflow=workflow, job=job):
                    steps = document['jobs'][job]['steps']
                    installs = [i for i, step in enumerate(steps)
                                if 'pip install -r scripts/ci/requirements-e2e.txt' in step.get('run', '')]
                    self.assertEqual(len(installs), 1)
                    self.assertTrue(any(step.get('uses', '').startswith('actions/setup-python@')
                                        for step in steps[:installs[0]]))
                    self.assertTrue(any('compose-e2e' in step.get('run', '') or
                                        'make compose-a1' in step.get('run', '') or
                                        'make compose-file-attachment' in step.get('run', '')
                                        for step in steps[installs[0] + 1:]))

    def test_compose_smoke_uses_one_project_for_startup_and_live_reads(self):
        workflow = yaml.safe_load((ROOT / '.github/workflows/ci.yml').read_text())
        job = workflow['jobs']['compose-e2e']
        project = job.get('env', {}).get('COMPOSE_PROJECT_NAME', '')
        self.assertTrue(project, 'Compose startup and live reads need a job-wide project binding')
        names = []
        for run_id, attempt in [('101', '1'), ('102', '1'), ('101', '2')]:
            resolved = project.replace('${{ github.run_id }}', run_id).replace(
                '${{ github.run_attempt }}', attempt)
            self.assertRegex(resolved, r'^[a-z0-9][a-z0-9_-]*$')
            names.append(resolved)
        self.assertEqual(len(set(names)), len(names), 'Each run and retry needs a distinct disposable project')
        commands = ('docker compose', 'make compose-migrate-all', 'compose-e2e-smoke.sh')
        stages = [step for step in job['steps']
                  if any(command in step.get('run', '') for command in commands)]
        self.assertTrue(stages)
        for step in stages:
            with self.subTest(step=step.get('name')):
                self.assertEqual(step.get('env', {}).get('COMPOSE_PROJECT_NAME', project), project)

    def test_runner_parser_failure_precedes_any_external_work(self):
        for runner in ['compose-e2e-smoke.sh', 'compose-e2e-live.sh']:
            with self.subTest(runner=runner), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                scripts = root / 'scripts/ci'
                scripts.mkdir(parents=True)
                (scripts / runner).write_text((SCRIPT.parent / runner).read_text(encoding='utf-8'), encoding='utf-8')
                (scripts / 'e2e-manifest.sh').write_text('#!/bin/bash\nprintf "TestFirst\\n"\nexit 19\n')
                (scripts / 'compose-fcm-diagnostic-lib.sh').write_text('')
                (scripts / 'compose-fcm-diagnostic-lib-test.sh').write_text('exit 0\n')
                (root / '.github/ci').mkdir(parents=True)
                (root / '.github/ci/e2e-features.yml').write_text("full_gateway_run: 'TestFirst'\n")
                for path in ['src/backend/gateway', 'src/backend/notification', 'src/frontend', 'bin']:
                    (root / path).mkdir(parents=True)
                marker = root / 'unexpected-work'
                for command in ['go', 'flutter', 'docker', 'sudo']:
                    path = root / 'bin' / command
                    path.write_text(f'#!/bin/bash\nprintf work >> "{marker}"\nexit 0\n')
                    path.chmod(0o755)
                pkg = root / 'bin/pkg-config'
                pkg.write_text('#!/bin/bash\nexit 0\n')
                pkg.chmod(0o755)
                env = dict(os.environ, PATH=str(root / 'bin') + os.pathsep + os.environ['PATH'])
                result = subprocess.run(['bash', str(scripts / runner)], env=env,
                                        text=True, capture_output=True, check=False)
                self.assertNotEqual(result.returncode, 0)
                self.assertFalse(marker.exists(), result.stdout + result.stderr)


if __name__ == '__main__':
    unittest.main()
