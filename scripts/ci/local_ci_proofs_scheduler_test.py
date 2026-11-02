"""Owner-bound proofs scheduler test API contract; no Docker or live Job Objects.

`ProofRun.start(journal_dir, timeout=..., capacity=...)` writes canonical JSON
`{schema: 1, run_id, slot: null, job_names: [], session_identity}` before acquiring a host slot
with owner `voiceci-<32 lowercase hex>`. The bound lease is reflected in the
journal before start returns. `reserve_job_name` persists unique Local Job Object
names for exactly `make-dry-run` and `make`. `close` removes the journal only
after releasing the bound lease; `abandon` preserves both. Recovery probes only
the exact persisted names and releases a slot only when all are absent.
"""
import json
from pathlib import Path
import tempfile
import unittest
from unittest import mock

import local_ci_proofs_scheduler as scheduler
import local_heavy_slots as slots


class ProofRunSchedulerTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix='voice-ci-proof-scheduler-')
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.journals = self.root / 'proof-runs'
        self.slots = self.root / 'host-slots'
        self.host_dir_patch = mock.patch.object(slots, 'host_slot_directory',
                                                return_value=self.slots)
        self.host_dir_patch.start()
        self.addCleanup(self.host_dir_patch.stop)

    def abandoned_run(self):
        run = scheduler.ProofRun.start(self.journals, timeout=0, capacity=2)
        names = [run.reserve_job_name(stage) for stage in ('make-dry-run', 'make')]
        run.abandon()
        return run, names

    def marker(self, slot):
        return (self.slots / f'heavy-{slot}.lock').read_bytes()[1:].decode('ascii')

    def assert_recovery_unresolved(self, run, job_exists):
        try:
            recovered = scheduler.recover_proof_runs(
                self.journals, capacity=2, job_exists=job_exists)
        except (OSError, RuntimeError, ValueError):
            return
        self.assertNotIn(run.run_id, recovered)

    def test_start_persists_intent_before_owner_bound_acquire_and_close_releases(self):
        acquire = slots.acquire
        observed = []

        def acquire_after_intent(directory, timeout, capacity=2, *, owner_id=None):
            self.assertEqual(directory, self.slots)
            self.assertRegex(owner_id, r'^voiceci-[0-9a-f]{32}$')
            records = [json.loads(path.read_text(encoding='utf-8'))
                       for path in self.journals.glob('*.json')]
            record = next(item for item in records if item['run_id'] == owner_id)
            self.assertEqual(record, {'schema': 1, 'run_id': owner_id,
                                      'slot': None, 'job_names': [],
                                      'session_identity': scheduler._session_identity()})
            observed.append(record)
            return acquire(directory, timeout, capacity=capacity, owner_id=owner_id)

        with mock.patch.object(slots, 'acquire', side_effect=acquire_after_intent):
            run = scheduler.ProofRun.start(self.journals, timeout=0, capacity=2)
        self.assertEqual(len(observed), 1)
        self.assertRegex(run.run_id, r'^voiceci-[0-9a-f]{32}$')
        self.assertIn(run.slot, (0, 1))
        self.assertEqual(run.journal_path.parent, self.journals)
        self.assertRegex(slots._marker(run._lease._descriptor).decode('ascii'),
                         rf'^busy:[0-9a-f]{{32}}:{run.run_id}\n$')

        dry_name = run.reserve_job_name('make-dry-run')
        make_name = run.reserve_job_name('make')
        self.assertTrue(dry_name.startswith('Local\\VoiceCIProof-'))
        self.assertTrue(make_name.startswith('Local\\VoiceCIProof-'))
        self.assertNotEqual(dry_name, make_name)
        record = json.loads(run.journal_path.read_text(encoding='utf-8'))
        self.assertEqual(record['job_names'], [dry_name, make_name])
        with self.assertRaises(ValueError):
            run.reserve_job_name('make-extra')
        self.assertEqual(json.loads(run.journal_path.read_text(encoding='utf-8'))['job_names'],
                         [dry_name, make_name])

        run.close(job_exists=lambda _name: False)
        self.assertFalse(run.journal_path.exists())
        self.assertEqual(self.marker(run.slot), 'idle\n')

    def test_close_queries_every_exact_job_before_releasing(self):
        run = scheduler.ProofRun.start(self.journals, timeout=0, capacity=2)
        names = [run.reserve_job_name(stage) for stage in ('make-dry-run', 'make')]
        queried = []
        run.close(job_exists=lambda name: queried.append(name) or False)
        self.assertEqual(queried, names)
        self.assertFalse(run.journal_path.exists())
        self.assertEqual(self.marker(run.slot), 'idle\n')

    def test_close_present_job_or_query_error_keeps_owner_bound_slot(self):
        for mode in ('present', 'query_error'):
            with self.subTest(mode=mode):
                run = scheduler.ProofRun.start(self.journals, timeout=0, capacity=2)
                name = run.reserve_job_name('make')
                if mode == 'present':
                    probe = lambda _name: True
                    expected = ValueError
                else:
                    def probe(_name):
                        raise OSError('named job query failed')
                    expected = OSError
                with self.assertRaises(expected):
                    run.close(job_exists=probe)
                self.assertTrue(run.journal_path.exists())
                self.assertIsNone(run._lease)
                self.assertRegex(self.marker(run.slot),
                                 rf'^busy:[0-9a-f]{{32}}:{run.run_id}\n$')
                recovered = scheduler.recover_proof_runs(
                    self.journals, capacity=2, job_exists=lambda _name: False)
                self.assertIn(run.run_id, recovered)
                self.assertEqual(self.marker(run.slot), 'idle\n')

    def test_abandon_retains_journal_and_owner_bound_busy_marker(self):
        run = scheduler.ProofRun.start(self.journals, timeout=0, capacity=2)
        name = run.reserve_job_name('make')
        journal_path = run.journal_path
        run_id, slot = run.run_id, run.slot
        run.abandon()
        self.assertTrue(journal_path.is_file())
        self.assertEqual(json.loads(journal_path.read_text(encoding='utf-8'))['job_names'], [name])
        self.assertRegex(self.marker(slot), rf'^busy:[0-9a-f]{{32}}:{run_id}\n$')

    def test_close_failure_keeps_recovery_journal_and_bound_slot(self):
        run = scheduler.ProofRun.start(self.journals, timeout=0, capacity=2)
        run.reserve_job_name('make')
        journal = run.journal_path
        slot, run_id = run.slot, run.run_id
        lease = run._lease
        with mock.patch.object(lease, 'close', side_effect=OSError('lease release failed')):
            with self.assertRaisesRegex(OSError, 'lease release failed'):
                run.close(job_exists=lambda _name: False)
        self.assertTrue(journal.is_file())
        self.assertIsNone(run._lease)
        self.assertRegex(self.marker(slot), rf'^busy:[0-9a-f]{{32}}:{run_id}\n$')
        recovered = scheduler.recover_proof_runs(
            self.journals, capacity=2, job_exists=lambda _name: False)
        self.assertIn(run_id, recovered)

    def test_start_crash_after_owner_marker_is_recovered_from_intent_journal(self):
        def simulated_crash(_descriptor, _slot, _run_id):
            raise KeyboardInterrupt('crash after owner marker durability')

        with mock.patch.object(slots, '_owned_slot_marker_written',
                               side_effect=simulated_crash, create=True):
            with self.assertRaisesRegex(KeyboardInterrupt, 'owner marker durability'):
                scheduler.ProofRun.start(self.journals, timeout=0, capacity=2)
        journals = list(self.journals.glob('*.json'))
        self.assertEqual(len(journals), 1)
        record = json.loads(journals[0].read_text(encoding='utf-8'))
        self.assertEqual(record['schema'], 1)
        self.assertIsNone(record['slot'])
        self.assertRegex(record['run_id'], r'^voiceci-[0-9a-f]{32}$')
        busy_markers = [path.read_bytes()[1:].decode('ascii')
                        for path in self.slots.glob('heavy-*.lock')]
        self.assertEqual(len(busy_markers), 1)
        self.assertRegex(busy_markers[0],
                         rf'^busy:[0-9a-f]{{32}}:{record["run_id"]}\n$')
        recovered = scheduler.recover_proof_runs(
            self.journals, capacity=2, job_exists=lambda _name: False)
        self.assertIn(record['run_id'], recovered)
        self.assertFalse(journals[0].exists())

    def test_confirmed_slot_timeout_removes_initial_intent(self):
        observed = []

        def acquire_without_slot(directory, timeout, capacity=2, *, owner_id=None):
            self.assertEqual(directory, self.slots)
            self.assertEqual(timeout, 0)
            self.assertEqual(capacity, 2)
            self.assertRegex(owner_id, r'^voiceci-[0-9a-f]{32}$')
            observed.append(owner_id)
            raise slots.SlotAcquireTimeout('all host slots are held')

        with mock.patch.object(slots, 'acquire', side_effect=acquire_without_slot):
            with self.assertRaisesRegex(slots.SlotAcquireTimeout, 'all host slots are held'):
                scheduler.ProofRun.start(self.journals, timeout=0, capacity=2)

        self.assertEqual(len(observed), 1)
        self.assertEqual(list(self.journals.glob('proof-run-*.json')), [])

    def test_second_journal_write_failure_retains_bound_slot_for_recovery(self):
        durable_write = scheduler._durable_write
        writes = []

        def fail_after_acquire(path, record):
            writes.append((Path(path), dict(record)))
            if len(writes) == 1:
                return durable_write(path, record)
            raise OSError('journal slot binding could not be persisted')

        with mock.patch.object(scheduler, '_durable_write', side_effect=fail_after_acquire):
            with self.assertRaisesRegex(OSError, 'slot binding could not be persisted'):
                scheduler.ProofRun.start(self.journals, timeout=0, capacity=2)

        self.assertEqual(len(writes), 2)
        journal = writes[0][0]
        self.assertTrue(journal.is_file())
        record = json.loads(journal.read_text(encoding='utf-8'))
        self.assertEqual(record, {'schema': 1, 'run_id': writes[0][1]['run_id'],
                                  'slot': None, 'job_names': [],
                                  'session_identity': scheduler._session_identity()})
        busy_markers = [path.read_bytes()[1:].decode('ascii')
                        for path in self.slots.glob('heavy-*.lock')]
        self.assertEqual(len(busy_markers), 1)
        self.assertRegex(busy_markers[0],
                         rf'^busy:[0-9a-f]{{32}}:{record["run_id"]}\n$')

        recovered = scheduler.recover_proof_runs(
            self.journals, capacity=2, job_exists=lambda _name: False)
        self.assertEqual(recovered, (record['run_id'],))
        self.assertFalse(journal.exists())

    def test_recovery_releases_only_after_exact_journaled_jobs_are_absent(self):
        run, names = self.abandoned_run()
        queried = []
        recovered = scheduler.recover_proof_runs(
            self.journals, capacity=2,
            job_exists=lambda name: queried.append(name) or False)
        self.assertIn(run.run_id, recovered)
        self.assertEqual(queried, names)
        self.assertFalse(run.journal_path.exists())
        self.assertEqual(self.marker(run.slot), 'idle\n')

    def test_recovery_reloads_job_inventory_after_owner_lock(self):
        run, names = self.abandoned_run()
        added = r'Local\VoiceCIProof-added-before-recovery-lock'
        original_recover = slots.recover_bound
        queried = []

        def append_before_lock(directory, slot, run_id, capacity=2):
            record = json.loads(run.journal_path.read_text(encoding='utf-8'))
            record['job_names'].append(added)
            scheduler._durable_write(run.journal_path, record)
            return original_recover(directory, slot, run_id, capacity=capacity)

        def job_exists(name):
            queried.append(name)
            return name == added

        with mock.patch.object(slots, 'recover_bound', side_effect=append_before_lock):
            self.assert_recovery_unresolved(run, job_exists)
        self.assertEqual(queried, names + [added])
        self.assertTrue(run.journal_path.exists())
        self.assertRegex(self.marker(run.slot),
                         rf'^busy:[0-9a-f]{{32}}:{run.run_id}\n$')

        recovered = scheduler.recover_proof_runs(
            self.journals, capacity=2, job_exists=lambda _name: False)
        self.assertIn(run.run_id, recovered)
        self.assertEqual(self.marker(run.slot), 'idle\n')

    def test_present_job_or_query_error_retains_journal_and_busy_lease(self):
        for mode in ('present', 'query_error'):
            with self.subTest(mode=mode):
                run, names = self.abandoned_run()

                def exists(name):
                    if mode == 'query_error':
                        raise OSError('named job query failed')
                    return name == names[0]

                self.assert_recovery_unresolved(run, exists)
                self.assertTrue(run.journal_path.is_file())
                self.assertRegex(self.marker(run.slot),
                                 rf'^busy:[0-9a-f]{{32}}:{run.run_id}\n$')

    def test_corrupt_journal_and_owner_mismatch_fail_closed(self):
        run, _names = self.abandoned_run()
        run.journal_path.write_bytes(b'{not-json')
        self.assert_recovery_unresolved(run, lambda _name: False)
        self.assertTrue(run.journal_path.exists())
        self.assertRegex(self.marker(run.slot),
                         rf'^busy:[0-9a-f]{{32}}:{run.run_id}\n$')

        # A valid journal cannot authorize recovery of a slot bound to another run.
        run.journal_path.write_text(json.dumps({'schema': 1, 'run_id': run.run_id,
                                                'slot': run.slot, 'job_names': [],
                                                'session_identity': scheduler._session_identity()}),
                                    encoding='utf-8')
        path = self.slots / f'heavy-{run.slot}.lock'
        path.write_bytes(b'\0busy:' + b'c' * 32 + b':voiceci-' + b'd' * 32 + b'\n')
        self.assert_recovery_unresolved(run, lambda _name: False)
        self.assertTrue(run.journal_path.exists())

    def test_recovery_does_not_release_a_slot_that_is_still_locked(self):
        run = scheduler.ProofRun.start(self.journals, timeout=0, capacity=2)
        run.reserve_job_name('make')
        self.assert_recovery_unresolved(run, lambda _name: False)
        self.assertTrue(run.journal_path.exists())
        self.assertRegex(slots._marker(run._lease._descriptor).decode('ascii'),
                         rf'^busy:[0-9a-f]{{32}}:{run.run_id}\n$')
        run.close(job_exists=lambda _name: False)

    def test_other_session_or_boot_cannot_recover_local_job_names(self):
        identity = scheduler._session_identity()
        for field, changed in (('session_id', identity['session_id'] + 1),
                               ('logon_id', 'different-logon'),
                               ('boot_id', 'different-boot')):
            with self.subTest(field=field), tempfile.TemporaryDirectory() as directory:
                root = Path(directory)
                journal_dir, slot_dir = root / 'journals', root / 'slots'
                with mock.patch.object(slots, 'host_slot_directory', return_value=slot_dir):
                    run = scheduler.ProofRun.start(journal_dir, timeout=0, capacity=2)
                    run.reserve_job_name('make')
                    run.abandon()
                    record = json.loads(run.journal_path.read_text(encoding='utf-8'))
                    record['session_identity'][field] = changed
                    scheduler._durable_write(run.journal_path, record)

                    recovered = scheduler.recover_proof_runs(
                        journal_dir, capacity=2, job_exists=lambda _name: False)
                    self.assertNotIn(run.run_id, recovered)
                    self.assertTrue(run.journal_path.is_file())
                    self.assertRegex((slot_dir / f'heavy-{run.slot}.lock').read_bytes()[1:].decode('ascii'),
                                     rf'^busy:[0-9a-f]{{32}}:{run.run_id}\n$')

                    other = slots.acquire(slot_dir, timeout=0, capacity=2)
                    try:
                        self.assertNotEqual(other.slot, run.slot)
                    finally:
                        other.close()

    def test_recovery_rechecks_session_identity_under_owner_lock(self):
        run, _names = self.abandoned_run()
        original_recover = slots.recover_bound
        queried = []

        def change_identity_before_lock(directory, slot, run_id, capacity=2):
            record = json.loads(run.journal_path.read_text(encoding='utf-8'))
            record['session_identity']['logon_id'] = 'changed-before-lock'
            scheduler._durable_write(run.journal_path, record)
            return original_recover(directory, slot, run_id, capacity=capacity)

        with mock.patch.object(slots, 'recover_bound', side_effect=change_identity_before_lock):
            recovered = scheduler.recover_proof_runs(
                self.journals, capacity=2,
                job_exists=lambda name: queried.append(name) or False)
        self.assertNotIn(run.run_id, recovered)
        self.assertEqual(queried, [])
        self.assertRegex(self.marker(run.slot),
                         rf'^busy:[0-9a-f]{{32}}:{run.run_id}\n$')

        record = json.loads(run.journal_path.read_text(encoding='utf-8'))
        record['session_identity'] = scheduler._session_identity()
        scheduler._durable_write(run.journal_path, record)
        recovered = scheduler.recover_proof_runs(
            self.journals, capacity=2, job_exists=lambda _name: False)
        self.assertIn(run.run_id, recovered)


if __name__ == '__main__':
    unittest.main()
