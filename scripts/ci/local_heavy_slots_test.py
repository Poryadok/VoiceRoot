"""Cross-process limits for local heavy CI runs."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import threading
import time
import unittest
from unittest import mock

import local_heavy_slots as slots
from local_heavy_slots import acquire, host_slot_directory


class HeavySlotsTest(unittest.TestCase):
    def test_budget_above_measured_cpu_or_memory_rejects_before_slot_creation(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for budget in (
                {'nano_cpus': 8_000_000_000, 'memory_bytes': 8_000, 'pids_limit': 512},
                {'nano_cpus': 4_000_000_000, 'memory_bytes': 16_000, 'pids_limit': 512},
            ):
                with self.subTest(budget=budget), \
                     mock.patch.object(slots, 'measured_host_capacity', return_value={
                         'nano_cpus': 4_000_000_000, 'memory_bytes': 8_000}), \
                     self.assertRaisesRegex(ValueError, 'measured host capacity'):
                    acquire(root, timeout=0, capacity=2, owner_id='voiceci-' + 'c' * 32,
                            resource_claim={'nano_cpus': 1, 'memory_bytes': 1, 'pids_limit': 1},
                            capacity_budget=budget)
                self.assertEqual(list(root.iterdir()), [])

    def test_bounded_claims_reserve_aggregate_budget_under_the_shared_lock(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            budget = {'nano_cpus': 4_000_000_000, 'memory_bytes': 8_000, 'pids_limit': 800}
            first_claim = {'nano_cpus': 2_500_000_000, 'memory_bytes': 5_000, 'pids_limit': 500}
            second_claim = {'nano_cpus': 1_500_000_000, 'memory_bytes': 3_000, 'pids_limit': 300}
            first = acquire(root, timeout=0, capacity=2, owner_id='voiceci-' + '1' * 32,
                            resource_claim=first_claim, capacity_budget=budget)
            second = None
            try:
                second = acquire(root, timeout=0, capacity=2, owner_id='voiceci-' + '2' * 32,
                                 resource_claim=second_claim, capacity_budget=budget)
                with self.assertRaises(TimeoutError):
                    acquire(root, timeout=0, capacity=2, owner_id='voiceci-' + '3' * 32,
                            resource_claim={'nano_cpus': 1, 'memory_bytes': 1, 'pids_limit': 1},
                            capacity_budget=budget)
            finally:
                if second is not None:
                    second.close(cleanup_verified=True)
                first.close(cleanup_verified=True)

    def test_budget_configuration_mismatch_and_overcommit_fail_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            budget = {'nano_cpus': 2_000_000_000, 'memory_bytes': 4_000, 'pids_limit': 400}
            lease = acquire(root, timeout=0, capacity=2, owner_id='voiceci-' + '4' * 32,
                            resource_claim={'nano_cpus': 1_000_000_000, 'memory_bytes': 2_000,
                                            'pids_limit': 200}, capacity_budget=budget)
            try:
                with self.assertRaisesRegex(ValueError, 'budget'):
                    acquire(root, timeout=0, capacity=2, owner_id='voiceci-' + '5' * 32,
                            resource_claim={'nano_cpus': 1, 'memory_bytes': 1, 'pids_limit': 1},
                            capacity_budget={'nano_cpus': 3_000_000_000, 'memory_bytes': 4_000,
                                             'pids_limit': 400})
                with self.assertRaises((ValueError, TimeoutError)):
                    acquire(root, timeout=0, capacity=2, owner_id='voiceci-' + '6' * 32,
                            resource_claim={'nano_cpus': 1_000_000_001, 'memory_bytes': 2_001,
                                            'pids_limit': 201}, capacity_budget=budget)
            finally:
                lease.close(cleanup_verified=True)

    def test_bounded_claim_refuses_legacy_busy_slot_with_unknown_usage(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            legacy = acquire(root, timeout=0, capacity=2)
            legacy.abandon()
            with self.assertRaises((ValueError, TimeoutError)):
                acquire(root, timeout=0, capacity=2, owner_id='voiceci-' + '7' * 32,
                        resource_claim={'nano_cpus': 1, 'memory_bytes': 1, 'pids_limit': 1},
                        capacity_budget={'nano_cpus': 2, 'memory_bytes': 2, 'pids_limit': 2})

    def test_unbounded_acquire_cannot_cross_a_live_bounded_reservation(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            bounded = acquire(root, timeout=0, capacity=2, owner_id='voiceci-' + '8' * 32,
                              resource_claim={'nano_cpus': 1, 'memory_bytes': 1, 'pids_limit': 1},
                              capacity_budget={'nano_cpus': 2, 'memory_bytes': 2, 'pids_limit': 2})
            try:
                with self.assertRaises((ValueError, TimeoutError)):
                    acquire(root, timeout=0, capacity=2)
            finally:
                bounded.abandon()
            self.assertIsNotNone(slots.active_resource_reservation(root, 'voiceci-' + '8' * 32))

    def test_abandoned_bounded_reservation_survives_and_recovery_releases_after_cleanup(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            owner_id = 'voiceci-' + '9' * 32
            budget = {'nano_cpus': 2, 'memory_bytes': 2, 'pids_limit': 2}
            claim = {'nano_cpus': 2, 'memory_bytes': 2, 'pids_limit': 2}
            child = '''import sys
from local_heavy_slots import acquire
lease = acquire(sys.argv[1], timeout=0, capacity=2, owner_id='voiceci-' + '9' * 32,
                resource_claim={'nano_cpus': 2, 'memory_bytes': 2, 'pids_limit': 2},
                capacity_budget={'nano_cpus': 2, 'memory_bytes': 2, 'pids_limit': 2})
print(lease.slot, flush=True)
sys.stdin.readline()
lease.abandon()
'''
            env = dict(os.environ, PYTHONPATH=str(Path(__file__).resolve().parent))
            process = subprocess.Popen([sys.executable, '-u', '-c', child, str(root)], env=env,
                                       stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                       text=True)
            slot_line = process.stdout.readline().strip()
            if not slot_line:
                error = process.stderr.read()
                process.wait(timeout=5)
                self.fail('bounded child failed before persisting its reservation: ' + error)
            slot = int(slot_line)
            process.kill()
            process.wait(timeout=5)
            process.stdin.close()
            process.stdout.close()
            process.stderr.close()
            self.assertEqual(slots.active_resource_reservation(root, owner_id), claim)

            with self.assertRaises((ValueError, TimeoutError)):
                acquire(root, timeout=0, capacity=2, owner_id='voiceci-' + 'a' * 32,
                        resource_claim={'nano_cpus': 1, 'memory_bytes': 1, 'pids_limit': 1},
                        capacity_budget=budget)

            recovered = slots.recover_bound(root, slot, owner_id, capacity=2)
            try:
                self.assertEqual(slots.active_resource_reservation(root, owner_id), claim)
                with self.assertRaisesRegex(ValueError, 'cleanup'):
                    slots.release_resource_reservation(root, owner_id, cleanup_verified=False)
                self.assertEqual(slots.active_resource_reservation(root, owner_id), claim)
                slots.release_resource_reservation(root, owner_id, cleanup_verified=True)
                self.assertIsNone(slots.active_resource_reservation(root, owner_id))
            finally:
                recovered.close()

    def test_reservation_write_failure_closes_slot_fd_and_leaves_recoverable_intent(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            owner_id = 'voiceci-' + 'c' * 32
            claim = {'nano_cpus': 1, 'memory_bytes': 2, 'pids_limit': 3}
            budget = {'nano_cpus': 2, 'memory_bytes': 4, 'pids_limit': 6}
            original = slots._write_marker

            def fail_busy_marker(descriptor, value):
                if value.startswith(b'busy:'):
                    raise KeyboardInterrupt('simulated crash after reservation fsync')
                return original(descriptor, value)

            with mock.patch.object(slots, '_write_marker', side_effect=fail_busy_marker), \
                 self.assertRaisesRegex(KeyboardInterrupt, 'after reservation fsync'):
                acquire(root, timeout=0, capacity=1, owner_id=owner_id,
                        resource_claim=claim, capacity_budget=budget)

            self.assertEqual(slots.active_resource_reservation(root, owner_id), claim)
            slot_path = root / 'heavy-0.lock'
            descriptor = os.open(slot_path, os.O_RDWR)
            try:
                self.assertTrue(slots._try_lock(descriptor), 'failed acquire leaked the slot lock')
                self.assertEqual(slots._marker(descriptor), b'idle\n')
            finally:
                slots._unlock_close(descriptor)
            slots.release_resource_reservation(root, owner_id, cleanup_verified=True)
            self.assertIsNone(slots.active_resource_reservation(root, owner_id))

    def test_idle_marker_with_orphan_reservation_is_excluded_from_new_acquisitions(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            owner_id = 'voiceci-' + 'c' * 32
            contender_id = 'voiceci-' + 'd' * 32
            claim = {'nano_cpus': 1, 'memory_bytes': 2, 'pids_limit': 3}
            budget = {'nano_cpus': 3, 'memory_bytes': 6, 'pids_limit': 9}
            original = slots._write_marker

            def fail_busy_marker(descriptor, value):
                if value.startswith(b'busy:'):
                    raise KeyboardInterrupt('crash after reservation')
                return original(descriptor, value)

            with mock.patch.object(slots, '_write_marker', side_effect=fail_busy_marker), \
                 self.assertRaises(KeyboardInterrupt):
                acquire(root, timeout=0, capacity=2, owner_id=owner_id,
                        resource_claim=claim, capacity_budget=budget)

            contender = acquire(root, timeout=0, capacity=2, owner_id=contender_id,
                               resource_claim=claim, capacity_budget=budget)
            try:
                self.assertEqual(contender.slot, 1)
                self.assertEqual(slots.resource_reservation_slot(root, owner_id), 0)
            finally:
                contender.close(cleanup_verified=True)

    def test_duplicate_reservation_slot_indices_fail_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            claim = {'nano_cpus': 1, 'memory_bytes': 2, 'pids_limit': 3}
            state = {'version': 1, 'budget': None, 'reservations': {
                'voiceci-' + 'a' * 32: {'claim': claim, 'slot': 0},
                'voiceci-' + 'b' * 32: {'claim': claim, 'slot': 0}}}
            slots._save_resource_state(root, state)
            with self.assertRaisesRegex(ValueError, 'multiple owners'):
                slots._resource_state(root)

    def test_acquire_waits_for_held_slot_to_close_before_deadline(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            first = acquire(root, timeout=0, capacity=1)
            closer = threading.Thread(target=lambda: (time.sleep(0.08), first.close()))
            closer.start()
            try:
                second = acquire(root, timeout=1, capacity=1)
            finally:
                closer.join(timeout=2)
            self.assertFalse(closer.is_alive())
            second.close()

    def test_admission_waiter_releases_lock_so_slot_close_can_proceed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            first = acquire(root, timeout=0, capacity=1)
            sleep_entered = threading.Event()
            allow_sleep = threading.Event()
            close_attempted = threading.Event()
            close_finished = threading.Event()
            acquired = []
            failures = []
            original_sleep = slots.time.sleep
            original_lock = slots._lock_admission

            def controlled_sleep(seconds):
                if threading.current_thread().name == 'waiting-acquire':
                    sleep_entered.set()
                    allow_sleep.wait()
                else:
                    original_sleep(seconds)

            def observed_lock(directory, deadline):
                if threading.current_thread().name == 'closing-slot':
                    close_attempted.set()
                return original_lock(directory, deadline)

            def wait_for_slot():
                try:
                    acquired.append(acquire(root, timeout=3, capacity=1))
                except BaseException as error:
                    failures.append(error)

            def close_slot():
                first.close()
                close_finished.set()

            waiter = threading.Thread(name='waiting-acquire', target=wait_for_slot)
            closer = threading.Thread(name='closing-slot', target=close_slot)
            try:
                with mock.patch.object(slots.time, 'sleep', side_effect=controlled_sleep), \
                     mock.patch.object(slots, '_lock_admission', side_effect=observed_lock):
                    waiter.start()
                    self.assertTrue(sleep_entered.wait(timeout=2))
                    closer.start()
                    self.assertTrue(close_attempted.wait(timeout=2))
                    self.assertTrue(close_finished.wait(timeout=2),
                        'waiting acquisition retained the admission lock while sleeping')
                    allow_sleep.set()
                    waiter.join(timeout=3)
                    closer.join(timeout=3)
                self.assertFalse(waiter.is_alive())
                self.assertFalse(closer.is_alive())
                self.assertEqual(len(acquired), 1)
                acquired[0].close()
            finally:
                allow_sleep.set()
                if waiter.is_alive():
                    waiter.join(timeout=5)
                if closer.is_alive():
                    closer.join(timeout=5)
                first.abandon()
                for lease in acquired:
                    lease.abandon()

    def test_concurrent_acquire_and_close_do_not_poison_released_slot(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            first = acquire(root, timeout=0, capacity=2)
            child = '''import sys
import local_heavy_slots as slots
def hold_after_owner_marker(_descriptor, _slot, _owner_id):
    print('locked', flush=True)
    sys.stdin.readline()
slots._owned_slot_marker_written = hold_after_owner_marker
lease = slots.acquire(sys.argv[1], timeout=3, capacity=2, owner_id='voiceci-' + 'd' * 32)
print('acquired', flush=True)
sys.stdin.readline()
lease.close()
print('closed', flush=True)
'''
            env = dict(os.environ, PYTHONPATH=str(Path(__file__).resolve().parent))
            process = subprocess.Popen([sys.executable, '-u', '-c', child, str(root)], env=env,
                                       stdin=subprocess.PIPE, stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                                       text=True)
            close_attempted = threading.Event()
            original_lock = slots._lock_admission

            def observed_lock(directory, deadline):
                if threading.current_thread().name == 'closing-slot':
                    close_attempted.set()
                return original_lock(directory, deadline)

            closer = threading.Thread(name='closing-slot', target=lambda: first.close())
            try:
                self.assertEqual(process.stdout.readline().strip(), 'locked')
                with mock.patch.object(slots, '_lock_admission', side_effect=observed_lock):
                    closer.start()
                    self.assertTrue(close_attempted.wait(timeout=2))
                    process.stdin.write('\n')
                    process.stdin.flush()
                    self.assertEqual(process.stdout.readline().strip(), 'acquired')
                    closer.join(timeout=2)
                self.assertFalse(closer.is_alive())
                process.stdin.write('\n')
                process.stdin.flush()
                self.assertEqual(process.stdout.readline().strip(), 'closed')
                self.assertEqual(process.wait(timeout=3), 0)
                self.assertEqual(process.stderr.read(), '')
                self.assertEqual((root / 'heavy-0.lock').read_bytes()[1:], b'idle\n')
            finally:
                if closer.is_alive():
                    closer.join(timeout=2)
                if process.poll() is None:
                    process.kill()
                    process.wait(timeout=3)
                process.stdin.close()
                process.stdout.close()
                process.stderr.close()

    def test_bounded_and_legacy_processes_race_through_one_admission_lock(self):
        child = '''import sys
from local_heavy_slots import acquire
mode, directory = sys.argv[1:]
print('ready', flush=True)
sys.stdin.readline()
try:
    if mode == 'bounded':
        lease = acquire(directory, timeout=0.5, capacity=2, owner_id='voiceci-' + 'b' * 32,
                        resource_claim={'nano_cpus': 2, 'memory_bytes': 2, 'pids_limit': 2},
                        capacity_budget={'nano_cpus': 2, 'memory_bytes': 2, 'pids_limit': 2})
    else:
        lease = acquire(directory, timeout=0.5, capacity=2)
except (ValueError, TimeoutError):
    print('rejected', flush=True)
else:
    print('acquired', flush=True)
    sys.stdin.readline()
    lease.close(cleanup_verified=(mode == 'bounded'))
'''
        env = dict(os.environ, PYTHONPATH=str(Path(__file__).resolve().parent))
        for _ in range(4):
            with tempfile.TemporaryDirectory() as directory:
                workers = [subprocess.Popen([sys.executable, '-u', '-c', child, mode, directory],
                                            env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                            stderr=subprocess.PIPE, text=True)
                           for mode in ('bounded', 'legacy')]
                try:
                    self.assertEqual([process.stdout.readline().strip() for process in workers],
                                     ['ready', 'ready'])
                    for process in workers:
                        process.stdin.write('\n')
                        process.stdin.flush()
                    outcomes = [process.stdout.readline().strip() for process in workers]
                    self.assertCountEqual(outcomes, ['acquired', 'rejected'])
                finally:
                    for process in workers:
                        if process.poll() is None:
                            process.stdin.write('\n')
                            process.stdin.flush()
                            process.wait(timeout=6)
                        process.stdin.close()
                        process.stdout.close()
                        process.stderr.close()

    def test_owner_id_is_persisted_in_busy_marker_and_legacy_acquire_stays_compatible(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            owner_id = 'voiceci-' + 'a' * 32
            lease = acquire(root, timeout=0, capacity=1, owner_id=owner_id)
            try:
                raw = b'\0' + slots._marker(lease._descriptor)
                self.assertRegex(raw[1:].decode('ascii'),
                                 r'^busy:[0-9a-f]{32}:voiceci-a{32}\n$')
            finally:
                lease.close()
            legacy = acquire(root, timeout=0, capacity=1)
            try:
                self.assertRegex(slots._marker(legacy._descriptor).decode('ascii'),
                                 r'^busy:[0-9a-f]{32}\n$')
            finally:
                legacy.close()

    def test_malformed_owner_id_fails_before_a_slot_is_reserved(self):
        with tempfile.TemporaryDirectory() as directory:
            for owner_id in ('', 'voiceci-short', 'voiceci-' + 'A' * 32, 7):
                with self.subTest(owner_id=owner_id), self.assertRaises(ValueError):
                    try:
                        unexpected = acquire(directory, timeout=0, owner_id=owner_id)
                    except ValueError:
                        raise
                    else:
                        unexpected.close()
                        self.fail('malformed owner identifier acquired a slot')
            self.assertFalse(any(Path(directory).glob('heavy-*.lock')))

    def test_crash_after_owner_marker_write_leaves_recoverable_bound_busy_slot(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            owner_id = 'voiceci-' + 'b' * 32

            def crash_after_write(descriptor, slot, run_id):
                self.assertEqual(run_id, owner_id)
                raise KeyboardInterrupt('simulated crash after durable owner binding')

            with mock.patch.object(slots, '_owned_slot_marker_written', side_effect=crash_after_write):
                with self.assertRaisesRegex(KeyboardInterrupt, 'simulated crash'):
                    acquire(root, timeout=0, capacity=1, owner_id=owner_id)

            busy = root / 'heavy-0.lock'
            marker = busy.read_bytes()[1:].decode('ascii')
            self.assertRegex(marker, r'^busy:[0-9a-f]{32}:voiceci-b{32}\n$')
            with self.assertRaises(TimeoutError):
                acquire(root, timeout=0, capacity=1)
            recovered = slots.recover_bound(root, 0, owner_id, capacity=1)
            recovered.close()
            self.assertEqual(busy.read_bytes()[1:], b'idle\n')

    def test_host_slot_directory_is_absolute_and_not_selected_by_environment(self):
        expected = host_slot_directory()
        self.assertTrue(expected.is_absolute())
        self.assertEqual(expected.name, 'voice-ci-heavy-slots-v1')
        with mock.patch.dict(os.environ, {'TMP': 'different-tmp', 'TEMP': 'different-temp',
                                        'PROGRAMDATA': 'different-data'}):
            self.assertEqual(host_slot_directory(), expected)

    def test_capacity_is_shared_and_conflicting_configuration_is_rejected(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            leases = [acquire(root, timeout=0, capacity=3) for _ in range(3)]
            try:
                self.assertEqual({lease.slot for lease in leases}, {0, 1, 2})
                with self.assertRaises(TimeoutError):
                    acquire(root, timeout=0, capacity=3)
                with self.assertRaisesRegex(ValueError, 'capacity'):
                    acquire(root, timeout=0, capacity=2)
            finally:
                for lease in leases:
                    lease.close()

    def test_invalid_capacity_fails_without_allocating_slots(self):
        with tempfile.TemporaryDirectory() as directory:
            for invalid in (0, -1, True, 2.5, 1000):
                with self.subTest(capacity=invalid), self.assertRaises(ValueError):
                    acquire(directory, timeout=0, capacity=invalid)
            self.assertEqual(list(Path(directory).iterdir()), [])

    def test_concurrent_conflicting_capacity_initialization_has_one_winner(self):
        child = '''import sys
from local_heavy_slots import acquire
sys.stdin.readline()
try:
    lease = acquire(sys.argv[1], timeout=5, capacity=int(sys.argv[2]))
except ValueError:
    print('rejected', flush=True)
else:
    print('acquired', flush=True)
    sys.stdin.readline()
    lease.close()
'''
        env = dict(os.environ, PYTHONPATH=str(Path(__file__).resolve().parent))
        for _ in range(3):
            with tempfile.TemporaryDirectory() as directory:
                workers = [subprocess.Popen([sys.executable, '-u', '-c', child, directory, str(capacity)],
                                            env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                            stderr=subprocess.PIPE, text=True) for capacity in (1, 3)]
                try:
                    for process in workers:
                        process.stdin.write('\n')
                        process.stdin.flush()
                    self.assertEqual(sorted(process.stdout.readline().strip() for process in workers),
                                     ['acquired', 'rejected'])
                finally:
                    for process in workers:
                        if process.poll() is None:
                            process.stdin.write('\n')
                            process.stdin.flush()
                            process.wait(timeout=6)
                        process.stdin.close()
                        process.stdout.close()
                        process.stderr.close()

    def test_invalid_capacity_file_fails_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            (Path(directory) / 'capacity').write_bytes(b'invalid\n')
            with self.assertRaisesRegex(ValueError, 'capacity'):
                acquire(directory, timeout=0)

    def test_incomplete_capacity_lock_initialization_fails_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            (Path(directory) / 'capacity.lock').write_bytes(b'')
            with self.assertRaisesRegex(ValueError, 'initialization incomplete'):
                acquire(directory, timeout=0)

    def test_initial_capacity_does_not_hide_existing_slots(self):
        with tempfile.TemporaryDirectory() as directory:
            (Path(directory) / 'heavy-2.lock').write_bytes(b'\0busy:orphan\n')
            try:
                lease = acquire(directory, timeout=0, capacity=2)
            except ValueError as error:
                self.assertIn('capacity', str(error))
            else:
                lease.close()
                self.fail('capacity hid an existing slot')

    def test_simultaneous_first_start_initializes_distinct_slots(self):
        child = ('import sys; from pathlib import Path; from local_heavy_slots import acquire; '
                 'sys.stdin.readline(); lease=acquire(Path(sys.argv[1]), timeout=5); '
                 'print(lease.slot, flush=True); sys.stdin.readline(); lease.close()')
        env = dict(os.environ, PYTHONPATH=str(Path(__file__).resolve().parent))
        for _ in range(4):
            with tempfile.TemporaryDirectory() as directory:
                workers = [subprocess.Popen([sys.executable, '-u', '-c', child, directory], env=env,
                                            stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                            stderr=subprocess.PIPE, text=True) for _ in range(2)]
                try:
                    for process in workers:
                        process.stdin.write('\n')
                        process.stdin.flush()
                    self.assertEqual({process.stdout.readline().strip() for process in workers}, {'0', '1'})
                finally:
                    for process in workers:
                        if process.poll() is None:
                            process.stdin.write('\n')
                            process.stdin.flush()
                            process.wait(timeout=6)
                        process.stdin.close()
                        process.stdout.close()
                        process.stderr.close()

    def test_invalid_timeout_and_unknown_marker_fail_closed(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            for invalid in (-1, float('nan'), float('inf'), True):
                with self.subTest(timeout=invalid), self.assertRaises(ValueError):
                    acquire(root, timeout=invalid)
            (root / 'heavy-0.lock').write_bytes(b'\0unknown\n')
            lease = acquire(root, timeout=0)
            try:
                self.assertEqual(lease.slot, 1)
                with self.assertRaises(TimeoutError):
                    acquire(root, timeout=0)
            finally:
                lease.close()

    def test_exception_retains_slot_for_recovery(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            with self.assertRaisesRegex(RuntimeError, 'interrupted'):
                with acquire(root, timeout=0):
                    raise RuntimeError('interrupted')
            with acquire(root, timeout=0):
                try:
                    unexpected = acquire(root, timeout=0)
                except TimeoutError:
                    pass
                else:
                    unexpected.close()
                    self.fail('exception released a slot without cleanup')

    def test_two_processes_exclude_third_and_crash_retains_its_slot(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            child = ('import sys; from pathlib import Path; from local_heavy_slots import acquire; '
                     'lease=acquire(Path(sys.argv[1]), timeout=5); print("acquired", flush=True); '
                     'sys.stdin.readline(); lease.close()')
            env = dict(os.environ)
            env['PYTHONPATH'] = str(Path(__file__).resolve().parent)
            workers = []
            def assert_unavailable():
                try:
                    unexpected = acquire(root, timeout=0.2)
                except TimeoutError:
                    return
                unexpected.close()
                self.fail('an occupied or abandoned slot was reused')
            try:
                for _ in range(2):
                    process = subprocess.Popen([sys.executable, '-u', '-c', child, str(root)],
                                               env=env, stdin=subprocess.PIPE, stdout=subprocess.PIPE,
                                               stderr=subprocess.PIPE, text=True)
                    workers.append(process)
                    self.assertEqual(process.stdout.readline().strip(), 'acquired')
                assert_unavailable()
                workers[0].kill()
                workers[0].wait(timeout=5)
                self.assertIsNone(workers[1].poll())
                assert_unavailable()
                workers[1].stdin.write('\n')
                workers[1].stdin.flush()
                workers[1].wait(timeout=5)
                lease = acquire(root, timeout=2)
                try:
                    assert_unavailable()
                finally:
                    lease.close()
            finally:
                for process in workers:
                    if process.poll() is None:
                        process.stdin.write('\n')
                        process.stdin.flush()
                        process.wait(timeout=5)
                    process.stdin.close()
                    process.stdout.close()
                    process.stderr.close()


if __name__ == '__main__':
    unittest.main()
