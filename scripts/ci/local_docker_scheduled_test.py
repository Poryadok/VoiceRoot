"""The heavy slot must outlive the exact owned fixture cleanup."""
from pathlib import Path
import tempfile
import unittest
from unittest import mock

from local_docker_owned import OwnershipError
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
            ScheduledRun.recover_host(directory, self.engine, slot, capacity=1)
            self.assertNotIn(('network', resource), self.engine.resources)
            later = ScheduledRun.start_host(other_checkout, self.engine, 0, capacity=1)
            later.close()

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
        with self.assertRaises((OwnershipError, ValueError)):
            ScheduledRun.recover(one.run.directory, self.slots, self.engine, one.slot)
        one.abandon()
        with self.assertRaises((OwnershipError, ValueError)):
            ScheduledRun.recover(other.run.directory, self.slots, self.engine, one.slot)
        self.assertIn(('network', resource), self.engine.resources)
        ScheduledRun.recover(one.run.directory, self.slots, self.engine, one.slot)
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

    def test_failed_owner_initialization_does_not_free_uncertain_slot(self):
        with mock.patch('local_docker_scheduled.OwnedRun.create', side_effect=OSError('journal failure')):
            with self.assertRaisesRegex(OSError, 'journal failure'):
                ScheduledRun.start(self.root, self.slots, self.engine, 0)
        other = ScheduledRun.start(self.root, self.slots, self.engine, 0)
        self.addCleanup(other.abandon)
        with self.assertRaises(TimeoutError):
            ScheduledRun.start(self.root, self.slots, self.engine, 0)
        other.close()


if __name__ == '__main__':
    unittest.main()
