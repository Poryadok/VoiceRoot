import copy
from datetime import datetime, timedelta, timezone
import hashlib
import io
import json
import struct
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import zipfile

import story_media_backup_cohort as cohort
import story_media_backup_cohort_custody as custody


class CohortCustodyTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name) / 'src'; self.root.mkdir(mode=0o700)
        self.destination = Path(self.tmp.name) / 'dst'; self.destination.mkdir(mode=0o700)
        self.key = bytes(range(32)); self.databases = ['auth_db', 'voice_db']
        self.op = 'media-c1-' + 'a' * 32; self.snapshot = 'capture-' + 'b' * 32
        self.binding = {'operation_id': self.op, 'snapshot_id': self.snapshot, 'source_sha': 'c' * 40,
            'databases': self.databases, 'sources': {
                'auth_db': {'engine': {'fixture': 'same'}, 'server_start': 'fixture', 'database_oid': 1},
                'voice_db': {'engine': {'fixture': 'same'}, 'server_start': 'fixture', 'database_oid': 2},
                'roles': {'engine': {'fixture': 'same'}, 'server_start': 'fixture'}},
            'dump_receipts': {m: 'd' * 64 for m in (*self.databases, 'roles')}}
        self.manifest = cohort.export(self.root, self.key, self.binding, self.databases,
                                      lambda member, out: out.write(('fixture-' + member).encode()))
        self.path = custody.build_bundle(self.root, self.key, self.binding, self.databases, self.manifest)
        self.raw = self.path.read_bytes(); self.wire = io.BytesIO()
        with zipfile.ZipFile(self.wire, 'w', compression=zipfile.ZIP_STORED) as archive:
            archive.writestr('cohort-backup.tar', self.raw)
        self.wire = self.wire.getvalue(); now = datetime.now(timezone.utc)
        self.artifact = {'operation_id': self.op, 'snapshot_id': self.snapshot, 'challenge': 'e' * 32,
            'run_id': 42, 'head_sha': 'f' * 40, 'created_at': (now-timedelta(seconds=1)).isoformat(),
            'bundle_bytes': len(self.raw), 'bundle_sha256': hashlib.sha256(self.raw).hexdigest()}
        self.metadata = {'id': 23, 'url': custody.API + '23', 'name': 'voice-cohort-backup-' + self.op + '-' + 'e' * 32,
            'expired': False, 'created_at': now.isoformat(), 'expires_at': (now+timedelta(days=1)).isoformat(),
            'workflow_run': {'id': 42, 'head_sha': 'f' * 40}, 'size_in_bytes': len(self.wire)}

    def download(self, url, headers, target, limit, deadline, redirects=False):
        raw = self.wire if redirects else json.dumps(self.metadata).encode()
        target.write(raw); return len(raw)

    def extract(self):
        with patch.object(custody.transport, '_download', side_effect=self.download):
            return custody.readback_extract('fixture-token', self.artifact, 23, self.path, self.key,
                                           self.binding, self.databases, self.destination)

    def test_full_readback_extracts_exact_selected_cohort(self):
        result = custody.verify_bundle(self.path, self.key, self.binding, self.databases)
        self.assertEqual(result, {'bytes': len(self.raw), 'sha256': hashlib.sha256(self.raw).hexdigest()})
        receipt, manifest = self.extract()
        self.assertEqual(receipt['artifact_id'], 23)
        self.assertEqual(cohort.verify(self.destination, self.key, self.binding, self.databases, manifest),
                         {'auth_db', 'voice_db', 'roles'})
        self.assertEqual({p.name for p in self.destination.iterdir()},
                         {'manifest.json', 'auth_db.enc', 'voice_db.enc', 'roles.enc'})

    def test_foreign_or_incomplete_artifact_fails_closed(self):
        for fault in ('name', 'run', 'snapshot', 'truncated'):
            with self.subTest(fault=fault):
                self.setUp()
                if fault == 'name': self.metadata['name'] += '-foreign'
                if fault == 'run': self.metadata['workflow_run']['id'] += 1
                if fault == 'snapshot': self.artifact['snapshot_id'] = 'capture-' + 'd' * 32
                if fault == 'truncated': self.wire = self.wire[:-1]
                with self.assertRaises(custody.CohortCustodyError): self.extract()
                self.assertEqual(list(self.destination.iterdir()), [])

    def test_trailing_plaintext_or_wrong_key_is_rejected(self):
        with self.assertRaises(custody.CohortCustodyError):
            custody.verify_bundle(self.path, bytes(reversed(self.key)), self.binding, self.databases)
        self.path.write_bytes(self.raw + b'private key outside tar')
        with self.assertRaises(custody.CohortCustodyError):
            custody.verify_bundle(self.path, self.key, self.binding, self.databases)

    def test_occupied_destination_is_preserved(self):
        sentinel = self.destination / 'unrelated'; sentinel.write_bytes(b'preserve')
        with self.assertRaises(custody.CohortCustodyError): self.extract()
        self.assertEqual(sentinel.read_bytes(), b'preserve')
        self.assertEqual({p.name for p in self.destination.iterdir()}, {'unrelated'})

    def test_zip64_oversized_directory_rejected_before_parser(self):
        end = len(self.wire) - 22
        fields = struct.unpack('<4s4H2LH', self.wire[end:])
        record = struct.pack('<4sQ2H2L4Q', b'PK\x06\x06', 44, 45, 45, 0, 0,
                             1, 1, 65536, fields[6])
        locator = struct.pack('<4sLQL', b'PK\x06\x07', 0, end, 1)
        self.wire = self.wire[:end] + record + locator + self.wire[end:]
        self.metadata['size_in_bytes'] = len(self.wire)
        with patch.object(custody.zipfile, 'ZipFile') as parser:
            with self.assertRaises(custody.CohortCustodyError): self.extract()
            parser.assert_not_called()


if __name__ == '__main__': unittest.main()
