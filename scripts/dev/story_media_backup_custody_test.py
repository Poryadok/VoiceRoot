import copy
from datetime import datetime, timedelta, timezone
import hashlib
import io
import json
import multiprocessing
import os
from pathlib import Path
import tarfile
import tempfile
from types import SimpleNamespace
import unittest
from unittest.mock import patch
import zipfile

import story_media_backup as backup
import story_media_backup_custody as custody


class CustodyTest(unittest.TestCase):
    def test_preupload_fifo_is_rejected_without_blocking(self):
        fifo = self.root / 'fifo.tar'
        os.mkfifo(fifo, 0o600)
        def attempt():
            try:
                custody.verify_bundle(fifo, self.key, self.binding)
            except custody.CustodyError:
                return
            raise RuntimeError('FIFO was accepted')
        process = multiprocessing.get_context('fork').Process(target=attempt)
        process.start(); process.join(2)
        stuck = process.is_alive()
        if stuck:
            process.terminate(); process.join(5)
        self.assertFalse(stuck, 'FIFO open blocked before validation')
        self.assertEqual(process.exitcode, 0)

    def test_member_and_readback_fifo_are_rejected_without_blocking(self):
        for fault in ('member', 'bundle'):
            with self.subTest(fault=fault):
                self.setUp()
                path = self.root / ('file_db.enc' if fault == 'member' else 'fifo.tar')
                if path.exists(): path.unlink()
                os.mkfifo(path, 0o600)
                def attempt():
                    try:
                        if fault == 'member':
                            backup.verify(self.root, self.key, self.binding, self.manifest)
                        else:
                            custody.readback_extract('fixture-token', self.artifact, 23, path,
                                                    self.key, self.binding, self.destination)
                    except (backup.BackupError, custody.CustodyError):
                        return
                    raise RuntimeError('FIFO was accepted')
                process = multiprocessing.get_context('fork').Process(target=attempt)
                process.start(); process.join(2)
                stuck = process.is_alive()
                if stuck:
                    process.terminate(); process.join(5)
                self.assertFalse(stuck, 'FIFO open blocked before validation')
                self.assertEqual(process.exitcode, 0)

    def rewrap(self, raw):
        self.raw = raw
        self.bundle.write_bytes(raw)
        self.artifact['bundle_bytes'] = len(raw)
        self.artifact['bundle_sha256'] = hashlib.sha256(raw).hexdigest()
        archive_bytes = io.BytesIO()
        with zipfile.ZipFile(archive_bytes, 'w', compression=zipfile.ZIP_STORED) as archive:
            archive.writestr('media-backup.tar', raw)
        self.archive = archive_bytes.getvalue(); self.metadata['size_in_bytes'] = len(self.archive)

    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name) / 'source'; self.root.mkdir(mode=0o700)
        self.destination = Path(self.tmp.name) / 'restore'; self.destination.mkdir(mode=0o700)
        self.key = bytes(range(32))
        self.operation = 'media-c1-' + 'a' * 32
        self.snapshot = 'capture-' + 'b' * 32
        self.binding = {'operation_id': self.operation, 'snapshot_id': self.snapshot, 'source_sha': 'c' * 40}
        self.manifest = backup.export(self.root, self.key, self.binding,
                                      lambda member, output: output.write(('private-' + member).encode()))
        self.bundle = custody.build_bundle(self.root, self.key, self.binding, self.manifest)
        self.raw = self.bundle.read_bytes()
        now = datetime.now(timezone.utc)
        self.artifact = {'operation_id': self.operation, 'snapshot_id': self.snapshot, 'challenge': 'd' * 32,
                         'run_id': 42, 'head_sha': 'e' * 40, 'created_at': (now-timedelta(seconds=1)).isoformat(),
                         'bundle_bytes': len(self.raw), 'bundle_sha256': hashlib.sha256(self.raw).hexdigest()}
        self.metadata = {'id': 23, 'url': custody.API + '23',
                         'name': 'voice-media-backup-' + self.operation + '-' + self.artifact['challenge'],
                         'expired': False, 'created_at': now.isoformat(), 'expires_at': (now+timedelta(days=1)).isoformat(),
                         'workflow_run': {'id': 42, 'head_sha': 'e' * 40}}
        archive = io.BytesIO()
        with zipfile.ZipFile(archive, 'w', compression=zipfile.ZIP_STORED) as stream:
            stream.writestr('media-backup.tar', self.raw)
        self.archive = archive.getvalue()
        self.metadata['size_in_bytes'] = len(self.archive)

    def download(self, url, headers, target, limit, deadline, redirects=False):
        value = self.archive if redirects else json.dumps(self.metadata).encode()
        target.write(value)
        return len(value)

    def extract(self, binding=None, key=None):
        with patch.object(custody.transport, '_download', side_effect=self.download):
            return custody.readback_extract('fixture-token', binding or self.artifact, 23, self.bundle,
                                           key or self.key, self.binding, self.destination)

    def test_full_readback_extracts_only_authenticated_encrypted_members(self):
        receipt, manifest = self.extract()
        self.assertEqual(receipt['artifact_id'], 23)
        self.assertEqual(manifest, self.manifest)
        self.assertEqual(backup.verify(self.destination, self.key, self.binding, manifest), backup.MEMBERS)
        self.assertEqual({p.name for p in self.destination.iterdir()},
                         {'manifest.json', *(name + '.enc' for name in backup.MEMBERS)})
        self.assertNotIn(self.key.hex(), repr(receipt))

    def test_preupload_verification_returns_only_bound_size_and_digest(self):
        result = custody.verify_bundle(self.bundle, self.key, self.binding)
        self.assertEqual(result, {'bytes': len(self.raw), 'sha256': hashlib.sha256(self.raw).hexdigest()})
        self.assertNotIn(self.key.hex(), repr(result))
        with self.assertRaises(custody.CustodyError):
            custody.verify_bundle(self.bundle, bytes(reversed(self.key)), self.binding)
        link = self.root / 'alias.tar'; link.symlink_to(self.bundle.name)
        with self.assertRaises(custody.CustodyError): custody.verify_bundle(link, self.key, self.binding)

    def test_preupload_path_replacement_is_rejected(self):
        original = custody._inspect_bundle
        def replace(stream, key, binding, deadline=None):
            result = original(stream, key, binding, deadline)
            self.bundle.rename(self.root / 'old.tar')
            self.bundle.write_bytes(self.raw); self.bundle.chmod(0o600)
            return result
        with patch.object(custody, '_inspect_bundle', side_effect=replace):
            with self.assertRaises(custody.CustodyError):
                custody.verify_bundle(self.bundle, self.key, self.binding)

    def test_wrong_artifact_identity_or_tamper_rejected_without_extract(self):
        for fault in ('head', 'run', 'name', 'snapshot', 'bytes', 'key'):
            with self.subTest(fault=fault):
                self.setUp()
                if fault == 'head': self.metadata['workflow_run']['head_sha'] = 'f' * 40
                if fault == 'run': self.metadata['workflow_run']['id'] = 43
                if fault == 'name': self.metadata['name'] += '-other'
                if fault == 'snapshot': self.artifact['snapshot_id'] = 'capture-' + 'c' * 32
                if fault == 'bytes': self.archive = self.archive[:-1] + b'X'
                with self.assertRaises(custody.CustodyError):
                    self.extract(key=bytes(reversed(self.key)) if fault == 'key' else None)
                self.assertEqual(list(self.destination.iterdir()), [])

    def test_traversal_link_extra_or_key_member_cannot_be_extracted(self):
        for fault in ('traversal', 'link', 'extra', 'key'):
            with self.subTest(fault=fault):
                self.setUp()
                malformed = io.BytesIO()
                with tarfile.open(fileobj=malformed, mode='w') as archive:
                    encoded = json.dumps(self.manifest, sort_keys=True, separators=(',', ':')).encode()
                    header = tarfile.TarInfo('manifest.json'); header.mode = 0o600; header.size = len(encoded)
                    archive.addfile(header, io.BytesIO(encoded))
                    for member in sorted(backup.MEMBERS):
                        name = member + '.enc'
                        content = (self.root / name).read_bytes()
                        if fault == 'traversal' and member == 'file_db': name = '../file_db.enc'
                        header = tarfile.TarInfo(name); header.mode = 0o600
                        if fault == 'link' and member == 'file_db':
                            header.type = tarfile.SYMTYPE; header.linkname = '/etc/passwd'; header.size = 0
                            archive.addfile(header)
                        else:
                            header.size = len(content); archive.addfile(header, io.BytesIO(content))
                    if fault in ('extra', 'key'):
                        header = tarfile.TarInfo('backup.key' if fault == 'key' else 'extra.enc'); header.mode = 0o600
                        header.size = 1; archive.addfile(header, io.BytesIO(b'x'))
                self.rewrap(malformed.getvalue())
                with self.assertRaises(custody.CustodyError): self.extract()
                self.assertEqual(list(self.destination.iterdir()), [])

    def test_data_after_tar_eof_is_never_accepted(self):
        self.rewrap(self.raw + b'private key outside tar inventory')
        with self.assertRaises(custody.CustodyError): self.extract()
        self.assertEqual(list(self.destination.iterdir()), [])

    def test_low_capacity_fails_before_extraction(self):
        # This amount passed the old ZIP-only admission but cannot hold the
        # concurrent ZIP, restored tar and extracted ciphertext on one device.
        free = self.metadata['size_in_bytes'] + custody.CHUNK + 1
        with patch.object(custody.shutil, 'disk_usage', return_value=SimpleNamespace(free=free)):
            with self.assertRaises(custody.CustodyError): self.extract()
        self.assertEqual(list(self.destination.iterdir()), [])

    def test_bundle_build_requires_capacity_before_creating_output(self):
        self.bundle.unlink()
        with patch.object(custody, '_free_fd', return_value=1):
            with self.assertRaises(custody.CustodyError):
                custody.build_bundle(self.root, self.key, self.binding, self.manifest)
        self.assertFalse(self.bundle.exists())

    def test_nonempty_destination_is_rejected_without_overwriting(self):
        sentinel = self.destination / 'unrelated'
        sentinel.write_bytes(b'preserve')
        with self.assertRaises(custody.CustodyError): self.extract()
        self.assertEqual(sentinel.read_bytes(), b'preserve')
        self.assertEqual({p.name for p in self.destination.iterdir()}, {'unrelated'})

    def test_extension_header_is_rejected_without_parsing_payload(self):
        extension = tarfile.TarInfo('extension')
        extension.type = tarfile.XHDTYPE
        extension.size = 1024 ** 3
        self.rewrap(extension.tobuf(format=tarfile.USTAR_FORMAT) + self.raw)
        with self.assertRaises(custody.CustodyError): self.extract()
        self.assertEqual(list(self.destination.iterdir()), [])

    def test_header_cannot_hide_plaintext_in_owner_name(self):
        raw = bytearray(self.raw)
        secret = b'PRIVATE_KEY_FIXTURE'
        raw[265:265+len(secret)] = secret
        raw[148:156] = b'        '
        raw[148:156] = ('%06o\0 ' % sum(raw[:512])).encode('ascii')
        self.rewrap(bytes(raw))
        with self.assertRaises(custody.CustodyError): self.extract()
        self.assertEqual(list(self.destination.iterdir()), [])

    def test_member_too_large_for_ustar_is_rejected_before_bundle_creation(self):
        self.bundle.unlink()
        original = os.stat(self.root / 'file_db.enc')
        huge = SimpleNamespace(st_mode=original.st_mode, st_uid=original.st_uid,
                               st_nlink=original.st_nlink, st_size=8 ** 11)
        with patch.object(custody.os, 'stat', return_value=huge):
            with self.assertRaises(custody.CustodyError):
                custody.build_bundle(self.root, self.key, self.binding, self.manifest)
        self.assertFalse(self.bundle.exists())


if __name__ == '__main__': unittest.main()
