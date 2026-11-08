"""Source-bound tests for the fetched PR proofs-shard component."""
import inspect
import hashlib
import copy
import json
import shutil
import subprocess
import tempfile
from types import SimpleNamespace
import unittest
from pathlib import Path
from unittest.mock import patch

import local_ci_proofs_component as proofs
import local_ci_proofs_scheduler as scheduler
import local_process_supervision as supervision


class ProofComponentTest(unittest.TestCase):
    @staticmethod
    def proof_evidence(limits, job_name, *, total_processes=2):
        return {**limits.to_dict(), 'job_name': job_name,
                'peak_job_memory_bytes': 4096, 'total_processes': total_processes,
                'active_processes_after_cleanup': 0}

    @staticmethod
    def fake_proof_run(events, names=None):
        names = names or ('Local\\VoiceCIProof-dry-name', 'Local\\VoiceCIProof-make-name')
        by_stage = {'make-dry-run': names[0], 'make': names[1]}
        run = SimpleNamespace()
        run.reserve_job_name = unittest.mock.Mock(
            side_effect=lambda stage: (events.append(('reserve', stage)) or by_stage[stage]))
        run.close = unittest.mock.Mock(side_effect=lambda: events.append(('close',)))
        run.abandon = unittest.mock.Mock(side_effect=lambda: events.append(('abandon',)))
        start = unittest.mock.Mock(side_effect=lambda **kwargs:
                                   (events.append(('start', kwargs)) or run))
        recover = unittest.mock.Mock(side_effect=lambda **kwargs:
                                     events.append(('recover', kwargs)))
        return run, start, recover

    def invoke_suite(self, results, limits, commands, *, raises=None):
        events = []
        run, start, recover = self.fake_proof_run(events)
        calls = []
        def execute(command, **kwargs):
            events.append(('execute', kwargs['job_name']))
            calls.append((command, kwargs))
            value = results.pop(0)
            if isinstance(value, BaseException):
                raise value
            return value
        output = self.repo / ('suite-output-' + str(len(list(self.repo.glob('suite-output-*')))))
        output.mkdir()
        with patch.object(proofs.scheduler, 'recover_proof_runs', recover), \
             patch.object(proofs.scheduler.ProofRun, 'start', start), \
             patch.object(proofs.execution, '_execute', side_effect=execute):
            if raises is None:
                result = proofs.run_proofs_recipe_suite(
                    self.repo / 'make.exe', self.repo, {}, output, commands, limits, timeout=30)
            else:
                with self.assertRaises(raises):
                    proofs.run_proofs_recipe_suite(
                        self.repo / 'make.exe', self.repo, {}, output, commands, limits, timeout=30)
                result = None
        return result, events, run, start, recover, calls

    def test_fetched_proofs_component_requires_caller_supplied_job_limits(self):
        parameter = inspect.signature(proofs.run).parameters.get('job_limits')
        self.assertIsNotNone(parameter, 'proof handoff must receive explicit resource limits')
        self.assertIs(parameter.default, inspect.Parameter.empty,
                      'proof limits must not be inferred or defaulted')

    def test_job_evidence_requires_exact_limits_and_zero_active_processes(self):
        limits = supervision.JobLimits(cpu_rate=1800, job_memory_bytes=16 * 1024 * 1024,
                                       active_process_limit=5)
        names = ('Local\\VoiceCIProof-dry-a', 'Local\\VoiceCIProof-make-b')
        evidence = {'make_dry_run': self.proof_evidence(limits, names[0]),
                    'make': self.proof_evidence(limits, names[1])}
        self.assertEqual(proofs.validate_job_evidence(evidence, limits, job_names=names), evidence)
        for key, changed in (
                ('missing', None),
                ('limit_mismatch', {**evidence['make'], 'cpu_rate': 1700}),
                ('negative_peak', {**evidence['make'], 'peak_job_memory_bytes': -1}),
                ('bad_total', {**evidence['make'], 'total_processes': True}),
                ('active_processes', {**evidence['make'], 'active_processes_after_cleanup': 1}),
                ('duplicate_job_name', evidence['make_dry_run'])):
            candidate = dict(evidence)
            if key == 'missing':
                candidate.pop('make')
            else:
                candidate['make'] = changed
            with self.subTest(key=key), self.assertRaises(ValueError):
                proofs.validate_job_evidence(candidate, limits, job_names=names)

    def test_component_receipt_verifier_binds_limits_and_job_accounting(self):
        limits = supervision.JobLimits(cpu_rate=1800, job_memory_bytes=8192,
                                       active_process_limit=3)
        names = ('Local\\VoiceCIProof-receipt-dry', 'Local\\VoiceCIProof-receipt-make')
        evidence = {'make_dry_run': self.proof_evidence(limits, names[0]),
                    'make': self.proof_evidence(limits, names[1])}
        expected = [f'cmd-{index}' for index in range(15)]
        dry = ('\n'.join(expected) + '\n').encode()
        stdout = ('\n'.join(expected) + '\nRan 1 test in 0.1s\n\nOK\n').encode()
        stderr = b''
        files = {}
        for name in ('bash.exe', 'make.exe'):
            path = self.repo / name
            path.write_bytes(name.encode())
            files[name] = (path, hashlib.sha256(path.read_bytes()).hexdigest())
        inventory = {'schema': 'voice-local-ci-matrix-inventory-v1',
                     'shards': {'proofs': [{'recipe': f'cmd-{i}'} for i in range(15)]},
                     'makefile_sha256': 'a' * 64, 'workflow_sha256': 'b' * 64}
        summary = {'recipes': 15, 'unittest_runs': 1, 'unittest_tests': 1, 'skipped': 0}
        identity = proofs.validate_candidate(self.repo, self.event_base, self.base,
                                             self.head, self.candidate)
        report = {'schema': 'voice-local-ci-proofs-component-v1', 'status': 'complete',
                  'scope': 'ci-script-tests/proofs', 'aggregate_preflight': 'incomplete',
                  'remaining_shards': ['policy', 'runtime', 'staging'],
                  'remaining_selected_checks': 'not_run', 'recipes': 15, 'skipped': 0,
                  'limitations': proofs.LIMITATIONS, 'event_base': self.event_base,
                  'base': self.base, 'head': self.head, 'candidate': self.candidate,
                  'candidate_identity': identity, 'candidate_tree': identity['tree'],
                  'makefile_sha256': inventory['makefile_sha256'],
                  'workflow_sha256': inventory['workflow_sha256'],
                  'recipe_manifest_sha256': hashlib.sha256(json.dumps(
                      inventory['shards']['proofs'], sort_keys=True,
                      separators=(',', ':')).encode()).hexdigest(),
                  'producer_sha256': hashlib.sha256(Path(proofs.__file__).read_bytes()).hexdigest(),
                  'tools': {'bash': {'path': str(files['bash.exe'][0]), 'version': 'bash fixture',
                                     'sha256': files['bash.exe'][1]},
                            'make': {'path': str(files['make.exe'][0]), 'version': 'make fixture',
                                     'sha256': files['make.exe'][1]}},
                  'logs': {'make-dry-run.stdout': hashlib.sha256(dry).hexdigest(),
                           'make.stdout': hashlib.sha256(stdout).hexdigest(),
                           'make.stderr': hashlib.sha256(stderr).hexdigest()},
                  'execution': summary, 'resource_limits': limits.to_dict(),
                  'job_evidence': evidence}
        original_git = proofs._git
        with patch.object(proofs, '_git', side_effect=lambda root, *args, **kw:
                          self.candidate if args == ('rev-parse', 'HEAD') else
                          identity['tree'] if args == ('rev-parse', 'HEAD^{tree}') else
                          original_git(root, *args, **kw)), \
             patch.object(proofs.matrix, 'build', return_value=inventory), \
             patch.object(proofs, '_read', side_effect=lambda path: {
                 'make-dry-run.stdout': dry, 'make.stdout': stdout,
                 'make.stderr': stderr}[Path(path).name]), \
             patch.object(proofs, 'expected_recipe_commands', return_value=expected), \
             patch.object(proofs, 'validate_execution', return_value=summary), \
             patch.object(proofs, '_tool', side_effect=lambda path, _name:
                          (Path(path), hashlib.sha256(Path(path).read_bytes()).hexdigest())), \
             patch.object(proofs, '_tool_version', side_effect=lambda _path, _name, _env:
                          'bash fixture' if _name == 'Bash' else 'make fixture'):
            proofs.verify_component(self.repo, self.repo / 'out', report,
                                    candidate_repo=self.repo)
            for mutate in (
                    lambda value: value.pop('resource_limits'),
                    lambda value: value['resource_limits'].__setitem__('cpu_rate', 1700),
                    lambda value: value['job_evidence'].pop('make'),
                    lambda value: value['job_evidence']['make'].__setitem__(
                        'active_processes_after_cleanup', 1),
                    lambda value: value['job_evidence']['make'].__setitem__(
                        'job_name', names[0])):
                broken = copy.deepcopy(report)
                mutate(broken)
                with self.assertRaises(ValueError):
                    proofs.verify_component(self.repo, self.repo / 'out', broken,
                                            candidate_repo=self.repo)

    def test_bounded_recipe_threads_limits_and_rejects_uncertain_or_unapplied_evidence(self):
        limits = supervision.JobLimits(cpu_rate=1800, job_memory_bytes=8192,
                                       active_process_limit=3)
        job_name = 'Local\\VoiceCIProof-bounded-test'
        evidence = self.proof_evidence(limits, job_name)
        result = SimpleNamespace(returncode=0, stdout=b'ok', stderr=b'', job_evidence=evidence)
        with patch.object(proofs.execution, '_execute', return_value=result) as execute:
            actual = proofs.execute_bounded_recipe(['make', 'target'], cwd=self.repo,
                                                   env={}, timeout=3, job_limits=limits,
                                                   job_name=job_name)
        self.assertIs(actual, result)
        self.assertEqual(execute.call_args.kwargs['job_limits'], limits)
        self.assertEqual(execute.call_args.kwargs['job_name'], job_name)
        for bad in (None, {**evidence, 'active_processes_after_cleanup': 1},
                    {**evidence, 'job_memory_bytes': 4096},
                    {**evidence, 'job_name': 'Local\\VoiceCIProof-other'}):
            with self.subTest(evidence=bad), \
                 patch.object(proofs.execution, '_execute', return_value=SimpleNamespace(
                     returncode=0, stdout=b'ok', stderr=b'', job_evidence=bad)):
                with self.assertRaises(ValueError):
                    proofs.execute_bounded_recipe(['make', 'target'], cwd=self.repo,
                                                  env={}, timeout=3, job_limits=limits,
                                                  job_name=job_name)

    def test_recipe_suite_holds_one_host_lease_until_both_jobs_are_clean(self):
        limits = supervision.JobLimits(cpu_rate=1800, job_memory_bytes=8192,
                                       active_process_limit=3)
        commands = [f'cmd-{index}' for index in range(15)]
        names = ('Local\\VoiceCIProof-dry-name', 'Local\\VoiceCIProof-make-name')
        dry = SimpleNamespace(returncode=0, stdout=('\n'.join(commands) + '\n').encode(),
                              stderr=b'', job_evidence=self.proof_evidence(limits, names[0]))
        actual = SimpleNamespace(returncode=0,
            stdout=('\n'.join(commands) + '\nRan 2 tests in 0.1s\n\nOK\n').encode(),
            stderr=b'', job_evidence=self.proof_evidence(limits, names[1]))
        (summary, reports), events, run, start, recover, calls = self.invoke_suite(
            [dry, actual], limits, commands)
        self.assertEqual(summary['unittest_tests'], 2)
        self.assertEqual(reports, {'make_dry_run': dry.job_evidence, 'make': actual.job_evidence})
        self.assertEqual([event[0] for event in events],
                         ['recover', 'start', 'reserve', 'execute', 'reserve', 'execute', 'close'])
        self.assertEqual(events[2:6], [('reserve', 'make-dry-run'), ('execute', names[0]),
                                      ('reserve', 'make'), ('execute', names[1])])
        self.assertEqual(start.call_args.kwargs, {'timeout': 300, 'capacity': 2})
        self.assertEqual(recover.call_args.kwargs, {'capacity': 2})
        self.assertEqual(run.close.call_count, 1)
        self.assertEqual(run.abandon.call_count, 0)
        self.assertEqual(len(calls), 2)
        self.assertTrue(all(call[1]['job_limits'] is limits for call in calls))
        self.assertEqual([call[1]['job_name'] for call in calls], list(names))

    def test_recipe_suite_abandons_lease_when_cleanup_evidence_is_uncertain(self):
        limits = supervision.JobLimits(cpu_rate=1800, job_memory_bytes=8192,
                                       active_process_limit=3)
        evidence = {'cpu_rate': 1800, 'job_memory_bytes': 8192,
                    'active_process_limit': 3, 'peak_job_memory_bytes': 4096,
                    'total_processes': 1, 'active_processes_after_cleanup': 0,
                    'job_name': 'Local\\VoiceCIProof-dry-name'}
        commands = [f'cmd-{index}' for index in range(15)]
        dry = SimpleNamespace(returncode=0, stdout=('\n'.join(commands) + '\n').encode(),
                              stderr=b'', job_evidence=evidence)
        uncertain = SimpleNamespace(returncode=0,
            stdout=('\n'.join(commands) + '\nRan 1 test in 0.1s\n\nOK\n').encode(),
            stderr=b'', job_evidence=None)
        _result, events, run, _start, _recover, calls = self.invoke_suite(
            [dry, uncertain], limits, commands, raises=ValueError)
        self.assertEqual([event[0] for event in events],
                         ['recover', 'start', 'reserve', 'execute', 'reserve', 'execute', 'abandon'])
        self.assertEqual(run.close.call_count, 0)
        self.assertEqual(run.abandon.call_count, 1)
        self.assertEqual(len(calls), 2)

    def test_recipe_suite_abandons_on_first_job_uncertainty_or_timeout(self):
        limits = supervision.JobLimits(cpu_rate=1800, job_memory_bytes=8192,
                                       active_process_limit=3)
        commands = [f'cmd-{index}' for index in range(15)]
        outcomes = (
            SimpleNamespace(returncode=0, stdout=b'\n'.join(c.encode() for c in commands) + b'\n',
                            stderr=b'', job_evidence=None),
            subprocess.TimeoutExpired(['make', '-n'], 1))
        for outcome in outcomes:
            with self.subTest(outcome=type(outcome).__name__):
                _result, events, run, _start, _recover, calls = self.invoke_suite(
                    [outcome], limits, commands, raises=(ValueError, subprocess.TimeoutExpired))
            self.assertEqual([event[0] for event in events],
                             ['recover', 'start', 'reserve', 'execute', 'abandon'])
            self.assertEqual(run.close.call_count, 0)
            self.assertEqual(run.abandon.call_count, 1)
            self.assertEqual(len(calls), 1)

    def test_recipe_suite_releases_after_nonzero_child_with_confirmed_cleanup(self):
        limits = supervision.JobLimits(cpu_rate=1800, job_memory_bytes=8192,
                                       active_process_limit=3)
        commands = [f'cmd-{index}' for index in range(15)]
        names = ('Local\\VoiceCIProof-dry-name', 'Local\\VoiceCIProof-make-name')
        dry = SimpleNamespace(returncode=0,
            stdout=('\n'.join(commands) + '\n').encode(), stderr=b'',
            job_evidence=self.proof_evidence(limits, names[0]))
        failed = SimpleNamespace(returncode=1,
            stdout=('\n'.join(commands) + '\nRan 1 test in 0.1s\n\nFAILED (failures=1)\n').encode(),
            stderr=b'', job_evidence=self.proof_evidence(limits, names[1]))
        _result, events, run, _start, _recover, calls = self.invoke_suite(
            [dry, failed], limits, commands, raises=ValueError)
        self.assertEqual([event[0] for event in events],
                         ['recover', 'start', 'reserve', 'execute', 'reserve', 'execute', 'close'])
        self.assertEqual(run.close.call_count, 1)
        self.assertEqual(run.abandon.call_count, 0)
        self.assertEqual(len(calls), 2)

    def test_recipe_suite_releases_after_dry_run_mismatch_with_confirmed_cleanup(self):
        limits = supervision.JobLimits(cpu_rate=1800, job_memory_bytes=8192,
                                       active_process_limit=3)
        commands = [f'cmd-{index}' for index in range(15)]
        names = ('Local\\VoiceCIProof-dry-name', 'Local\\VoiceCIProof-make-name')
        dry = SimpleNamespace(returncode=0, stdout=b'not the planned commands\n',
                              stderr=b'', job_evidence=self.proof_evidence(limits, names[0]))
        _result, events, run, _start, _recover, calls = self.invoke_suite(
            [dry], limits, commands, raises=ValueError)
        self.assertEqual([event[0] for event in events],
                         ['recover', 'start', 'reserve', 'execute', 'close'])
        self.assertEqual(run.close.call_count, 1)
        self.assertEqual(run.abandon.call_count, 0)
        self.assertEqual(len(calls), 1)

    def _run_component_with_fixtures(self, output, actual_evidence):
        limits = supervision.JobLimits(cpu_rate=1800, job_memory_bytes=8192,
                                       active_process_limit=3)
        events = []
        names = ('Local\\VoiceCIProof-dry-name', 'Local\\VoiceCIProof-make-name')
        proof_run, proof_start, proof_recover = self.fake_proof_run(events, names)
        self.git('remote', 'add', 'origin', str(self.repo))
        self.git('update-ref', 'refs/heads/master', self.base)
        self.git('update-ref', 'refs/heads/codex/proof-fixture', self.head)
        self.git('update-ref', 'refs/pull/1089/merge', self.candidate)
        refs = {'refs/heads/master': self.base,
                'refs/heads/codex/proof-fixture': self.head,
                'refs/pull/1089/merge': self.candidate}
        expected = [f'command-{index}' for index in range(15)]
        rows = [{'recipe': command} for command in expected]
        source = {'head': self.head, 'tree': self.git('rev-parse', 'HEAD^{tree}'),
                  'changed_paths': [], 'raw_mismatch_paths': []}
        inventory = {'schema': 'voice-local-ci-matrix-inventory-v1',
                     'shards': {'proofs': rows}, 'makefile_sha256': 'a' * 64,
                     'workflow_sha256': 'b' * 64}
        evidence = self.proof_evidence(limits, names[0])
        dry = ('\n'.join(expected) + '\n').encode()
        actual = ('\n'.join(expected) + '\nRan 2 tests in 0.1s\n\nOK\n').encode()
        results = [SimpleNamespace(returncode=0, stdout=dry, stderr=b'', job_evidence=evidence),
                   SimpleNamespace(returncode=0, stdout=actual, stderr=b'',
                                   job_evidence=actual_evidence)]
        fake_tools = {}
        for name in ('bash.exe', 'make.exe'):
            path = self.repo / name
            path.write_bytes(name.encode())
            fake_tools[name.removesuffix('.exe')] = (path, hashlib.sha256(path.read_bytes()).hexdigest())
        real_tool = proofs._tool

        with patch.object(proofs, '_windows', return_value=True), \
             patch.object(proofs.local_source_snapshot, 'capture', return_value=source), \
             patch.object(proofs.local_current_diff, '_remote_refs', return_value=refs), \
             patch.object(proofs.matrix, 'build', return_value=inventory), \
             patch.object(proofs, '_tool', side_effect=lambda value, name:
                          fake_tools[name.lower()] if name.lower() in fake_tools else real_tool(value, name)), \
             patch.object(proofs, '_tool_version', side_effect=lambda _path, name, _env:
                          'GNU bash, version fixture' if name == 'Bash' else 'GNU Make 4.4.1'), \
             patch.object(proofs, 'expected_recipe_commands', return_value=expected), \
             patch.object(proofs.scheduler, 'recover_proof_runs', proof_recover), \
             patch.object(proofs.scheduler.ProofRun, 'start', proof_start), \
             patch.object(proofs.execution, '_execute', side_effect=results) as execute:
            report = proofs.run(self.repo, self.event_base, self.head, 'codex/proof-fixture',
                                'refs/pull/1089/merge', output, bash_binary=str(fake_tools['bash'][0]),
                                make_binary=str(fake_tools['make'][0]), job_limits=limits)
        return report, proof_run, proof_start, proof_recover, execute, limits, evidence

    def test_run_orchestrates_suite_and_publishes_verified_limits_and_job_evidence_receipt(self):
        output = self.repo.parent / (self.repo.name + '-proof-success')
        self.addCleanup(shutil.rmtree, output, ignore_errors=True)
        limits = supervision.JobLimits(cpu_rate=1800, job_memory_bytes=8192,
                                       active_process_limit=3)
        names = ('Local\\VoiceCIProof-dry-name', 'Local\\VoiceCIProof-make-name')
        actual_evidence = self.proof_evidence(limits, names[1])
        report, proof_run, proof_start, proof_recover, execute, limits, dry_evidence = \
            self._run_component_with_fixtures(output, actual_evidence)
        receipt = json.loads((output / 'proofs-component.json').read_text(encoding='utf-8'))
        self.assertEqual(report['resource_limits'], limits.to_dict())
        self.assertEqual(receipt['resource_limits'], limits.to_dict())
        expected_evidence = {'make_dry_run': dry_evidence, 'make': actual_evidence}
        self.assertEqual(receipt['job_evidence'], expected_evidence)
        self.assertEqual(report['status'], 'complete')
        self.assertEqual(proof_run.close.call_count, 1)
        self.assertEqual(proof_run.abandon.call_count, 0)
        self.assertEqual(proof_start.call_args.kwargs, {'timeout': 300, 'capacity': 2})
        self.assertEqual(proof_recover.call_args.kwargs, {'capacity': 2})
        self.assertEqual(execute.call_count, 2)
        self.assertEqual([call.kwargs['job_name'] for call in execute.call_args_list], list(names))

    def test_run_does_not_publish_complete_receipt_when_job_evidence_is_invalid(self):
        output = self.repo.parent / (self.repo.name + '-proof-failure')
        self.addCleanup(shutil.rmtree, output, ignore_errors=True)
        with self.assertRaises(ValueError):
            self._run_component_with_fixtures(output, None)
        self.assertFalse((output / 'proofs-component.json').exists())

    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='voice-ci-proofs-component-')
        self.addCleanup(self.temp.cleanup)
        self.repo = Path(self.temp.name)
        (self.repo / 'out').mkdir()
        self.git('init', '-q')
        self.git('config', 'user.email', 'fixture@example.invalid')
        self.git('config', 'user.name', 'CI fixture')
        self.write('README.md', 'base\n')
        self.event_base = self.commit('base')
        self.base = self.event_base
        self.write('README.md', 'head\n')
        self.head = self.commit('head')
        self.candidate = self.merge(self.base, self.head)

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.repo), *args],
                                       stderr=subprocess.PIPE, text=True).strip()

    def write(self, name, content):
        path = self.repo / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_text(content, encoding='utf-8')

    def commit(self, message):
        self.git('add', '.')
        self.git('commit', '-qm', message)
        return self.git('rev-parse', 'HEAD')

    def merge(self, base, head, tree=None):
        tree = tree or self.git('merge-tree', '--write-tree', base, head).splitlines()[0]
        return self.git('commit-tree', tree, '-p', base, '-p', head, '-m', 'PR merge')

    def test_candidate_requires_exact_parents_and_clean_merge_tree(self):
        result = proofs.validate_candidate(self.repo, self.event_base, self.base,
                                           self.head, self.candidate)
        self.assertEqual(result['parents'], [self.base, self.head])
        self.assertEqual(result['tree'], self.git('rev-parse', self.candidate + '^{tree}'))

    def test_candidate_rejects_wrong_head_parent_and_forged_tree(self):
        wrong_parent = self.merge(self.base, self.event_base)
        wrong_tree = self.merge(self.base, self.head,
                                self.git('rev-parse', self.base + '^{tree}'))
        with self.assertRaisesRegex(ValueError, 'parent'):
            proofs.validate_candidate(self.repo, self.event_base, self.base,
                                      self.head, wrong_parent)
        with self.assertRaisesRegex(ValueError, 'tree'):
            proofs.validate_candidate(self.repo, self.event_base, self.base,
                                      self.head, wrong_tree)

    def test_stale_event_base_must_be_ancestor_of_actual_merge_base(self):
        self.write('docs/advance.md', 'advanced base\n')
        self.base = self.commit('advance target base')
        self.git('checkout', '-q', self.event_base)
        self.write('docs/source.md', 'source\n')
        self.head = self.commit('source')
        self.git('checkout', '-q', self.base)
        self.candidate = self.merge(self.base, self.head)
        result = proofs.validate_candidate(self.repo, self.event_base, self.base,
                                           self.head, self.candidate)
        self.assertEqual(result['base'], self.base)
        with self.assertRaisesRegex(ValueError, 'ancestor'):
            proofs.validate_candidate(self.repo, self.head, self.base,
                                      self.head, self.candidate)

    def test_execution_requires_every_recipe_and_rejects_unittest_skips(self):
        commands = [f'command-{number}' for number in range(15)]
        dry_run = ('\n'.join(commands) + '\n').encode()
        output = ('\n'.join(commands) + '\nRan 4 tests in 0.1s\n\nOK\n').encode()
        result = proofs.validate_execution(dry_run, output, b'', 0, expected_recipes=15)
        self.assertEqual(result['recipes'], 15)
        for text in (
                '\n'.join(commands[:-1]) + '\n',
                '\n'.join(commands[:-1] + ['other-command']) + '\n'):
            with self.subTest(text=text), self.assertRaisesRegex(ValueError, 'recipe'):
                proofs.validate_execution(dry_run, text.encode(), b'Ran 4 tests in 0.1s\n\nOK\n',
                                          0, expected_recipes=15)
        skipped = ('\n'.join(commands) + '\nRan 4 tests in 0.1s\n\n'
                   'OK (skipped=1)\n').encode()
        with self.assertRaisesRegex(ValueError, 'skip'):
            proofs.validate_execution(dry_run, skipped, b'', 0, expected_recipes=15)

    def test_execution_rejects_empty_test_run_failure_and_bounded_output(self):
        commands = ('\n'.join(f'command-{number}' for number in range(15)) + '\n').encode()
        for output, error, code in ((commands, b'', 0),
                                    (commands + b'Ran 0 tests in 0.1s\n\nOK\n', b'', 0),
                                    (commands + b'Ran 1 test in 0.1s\n\nFAILED\n', b'', 1),
                                    (commands + b'Ran 1 test in 0.1s\n\nOK\n', b'SKIP: unavailable\n', 0)):
            with self.subTest(output=output[-32:], error=error, code=code), self.assertRaises(ValueError):
                proofs.validate_execution(commands, output, error, code, expected_recipes=15)

    def test_child_environment_removes_make_and_shell_failure_overrides(self):
        source = {'PATH': 'private-shims', 'MAKEFLAGS': '-i', 'GNUMAKEFLAGS': '-k',
                  'MFLAGS': '-s', 'MAKEFILES': 'untrusted.mk', 'MAKEOVERRIDES': 'x=y',
                  'BASH_ENV': 'startup.sh', 'ENV': 'startup.sh', 'SHELLOPTS': 'xtrace',
                  'BASHOPTS': 'expand_aliases', 'GIT_DIR': 'foreign.git',
                  'PYTHONPATH': 'foreign-python', 'OTHER_VALUE': 'keep'}
        result = proofs.child_environment(source, self.repo / 'bash.exe')
        self.assertEqual(result['PATH'], 'private-shims')
        self.assertEqual(result['OTHER_VALUE'], 'keep')
        self.assertEqual(result['VOICE_TEST_BASH'], str(self.repo / 'bash.exe'))
        self.assertFalse({'MAKEFLAGS', 'GNUMAKEFLAGS', 'MFLAGS', 'MAKEFILES',
                          'MAKEOVERRIDES', 'BASH_ENV', 'ENV', 'SHELLOPTS', 'BASHOPTS',
                          'GIT_DIR', 'PYTHONPATH'} & set(result))

    def test_tool_version_accepts_standard_multiline_gnu_make_output(self):
        output = (b'GNU Make 4.4.1\nBuilt for Windows32\nCopyright (C) Free Software Foundation\n')
        with patch.object(proofs.execution, '_execute', return_value=SimpleNamespace(
                returncode=0, stdout=output, stderr=b'')):
            self.assertEqual(proofs._tool_version(self.repo / 'make.exe', 'GNU Make', {}),
                             output.decode().strip())
        for malformed in (b'Fake Make 4.4.1\n', b'GNU Make unknown\n'):
            with self.subTest(output=malformed), patch.object(
                    proofs.execution, '_execute', return_value=SimpleNamespace(
                        returncode=0, stdout=malformed, stderr=b'')):
                with self.assertRaisesRegex(ValueError, 'not recognized'):
                    proofs._tool_version(self.repo / 'make.exe', 'GNU Make', {})

    def test_private_git_config_pins_newline_behavior_and_rejects_deep_windows_paths(self):
        proofs.configure_private_git('git', self.repo)
        self.assertEqual(self.git('config', '--get', 'core.autocrlf'), 'false')
        self.assertEqual(self.git('config', '--get', 'core.longpaths'), 'false')
        with patch.object(proofs, '_windows', return_value=True):
            proofs.validate_windows_checkout_paths(self.repo, ['short/file.txt'])
            long_name = 'nested/' + ('x' * proofs.WINDOWS_PATH_LIMIT)
            with self.assertRaisesRegex(ValueError, 'shorter output path'):
                proofs.validate_windows_checkout_paths(self.repo, [long_name])
            astral_name = 'nested/' + ('😀' * 130)
            with self.assertRaisesRegex(ValueError, 'shorter output path'):
                proofs.validate_windows_checkout_paths(self.repo, [astral_name])
            long_root = self.repo / ('x' * proofs.WINDOWS_PATH_LIMIT)
            with self.assertRaisesRegex(ValueError, 'shorter output path'):
                proofs.validate_windows_checkout_paths(long_root, [])

    def test_recipe_expansion_is_exact_and_unknown_make_syntax_refuses(self):
        bash = self.repo / 'bash.exe'
        rows = [{'recipe': '$(BASH) "$(ROOT)/scripts/ci/a.sh"'},
                {'recipe': 'python3 "$(ROOT)/scripts/ci/b.py"'}]
        expanded = proofs.expected_recipe_commands(rows, self.repo, bash)
        if proofs.os.name == 'nt':
            self.assertEqual(expanded[0], '"' + bash.resolve().as_posix() + '" "' +
                             self.repo.resolve().as_posix() + '/scripts/ci/a.sh"')
        else:
            self.assertEqual(expanded[0], str(bash.resolve()) + ' "' +
                             self.repo.resolve().as_posix() + '/scripts/ci/a.sh"')
        with self.assertRaisesRegex(ValueError, 'unsupported expansion'):
            proofs.expected_recipe_commands([{'recipe': 'make $(shell true)'}], self.repo, bash)

    def test_runner_refuses_platform_without_descendant_supervision(self):
        from unittest.mock import patch
        limits = supervision.JobLimits(cpu_rate=1000, job_memory_bytes=1024,
                                       active_process_limit=2)
        with patch.object(proofs, '_windows', return_value=False):
            with self.assertRaisesRegex(ValueError, 'Windows descendant supervision'):
                proofs.run(self.repo, self.event_base, self.head, 'codex/test',
                           'refs/pull/1089/merge',
                           Path(self.temp.name).parent / (Path(self.temp.name).name + '-outside'),
                           job_limits=limits)


if __name__ == '__main__':
    unittest.main()
