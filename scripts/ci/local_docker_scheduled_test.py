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
