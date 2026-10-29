import hashlib
import base64
import io
import os
import tarfile
import zlib
import re
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch

import dev_peer_issuer_installer as installer


class InstallerTests(unittest.TestCase):
    def test_wrong_digest_never_touches_install_paths(self):
        with patch.object(installer.os, 'geteuid', return_value=0), \
             patch.object(installer.custody, '_check_ancestors') as custody:
            with self.assertRaises(ValueError):
                installer.install(b'archive', 'a' * 64)
            custody.assert_not_called()

    def test_symlink_is_rejected_before_filesystem_mutation(self):
        output = io.BytesIO()
        with tarfile.open(fileobj=output, mode='w') as archive:
            entry = tarfile.TarInfo(installer.authority.FILES[0])
            entry.type = tarfile.SYMTYPE
            entry.linkname = '/etc/shadow'
            archive.addfile(entry)
        raw = output.getvalue()
        with patch.object(installer.os, 'geteuid', return_value=0), \
             patch.object(installer.custody, '_check_ancestors') as custody:
            with self.assertRaises(ValueError):
                installer.install(raw, hashlib.sha256(raw).hexdigest())
            custody.assert_not_called()

    def test_unreviewed_source_cannot_build_bundle(self):
        with self.assertRaises(installer.reviewed.SourceError):
            installer.build('/not/a/checkout', 'a' * 40, '/tmp/not-created')

    def test_inline_bootstrap_binds_exact_digest_and_fixed_archive(self):
        script = installer.bootstrap('a' * 64)
        self.assertEqual(len(script.splitlines()), 1)
        encoded = re.search(r'b64decode\("([A-Za-z0-9+/=]+)"\)', script).group(1)
        program = zlib.decompress(base64.b64decode(encoded, validate=True)).decode()
        compile(program, '<bootstrap>', 'exec')
        self.assertIn("EXPECTED = '" + 'a' * 64 + "'", program)
        self.assertIn(installer.BOOTSTRAP_ARCHIVE, program)
        with self.assertRaises(ValueError):
            installer.bootstrap('bad')

    @unittest.skipUnless(os.name == 'posix' and os.path.exists('/.dockerenv') and os.geteuid() == 0,
                         'isolated root container required')
    def test_install_fixed_bundle_without_starting_systemd(self):
        units = [Path('/etc/systemd/system/voice-dev-peer-issuer.socket'),
                 Path('/etc/systemd/system/voice-dev-peer-issuer@.service')]
        if any(path.exists() or path.is_symlink() for path in units):
            self.skipTest('fixed unit path already exists')
        output = io.BytesIO()
        with tarfile.open(fileobj=output, mode='w') as archive:
            for name in installer.authority.FILES:
                item = tarfile.TarInfo(name)
                item.size = 7
                archive.addfile(item, io.BytesIO(b'fixture'))
        raw = output.getvalue()
        with tempfile.TemporaryDirectory() as temp:
            target = Path(temp) / 'installed'
            try:
                with patch.object(installer.authority, 'INSTALLED', target), \
                     patch.object(installer.custody, '_check_ancestors'), \
                     patch.object(installer.subprocess, 'run') as systemctl:
                    installer.install(raw, hashlib.sha256(raw).hexdigest())
                    self.assertEqual(len(systemctl.call_args_list), 2)
                    self.assertTrue(all((target / name).read_bytes() == b'fixture' for name in installer.authority.FILES))
                    self.assertEqual(units[0].read_bytes(), installer.SOCKET_UNIT)
                    self.assertEqual(units[1].read_bytes(), installer.SERVICE_UNIT)
                    with self.assertRaises(FileExistsError):
                        installer.install(raw, hashlib.sha256(raw).hexdigest())
                    self.assertEqual(len(systemctl.call_args_list), 2)
            finally:
                for unit in units:
                    if unit.exists() and not unit.is_symlink():
                        unit.unlink()

    @unittest.skipUnless(os.name == 'posix' and os.path.exists('/.dockerenv') and os.geteuid() == 0,
                         'isolated root container required')
    def test_prepared_archive_installs_only_inside_fixture(self):
        archive_path = os.environ.get('VOICE_PEER_TEST_ARCHIVE')
        if not archive_path:
            self.skipTest('prepared archive not supplied')
        raw = Path(archive_path).read_bytes()
        digest = hashlib.sha256(raw).hexdigest()
        self.assertEqual(digest, os.environ['VOICE_PEER_TEST_SHA256'])
        units = [Path('/etc/systemd/system/voice-dev-peer-issuer.socket'),
                 Path('/etc/systemd/system/voice-dev-peer-issuer@.service')]
        if any(path.exists() or path.is_symlink() for path in units):
            self.skipTest('fixed unit path already exists')
        with tempfile.TemporaryDirectory() as temp:
            target = Path(temp) / 'installed'
            try:
                with patch.object(installer.authority, 'INSTALLED', target), \
                     patch.object(installer.custody, '_check_ancestors'), \
                     patch.object(installer.subprocess, 'run') as systemctl:
                    installer.install(raw, digest)
                    self.assertTrue(all((target / name).is_file() for name in installer.authority.FILES))
                    self.assertEqual(len(systemctl.call_args_list), 2)
                    self.assertEqual(units[0].read_bytes(), installer.SOCKET_UNIT)
                    self.assertEqual(units[1].read_bytes(), installer.SERVICE_UNIT)
            finally:
                for unit in units:
                    if unit.exists() and not unit.is_symlink():
                        unit.unlink()


if __name__ == '__main__':
    unittest.main()
