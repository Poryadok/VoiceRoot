import json
import os
import subprocess
import tempfile
from types import SimpleNamespace
import unittest
from pathlib import Path
from unittest.mock import patch

import realtime_p3_missing_issuer as issuer


class IssuerTests(unittest.TestCase):
    def test_public_input_rejects_fifo_symlink_and_oversize(self):
        with tempfile.TemporaryDirectory() as temporary:
            root = Path(temporary)
            with patch.object(issuer.pwd, 'getpwnam', return_value=SimpleNamespace(pw_uid=os.getuid())):
                good = root / 'good'; good.write_bytes(b'public'); good.chmod(0o600)
                self.assertEqual(issuer._public_input(good), b'public')
                link = root / 'link'; link.symlink_to(good)
                with self.assertRaises(OSError): issuer._public_input(link)
                fifo = root / 'fifo'; os.mkfifo(fifo, 0o600)
                with self.assertRaises(ValueError): issuer._public_input(fifo)
                good.write_bytes(b'x' * 65537)
                with self.assertRaises(ValueError): issuer._public_input(good)

    def test_existing_destination_fails_before_key_generation(self):
        with tempfile.TemporaryDirectory() as parent:
            destination = Path(parent) / ('p3-' + '1' * 32)
            destination.mkdir()
            with patch.object(issuer, '_prepare') as prepare, patch.object(issuer, 'PARENT', Path(parent)), patch.object(issuer, '_check_ancestors'):
                with self.assertRaisesRegex(issuer.IssueError, 'issuance refused'):
                    issuer.stage(destination, b'principal', b'media', b'peer')
                prepare.assert_not_called()

    def test_private_failures_are_redacted(self):
        with tempfile.TemporaryDirectory() as parent:
            destination = Path(parent) / ('p3-' + '1' * 32)
            with patch.object(issuer, '_prepare', side_effect=ValueError('private information')), patch.object(issuer, 'PARENT', Path(parent)), patch.object(issuer, '_check_ancestors'):
                with self.assertRaises(issuer.IssueError) as error:
                    issuer.stage(destination, b'principal', b'media', b'peer')
                self.assertNotIn('private information', str(error.exception))
                self.assertTrue(destination.is_dir())
                with self.assertRaises(issuer.IssueError):
                    issuer.stage(Path(parent) / ('p3-' + '2' * 32), b'principal', b'media', b'peer')

    def test_real_crypto_creates_only_seven_private_objects(self):
        with tempfile.TemporaryDirectory() as scratch:
            root = Path(scratch)
            archive = root / 'archive'
            archive.mkdir(mode=0o700)
            def openssl(*args):
                subprocess.run(['openssl', *map(str, args)], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            for name in ('principal', 'media'):
                openssl('req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-days', '2',
                        '-subj', '/CN=fixture-' + name, '-addext', 'basicConstraints=critical,CA:TRUE',
                        '-keyout', archive / (name + '-ca.key'), '-out', archive / (name + '-ca.crt'))
                (archive / (name + '-ca.key')).chmod(0o600)
            openssl('req', '-new', '-newkey', 'rsa:2048', '-nodes', '-subj', '/CN=voice-space.voice-dev.svc',
                    '-keyout', archive / 'peer.key', '-out', archive / 'peer.csr')
            extensions = archive / 'extensions'
            extensions.write_text('basicConstraints=critical,CA:FALSE\nextendedKeyUsage=serverAuth\nsubjectAltName=DNS:voice-space.voice-dev.svc\n')
            openssl('x509', '-req', '-in', archive / 'peer.csr', '-CA', archive / 'media-ca.crt',
                    '-CAkey', archive / 'media-ca.key', '-set_serial', '42', '-days', '2',
                    '-extfile', extensions, '-out', archive / 'peer.crt')
            principal, media, peer = [(archive / name).read_bytes() for name in ('principal-ca.crt', 'media-ca.crt', 'peer.crt')]
            with patch.object(issuer, 'ARCHIVE', archive), patch.object(issuer, '_check_ancestors'), patch.object(issuer.reset, 'ResetMaterial') as material:
                for n, ca, public_ca in ((1, media, media), (2, principal, principal)):
                    trial = root / ('trial' + str(n))
                    trial.mkdir(mode=0o700)
                    with self.assertRaises(ValueError):
                        issuer._prepare(trial, ca, public_ca, peer)
                material.assert_not_called()
            stage = root / ('p3-' + '1' * 32)
            with patch.object(issuer, 'ARCHIVE', archive), patch.object(issuer, 'PARENT', root), patch.object(issuer, '_check_ancestors'):
                receipt = issuer.stage(stage, principal, media, peer)
            self.assertEqual(len(receipt['object_names']), 7)
            self.assertFalse(receipt['runtime_qualified'])
            packet = json.loads((stage / 'objects.json').read_bytes())
            self.assertEqual(len(packet['items']), 7)
            self.assertEqual((stage / 'objects.json').stat().st_mode & 0o777, 0o600)
            self.assertFalse((stage / 'principal-ca.key').exists())


if __name__ == '__main__': unittest.main()
