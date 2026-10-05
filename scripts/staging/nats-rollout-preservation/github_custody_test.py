import copy
from datetime import datetime, timezone, timedelta
import hashlib
import io
import json
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch
import zipfile
import stat
import time
import github_custody as custody


def archive(content=b'private-ciphertext', extra=False):
    stream = io.BytesIO()
    with zipfile.ZipFile(stream, 'w') as z:
        z.writestr('rollout-backup.cms', content)
        if extra: z.writestr('extra', b'no')
    return stream.getvalue()


class Reply(io.BytesIO):
    def __init__(self, status, body=b'', headers=None):
        super().__init__(body)
        self.status, self.headers = status, headers or {}


class CustodyTests(unittest.TestCase):
    def setUp(self):
        self.directory = tempfile.TemporaryDirectory()
        self.addCleanup(self.directory.cleanup)
        self.path = Path(self.directory.name) / 'cipher.cms'
        self.path.write_bytes(b'private-ciphertext')
        self.binding = {'operation': 'a' * 32, 'challenge': 'b' * 32, 'run_id': 123,
            'head_sha': 'c' * 40, 'cipher_sha256': hashlib.sha256(self.path.read_bytes()).hexdigest(),
            'cipher_bytes': self.path.stat().st_size, 'created_at': datetime.now(timezone.utc).isoformat()}
        self.zip = archive()
        self.metadata = {'id': 99, 'name': 'voice-nats-rollout-' + self.binding['operation'] + '-' + self.binding['challenge'],
            'url': 'https://api.github.com/repos/Poryadok/VoiceRoot/actions/artifacts/99',
            'workflow_run': {'id': 123, 'head_sha': 'c' * 40}, 'expired': False,
            'created_at': self.binding['created_at'], 'expires_at': (datetime.now(timezone.utc) + timedelta(days=1)).isoformat(), 'size_in_bytes': len(self.zip)}
        self.requests = []

    def request(self, url, headers, limit, deadline):
        self.requests.append((url, headers))
        if url.endswith('/99'): return Reply(200, json.dumps(self.metadata).encode())
        if url.endswith('/zip'): return Reply(302, headers={'Location': 'https://productionresultssa1.blob.core.windows.net/artifact?private=signed'})
        return Reply(200, self.zip)

    def verify(self):
        with patch.object(custody, '_request', self.request, create=True):
            return custody.verify_artifact('private-token', self.binding, self.path, 99)

    def test_valid_full_byte_readback_and_no_redirect_bearer(self):
        receipt = self.verify()
        self.assertTrue(receipt['verified'])
        self.assertEqual(receipt['destination'], 'github:Poryadok/VoiceRoot')
        self.assertEqual(receipt['operation'], self.binding['operation'])
        self.assertNotIn('Authorization', self.requests[-1][1])
        self.assertNotIn('private', json.dumps(receipt))

    def test_wrong_metadata(self):
        for key, value in [('name', 'wrong'), ('expired', True), ('created_at', '2000-01-01T00:00:00Z'), ('size_in_bytes', 1), ('url', 'https://evil.invalid')]:
            original = copy.deepcopy(self.metadata)
            self.metadata[key] = value
            with self.subTest(key=key), self.assertRaises(custody.CustodyError): self.verify()
            self.metadata = original
        for key in ('id', 'head_sha'):
            original = copy.deepcopy(self.metadata)
            self.metadata['workflow_run'][key] = 999 if key == 'id' else 'd' * 40
            with self.subTest(key=key), self.assertRaises(custody.CustodyError): self.verify()
            self.metadata = original

    def test_tampered_malformed_extra_member(self):
        for content in (archive(b'tampered'), b'not a zip', archive(extra=True)):
            self.zip = content
            self.metadata['size_in_bytes'] = len(content)
            with self.subTest(), self.assertRaises(custody.CustodyError): self.verify()

    def test_wrong_local_hash(self):
        self.path.write_bytes(b'tampered')
        with self.assertRaises(custody.CustodyError): self.verify()

    def test_redirect_not_allowlisted(self):
        def request(url, headers, limit, deadline): return Reply(302, headers={'Location': 'https://evil.invalid/?private=token'})
        with patch.object(custody, '_request', request, create=True), self.assertRaises(custody.CustodyError) as error:
            custody.verify_artifact('private-token', self.binding, self.path, 99)
        self.assertNotIn('private', str(error.exception))

    def test_zip_member_path_symlink_and_bomb_bounds(self):
        for name, mode, content in [('other.cms', stat.S_IFREG, b'private-ciphertext'),
                ('rollout-backup.cms', stat.S_IFLNK, b'private-ciphertext'),
                ('rollout-backup.cms', stat.S_IFREG, b'0' * 100000)]:
            stream = io.BytesIO()
            with zipfile.ZipFile(stream, 'w', compression=zipfile.ZIP_DEFLATED) as z:
                member = zipfile.ZipInfo(name)
                member.external_attr = mode << 16
                member.compress_type = zipfile.ZIP_DEFLATED
                z.writestr(member, content)
            self.zip = stream.getvalue()
            self.metadata['size_in_bytes'] = len(self.zip)
            with self.subTest(name=name, mode=mode), self.assertRaises(custody.CustodyError): self.verify()

    def test_body_and_deadline_bounds(self):
        with self.assertRaises(custody.CustodyError):
            custody._copy(Reply(200, b'x' * 101), io.BytesIO(), 100, time.monotonic() + 5)
        with self.assertRaises(custody.CustodyError):
            custody._copy(Reply(200, b'x'), io.BytesIO(), 100, time.monotonic() - 1)

    def test_redirect_count_bound(self):
        def request(url, headers, limit, deadline):
            self.requests.append((url, headers))
            return Reply(302, headers={'Location': 'https://productionresultssa1.blob.core.windows.net/loop'})
        with patch.object(custody, '_request', request), self.assertRaises(custody.CustodyError):
            custody._download(custody.API + '99/zip', {'Authorization': 'Bearer private-token'}, io.BytesIO(), 100, time.monotonic() + 5, redirects=True)
        self.assertEqual(len(self.requests), 4)
        self.assertTrue(all('Authorization' not in headers for _, headers in self.requests[1:]))

    def test_archive_expired_timestamp_even_false_flag(self):
        self.metadata['expires_at'] = '2000-01-01T00:00:00Z'
        with self.assertRaises(custody.CustodyError): self.verify()


if __name__ == '__main__': unittest.main()
