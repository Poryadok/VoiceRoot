"""Real journal and Fence integration for bounded read-only dump phases."""
import copy
import json
from pathlib import Path
import tempfile
import threading
import unittest
from unittest import mock

import media_backup_phase as phase
import media_operation_journal as journal
import resource_fencing
import update_images


class Ledger:
    def __init__(self, context):
        self.row = context | {'status': 'active', 'step': 'admitted'}
        self.version = 1
        self.lock = threading.Lock()

    def read(self):
        with self.lock:
            return copy.deepcopy(self.row), ('ledger', str(self.version))

    def write(self, previous, row):
        with self.lock:
            if previous != ('ledger', str(self.version)):
                raise RuntimeError('CAS changed')
            self.row = copy.deepcopy(row)
            self.version += 1


class Client:
    def __init__(self, lock): self.lock = lock
    def call(self, *args):
        if args[1] == 'namespace':
            return {'metadata': {'name': 'voice-dev', 'uid': 'ns', 'labels': update_images.LABELS}}
        return copy.deepcopy(self.lock)


class PhaseTest(unittest.TestCase):
    def setUp(self):
        self.context = {'project': 'voice', 'resource': 'DEV', 'actor': 'scheduled_devops',
            'claim': '11111111-1111-4111-8111-111111111111', 'window': '22222222-2222-4222-8222-222222222222',
            'revision': 4, 'generation': 3, 'operation_id': 'media-op', 'commit': 'a' * 40,
            'images': {'voice-file/file': 'ghcr.io/x@sha256:' + 'b' * 64}}
        self.card = {'code': 'DEV', 'environment': 'dev', 'owner': 'scheduled_devops',
            'claim': self.context['claim'], 'generation': 3, 'window': self.context['window'],
            'revision': 4, 'state': 'held', 'phase': 'deploy',
            'candidate': {'state': 'known', 'commit': self.context['commit'], 'images': self.context['images']}}
        self.ledger = Ledger(self.context)
        self.fence = resource_fencing.Fence(self.context, lambda: copy.deepcopy(self.card), self.ledger)
        self.lock = {'metadata': {'name': update_images.LOCK, 'namespace': 'voice-dev', 'uid': 'lock',
                        'resourceVersion': '8', 'labels': update_images.LABELS},
                     'data': {'operation': 'token', 'resource_operation': 'media-op',
                              'resource_context': json.dumps(self.context, sort_keys=True, separators=(',', ':'))}}
        self.capture = {'snapshot_id': 'capture-1', 'pod_uid': 'pod', 'pvc_uid': 'pvc', 'pv_uid': 'pv',
                        'container_id': 'containerd://' + 'c' * 64, 'restart_count': 1}
        self.intent = {'resource_context': self.context, 'namespace_uid': 'ns', 'backup_capture': self.capture}
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.journal = journal.Journal(Path(self.temp.name) / 'journal', self.intent)
        self.addCleanup(self.journal.close)
        self.phases = phase.BackupPhases(self.journal, self.fence, Client(self.lock), self.lock, 'ns')

    def receipt(self, member):
        return self.capture | {'operation_id': 'media-op', 'member': member,
                               'exit': 0, 'scope': 'dump-terminal-only',
                               'attempt_nonce': self.journal.state['steps']['dump-' + member]['nonce']}

    def test_exact_terminal_receipt_and_no_replay(self):
        self.phases.begin('file_db')
        self.phases.complete('file_db', self.receipt('file_db'))
        self.assertEqual(self.journal.state['steps']['dump-file_db']['status'], 'done')
        with self.assertRaises(phase.RecoveryRequired): self.phases.begin('file_db')

    def test_sequential_distinct_members_use_same_owned_invocation(self):
        self.phases.begin('file_db')
        self.phases.complete('file_db', self.receipt('file_db'))
        self.phases.begin('roles')
        self.phases.complete('roles', self.receipt('roles'))
        self.assertEqual(self.ledger.row['backup_members']['file_db']['status'], 'done')
        self.assertEqual(self.ledger.row['backup_members']['roles']['status'], 'done')

    def test_uncertain_between_guard_and_final_read_refuses_submit(self):
        actual_guard = self.phases._guard
        def change_after_guard():
            actual_guard()
            with self.ledger.lock:
                if self.ledger.row.get('backup_members', {}).get('file_db', {}).get('status') == 'pending':
                    self.ledger.row['status'] = 'uncertain'
                    self.ledger.version += 1
        with mock.patch.object(self.phases, '_guard', side_effect=change_after_guard):
            with self.assertRaises(phase.RecoveryRequired): self.phases.begin('file_db')

    def test_bad_receipt_marks_permanent_uncertain(self):
        self.phases.begin('story_db')
        wrong = self.receipt('story_db') | {'pod_uid': 'different'}
        with self.assertRaises(phase.RecoveryRequired): self.phases.complete('story_db', wrong)
        self.assertEqual(self.ledger.row['status'], 'uncertain')
        with self.assertRaises(phase.RecoveryRequired): self.phases.begin('messaging_db')

    def test_boolean_restart_count_is_not_an_integer_receipt(self):
        self.phases.begin('story_db')
        wrong = self.receipt('story_db') | {'restart_count': True}
        with self.assertRaises(phase.RecoveryRequired): self.phases.complete('story_db', wrong)
        self.assertEqual(self.ledger.row['status'], 'uncertain')

    def test_two_journals_cannot_reserve_the_same_dump(self):
        other = journal.Journal(Path(self.temp.name) / 'other-journal', self.intent)
        self.addCleanup(other.close)
        second = phase.BackupPhases(other, self.fence, Client(self.lock), self.lock, 'ns')
        barrier = threading.Barrier(2, timeout=5)
        actual_write = self.ledger.write
        reservations = []
        def racing_write(revision, row):
            if row.get('status') == 'active' and 'backup_invocation' in row and 'backup_members' not in row:
                barrier.wait()
                actual_write(revision, row)
                reservations.append('committed')
                return
            return actual_write(revision, row)
        outcomes = []
        def attempt(obj):
            try:
                obj.begin('file_db')
                outcomes.append('admitted')
            except phase.RecoveryRequired:
                outcomes.append('refused')
        with mock.patch.object(self.ledger, 'write', side_effect=racing_write):
            threads = [threading.Thread(target=attempt, args=(obj,)) for obj in (self.phases, second)]
            for thread in threads: thread.start()
            for thread in threads: thread.join(10)
        self.assertTrue(all(not thread.is_alive() for thread in threads))
        self.assertEqual(len(outcomes), 2)
        self.assertEqual(len(reservations), 1)
        self.assertLessEqual(outcomes.count('admitted'), 1)

    def test_committed_done_with_lost_ack_and_failed_uncertain_blocks_new_invocation(self):
        self.phases.begin('file_db')
        actual_write = self.ledger.write
        def lost_after_done(revision, row):
            actual_write(revision, row)
            if row.get('backup_members', {}).get('file_db', {}).get('status') == 'done':
                raise OSError('lost response')
        with mock.patch.object(self.ledger, 'write', side_effect=lost_after_done), \
                mock.patch.object(self.fence, 'uncertain', side_effect=OSError('unavailable')):
            with self.assertRaises(phase.RecoveryRequired):
                self.phases.complete('file_db', self.receipt('file_db'))
        other = journal.Journal(Path(self.temp.name) / 'restart-journal', self.intent)
        self.addCleanup(other.close)
        restarted = phase.BackupPhases(other, self.fence, Client(self.lock), self.lock, 'ns')
        with self.assertRaises(phase.RecoveryRequired): restarted.begin('story_db')

    def test_lost_done_save_marks_uncertain_and_never_replays(self):
        self.phases.begin('messaging_db')
        with mock.patch.object(self.journal, 'save', side_effect=OSError('lost')):
            with self.assertRaises(phase.RecoveryRequired):
                self.phases.complete('messaging_db', self.receipt('messaging_db'))
        self.assertEqual(self.ledger.row['status'], 'uncertain')
        with self.assertRaises(phase.RecoveryRequired): self.phases.begin('messaging_db')

    def test_unknown_member_and_mismatched_intent_never_begin(self):
        with self.assertRaises(phase.RecoveryRequired): self.phases.begin('other')
        self.journal.intent['namespace_uid'] = 'different'
        with self.assertRaises(phase.RecoveryRequired): self.phases.begin('roles')
        self.assertEqual(self.journal.state['steps'], {})


if __name__ == '__main__': unittest.main()
