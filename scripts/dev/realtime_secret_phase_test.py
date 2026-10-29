import copy
from pathlib import Path
import threading
import unittest
from unittest.mock import patch

import media_backup_phase_test as fixtures
import media_operation_journal as journal
import realtime_secret_phase as phase


class Client(fixtures.Client):
    def __init__(self, lock):
        super().__init__(lock)
        self.secret = None
        self.lost = False

    def call(self, *args, body=None):
        if args[:2] == ('create', '-f'):
            if self.secret is not None: raise RuntimeError('already exists')
            self.secret = copy.deepcopy(body)
            self.secret['metadata'].update(uid='uid', resourceVersion='10')
            if self.lost: raise OSError('response lost')
            return copy.deepcopy(self.secret)
        if args[:2] == ('get', 'secret'):
            if self.secret is None: raise RuntimeError('missing')
            return copy.deepcopy(self.secret)
        return super().call(*args)


class SecretPhaseTest(unittest.TestCase):
    def setup_phase(self):
        fixture = fixtures.PhaseTest(); fixture.setUp(); self.addCleanup(fixture.doCleanups)
        client = Client(fixture.lock)
        return fixture, client, phase.SecretPhase(fixture.journal, fixture.fence, client, fixture.lock, 'ns')

    def test_create_receipt_and_no_replay(self):
        fixture, client, subject = self.setup_phase()
        receipt = subject.run(b'a'*32)
        self.assertEqual(receipt['uid'], 'uid')
        self.assertEqual(fixture.ledger.row['realtime_secret']['receipt'],
                         {k:receipt[k] for k in ('name','uid','resourceVersion')})
        self.assertNotIn('password_sha256', str(fixture.ledger.row))
        with self.assertRaises(phase.RecoveryRequired): subject.run(b'a'*32)
        self.assertNotIn('postgres://', str(fixture.journal.state))
        self.assertNotIn('postgres://', str(fixture.ledger.row))

    def test_lost_response_is_uncertain_and_no_adoption(self):
        fixture, client, subject = self.setup_phase()
        client.lost = True
        with self.assertRaises(phase.RecoveryRequired): subject.run(b'a'*32)
        self.assertEqual(fixture.ledger.row['status'], 'uncertain')
        with self.assertRaises(phase.RecoveryRequired): subject.run(b'a'*32)

    def test_existing_secret_is_not_adopted(self):
        fixture, client, subject = self.setup_phase()
        client.secret = {'metadata': {'name': 'foreign'}}
        with self.assertRaises(phase.RecoveryRequired): subject.run(b'a'*32)
        self.assertEqual(fixture.ledger.row['status'], 'uncertain')

    def test_changed_reservation_stops_before_create(self):
        fixture, client, subject = self.setup_phase()
        actual_guard = subject._guard
        def changed():
            actual_guard()
            if fixture.ledger.row.get('realtime_secret', {}).get('status') == 'pending':
                fixture.ledger.row['realtime_secret']['nonce'] = 'different'
        with patch.object(subject, '_guard', side_effect=changed):
            with self.assertRaises(phase.RecoveryRequired): subject.run(b'a'*32)
        self.assertIsNone(client.secret)
        self.assertEqual(fixture.ledger.row['status'], 'uncertain')

    def test_failed_terminal_save_never_replays(self):
        fixture, client, subject = self.setup_phase()
        real_save = fixture.journal.save
        def save():
            if fixture.journal.state['steps'].get('realtime-secret', {}).get('status') == 'done':
                raise OSError('disk error')
            real_save()
        with patch.object(fixture.journal, 'save', side_effect=save):
            with self.assertRaises(phase.RecoveryRequired): subject.run(b'a'*32)
        self.assertIsNotNone(client.secret)
        self.assertEqual(fixture.ledger.row['status'], 'uncertain')

    def test_done_cas_lost_ack_cannot_be_replayed(self):
        fixture, client, subject = self.setup_phase()
        actual = fixture.ledger.write
        def lost(revision, row):
            actual(revision, row)
            if row.get('realtime_secret', {}).get('status') == 'done': raise OSError('lost acknowledgement')
        with patch.object(fixture.ledger, 'write', side_effect=lost), \
                patch.object(fixture.fence, 'uncertain', side_effect=OSError('unavailable')):
            with self.assertRaises(phase.RecoveryRequired): subject.run(b'a'*32)
        other = journal.Journal(Path(fixture.temp.name)/'other', fixture.intent)
        self.addCleanup(other.close)
        with self.assertRaises(phase.RecoveryRequired):
            phase.SecretPhase(other,fixture.fence,client,fixture.lock,'ns').run(b'a'*32)
        self.assertEqual(fixture.ledger.row['realtime_secret']['status'], 'done')

    def test_competing_journals_at_most_one_create(self):
        fixture, client, subject = self.setup_phase()
        other = journal.Journal(Path(fixture.temp.name)/'other', fixture.intent)
        self.addCleanup(other.close)
        second = phase.SecretPhase(other,fixture.fence,client,fixture.lock,'ns')
        barrier = threading.Barrier(2, timeout=5)
        actual = fixture.ledger.write
        commits = []
        def race(revision, row):
            if row.get('realtime_secret', {}).get('status') == 'pending':
                barrier.wait()
                actual(revision,row)
                commits.append(1)
            else: actual(revision,row)
        results = []
        def invoke(item):
            try: item.run(b'a'*32); results.append('ok')
            except phase.RecoveryRequired: results.append('refused')
        with patch.object(fixture.ledger,'write',side_effect=race):
            threads = [threading.Thread(target=invoke,args=(item,)) for item in (subject,second)]
            for thread in threads: thread.start()
            for thread in threads: thread.join(10)
        self.assertTrue(all(not thread.is_alive() for thread in threads))
        self.assertEqual(len(commits),1)
        self.assertEqual(len(results),2)
        self.assertLessEqual(results.count('ok'),1)


if __name__ == '__main__': unittest.main()
