"""Only reviewed service profiles may run full private Go diagnostics."""
import hashlib
import io
import json
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest import mock

import local_go_full_integration as subject
import local_ci_proofs_scheduler as proof_scheduler
import local_heavy_slots as heavy_slots
from local_docker_owned import OwnedRun
from local_docker_owned_test import FakeEngine


class FullGoTests(unittest.TestCase):
    def setUp(self):
        patcher = mock.patch.object(subject.auth_broker, '_require_worker_job')
        patcher.start()
        self.addCleanup(patcher.stop)

    @staticmethod
    def resource_limits():
        return {'client': {'nano_cpus': 1500000000, 'memory_bytes': 2 * 1024 * 1024 * 1024,
                           'memory_swap_bytes': 3 * 1024 * 1024 * 1024, 'pids_limit': 384},
                'daemon': {'nano_cpus': 1000000000, 'memory_bytes': 512 * 1024 * 1024,
                           'memory_swap_bytes': 1024 * 1024 * 1024, 'pids_limit': 256},
                'host_job': {'cpu_rate': 1500, 'job_memory_bytes': 3 * 1024 * 1024 * 1024,
                             'active_process_limit': 512},
                'capacity_budget': {'nano_cpus': 8_000_000_000,
                                    'memory_bytes': 12 * 1024 * 1024 * 1024,
                                    'pids_limit': 2048}}

    @staticmethod
    def source_identity(root, source):
        canonical = json.dumps(source, allow_nan=False, separators=(',', ':'), sort_keys=True).encode('utf-8')
        return {'root': str(root.resolve()), 'head': source['head'],
                'sha256': hashlib.sha256(canonical).hexdigest()}

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
                       'go_binary': 'go', 'base': 'd' * 40, 'head_ref': 'codex/test',
                       'resource_limits': self.resource_limits()}
            limits = self.resource_limits()
            readback = {'client': limits['client'], 'daemon': limits['daemon']}
            with mock.patch.object(sys, 'argv', ['worker', '--worker']), \
                 mock.patch.object(sys, 'stdin', SimpleNamespace(buffer=io.BytesIO(json.dumps(request).encode()))), \
                 mock.patch.object(sys, 'stdout', new_callable=io.StringIO), \
                 mock.patch.object(subject.auth_broker, '_require_worker_job'), \
                 mock.patch.object(subject, 'run_selected', return_value={'status': 'incomplete',
                     'source': {'head': 'f' * 40}, 'resource_limit_readback': readback}) as run:
                self.assertEqual(subject.main(), 0)
            report = json.loads((output / 'diagnostic.json').read_text(encoding='utf-8'))
            self.assertEqual(report['input_identity'], {'helper_sha256': 'a' * 64, 'archives': {
                'postgres:16-bookworm': {'archive_sha256': 'b' * 64, 'image_id': 'sha256:' + 'c' * 64}},
                'resource_limits': self.resource_limits()})
            self.assertEqual(report['resource_limit_readback'], readback)
            self.assertEqual(run.call_args.kwargs['resource_limits'], self.resource_limits())
            self.assertEqual(report['status'], 'incomplete')

    def test_worker_rejects_invalid_resource_limits_before_selected_run(self):
        limits = self.resource_limits()
        for domain, key, value in (('client', 'nano_cpus', True), ('client', 'memory_bytes', 0),
                                   ('daemon', 'pids_limit', 1), ('host_job', 'cpu_rate', -1),
                                   ('capacity_budget', 'pids_limit', 0)):
            changed = json.loads(json.dumps(limits))
            changed[domain][key] = value
            request = {'plan': {}, 'service': 'bot', 'root': '.', 'output': '.', 'helper': 'helper',
                       'helper_digest': 'a' * 64, 'archives': {'postgres:16-bookworm': {}},
                       'go_binary': 'go', 'base': 'd' * 40, 'head_ref': 'codex/test',
                       'resource_limits': changed}
            with self.subTest(domain=domain, key=key), \
                 mock.patch.object(sys, 'argv', ['worker', '--worker']), \
                 mock.patch.object(sys, 'stdin', SimpleNamespace(buffer=io.BytesIO(json.dumps(request).encode()))), \
                 mock.patch.object(sys, 'stdout', new_callable=io.StringIO), \
                 mock.patch.object(subject.auth_broker, '_require_worker_job'), \
                 mock.patch.object(subject, 'run_selected') as run, self.assertRaises(ValueError):
                subject.main()
            run.assert_not_called()

    def test_selected_service_passes_resource_limits_to_its_executor(self):
        source = {'head': 'a' * 40}
        base = 'b' * 40
        plan = {'base': base, 'source': source,
                'checks': {'backend-go-integration-pr': ['bot']},
                'components': {'go_integration': ['bot']}}
        limits = self.resource_limits()
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'; root.mkdir()
            output = Path(directory) / 'output'
            archives = {tag: {'archive': str(Path(directory) / str(index)),
                              'archive_sha256': 'c' * 64, 'image_id': 'sha256:' + 'd' * 64}
                        for index, tag in enumerate(subject.profile('bot')['images'])}
            with mock.patch.object(subject.affected_check_plan, 'build_local_plan', return_value=plan), \
                 mock.patch.object(subject.snapshot, 'capture', return_value=source), \
                 mock.patch.object(subject, 'run', return_value={'status': 'incomplete', 'source': source}) as run:
                result = subject.run_selected(plan, 'bot', root, output, 'helper', 'e' * 64, archives, 'go',
                                              base=base, head_ref='codex/test', resource_limits=limits)
            self.assertEqual(result['status'], 'incomplete')
            self.assertEqual(run.call_args.kwargs['resource_limits'], limits)

    def test_supervised_worker_accepts_only_matching_saved_incomplete_report(self):
        plan = {'source': {'head': 'a' * 40}}
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'out'; output.mkdir()
            limits = self.resource_limits()
            report = {'status': 'incomplete', 'source': plan['source'],
                      'input_identity': {'helper_sha256': 'c' * 64, 'archives': {},
                                         'resource_limits': limits},
                      'resource_limit_readback': {'client': limits['client'], 'daemon': limits['daemon']}}
            (output / 'diagnostic.json').write_text(json.dumps(report), encoding='utf-8')
            fake = subprocess.CompletedProcess([], 0, stdout=json.dumps(report).encode(), stderr=b'')
            fake.job_evidence = {'cpu_rate': 1500, 'job_memory_bytes': 3 * 1024 * 1024 * 1024,
                                 'active_process_limit': 512, 'peak_job_memory_bytes': 1024,
                                 'active_processes_after_cleanup': 0}
            with mock.patch.object(subject.supervision, 'run_diagnostic', return_value=fake) as broker:
                result = subject.run_supervised(plan, 'bot', Path(directory), output, 'helper', 'c' * 64, {},
                                                base='b' * 40, head_ref='codex/test', resource_limits=limits)
            self.assertEqual(result['resource_limit_readback']['host_job'], fake.job_evidence)
            request = json.loads(broker.call_args.kwargs['input'])
            self.assertEqual(request['resource_limits'], limits)
            self.assertEqual(broker.call_args.kwargs['job_limits'], subject.supervision.JobLimits(
                cpu_rate=1500, job_memory_bytes=3 * 1024 * 1024 * 1024, active_process_limit=512))
            self.assertEqual(result['input_identity']['resource_limits'], limits)
            (output / 'diagnostic.json').write_text('{}', encoding='utf-8')
            with mock.patch.object(subject.supervision, 'run_diagnostic', return_value=fake), \
                 self.assertRaises(ValueError):
                subject.run_supervised(plan, 'bot', Path(directory), output, 'helper', 'c' * 64, {},
                                       base='b' * 40, head_ref='codex/test', resource_limits=limits)

    def test_capacity_budget_above_measured_host_rejects_before_owner_slot_or_worker(self):
        limits = self.resource_limits()
        limits['capacity_budget']['nano_cpus'] = 5_000_000_000
        limits['capacity_budget']['memory_bytes'] = 16 * 1024 * 1024 * 1024
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'; root.mkdir()
            output = Path(directory) / 'out'; output.mkdir()
            slots = Path(directory) / 'slots'
            with mock.patch.object(heavy_slots, 'measured_host_capacity', return_value={
                     'nano_cpus': 4_000_000_000, 'memory_bytes': 8 * 1024 * 1024 * 1024}), \
                 mock.patch.object(subject, 'OwnedRun') as owned_run, \
                 mock.patch.object(subject.supervision, 'run_diagnostic') as worker, \
                 mock.patch('local_docker_scheduled.host_slot_directory', return_value=slots), \
                 self.assertRaisesRegex(ValueError, 'measured host capacity'):
                subject.run_supervised({'source': {'head': 'a' * 40}}, 'bot', root, output,
                                       'helper', 'c' * 64, {}, base='b' * 40,
                                       head_ref='codex/test', resource_limits=limits)
            owned_run.create.assert_not_called()
            worker.assert_not_called()
            self.assertFalse(slots.exists())
            self.assertEqual(list(root.rglob('ownership.json')), [])

    def test_supervised_launch_has_durable_named_job_intent_and_owner_bound_slot(self):
        limits = self.resource_limits()
        source = {'head': 'a' * 40}
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'; root.mkdir()
            output = Path(directory) / 'out'; output.mkdir()
            slots = Path(directory) / 'slots'
            readback = {'client': limits['client'], 'daemon': limits['daemon']}
            report = {'status': 'incomplete', 'source': source,
                      'input_identity': {'helper_sha256': 'c' * 64, 'archives': {},
                                         'resource_limits': limits},
                      'resource_limit_readback': readback}
            slot_marker_written = []
            owner_ids = []
            admission_calls = []

            def acquire_with_capacity(directory, timeout, capacity=2, *, owner_id=None,
                                     resource_claim=None, capacity_budget=None):
                admission_calls.append((owner_id, resource_claim, capacity_budget))
                return heavy_slots.acquire(directory, timeout, capacity=capacity, owner_id=owner_id,
                                           resource_claim=resource_claim, capacity_budget=capacity_budget)

            def after_owner_marker(_descriptor, slot, owner_id):
                slot_marker_written.append((slot, owner_id))
                journals = list(root.rglob('ownership.json'))
                self.assertEqual(len(journals), 1)
                journal = json.loads(journals[0].read_text(encoding='utf-8'))
                self.assertEqual(journal['run_id'], owner_id)
                self.assertEqual(journal['host_job']['slot'], None)
                self.assertEqual(journal['resources'], [])

            def launch(command, *, input, job_limits, job_name, **kwargs):
                request = json.loads(input)
                owner_id = request['owner_id']
                owner_ids.append(owner_id)
                self.assertRegex(owner_id, r'^voiceci-[0-9a-f]{32}$')
                self.assertEqual(job_name, 'Local\\VoiceCIProof-Go-' + owner_id.removeprefix('voiceci-'))
                self.assertEqual(job_limits, subject.supervision.JobLimits(**limits['host_job']))
                journal_path = Path(request['ownership_journal'])
                journal = json.loads(journal_path.read_text(encoding='utf-8'))
                self.assertEqual(journal['version'], 2)
                self.assertEqual(journal['run_id'], owner_id)
                intent = journal['host_job']
                self.assertEqual(intent['schema'], 1)
                self.assertEqual(intent['slot'], request['slot'])
                self.assertEqual(intent['job_names'], [job_name])
                self.assertEqual(intent['limits'], limits['host_job'])
                self.assertEqual(set(intent['session_identity']), {'boot_id', 'logon_id', 'session_id'})
                self.assertEqual(intent['source_identity'], self.source_identity(root, source))
                self.assertNotEqual(intent['slot'], None)
                self.assertIn(owner_id.encode('ascii'), (slots / ('heavy-' + str(request['slot']) + '.lock')).read_bytes())
                (output / 'diagnostic.json').write_text(json.dumps(report), encoding='utf-8')
                completed = subprocess.CompletedProcess(command, 0, stdout=json.dumps(report).encode(), stderr=b'')
                completed.job_evidence = {'cpu_rate': 1500, 'job_memory_bytes': 3 * 1024 * 1024 * 1024,
                                          'active_process_limit': 512, 'peak_job_memory_bytes': 1024,
                                          'active_processes_after_cleanup': 0}
                return completed

            with mock.patch.object(subject, '_worker_command', return_value=['worker']), \
                 mock.patch.object(subject, 'DockerCLI', return_value=SimpleNamespace(daemon=lambda: 'daemon-a')), \
                 mock.patch.object(heavy_slots, 'measured_host_capacity', return_value={
                     'nano_cpus': 8_000_000_000, 'memory_bytes': 32 * 1024 * 1024 * 1024}), \
                 mock.patch('local_docker_scheduled.acquire', side_effect=acquire_with_capacity), \
                 mock.patch('local_docker_scheduled.host_slot_directory', return_value=slots), \
                 mock.patch('local_heavy_slots._owned_slot_marker_written', side_effect=after_owner_marker), \
                 mock.patch.object(subject.supervision, 'run_diagnostic', side_effect=launch):
                result = subject.run_supervised({'source': source}, 'bot', root, output, 'helper', 'c' * 64, {},
                                                base='b' * 40, head_ref='codex/test', resource_limits=limits)
            self.assertEqual(slot_marker_written, [(result['slot'], owner_ids[0])])
            cpu_job = (8_000_000_000 * limits['host_job']['cpu_rate'] + 9_999) // 10_000
            self.assertEqual(admission_calls, [(owner_ids[0], {
                'nano_cpus': limits['client']['nano_cpus'] + limits['daemon']['nano_cpus'] + cpu_job,
                'memory_bytes': limits['client']['memory_bytes'] + limits['daemon']['memory_bytes'] +
                                limits['host_job']['job_memory_bytes'],
                'pids_limit': limits['client']['pids_limit'] + limits['daemon']['pids_limit'] +
                              limits['host_job']['active_process_limit']}, limits['capacity_budget'])])
            self.assertEqual(result['status'], 'incomplete')
            self.assertEqual(result['input_identity']['resource_limits'], limits)
            self.assertEqual(result['resource_limit_readback']['host_job']['cpu_rate'], limits['host_job']['cpu_rate'])
            limitations = ' '.join(result.get('unbound_inputs', [])).lower()
            self.assertIn('pid', limitations)
            self.assertTrue('foreign' in limitations or 'non-cooperating' in limitations)

    def _run_interrupted_worker(self, *, probe=True, probe_error=False, change_daemon=False,
                                session_identities=None):
        limits = self.resource_limits()
        source = {'head': 'a' * 40}
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'; root.mkdir()
            output = Path(directory) / 'out'; output.mkdir()
            slots = Path(directory) / 'slots'
            engine = FakeEngine()
            owner_ids, resources, job_names = [], [], []

            def interrupted_worker(_command, *, input, job_name, **_kwargs):
                request = json.loads(input)
                owner_ids.append(request['owner_id'])
                job_names.append(job_name)
                owner = OwnedRun.load(Path(request['ownership_journal']).parent, engine)
                try:
                    resources.append(owner.create_resource('network'))
                finally:
                    owner.close()
                if change_daemon:
                    engine.identity = 'different-daemon'
                raise RuntimeError('worker outcome is uncertain')

            identity = {'boot_id': 'boot-a', 'logon_id': 'logon-a', 'session_id': 7}
            identity_patch = mock.patch.object(proof_scheduler, '_session_identity',
                side_effect=session_identities if session_identities is not None else lambda: identity)
            probe_patch = (mock.patch.object(subject.supervision, 'named_job_exists', side_effect=RuntimeError('probe uncertain'))
                           if probe_error else
                           mock.patch.object(subject.supervision, 'named_job_exists', return_value=probe))
            with mock.patch.object(subject, '_worker_command', return_value=['worker']), \
                 mock.patch.object(subject, 'DockerCLI', return_value=engine), \
                 mock.patch('local_docker_scheduled.host_slot_directory', return_value=slots), \
                 mock.patch.object(subject.supervision, 'run_diagnostic', side_effect=interrupted_worker), \
                 probe_patch as exists, identity_patch, self.assertRaisesRegex(RuntimeError, 'uncertain'):
                subject.run_supervised({'source': source}, 'bot', root, output, 'helper', 'c' * 64, {},
                                       base='b' * 40, head_ref='codex/test', resource_limits=limits)

            markers = list(slots.glob('heavy-*.lock'))
            marker = markers[0].read_bytes() if markers else b''
            return engine, slots, marker, exists, owner_ids, resources, job_names

    def test_named_job_present_or_probe_uncertain_keeps_resources_and_busy_slot(self):
        for probe_error in (False, True):
            with self.subTest(probe_error=probe_error):
                engine, _slots, marker, exists, owner_ids, resources, job_names = self._run_interrupted_worker(
                    probe=True, probe_error=probe_error)
                exists.assert_called_once_with(job_names[0])
                self.assertEqual(len(resources), 1)
                self.assertTrue(engine.exists('network', resources[0]))
                self.assertIn(owner_ids[0].encode('ascii'), marker)
                self.assertIn(b'busy:', marker)

    def test_absent_named_job_with_changed_daemon_keeps_resources_and_busy_slot(self):
        engine, _slots, marker, exists, owner_ids, resources, job_names = self._run_interrupted_worker(
            probe=False, change_daemon=True)
        exists.assert_called_once_with(job_names[0])
        self.assertEqual(len(resources), 1)
        self.assertTrue(engine.exists('network', resources[0]))
        self.assertIn(owner_ids[0].encode('ascii'), marker)
        self.assertIn(b'busy:', marker)

    def test_session_identity_change_prevents_named_job_probe_or_release(self):
        original = {'boot_id': 'boot-a', 'logon_id': 'logon-a', 'session_id': 7}
        changed = {'boot_id': 'boot-a', 'logon_id': 'logon-a', 'session_id': 8}
        engine, _slots, marker, exists, owner_ids, resources, _job_names = self._run_interrupted_worker(
            probe=False, session_identities=[original, changed])
        exists.assert_not_called()
        self.assertEqual(len(resources), 1)
        self.assertTrue(engine.exists('network', resources[0]))
        self.assertIn(owner_ids[0].encode('ascii'), marker)
        self.assertIn(b'busy:', marker)

    def test_source_mismatch_rejects_receipt_after_exact_owned_cleanup(self):
        limits = self.resource_limits()
        source = {'head': 'a' * 40}
        changed_source = {'head': 'f' * 40}
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'; root.mkdir()
            output = Path(directory) / 'out'; output.mkdir()
            slots = Path(directory) / 'slots'
            engine = FakeEngine()
            resources, owner_ids = [], []
            report = {'status': 'incomplete', 'source': changed_source,
                      'input_identity': {'helper_sha256': 'c' * 64, 'archives': {},
                                         'resource_limits': limits},
                      'resource_limit_readback': {'client': limits['client'], 'daemon': limits['daemon']}}

            def worker(command, *, input, **kwargs):
                request = json.loads(input)
                owner_ids.append(request['owner_id'])
                owner = OwnedRun.load(Path(request['ownership_journal']).parent, engine)
                try:
                    resources.append(owner.create_resource('network'))
                finally:
                    owner.close()
                (output / 'diagnostic.json').write_text(json.dumps(report), encoding='utf-8')
                completed = subprocess.CompletedProcess(command, 0, stdout=json.dumps(report).encode(), stderr=b'')
                completed.job_evidence = {'cpu_rate': 1500, 'job_memory_bytes': 3 * 1024 * 1024 * 1024,
                                          'active_process_limit': 512, 'peak_job_memory_bytes': 1024,
                                          'active_processes_after_cleanup': 0}
                return completed

            with mock.patch.object(subject, '_worker_command', return_value=['worker']), \
                 mock.patch.object(subject, 'DockerCLI', return_value=engine), \
                 mock.patch('local_docker_scheduled.host_slot_directory', return_value=slots), \
                 mock.patch.object(subject.supervision, 'run_diagnostic', side_effect=worker), \
                 mock.patch.object(subject.supervision, 'named_job_exists', return_value=False), \
                 self.assertRaises(ValueError):
                subject.run_supervised({'source': source}, 'bot', root, output, 'helper', 'c' * 64, {},
                                       base='b' * 40, head_ref='codex/test', resource_limits=limits)

            self.assertEqual(len(resources), 1)
            self.assertFalse(engine.exists('network', resources[0]))
            marker = next(slots.glob('heavy-*.lock')).read_bytes()
            self.assertNotIn(owner_ids[0].encode('ascii'), marker)
            self.assertEqual(marker[1:], b'idle\n')

    def test_source_file_mutation_during_worker_invalidates_result_after_exact_owned_cleanup(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'source'; root.mkdir()
            output = Path(directory) / 'output'
            source_file = root / 'fixture.txt'
            source_file.write_text('before', encoding='utf-8')
            head = 'a' * 40
            engine = FakeEngine()
            foreign_key = ('volume', 'foreign-sentinel')
            engine.resources[foreign_key] = {'id': 'foreign-sentinel', 'name': 'foreign-sentinel',
                                              'kind': 'volume', 'labels': {'owner': 'foreign'}}
            owned_checkout = Path(directory) / 'owned'; owned_checkout.mkdir()
            owner = OwnedRun.create(owned_checkout, engine)
            scheduled = SimpleNamespace(run=owner, close=lambda: (owner.cleanup(), owner.close()))
            captures = []

            def capture(actual_root, actual_head):
                self.assertEqual(Path(actual_root), root.resolve())
                self.assertEqual(actual_head, head)
                result = {'head': actual_head,
                          'fixture_sha256': hashlib.sha256(source_file.read_bytes()).hexdigest()}
                captures.append(result)
                return result

            def execute_worker(*args, **kwargs):
                owned = owner.create_resource('network')
                source_file.write_text('mutated while worker ran', encoding='utf-8')
                scheduled.close()
                return {'status': 'incomplete', 'source': captures[0], 'owned_resource': owned}

            archives = {tag: {'archive': str(root / 'archive'), 'archive_sha256': 'b' * 64,
                              'image_id': 'sha256:' + 'c' * 64}
                        for tag in subject.profile('bot')['images']}
            try:
                with mock.patch.object(subject.auth_broker, '_require_worker_job'), \
                     mock.patch.object(subject, 'DockerCLI', return_value=engine), \
                     mock.patch.object(subject.orchestration, '_verify_file'), \
                     mock.patch.object(subject.local_go_plan, '_check_go_toolchain', return_value='go'), \
                     mock.patch.object(subject.snapshot, 'capture', side_effect=capture), \
                     mock.patch.object(subject.snapshot, 'materialize', return_value=root / 'staged'), \
                     mock.patch.object(subject.local_go_dependencies, 'acquire', return_value={}), \
                     mock.patch.object(subject.ScheduledRun, 'start_host', return_value=scheduled), \
                     mock.patch.object(subject.chat, 'publish_owner'), \
                     mock.patch.object(subject.chat, 'run_owned', side_effect=execute_worker), \
                     self.assertRaisesRegex(ValueError, 'source changed during cleanup'):
                    subject.run('bot', root, head, output, 'helper', 'd' * 64, archives, 'go')
                self.assertEqual(len(captures), 2)
                self.assertNotEqual(captures[0], captures[1])
                self.assertFalse(any(key[0] == 'network' for key in engine.resources))
                self.assertIn(foreign_key, engine.resources)
                self.assertFalse((output / 'diagnostic.json').exists())
            finally:
                owner.close()

    def test_supervised_worker_cannot_run_inline_or_accept_forged_receipt(self):
        plan = {'base': 'b' * 40, 'source': {'head': 'a' * 40},
                'checks': {'backend-go-integration-pr': ['bot']}, 'components': {'go_integration': ['bot']}}
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory) / 'out'
            fake = subprocess.CompletedProcess([], 0, stdout=b'{"status":"pass"}', stderr=b'')
            with mock.patch.object(subject.supervision, 'run_diagnostic', return_value=fake) as broker, \
                 mock.patch.object(subject, 'run_selected') as inline, self.assertRaises(ValueError):
                subject.run_supervised(plan, 'bot', Path(directory), output, 'helper', 'c' * 64, {},
                                       base='b' * 40, head_ref='codex/test', resource_limits=self.resource_limits())
            broker.assert_called_once()
            inline.assert_not_called()
            request = json.loads(broker.call_args.kwargs['input'])
            self.assertEqual(request['plan'], plan)
            self.assertEqual(request['service'], 'bot')
            self.assertEqual(request['resource_limits'], self.resource_limits())
    def test_profiles_are_explicit_and_unsupported_services_refuse(self):
        self.assertEqual(subject.profile('voice')['images'],
                         ('postgres:16-bookworm', 'redis:7.4-bookworm', 'nats:2.12-alpine'))
        self.assertEqual(subject.profile('voice')['module'], 'src/backend/voice')
        self.assertEqual(subject.profile('notification')['images'], ('postgres:16-bookworm', 'redis:7-alpine'))
        self.assertEqual(subject.profile('notification')['module'], 'src/backend/notification')

    def test_notification_acquires_both_source_bound_module_closures(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'; root.mkdir()
            source = {'head': 'a' * 40}
            with mock.patch.object(subject, 'os', SimpleNamespace(name='nt')), \
                 mock.patch.object(subject.orchestration, '_verify_file'), \
                 mock.patch.object(subject.local_go_plan, '_check_go_toolchain', return_value='go'), \
                 mock.patch.object(subject.snapshot, 'capture', return_value=source), \
                 mock.patch.object(subject.snapshot, 'materialize', return_value=Path(directory) / 'copy'), \
                 mock.patch.object(subject.local_go_dependencies, 'acquire') as single, \
                 mock.patch.object(subject.local_go_multi_dependencies, 'acquire', return_value={'module_dirs': ['src/backend/notification', 'src/backend/user']}) as multi, \
                 mock.patch.object(subject, 'DockerCLI'), \
                 mock.patch.object(subject.ScheduledRun, 'start_host', return_value=mock.Mock()), \
                 mock.patch.object(subject.chat, 'publish_owner'), \
                 mock.patch.object(subject.chat, 'run_owned', return_value={'status': 'incomplete'}) as execute:
                subject.run('notification', root, source['head'], Path(directory) / 'out', 'helper', 'b' * 64,
                            {tag: {'archive': str(root / str(i)), 'archive_sha256': 'c' * 64,
                                   'image_id': 'sha256:' + 'd' * 64}
                             for i, tag in enumerate(subject.profile('notification')['images'])})
            single.assert_not_called()
            multi.assert_called_once()
            self.assertEqual(execute.call_args.kwargs['service'], 'notification')
        self.assertEqual(subject.profile('bot')['images'], ('postgres:16-bookworm', 'nats:2.10-alpine'))
        self.assertEqual(subject.profile('search')['images'], ('postgres:16-bookworm',))
        self.assertEqual(subject.profile('realtime')['images'], ())
        self.assertEqual(subject.profile('analytics')['images'], ())
        self.assertEqual(subject.profile('user')['images'], ('postgres:16-bookworm', 'migrate/migrate:v4.18.1'))
        self.assertEqual(subject.profile('gameintegration')['images'], ('postgres:16-bookworm', 'migrate/migrate:v4.18.1'))
        self.assertEqual(subject.profile('role')['images'], ('postgres:16-bookworm', 'migrate/migrate:v4.18.1',
                                                             'redis:7-alpine'))
        self.assertEqual(subject.profile('messaging')['images'], ('postgres:16-bookworm', 'migrate/migrate:v4.18.1',
                                                                  'redis:7-alpine'))
        for service in ('controlledgame', 'moderation', 'social', 'subscription', 'story'):
            with self.subTest(service=service):
                self.assertEqual(subject.profile(service)['images'], ('postgres:16-bookworm',))
                self.assertEqual(subject.profile(service)['module'], 'src/backend/' + service)
        self.assertEqual(subject.profile('chat')['images'], subject.chat.IMAGES)
        for name in ('gateway', 'file', '../bot', 'unknown'):
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

    def test_empty_image_set_is_valid_only_for_declared_profiles(self):
        owner = mock.Mock()
        for service in ('realtime', 'analytics'):
            subject.chat.validate_archives({}, subject.profile(service)['images'])
            subject.chat.verify_images(owner, 'client', {}, subject.profile(service)['images'])
        owner.execute.assert_not_called()
        for service in ('bot', 'user', 'chat', 'search', 'gameintegration', 'role', 'messaging', 'voice'):
            with self.subTest(service=service), self.assertRaises(ValueError):
                subject.chat.validate_archives({}, subject.profile(service)['images'])
        with self.assertRaises(ValueError):
            subject.chat.validate_archives({'unexpected': {}}, subject.profile('realtime')['images'])
        with tempfile.TemporaryDirectory() as directory:
            for service in ('gameintegration', 'role', 'messaging', 'voice'):
                tags = subject.profile(service)['images']
                archives = {tag: {'archive': str(Path(directory) / str(index)), 'archive_sha256': 'a' * 64,
                                  'image_id': 'sha256:' + 'b' * 64} for index, tag in enumerate(tags)}
                subject.chat.validate_archives(archives, tags)
                archives.pop(tags[-1])
                with self.subTest(service=service), self.assertRaises(ValueError):
                    subject.chat.validate_archives(archives, tags)

    def test_new_profiles_require_their_own_source_inventory(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            path = root / 'scripts/ci/go_integration_required.json'
            path.parent.mkdir(parents=True)
            for service in ('user', 'realtime', 'analytics', 'gameintegration', 'role', 'messaging', 'notification', 'voice'):
                package = 'voice/backend/' + service
                path.write_text(json.dumps({service: [[package, 'TestRequired']]}), encoding='utf-8')
                self.assertEqual(subject.read_required(root, service, [package]), [[package, 'TestRequired']])
                with self.assertRaises(ValueError):
                    subject.read_required(root, service, ['voice/backend/other'])
                with mock.patch.object(subject.chat, 'run_full', return_value={'status': 'incomplete'}) as run:
                    subject.run_profile(service, 'owner', 'daemon', 'client', root, {}, root, 'a' * 64,
                                        {tag: 'sha256:' + 'b' * 64 for tag in subject.profile(service)['images']},
                                        root / 'out')
                self.assertEqual(run.call_args.kwargs, {'service': service})

    def test_analytics_report_keeps_clickhouse_gap_explicit(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'
            root.mkdir()
            output = Path(directory) / 'output'
            source = {'head': 'a' * 40}
            with mock.patch.object(subject, 'os', SimpleNamespace(name='nt')), \
                 mock.patch.object(subject.orchestration, '_verify_file'), \
                 mock.patch.object(subject.local_go_plan, '_check_go_toolchain', return_value='go'), \
                 mock.patch.object(subject.snapshot, 'capture', return_value=source), \
                 mock.patch.object(subject.snapshot, 'materialize', return_value=Path(directory) / 'copy'), \
                 mock.patch.object(subject.local_go_dependencies, 'acquire', return_value={}), \
                 mock.patch.object(subject, 'DockerCLI'), \
                 mock.patch.object(subject.ScheduledRun, 'start_host'), \
                 mock.patch.object(subject.chat, 'publish_owner'), \
                 mock.patch.object(subject.chat, 'run_owned', return_value={'status': 'incomplete', 'source': source}):
                report = subject.run('analytics', root, source['head'], output, 'helper', 'a' * 64, {})
            self.assertIn('tagged Analytics ClickHouse schema, fixture and dedicated job', report['unbound_inputs'])
            self.assertEqual(report['status'], 'incomplete')

    def test_messaging_report_keeps_dedicated_fixture_gaps(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'; root.mkdir()
            output, source = Path(directory) / 'output', {'head': 'a' * 40}
            archives = {tag: {'archive': str(Path(directory) / str(i)), 'archive_sha256': 'b' * 64,
                              'image_id': 'sha256:' + 'c' * 64}
                        for i, tag in enumerate(subject.profile('messaging')['images'])}
            with mock.patch.object(subject, 'os', SimpleNamespace(name='nt')), \
                 mock.patch.object(subject.orchestration, '_verify_file'), \
                 mock.patch.object(subject.local_go_plan, '_check_go_toolchain', return_value='go'), \
                 mock.patch.object(subject.snapshot, 'capture', return_value=source), \
                 mock.patch.object(subject.snapshot, 'materialize', return_value=Path(directory) / 'copy'), \
                 mock.patch.object(subject.local_go_dependencies, 'acquire', return_value={}), \
                 mock.patch.object(subject, 'DockerCLI'), \
                 mock.patch.object(subject.ScheduledRun, 'start_host'), \
                 mock.patch.object(subject.chat, 'publish_owner'), \
                 mock.patch.object(subject.chat, 'run_owned', return_value={'status': 'incomplete', 'source': source}):
                report = subject.run('messaging', root, source['head'], output, 'helper', 'a' * 64, archives)
            self.assertIn('T16 cross-service, nonroot mounted-key, threadlist DSN and BE131 projection dedicated fixtures and jobs',
                          report['unbound_inputs'])
            self.assertEqual(report['status'], 'incomplete')

    def test_voice_report_keeps_real_redis_and_lifecycle_gap(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'; root.mkdir()
            output, source = Path(directory) / 'output', {'head': 'a' * 40}
            archives = {tag: {'archive': str(Path(directory) / str(i)), 'archive_sha256': 'b' * 64,
                              'image_id': 'sha256:' + 'c' * 64}
                        for i, tag in enumerate(subject.profile('voice')['images'])}
            with mock.patch.object(subject, 'os', SimpleNamespace(name='nt')), \
                 mock.patch.object(subject.orchestration, '_verify_file'), \
                 mock.patch.object(subject.local_go_plan, '_check_go_toolchain', return_value='go'), \
                 mock.patch.object(subject.snapshot, 'capture', return_value=source), \
                 mock.patch.object(subject.snapshot, 'materialize', return_value=Path(directory) / 'copy'), \
                 mock.patch.object(subject.local_go_dependencies, 'acquire', return_value={}), \
                 mock.patch.object(subject, 'DockerCLI'), \
                 mock.patch.object(subject.ScheduledRun, 'start_host'), \
                 mock.patch.object(subject.chat, 'publish_owner'), \
                 mock.patch.object(subject.chat, 'run_owned', return_value={'status': 'incomplete', 'source': source}):
                report = subject.run('voice', root, source['head'], output, 'helper', 'a' * 64, archives)
            self.assertIn('Voice R18 real Redis endpoint and dedicated Voice ledger/lifecycle integration jobs', report['unbound_inputs'])
            self.assertEqual(report['status'], 'incomplete')

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
                self.assertIn('exec env -i PATH=', command[2])
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

    def test_notification_checks_user_dependency_cache_after_full_suite(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory) / 'root'; root.mkdir()
            copy = Path(directory) / 'copy'
            inventory_path = copy / 'scripts/ci/go_integration_required.json'
            inventory_path.parent.mkdir(parents=True)
            package = 'voice/backend/notification'
            inventory_path.write_text(json.dumps({'notification': [[package, 'TestRequired']]}), encoding='utf-8')
            events = [dict(Package=package, Action='start'), dict(Package=package, Test='TestRequired', Action='run'),
                      dict(Package=package, Test='TestRequired', Action='pass'), dict(Package=package, Action='pass')]
            owner = mock.Mock()
            def execute(client, command, out, err, timeout):
                self.assertIn('cd /workspace/source/src/backend/notification', command[2])
                self.assertNotIn('-short', command[2])
                out.write(b''.join((json.dumps(event) + '\n').encode() for event in events))
                return 0
            owner.execute.side_effect = execute
            source = {'head': 'a' * 40}
            inventory = {'packages_with_tests': [package], 'packages_without_tests': []}
            with mock.patch.object(subject.chat.local_chat_private, 'inspect_pair', return_value='private'), \
                 mock.patch.object(subject.chat, 'verify_images'), \
                 mock.patch.object(subject.chat.local_go_execution, 'discover_packages', return_value=inventory), \
                 mock.patch.object(subject.chat.local_go_execution, '_verify_modules') as verify, \
                 mock.patch.object(subject.chat.local_go_execution, '_verify_workspace'), \
                 mock.patch.object(subject.chat.snapshot, 'verify_materialized'), \
                 mock.patch.object(subject.chat.snapshot, 'capture', return_value=source):
                result = subject.run_profile('notification', owner, 'daemon', 'client', root, source, copy,
                                             'b' * 64, {tag: 'sha256:' + 'c' * 64 for tag in subject.profile('notification')['images']},
                                             Path(directory) / 'output')
            self.assertEqual(result['required_count'], 1)
            self.assertEqual([call.args[2] for call in verify.call_args_list], ['src/backend/notification', 'src/backend/user'])

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

    def test_postgres_profiles_pass_only_fixed_service_to_shared_executor(self):
        for service in ('controlledgame', 'moderation', 'social', 'subscription', 'story'):
            with self.subTest(service=service), mock.patch.object(subject.chat, 'run_full',
                                                               return_value={'status': 'incomplete'}) as run:
                subject.run_profile(service, 'owner', 'daemon', 'client', Path('.'), {}, Path('.'),
                                    'a' * 64, {}, Path('out'))
                self.assertEqual(run.call_args.kwargs, {'service': service})

    def test_repository_profile_inventories_are_nonempty_and_service_scoped(self):
        root = Path(__file__).resolve().parents[2]
        inventory = json.loads((root / 'scripts/ci/go_integration_required.json').read_text(encoding='utf-8'))
        for service in ('bot', 'search', 'controlledgame', 'moderation', 'social', 'subscription', 'story',
                        'realtime', 'analytics', 'user', 'gameintegration', 'role', 'messaging', 'voice'):
            packages = sorted({item[0] for item in inventory[service]})
            with self.subTest(service=service):
                required = subject.read_required(root, service, packages)
                self.assertEqual(required, inventory[service])
                self.assertTrue(required)
                if service == 'analytics':
                    self.assertEqual(required, [['voice/backend/analytics/internal/adapters', 'TestMapperFromMessageSent']])

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
