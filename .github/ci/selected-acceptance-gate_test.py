"""Offline gate contract against actual workflow job and gate predicates."""
import ast
import itertools
import os
from pathlib import Path
import re
import subprocess
import unittest

ROOT = Path(__file__).resolve().parents[2]
WORKFLOW = (ROOT / '.github/workflows/ci.yml').read_text(encoding='utf-8')
JOBS = (
    'flutter-key-backup-goldens', 'flutter-android-smoke',
    'flutter-web-integration', 'flutter-ios', 'a1-e2e',
    'a1-flutter-profile-handoff', 'a1-attachment-restart-proof',
    'analytics-clickhouse-integration', 'grafana-analytics-smoke',
    'nats-hub-config-renderer-image', 'f13-historical-replay',
)


def block(name):
    return re.search(r'^  ' + re.escape(name) + r':\n(.*?)(?=^  [\w-]+:|\Z)',
                     WORKFLOW, re.M | re.S).group(1)


def job_condition(name):
    return re.search(r'^    if: >-\n(.*?)(?=^    \S)', block(name),
                     re.M | re.S).group(1).strip()


def evaluate(expression, values):
    expression = expression.strip().removeprefix('${{').removesuffix('}}').strip()
    expression = ' '.join(expression.split())
    expression = expression.replace('always()', 'true')
    expression = expression.replace('!cancelled()', 'workflow_not_cancelled')

    def token(match):
        value = match.group()
        if value.startswith("'"):
            return value
        if value in ('true', 'false'):
            return 'True' if value == 'true' else 'False'
        return repr(values[value])  # Unknown workflow grammar fails the test.

    expression = re.sub(r"'[^']*'|[A-Za-z_][A-Za-z0-9_.-]*", token, expression)
    expression = expression.replace('&&', ' and ').replace('||', ' or ')
    parsed = ast.parse(expression, mode='eval')
    allowed = (ast.Expression, ast.BoolOp, ast.And, ast.Or, ast.Compare,
               ast.Eq, ast.NotEq, ast.Constant, ast.Load)
    assert all(isinstance(node, allowed) for node in ast.walk(parsed)), expression
    return eval(compile(parsed, '<workflow predicate>', 'eval'), {'__builtins__': {}})


GATE = block('ci-gate')
LEGACY_GATE = GATE.split('      - name: Verify required jobs ran\n', 1)[1]
ENV = dict(re.findall(r'^          (\w+): (\$\{\{.*?\}\})$', LEGACY_GATE, re.M))
# The existing Compose selection uses a folded YAML scalar.
ENV['RUN_COMPOSE_E2E'] = re.search(
    r'RUN_COMPOSE_E2E: >-\n(.*?)(?=^          JOB_)', LEGACY_GATE, re.M | re.S
).group(1).strip()
OUTPUTS = set(re.findall(r'needs\.changes\.outputs\.([\w-]+)', WORKFLOW))


def context(event, **flags):
    values = {'github.event_name': event, 'github.ref': 'refs/heads/master',
              'needs.changes.result': 'success', 'inputs.profile': 'auto',
              'workflow_not_cancelled': True}
    values.update({'needs.changes.outputs.' + key: 'false' for key in OUTPUTS})
    values['needs.changes.outputs.integration_go_services'] = '[]'
    values.update({'needs.changes.outputs.' + key: value for key, value in flags.items()})
    return values


def gate_env(values):
    env = os.environ.copy()
    for name, expression in ENV.items():
        if name.startswith('JOB_'):
            env[name] = 'success'
        else:
            value = evaluate(expression, values)
            env[name] = str(value).lower() if isinstance(value, bool) else value
    return env


def run_gate(values, job=None, result=None):
    env = gate_env(values)
    if job:
        env['JOB_' + job.upper().replace('-', '_')] = result
    return subprocess.run(
        [bash(), str(ROOT / '.github/ci/verify-required-jobs.sh'),
         values['github.event_name'], values['needs.changes.outputs.code'], 'auto'],
        env=env, capture_output=True, text=True, timeout=10,
    )


