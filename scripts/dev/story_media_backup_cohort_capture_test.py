import io
import json
from types import SimpleNamespace
import unittest

import story_media_backup_capture as media
import story_media_backup_capture_test as fixtures
import story_media_backup_cohort_capture as capture


class CohortCaptureTest(unittest.TestCase):
    def setUp(self):
        self.fixture = fixtures.CaptureTest(); self.fixture.setUp()
        self.expected = self.fixture.expected
        self.expected.update(operation='media-c2-' + 'a' * 32, snapshot_id='capture-' + 'b' * 32)
        engine = {name: self.expected[name] for name in
                  ('namespace_uid', 'pod_uid', 'pvc_uid', 'pv_uid', 'container_id', 'restart_count')}
        source = {'engine': engine, 'server_start': '2026-10-28T21:38:52+00:00'}
        self.binding = {'operation_id': self.expected['operation'], 'snapshot_id': self.expected['snapshot_id'],
                        'source_sha': 'c' * 40, 'databases': ['auth_db', 'voice_db'], 'sources': {
                            'auth_db': {**source, 'database_oid': 10},
                            'voice_db': {**source, 'database_oid': 11}, 'roles': source}}
        self.fault = None

    def runner(self, command, **kwargs):
        if 'pg_postmaster_start_time' in ' '.join(command):
            db = command[-2]
            value = {'database': db, 'database_oid': {'auth_db': 10, 'voice_db': 11, 'postgres': 5}[db],
                     'role': 'source_admin', 'server_start': '2026-10-29T00:38:52+03:00'}
            if self.fault == 'oid_post' and self.fixture.execs: value['database_oid'] = 99
            if self.fault == 'role_pre': value['role'] = 'other'
            if self.fault == 'start_post' and self.fixture.execs: value['server_start'] = '2026-10-28T21:38:53+00:00'
            if self.fault == 'guard_post' and self.fixture.execs: self.fixture.lost = True
            return SimpleNamespace(returncode=0, stdout=json.dumps(value).encode())
        return self.fixture.runner(command, **kwargs)

    def make(self):
        return capture.Producer(self.binding, ['auth_db', 'voice_db'], self.expected, 'source_admin',
                                self.fixture.guard, self.fixture.drain, self.fixture.journal, self.runner)

    def test_exact_receipt_and_roles(self):
        producer = self.make(); output = io.BytesIO(); producer('auth_db', output)
        self.assertEqual(output.getvalue(), b'private fixture')
        receipt = self.fixture.journal.events[-1][2]
        self.assertEqual((receipt['operation_id'], receipt['database_oid'], receipt['server_start']),
                         (self.expected['operation'], 10, '2026-10-28T21:38:52+00:00'))
        self.assertEqual(receipt['attempt_nonce'], 'a' * 32)
        producer('roles', io.BytesIO())
        self.assertIsNone(self.fixture.journal.events[-1][2]['database_oid'])

    def test_identity_and_authority_faults_never_complete(self):
        for fault in ('role_pre', 'oid_post', 'start_post', 'guard_post'):
            with self.subTest(fault=fault):
                self.setUp(); self.fault = fault
                with self.assertRaises(media.CaptureError): self.make()('voice_db', io.BytesIO())
                self.assertFalse(any(event[0] == 'complete' for event in self.fixture.journal.events))
                self.assertEqual(self.fixture.execs, 0 if fault == 'role_pre' else 1)
                if fault != 'role_pre': self.assertEqual(self.fixture.journal.events[-1][0], 'uncertain')

    def test_unadmitted_member_never_starts(self):
        with self.assertRaises(media.CaptureError): self.make()('gateway_db', io.BytesIO())
        self.assertEqual(self.fixture.journal.events, [])

    def test_unknown_dump_and_invalid_nonce_are_uncertain_without_retry(self):
        for fault in ('timeout', 'nonce'):
            with self.subTest(fault=fault):
                self.setUp()
                if fault == 'timeout': self.fixture.fault = 'timeout'
                else: self.fixture.journal.begin = lambda member: 'invalid'
                with self.assertRaises(media.CaptureUncertain): self.make()('voice_db', io.BytesIO())
                self.assertEqual(self.fixture.journal.events[-1][0], 'uncertain')
                self.assertFalse(any(e[0] == 'complete' for e in self.fixture.journal.events))
                self.assertEqual(self.fixture.execs, int(fault == 'timeout'))

    def test_binding_is_frozen_and_engine_mismatch_rejected(self):
        producer = self.make()
        self.binding['sources']['auth_db']['database_oid'] = 999
        producer('auth_db', io.BytesIO())
        self.assertEqual(self.fixture.journal.events[-1][2]['database_oid'], 10)
        self.setUp(); self.expected['pv_uid'] = 'different'
        with self.assertRaises(media.CaptureError): self.make()


if __name__ == '__main__': unittest.main()
