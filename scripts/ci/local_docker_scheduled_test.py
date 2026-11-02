"""The heavy slot must outlive the exact owned fixture cleanup."""
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import local_ci_proofs_scheduler as proof_scheduler
import local_heavy_slots as heavy_slots
from local_docker_owned import OwnedRun, OwnershipError
from local_heavy_slots import acquire as acquire_slot
from local_docker_owned_test import FakeEngine
from local_docker_scheduled import ScheduledRun


class ScheduledRunTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.slots = self.root / 'host-slots'
        self.engine = FakeEngine()

    def test_host_scheduler_shares_slots_across_checkouts_and_recovers_exact_owner(self):
        other_checkout = self.root / 'other-checkout'
        other_checkout.mkdir()
        with mock.patch('local_docker_scheduled.host_slot_directory', return_value=self.slots):
            one = ScheduledRun.start_host(self.root, self.engine, 0, capacity=1)
            self.addCleanup(one.abandon)
            resource = one.run.create_resource('network')
            with self.assertRaises(TimeoutError):
                ScheduledRun.start_host(other_checkout, self.engine, 0, capacity=1)
            directory, slot = one.run.directory, one.slot
            one.abandon()
            with mock.patch('local_docker_scheduled.host_slot_directory', return_value=self.slots):
                ScheduledRun.recover_host(directory, self.engine, slot, capacity=1)
            self.assertNotIn(('network', resource), self.engine.resources)
            later = ScheduledRun.start_host(other_checkout, self.engine, 0, capacity=1)
            later.close()

    def test_durable_run_identity_precedes_owner_bound_slot_acquisition(self):
        events = []
        create = OwnedRun.create.__func__

        def create_with_receipt(cls, checkout, engine):
            run = create(cls, checkout, engine)
            journal = json.loads((run.directory / 'ownership.json').read_text(encoding='utf-8'))
            self.assertEqual(journal['run_id'], run.run_id)
            events.append(('journal', run.run_id))
            return run

        def acquire_after_journal(directory, timeout, capacity=2, *, owner_id=None, **kwargs):
            events.append(('acquire', owner_id))
            self.assertTrue(events and events[0][0] == 'journal')
            self.assertEqual(owner_id, events[0][1])
            return acquire_slot(directory, timeout, capacity=capacity, owner_id=owner_id)

        with mock.patch.object(OwnedRun, 'create', classmethod(create_with_receipt)), \
             mock.patch('local_docker_scheduled.acquire', side_effect=acquire_after_journal):
            scheduled = ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        self.assertEqual(events[0][0], 'journal')
        self.assertEqual(events[1], ('acquire', events[0][1]))
        self.assertEqual(scheduled.run.run_id, events[0][1])
        scheduled.close()

    def test_positive_named_job_recovery_cleans_exact_owner_before_releasing_reservation(self):
        owner_id = 'voiceci-' + 'd' * 32
        identity = {'boot_id': 'boot-a', 'logon_id': 'logon-a', 'session_id': 7}
        limits = {'cpu_rate': 1000, 'job_memory_bytes': 2048, 'active_process_limit': 128}
        host_job = {'schema': 1, 'slot': None,
                    'job_names': ['Local\\VoiceCIProof-Go-' + owner_id.removeprefix('voiceci-')],
                    'limits': limits, 'session_identity': identity,
                    'source_identity': {'root': str(self.root.resolve()), 'head': 'a' * 40,
                                        'sha256': 'b' * 64}}
        budget = {'nano_cpus': 4_000_000_000, 'memory_bytes': 8_000, 'pids_limit': 512}
        claim = {'nano_cpus': 2_000_000_000, 'memory_bytes': 4_000, 'pids_limit': 256}
        owner = OwnedRun.create(self.root, self.engine, run_id=owner_id, host_job=host_job)
        lease = None
        try:
            lease = acquire_slot(self.slots, 0, capacity=1, owner_id=owner_id,
                                 resource_claim=claim, capacity_budget=budget)
            owner.state['host_job']['slot'] = lease.slot
            owner._save()
            owned_name = owner.create_resource('network')
            foreign_key = ('volume', 'foreign-sentinel')
            self.engine.resources[foreign_key] = {'id': 'foreign-sentinel', 'name': 'foreign-sentinel',
                                                  'kind': 'volume', 'labels': {'owner': 'foreign'}}
            directory, slot = owner.directory, lease.slot
        finally:
            owner.close()
            if lease is not None:
                lease.abandon()

        with mock.patch.object(proof_scheduler, '_session_identity', return_value=identity), \
             mock.patch('local_docker_scheduled.supervision.named_job_exists', return_value=False) as job_exists:
            ScheduledRun.recover_host(directory, self.engine, slot, capacity=1, slot_directory=self.slots)
        self.assertEqual(job_exists.call_args_list, [mock.call(host_job['job_names'][0]),
                                                     mock.call(host_job['job_names'][0])])
        self.assertNotIn(('network', owned_name), self.engine.resources)
        self.assertIn(foreign_key, self.engine.resources)
        marker = (self.slots / ('heavy-' + str(slot) + '.lock')).read_bytes()
        self.assertEqual(marker[1:], b'idle\n')
        self.assertIsNone(heavy_slots.active_resource_reservation(self.slots, owner_id))

    def test_recover_host_recovers_reserved_idle_marker_after_acquire_crash(self):
        owner_id = 'voiceci-' + 'c' * 32
        identity = {'boot_id': 'boot-a', 'logon_id': 'logon-a', 'session_id': 7}
        limits = {'cpu_rate': 1000, 'job_memory_bytes': 2048, 'active_process_limit': 128}
        host_job = {'schema': 1, 'slot': None,
                    'job_names': ['Local\\VoiceCIProof-Go-' + owner_id.removeprefix('voiceci-')],
                    'limits': limits, 'session_identity': identity,
                    'source_identity': {'root': str(self.root.resolve()), 'head': 'a' * 40,
                                        'sha256': 'b' * 64}}
        budget = {'nano_cpus': 4_000_000_000, 'memory_bytes': 8_000, 'pids_limit': 512}
        claim = {'nano_cpus': 2_000_000_000, 'memory_bytes': 4_000, 'pids_limit': 256}
        original = heavy_slots._write_marker

        def crash_after_reservation(descriptor, value):
            if value.startswith(b'busy:'):
                raise KeyboardInterrupt('simulated process death after durable reservation')
            return original(descriptor, value)

        with mock.patch.object(heavy_slots, '_write_marker', side_effect=crash_after_reservation), \
             self.assertRaisesRegex(KeyboardInterrupt, 'durable reservation'):
            ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1,
                run_id=owner_id, resource_claim=claim, capacity_budget=budget,
                host_job=host_job)

        directory = self.root / 'tmp' / 'ci-local' / owner_id
        self.assertTrue(directory.is_dir())
        self.assertEqual(heavy_slots.resource_reservation_slot(self.slots, owner_id), 0)
        self.assertEqual((self.slots / 'heavy-0.lock').read_bytes()[1:], b'idle\n')
        with mock.patch.object(proof_scheduler, '_session_identity', return_value=identity), \
             mock.patch('local_docker_scheduled.supervision.named_job_exists', return_value=False):
            ScheduledRun.recover_host(directory, self.engine, slot=None, capacity=1,
                                      slot_directory=self.slots)
        self.assertIsNone(heavy_slots.active_resource_reservation(self.slots, owner_id))
        self.assertEqual((self.slots / 'heavy-0.lock').read_bytes()[1:], b'idle\n')

    def test_recover_host_recovers_idle_marker_after_release_crash(self):
        owner_id = 'voiceci-' + 'e' * 32
        identity = {'boot_id': 'boot-a', 'logon_id': 'logon-a', 'session_id': 7}
        limits = {'cpu_rate': 1000, 'job_memory_bytes': 2048, 'active_process_limit': 128}
        host_job = {'schema': 1, 'slot': None,
                    'job_names': ['Local\\VoiceCIProof-Go-' + owner_id.removeprefix('voiceci-')],
                    'limits': limits, 'session_identity': identity,
                    'source_identity': {'root': str(self.root.resolve()), 'head': 'a' * 40,
                                        'sha256': 'b' * 64}}
        budget = {'nano_cpus': 4_000_000_000, 'memory_bytes': 8_000, 'pids_limit': 512}
        claim = {'nano_cpus': 2_000_000_000, 'memory_bytes': 4_000, 'pids_limit': 256}
        scheduled = ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1,
            run_id=owner_id, resource_claim=claim, capacity_budget=budget, host_job=host_job)
        directory, slot = scheduled.run.directory, scheduled.slot
        with mock.patch.object(proof_scheduler, '_session_identity', return_value=identity), \
             mock.patch('local_docker_scheduled.supervision.named_job_exists', return_value=False), \
             mock.patch.object(heavy_slots, '_release_reservation_locked',
                               side_effect=OSError('simulated crash after idle marker')):
            with self.assertRaisesRegex(OSError, 'after idle marker'):
                scheduled.close()
        self.assertEqual((self.slots / ('heavy-' + str(slot) + '.lock')).read_bytes()[1:], b'idle\n')
        self.assertIsNotNone(heavy_slots.active_resource_reservation(self.slots, owner_id))
        with mock.patch.object(proof_scheduler, '_session_identity', return_value=identity), \
             mock.patch('local_docker_scheduled.supervision.named_job_exists', return_value=False):
            ScheduledRun.recover_host(directory, self.engine, slot, capacity=1, slot_directory=self.slots)
        self.assertIsNone(heavy_slots.active_resource_reservation(self.slots, owner_id))
        self.assertEqual((self.slots / ('heavy-' + str(slot) + '.lock')).read_bytes()[1:], b'idle\n')

    def test_replacement_container_id_keeps_journal_and_host_slot_for_recovery(self):
        owner_id = 'voiceci-' + 'f' * 32
        identity = {'boot_id': 'boot-a', 'logon_id': 'logon-a', 'session_id': 7}
        limits = {'cpu_rate': 1000, 'job_memory_bytes': 2048, 'active_process_limit': 128}
        host_job = {'schema': 1, 'slot': None,
                    'job_names': ['Local\\VoiceCIProof-Go-' + owner_id.removeprefix('voiceci-')],
                    'limits': limits, 'session_identity': identity,
                    'source_identity': {'root': str(self.root.resolve()), 'head': 'a' * 40,
                                        'sha256': 'b' * 64}}
        budget = {'nano_cpus': 4_000_000_000, 'memory_bytes': 8_000, 'pids_limit': 512}
        claim = {'nano_cpus': 2_000_000_000, 'memory_bytes': 4_000, 'pids_limit': 256}
        client_limits = {'nano_cpus': 1_000_000_000, 'memory_bytes': 4096,
                         'memory_swap_bytes': 4096, 'pids_limit': 32}
        scheduled = ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1,
            run_id=owner_id, resource_claim=claim, capacity_budget=budget, host_job=host_job)
        network = scheduled.run.create_resource('network')
        volume = scheduled.run.create_resource('volume')
        name = scheduled.run.create_container('sha256:' + 'd' * 64, network, {'/data': volume}, [],
            command=['sleep', 'infinity'], resource_limits=client_limits)
        item = scheduled.run.state['resources'][-1]
        original_id = item['id']
        item['state'] = 'creating'
        item.pop('verified_resource_limits', None)
        scheduled.run._save()
        replacement_id = 'f' + 'e' * 63
        self.engine.resources[('container', name)]['id'] = replacement_id
        directory, slot = scheduled.run.directory, scheduled.slot

        with mock.patch.object(proof_scheduler, '_session_identity', return_value=identity), \
             mock.patch('local_docker_scheduled.supervision.named_job_exists', return_value=False):
            with self.assertRaisesRegex(OwnershipError, 'identity changed'):
                scheduled.close()
            with self.assertRaisesRegex(OwnershipError, 'identity changed'):
                ScheduledRun.recover_host(directory, self.engine, slot, capacity=1,
                                          slot_directory=self.slots)

        self.engine.identity = 'daemon-replacement'
        with self.assertRaisesRegex(OwnershipError, 'daemon identity changed'):
            ScheduledRun.recover_host(directory, self.engine, slot, capacity=1,
                                      slot_directory=self.slots)

        self.assertEqual(item['id'], original_id)
        self.assertEqual(item['state'], 'creating')
        self.assertTrue(self.engine.exists('container', replacement_id))
        self.assertEqual(self.engine.started, [])
        self.assertNotIn(('container', name), self.engine.removed)
        marker = (self.slots / ('heavy-' + str(slot) + '.lock')).read_bytes()[1:]
        self.assertTrue(marker.endswith((':' + owner_id + '\n').encode()))

    def test_worker_close_keeps_owned_resources_and_slot_for_parent_job_verification(self):
        owner_id = 'voiceci-' + 'e' * 32
        identity = {'boot_id': 'boot-a', 'logon_id': 'logon-a', 'session_id': 7}
        limits = {'cpu_rate': 1000, 'job_memory_bytes': 2048, 'active_process_limit': 128}
        host_job = {'schema': 1, 'slot': None,
                    'job_names': ['Local\\VoiceCIProof-Go-' + owner_id.removeprefix('voiceci-')],
                    'limits': limits, 'session_identity': identity,
                    'source_identity': {'root': str(self.root.resolve()), 'head': 'a' * 40,
                                        'sha256': 'b' * 64}}
        owner = OwnedRun.create(self.root, self.engine, run_id=owner_id, host_job=host_job)
        lease = acquire_slot(self.slots, 0, capacity=1, owner_id=owner_id)
        owner.state['host_job']['slot'] = lease.slot
        owner._save()
        name = owner.create_resource('network')
        directory, slot = owner.directory, lease.slot
        owner.close()
        lease.abandon()

        worker = ScheduledRun.attach_worker(directory, self.engine, slot, owner_id, None)
        self.addCleanup(worker.close)
        self.assertEqual(worker.slot, slot)
        with mock.patch('local_docker_scheduled.supervision.named_job_exists',
                        side_effect=AssertionError('worker cannot prove its own Job absent')):
            worker.close()
        self.assertIn(('network', name), self.engine.resources)
        self.assertTrue((self.slots / ('heavy-' + str(slot) + '.lock')).read_bytes()[1:].endswith(
            (':' + owner_id + '\n').encode()))
        with mock.patch.object(proof_scheduler, '_session_identity', return_value=identity), \
             mock.patch('local_docker_scheduled.supervision.named_job_exists', return_value=False):
            ScheduledRun.recover_host(directory, self.engine, slot, capacity=1, slot_directory=self.slots)
        self.assertNotIn(('network', name), self.engine.resources)

    def test_close_returns_lock_scoped_release_evidence_even_after_slot_is_reused(self):
        owner_id = 'voiceci-' + '9' * 32
        identity = {'boot_id': 'boot-a', 'logon_id': 'logon-a', 'session_id': 7}
        limits = {'cpu_rate': 1000, 'job_memory_bytes': 2048, 'active_process_limit': 128}
        host_job = {'schema': 1, 'slot': None,
                    'job_names': ['Local\\VoiceCIProof-Go-' + owner_id.removeprefix('voiceci-')],
                    'limits': limits, 'session_identity': identity,
                    'source_identity': {'root': str(self.root.resolve()), 'head': 'a' * 40,
                                        'sha256': 'b' * 64}}
        budget = {'nano_cpus': 4_000_000_000, 'memory_bytes': 8_000, 'pids_limit': 512}
        claim = {'nano_cpus': 2_000_000_000, 'memory_bytes': 4_000, 'pids_limit': 256}
        with mock.patch('local_docker_scheduled.host_slot_directory', return_value=self.slots), \
             mock.patch.object(proof_scheduler, '_session_identity', return_value=identity), \
             mock.patch('local_docker_scheduled.supervision.named_job_exists', return_value=False):
            scheduled = ScheduledRun.start_host(self.root, self.engine, 0, capacity=1, run_id=owner_id,
                resource_claim=claim, capacity_budget=budget, host_job=host_job)
            owner_slot = scheduled.slot
            release = scheduled.close()
            peer = ScheduledRun.start_host(self.root, self.engine, 0, capacity=1)
            self.addCleanup(peer.close)
        self.assertEqual(release['slot_release'], {'slot': owner_slot, 'owner_id': owner_id,
            'marker_idle': True, 'reservation_released': True})
        peer.close()

    def test_parent_close_requires_named_job_absence_before_cleanup(self):
        owner_id = 'voiceci-' + 'f' * 32
        identity = {'boot_id': 'boot-a', 'logon_id': 'logon-a', 'session_id': 7}
        limits = {'cpu_rate': 1000, 'job_memory_bytes': 2048, 'active_process_limit': 128}
        host_job = {'schema': 1, 'slot': None,
                    'job_names': ['Local\\VoiceCIProof-Go-' + owner_id.removeprefix('voiceci-')],
                    'limits': limits, 'session_identity': identity,
                    'source_identity': {'root': str(self.root.resolve()), 'head': 'a' * 40,
                                        'sha256': 'b' * 64}}
        owner = OwnedRun.create(self.root, self.engine, run_id=owner_id, host_job=host_job)
        lease = acquire_slot(self.slots, 0, capacity=1, owner_id=owner_id)
        owner.state['host_job']['slot'] = lease.slot
        owner._save()
        name = owner.create_resource('network')
        directory, slot = owner.directory, lease.slot
        parent = ScheduledRun(owner, lease, self.engine, self.slots)
        with mock.patch.object(proof_scheduler, '_session_identity', return_value=identity), \
             mock.patch('local_docker_scheduled.supervision.named_job_exists', return_value=True):
            with self.assertRaisesRegex(OwnershipError, 'still present'):
                parent.close()
        self.assertIn(('network', name), self.engine.resources)
        self.assertTrue((self.slots / ('heavy-' + str(slot) + '.lock')).read_bytes()[1:].endswith(
            (':' + owner_id + '\n').encode()))
        with mock.patch.object(proof_scheduler, '_session_identity', return_value=identity), \
             mock.patch('local_docker_scheduled.supervision.named_job_exists', return_value=False):
            ScheduledRun.recover_host(directory, self.engine, slot, capacity=1, slot_directory=self.slots)
        self.assertNotIn(('network', name), self.engine.resources)

    def test_recovers_a_created_resource_after_lost_create_response(self):
        one = ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        self.addCleanup(one.abandon)
        directory, slot = one.run.directory, one.slot
        self.engine.uncertain_create = True
        with self.assertRaises(OwnershipError):
            one.run.create_resource('volume')
        item = dict(one.run.state['resources'][-1])
        one.abandon()
        self.engine.uncertain_create = False
        ScheduledRun.recover(directory, self.slots, self.engine, slot, capacity=1)
        self.assertNotIn(('volume', item['name']), self.engine.resources)
        later = ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        later.close()

    def test_uncertain_create_with_foreign_or_missing_resource_keeps_slot(self):
        for change in ('foreign', 'missing'):
            with self.subTest(change=change), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                slots = root / 'slots'
                engine = FakeEngine()
                one = ScheduledRun.start(root, slots, engine, 0, capacity=1)
                engine.uncertain_create = True
                with self.assertRaises(OwnershipError):
                    one.run.create_resource('volume')
                item = dict(one.run.state['resources'][-1])
                run_directory, slot = one.run.directory, one.slot
                one.abandon()
                key = ('volume', item['name'])
                if change == 'foreign':
                    engine.resources[key]['labels']['voice.ci.run'] = 'foreign'
                else:
                    del engine.resources[key]
                with self.assertRaises(OwnershipError):
                    ScheduledRun.recover(run_directory, slots, engine, slot, capacity=1)
                self.assertEqual(engine.removed, [])
                with self.assertRaises(TimeoutError):
                    ScheduledRun.start(root, slots, engine, 0, capacity=1)

    def test_lost_create_recovery_preserves_another_run_and_foreign_resource(self):
        one = ScheduledRun.start(self.root, self.slots, self.engine, 0)
        other = ScheduledRun.start(self.root, self.slots, self.engine, 0)
        self.addCleanup(one.abandon)
        self.addCleanup(other.abandon)
        other_network = other.run.create_resource('network')
        foreign_key = ('volume', 'foreign-sentinel')
        self.engine.resources[foreign_key] = {'id': 'foreign-sentinel', 'name': 'foreign-sentinel',
                                              'kind': 'volume', 'labels': {'owner': 'other'}}
        self.engine.uncertain_create = True
        with self.assertRaises(OwnershipError):
            one.run.create_resource('volume')
        name = one.run.state['resources'][-1]['name']
        directory, slot = one.run.directory, one.slot
        one.abandon()
        self.engine.uncertain_create = False
        ScheduledRun.recover(directory, self.slots, self.engine, slot)
        self.assertNotIn(('volume', name), self.engine.resources)
        self.assertIn(('network', other_network), self.engine.resources)
        self.assertIn(foreign_key, self.engine.resources)
        other.close()
        self.assertIn(foreign_key, self.engine.resources)

    def test_lost_container_create_requires_exact_inspected_specification(self):
        one = ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        self.addCleanup(one.abandon)
        network = one.run.create_resource('network')
        volume = one.run.create_resource('volume')
        self.engine.uncertain_container_create = True
        with self.assertRaises(OwnershipError):
            one.run.create_container('sha256:' + 'd' * 64, network, {'/data': volume}, [],
                                     command=['sleep', 'infinity'])
        name = one.run.state['resources'][-1]['name']
        directory, slot = one.run.directory, one.slot
        one.abandon()
        self.engine.uncertain_container_create = False
        container = self.engine.resources[('container', name)]
        container['image'] = 'sha256:' + 'e' * 64
        with self.assertRaises(OwnershipError):
            ScheduledRun.recover(directory, self.slots, self.engine, slot, capacity=1)
        self.assertEqual(self.engine.removed, [])
        container['image'] = 'sha256:' + 'd' * 64
        ScheduledRun.recover(directory, self.slots, self.engine, slot, capacity=1)
        self.assertNotIn(('container', name), self.engine.resources)

    def test_lost_create_recovery_refuses_daemon_change_during_inspection(self):
        one = ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        self.addCleanup(one.abandon)
        self.engine.uncertain_create = True
        with self.assertRaises(OwnershipError):
            one.run.create_resource('volume')
        item = dict(one.run.state['resources'][-1])
        directory, slot = one.run.directory, one.slot
        one.abandon()
        original = self.engine.inspect
        def changed_daemon(kind, name):
            result = original(kind, name)
            self.engine.identity = 'different-daemon'
            return result
        self.engine.inspect = changed_daemon
        with self.assertRaises(OwnershipError):
            ScheduledRun.recover(directory, self.slots, self.engine, slot, capacity=1)
        self.assertEqual(self.engine.removed, [])
        self.assertIn(('volume', item['name']), self.engine.resources)
        with self.assertRaises(TimeoutError):
            ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        self.engine.identity = 'daemon-a'
        self.engine.inspect = original
        self.engine.uncertain_create = False
        ScheduledRun.recover(directory, self.slots, self.engine, slot, capacity=1)

    def test_two_runs_hold_slots_until_own_cleanup(self):
        one = ScheduledRun.start(self.root, self.slots, self.engine, 0)
        two = ScheduledRun.start(self.root, self.slots, self.engine, 0)
        self.addCleanup(one.abandon)
        self.addCleanup(two.abandon)
        first = one.run.create_resource('network')
        second = two.run.create_resource('volume')
        with self.assertRaises(TimeoutError):
            ScheduledRun.start(self.root, self.slots, self.engine, 0)
        one.close()
        self.assertNotIn(('network', first), self.engine.resources)
        self.assertIn(('volume', second), self.engine.resources)
        three = ScheduledRun.start(self.root, self.slots, self.engine, 0)
        three.close()
        two.close()
        self.assertNotIn(('volume', second), self.engine.resources)

    def test_configured_capacity_is_enforced_through_owned_cleanup(self):
        one = ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        self.addCleanup(one.abandon)
        resource = one.run.create_resource('network')
        with self.assertRaises(TimeoutError):
            ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        with self.assertRaisesRegex(ValueError, 'capacity'):
            ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=2)
        one.close()
        self.assertNotIn(('network', resource), self.engine.resources)
        later = ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        later.close()

    def test_abandoned_owned_run_can_be_recovered_by_exact_binding(self):
        one = ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        self.addCleanup(one.abandon)
        directory, slot = one.run.directory, one.slot
        name = one.run.create_resource('network')
        one.abandon()
        with self.assertRaises(TimeoutError):
            ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        ScheduledRun.recover(directory, self.slots, self.engine, slot, capacity=1)
        self.assertNotIn(('network', name), self.engine.resources)
        later = ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        later.close()

    def test_recovery_stops_abandoned_execution_without_touching_other_run(self):
        one = ScheduledRun.start(self.root, self.slots, self.engine, 0)
        other = ScheduledRun.start(self.root, self.slots, self.engine, 0)
        self.addCleanup(one.abandon)
        self.addCleanup(other.abandon)
        network = one.run.create_resource('network')
        volume = one.run.create_resource('volume')
        name = one.run.create_container('sha256:' + 'd' * 64, network, {'/data': volume}, [],
                                        command=['sleep', 'infinity'])
        one.run.start_container(name)
        sentinel = other.run.create_resource('volume')
        self.engine.uncertain_exec = True
        with self.assertRaises(OwnershipError):
            one.run.execute(name, ['sleep', '30'], None, None, 1)
        directory, slot = one.run.directory, one.slot
        one.abandon()
        ScheduledRun.recover(directory, self.slots, self.engine, slot)
        self.assertNotIn(('container', name), self.engine.resources)
        self.assertIn(('volume', sentinel), self.engine.resources)
        other.close()

    def test_uncertain_stop_keeps_slot_until_reconciled(self):
        one = ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        self.addCleanup(one.abandon)
        network = one.run.create_resource('network')
        volume = one.run.create_resource('volume')
        name = one.run.create_container('sha256:' + 'd' * 64, network, {'/data': volume}, [],
                                        command=['sleep', 'infinity'])
        one.run.start_container(name)
        self.engine.uncertain_exec = True
        with self.assertRaises(OwnershipError):
            one.run.execute(name, ['sleep', '30'], None, None, 1)
        directory, slot = one.run.directory, one.slot
        one.abandon()
        self.engine.uncertain_stop = True
        with self.assertRaises(OwnershipError):
            ScheduledRun.recover(directory, self.slots, self.engine, slot, capacity=1)
        with self.assertRaises(TimeoutError):
            ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        self.assertIn(('container', name), self.engine.resources)
        self.engine.uncertain_stop = False
        ScheduledRun.recover(directory, self.slots, self.engine, slot, capacity=1)
        self.assertNotIn(('container', name), self.engine.resources)
        later = ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        later.close()

    def test_body_timeout_stops_owned_container_before_releasing_slot(self):
        with self.assertRaises(OwnershipError):
            with ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1) as run:
                network = run.create_resource('network')
                volume = run.create_resource('volume')
                name = run.create_container('sha256:' + 'd' * 64, network, {'/data': volume}, [],
                                            command=['sleep', 'infinity'])
                run.start_container(name)
                self.engine.uncertain_exec = True
                run.execute(name, ['sleep', '30'], None, None, 1)
        self.assertNotIn(('container', name), self.engine.resources)
        later = ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        later.close()

    def test_recovery_refuses_a_live_owner_or_different_run(self):
        one = ScheduledRun.start(self.root, self.slots, self.engine, 0)
        other = ScheduledRun.start(self.root, self.slots, self.engine, 0)
        self.addCleanup(one.abandon)
        self.addCleanup(other.abandon)
        resource = one.run.create_resource('network')
        one_directory, one_slot = one.run.directory, one.slot
        with self.assertRaises((OwnershipError, ValueError)):
            ScheduledRun.recover(one_directory, self.slots, self.engine, one_slot)
        one.abandon()
        with self.assertRaises((OwnershipError, ValueError)):
            ScheduledRun.recover(other.run.directory, self.slots, self.engine, one.slot)
        self.assertIn(('network', resource), self.engine.resources)
        ScheduledRun.recover(one_directory, self.slots, self.engine, one_slot)
        other.close()

    def test_uncertain_recovery_does_not_release_capacity(self):
        one = ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        self.addCleanup(one.abandon)
        directory, slot = one.run.directory, one.slot
        one.run.create_resource('network')
        one.abandon()
        self.engine.uncertain_remove = True
        with self.assertRaises(OwnershipError):
            ScheduledRun.recover(directory, self.slots, self.engine, slot, capacity=1)
        with self.assertRaises(TimeoutError):
            ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)

    def test_recovery_refuses_another_daemon_without_touching_resources(self):
        one = ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        self.addCleanup(one.abandon)
        directory, slot = one.run.directory, one.slot
        resource = one.run.create_resource('network')
        one.abandon()
        self.engine.identity = 'different-daemon'
        with self.assertRaises(OwnershipError):
            ScheduledRun.recover(directory, self.slots, self.engine, slot, capacity=1)
        self.assertIn(('network', resource), self.engine.resources)
        with self.assertRaises(TimeoutError):
            ScheduledRun.start(self.root, self.slots, self.engine, 0, capacity=1)
        self.engine.identity = 'daemon-a'
        ScheduledRun.recover(directory, self.slots, self.engine, slot, capacity=1)

    def test_body_failure_cleans_owned_resources_before_releasing_slot(self):
        with self.assertRaisesRegex(RuntimeError, 'body failed'):
            with ScheduledRun.start(self.root, self.slots, self.engine, 0) as run:
                name = run.create_resource('network')
                raise RuntimeError('body failed')
        self.assertNotIn(('network', name), self.engine.resources)
        later = ScheduledRun.start(self.root, self.slots, self.engine, 0)
        later.close()

    def test_uncertain_cleanup_abandons_slot_and_preserves_other_run(self):
        one = ScheduledRun.start(self.root, self.slots, self.engine, 0)
        two = ScheduledRun.start(self.root, self.slots, self.engine, 0)
        self.addCleanup(one.abandon)
        self.addCleanup(two.abandon)
        one.run.create_resource('network')
        other = two.run.create_resource('volume')
        self.engine.uncertain_remove = True
        with self.assertRaises(OwnershipError):
            one.close()
        self.assertIn(('volume', other), self.engine.resources)
        with self.assertRaises(TimeoutError):
            ScheduledRun.start(self.root, self.slots, self.engine, 0)
        self.engine.uncertain_remove = False
        two.close()
        later = ScheduledRun.start(self.root, self.slots, self.engine, 0)
        later.close()
        # The uncertain slot remains occupied even after the other slot is freed.
        occupied = ScheduledRun.start(self.root, self.slots, self.engine, 0)
        self.addCleanup(occupied.abandon)
        with self.assertRaises(TimeoutError):
            ScheduledRun.start(self.root, self.slots, self.engine, 0)
        occupied.close()

    def test_failed_owner_initialization_never_acquires_or_leaves_a_busy_slot(self):
        with mock.patch('local_docker_scheduled.OwnedRun.create', side_effect=OSError('journal failure')), \
             mock.patch('local_docker_scheduled.acquire') as acquire:
            with self.assertRaisesRegex(OSError, 'journal failure'):
                ScheduledRun.start(self.root, self.slots, self.engine, 0)
        acquire.assert_not_called()
        other = ScheduledRun.start(self.root, self.slots, self.engine, 0)
        other.close()
        later = ScheduledRun.start(self.root, self.slots, self.engine, 0)
        later.close()


if __name__ == '__main__':
    unittest.main()