class SelectedAcceptanceGateTests(unittest.TestCase):
    def test_f13_selected_result_cannot_be_skipped(self):
        values = context('pull_request', code='true', ci_scripts='true', f13_replay='true')
        self.assertTrue(evaluate(job_condition('f13-historical-replay'), values))
        self.assertTrue(evaluate(ENV['RUN_F13_HISTORICAL_REPLAY'], values))
        self.assertEqual(run_gate(values).returncode, 0)
        for outcome in ('skipped', 'failure', 'cancelled'):
            with self.subTest(outcome=outcome):
                self.assertNotEqual(run_gate(values, 'f13-historical-replay', outcome).returncode, 0)
        unrelated = context('pull_request', code='true', ci_scripts='true')
        self.assertFalse(evaluate(job_condition('f13-historical-replay'), unrelated))
        self.assertFalse(evaluate(ENV['RUN_F13_HISTORICAL_REPLAY'], unrelated))

    def test_changed_live_flutter_suite_requires_compose_proof(self):
        values = context('pull_request', code='true', frontend='true', flutter_live_suite='true')
        self.assertTrue(evaluate(job_condition('compose-e2e'), values))
        self.assertTrue(evaluate(ENV['RUN_COMPOSE_E2E'], values))
        self.assertEqual(run_gate(values).returncode, 0)
        for outcome in ('skipped', 'failure', 'cancelled'):
            with self.subTest(outcome=outcome):
                self.assertNotEqual(run_gate(values, 'compose-e2e', outcome).returncode, 0)

    def test_independent_go_jobs_stop_on_cancellation_and_pkg_failure_still_blocks_gate(self):
        values = context('pull_request', code='true', run_go='true')
        values['needs.backend-go-pkg.result'] = 'skipped'
        condition = job_condition('backend-go')
        self.assertTrue(evaluate(condition, values))

        values['needs.backend-go-pkg.result'] = 'failure'
        self.assertTrue(evaluate(condition, values))
        values['needs.changes.outputs.run_pkg'] = 'true'
        self.assertNotEqual(run_gate(values, 'backend-go-pkg', 'failure').returncode, 0)

        values['needs.backend-go-pkg.result'] = 'success'
        values['workflow_not_cancelled'] = False
        self.assertFalse(evaluate(condition, values))

    def test_changed_t055_path_selects_live_handoff_and_gate(self):
        filters = (ROOT / '.github/ci/path-filters.yml').read_text(encoding='utf-8')
        selected = re.search(r'^a1_t106_live:\n(.*?)(?=^\S)', filters,
                             re.M | re.S).group(1)
        paths = re.findall(r'^  - (\S+)$', selected, re.M)
        t055 = 'src/frontend/test/t055_profile_switch_reconnect_inbox_e2e_live_test.dart'
        self.assertIn(t055, paths)
        values = context('pull_request', code='true', frontend='true',
                         ci_global='true', a1_t106_live='true' if t055 in paths else 'false')
        self.assertTrue(evaluate(job_condition('a1-flutter-profile-handoff'), values))
        self.assertEqual(run_gate(values).returncode, 0)
        self.assertNotEqual(run_gate(values, 'a1-flutter-profile-handoff', 'skipped').returncode, 0)

    def test_actual_selection_matches_gate_across_filters_and_events(self):
        for job in JOBS:
            self.assertTrue('      - ' + job + '\n' in GATE, 'missing gate dependency: ' + job)
            self.assertIn('${{ needs.' + job + '.result }}', ENV[
                'JOB_' + job.upper().replace('-', '_')])
        for job in JOBS:
            name = 'RUN_' + job.upper().replace('-', '_')
            # Exhaust every input of this job and its gate, including new flags,
            # without multiplying by unrelated platform/service selectors.
            flags = sorted(set(re.findall(r'needs\.changes\.outputs\.([\w-]+)',
                                          job_condition(job) + ENV[name])))
            for event, bits in itertools.product(
                    ('pull_request', 'push'), itertools.product(('false', 'true'), repeat=len(flags))):
                values = context(event, **dict(zip(flags, bits)))
                self.assertEqual(bool(evaluate(job_condition(job), values)),
                                 bool(evaluate(ENV[name], values)), (event, bits, job))

    def test_selected_failure_cancel_skip_or_missing_blocks_gate(self):
        values = context('push', **{key: 'true' for key in OUTPUTS})
        values['needs.changes.outputs.integration_go_services'] = '[]'
        self.assertEqual(run_gate(values).returncode, 0)
        for job, result in itertools.product(JOBS, ('failure', 'cancelled', 'skipped', '')):
            actual = run_gate(values, job, result)
            self.assertNotEqual(actual.returncode, 0, (job, result, actual.stdout))
            self.assertIn(job, actual.stderr)

    def test_docs_only_global_ci_still_checks_selected_jobs(self):
        values = context('pull_request', ci_global='true')
        self.assertEqual(run_gate(values).returncode, 0)
        for job in ('golangci', 'backend-go-pkg', 'backend-go', 'a1-e2e',
                    'nats-hub-config-renderer-image'):
            self.assertNotEqual(run_gate(values, job, 'failure').returncode, 0, job)
        # Unknown CI inputs conservatively require the profile-handoff proof.
        self.assertNotEqual(run_gate(values, 'a1-flutter-profile-handoff', 'skipped').returncode, 0)

    def test_unselected_and_plain_docs_only_skips_remain_valid(self):
        for event in ('pull_request', 'push'):
            values = context(event)
            env = gate_env(values)
            env.update({key: 'skipped' for key in env if key.startswith('JOB_')})
            # Current merge validation is mandatory even for docs PRs;
            # historical audit is a separate mandatory master release check.
            env['JOB_CURRENT_DIFF_CONTRACT'] = 'success'
            env['JOB_VOICE_HISTORY_AUDIT'] = 'success'
            actual = subprocess.run([bash(), str(ROOT / '.github/ci/verify-required-jobs.sh'),
                                     event, 'false', 'auto'], env=env,
                                    capture_output=True, text=True, timeout=10)
            self.assertEqual(actual.returncode, 0, actual.stderr)


def bash():
    return 'C:/Program Files/Git/bin/bash.exe' if os.name == 'nt' else 'bash'


if __name__ == '__main__':
    unittest.main()
