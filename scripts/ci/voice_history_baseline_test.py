import os
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

import voice_history_baseline as baseline


class HistoryBaselineTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name)
        self.git('init', '-q')
        self.git('config', 'user.email', 'test@example.invalid')
        self.git('config', 'user.name', 'Test')
        self.commit('src/backend/voice/main.go', 'base')
        self.base = self.git('rev-parse', 'HEAD')
        self.branch = self.git('symbolic-ref', '--short', 'HEAD')

    def git(self, *args):
        return subprocess.check_output(['git', *args], cwd=self.repo, text=True).strip()

    def commit(self, path, text):
        target = self.repo / path
        target.parent.mkdir(parents=True, exist_ok=True)
        target.write_text(text, encoding='utf-8')
        self.git('add', path)
        self.git('commit', '-qm', text)
        return self.git('rev-parse', 'HEAD')

    def test_identical_baseline_is_reusable(self):
        self.assertFalse(baseline.requires_audit(self.repo, self.base, self.base))

    def test_known_unrelated_asset_does_not_invalidate(self):
        head = self.commit('src/frontend/assets/test.png', 'asset')
        self.assertFalse(baseline.requires_audit(self.repo, self.base, head))

    def test_relevant_change_then_revert_still_invalidates(self):
        self.commit('src/backend/voice/main.go', 'changed')
        head = self.commit('src/backend/voice/main.go', 'base')
        self.assertTrue(baseline.requires_audit(self.repo, self.base, head))

    def test_merge_topology_invalidates_even_for_unrelated_assets(self):
        self.git('checkout', '-qb', 'side')
        self.commit('src/frontend/assets/side.png', 'side')
        self.git('checkout', '-q', self.branch)
        self.commit('src/frontend/assets/main.png', 'main')
        self.git('merge', '--no-ff', '-qm', 'merge assets', 'side')
        self.assertTrue(baseline.requires_audit(self.repo, self.base, self.git('rev-parse', 'HEAD')))

    def test_unknown_paths_and_unrelated_baseline_fail_closed(self):
        head = self.commit('new-area/file', 'unknown')
        self.assertTrue(baseline.requires_audit(self.repo, self.base, head))
        self.assertTrue(baseline.requires_audit(self.repo, head, self.base))

    def test_rename_from_relevant_path_to_asset_invalidates(self):
        target = self.repo / 'src/frontend/assets/renamed.png'
        target.parent.mkdir(parents=True, exist_ok=True)
        self.git('mv', 'src/backend/voice/main.go', 'src/frontend/assets/renamed.png')
        self.git('config', 'diff.renames', 'true')
        self.git('commit', '-qm', 'rename')
        self.assertTrue(baseline.requires_audit(self.repo, self.base, self.git('rev-parse', 'HEAD')))

    def test_scheduled_job_must_be_unique_completed_and_successful(self):
        jobs = [{'name': 'voice-history-audit', 'status': 'completed', 'conclusion': 'success'}]
        self.assertTrue(baseline.successful_audit(jobs))
        self.assertFalse(baseline.successful_audit([]))
        self.assertFalse(baseline.successful_audit(jobs * 2))
        self.assertFalse(baseline.successful_audit([dict(jobs[0], conclusion='failure')]))
        self.assertFalse(baseline.successful_audit([dict(jobs[0], status='in_progress')]))

    def test_only_successful_scheduled_master_job_is_a_baseline(self):
        run = {'id': 123, 'run_attempt': 2, 'event': 'schedule', 'status': 'completed',
               'head_branch': 'master', 'head_sha': self.base,
               'head_repository': {'full_name': 'owner/repo'}}
        job = {'name': 'voice-history-audit', 'status': 'completed', 'conclusion': 'success',
               'run_id': 123, 'run_attempt': 2}
        def get(path):
            if '/runs?' in path:
                return {'workflow_runs': [run]}
            return {'total_count': 1, 'jobs': [job]}
        self.assertEqual(baseline.find_nightly('owner/repo', get), (self.base, 123, 2))
        for key, value in [('event', 'push'), ('head_branch', 'other'), ('status', 'in_progress')]:
            with self.subTest(key=key), patch.dict(run, {key: value}):
                self.assertIsNone(baseline.find_nightly('owner/repo', get))
        with patch.dict(job, {'run_attempt': 1}):
            self.assertIsNone(baseline.find_nightly('owner/repo', get))

    def test_job_pages_and_failed_newer_runs_do_not_mask_last_success(self):
        def run(number):
            return {'id': number, 'run_attempt': 1, 'event': 'schedule', 'status': 'completed',
                    'head_branch': 'master', 'head_sha': self.base,
                    'head_repository': {'full_name': 'owner/repo'}}
        def get(path):
            if '/runs?' in path:
                return {'workflow_runs': [run(2), run(1)]}
            if '/2/jobs' in path:
                return {'total_count': 1, 'jobs': [{'name': 'voice-history-audit', 'status': 'completed',
                                                     'conclusion': 'failure', 'run_id': 2, 'run_attempt': 1}]}
            if '&page=1' in path:
                return {'total_count': 101, 'jobs': [{'name': 'unrelated'}] * 100}
            return {'total_count': 101, 'jobs': [{'name': 'voice-history-audit', 'status': 'completed',
                                                   'conclusion': 'success', 'run_id': 1, 'run_attempt': 1}]}
        self.assertEqual(baseline.find_nightly('owner/repo', get), (self.base, 1, 1))

    def test_production_gate_precedes_deployment_and_missing_output_runs_audit(self):
        import yaml
        root = Path(__file__).resolve().parents[2]
        workflow = yaml.safe_load((root / '.github/workflows/prod-deploy.yml').read_text(encoding='utf-8'))
        self.assertEqual(workflow['permissions']['actions'], 'read')
        steps = workflow['jobs']['deploy']['steps']
        by_name = {step.get('name'): (index, step) for index, step in enumerate(steps)}
        self.assertLess(by_name['Validate Voice history audit override'][0], by_name['Select Voice history audit'][0])
        self.assertLess(by_name['Audit production Voice history'][0], by_name['Apply production manifests'][0])
        self.assertIn("outputs.run_audit != 'false'", by_name['Audit production Voice history'][1]['if'])
        self.assertIn('SKIP_REASON', by_name['Validate Voice history audit override'][1]['env'])
        self.assertIn('voice_history_baseline_test.py', (root / 'Makefile').read_text(encoding='utf-8'))

    def test_override_requires_a_single_line_reason_and_records_it(self):
        import yaml
        root = Path(__file__).resolve().parents[2]
        workflow = yaml.safe_load((root / '.github/workflows/prod-deploy.yml').read_text(encoding='utf-8'))
        step = next(s for s in workflow['jobs']['deploy']['steps']
                    if s.get('name') == 'Validate Voice history audit override')
        program = step['run'].split("python3 - <<'PY'\n", 1)[1].rsplit('\nPY', 1)[0]
        summary = self.repo / 'summary'
        for skip, reason, expected in [('true', '', False), ('true', 'a\nb', False),
                                       ('false', 'unexpected', False), ('false', '', True),
                                       ('true', 'approved incident response', True)]:
            with self.subTest(skip=skip, reason=reason):
                env = dict(os.environ, SKIP_AUDIT=skip, SKIP_REASON=reason, GITHUB_STEP_SUMMARY=str(summary))
                result = subprocess.run([sys.executable, '-c', program], env=env, capture_output=True)
                self.assertEqual(result.returncode == 0, expected, result.stderr)
        self.assertIn('approved incident response', summary.read_text(encoding='utf-8'))


if __name__ == '__main__':
    unittest.main()
