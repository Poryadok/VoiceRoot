"""Cross-process limits for local heavy CI runs."""
import os
from pathlib import Path
import subprocess
import sys
import tempfile
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
                    second.close()
                first.close()

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
                lease.close()

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
                bounded.close()

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
    lease.close()
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
