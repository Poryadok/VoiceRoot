"""Offline issuer profile, custody and fail-closed recovery tests."""
import importlib.util
import hashlib
import base64
import fcntl
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
from unittest import mock


path = Path(__file__).parent / 'story-media-pki-issuer' / 'issuer.py'
spec = importlib.util.spec_from_file_location('story_media_issuer', path)
issuer = importlib.util.module_from_spec(spec)
spec.loader.exec_module(issuer)


class StoryMediaIssuerTest(unittest.TestCase):
    def test_stage_is_bound_and_preserves_exact_private_material(self):
        op = 'smp-' + '1' * 32
        source = hashlib.sha256(path.read_bytes()).hexdigest()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / 'state'
            root.mkdir(mode=0o700)
            receipt = issuer.stage(root, op, source)
            self.assertEqual(receipt['operation'], op)
            self.assertEqual(receipt['issuer_source_sha256'], source)
            self.assertNotIn('manifest', json.dumps(receipt))
            self.assertEqual(set(receipt['objects']), {
                'voice-file-principal-grpc-tls', 'voice-story-jwks-tls',
                'voice-messaging-jwks-tls', 'voice-principal-trust',
                'voice-file-principal-client-ca', 'voice-story-file-client-tls',
                'voice-messaging-jwks-client-ca', 'voice-file-jwks-client-tls',
                'voice-story-principal-keys', 'voice-messaging-principal-signing-' + '1' * 12})
            self.assertEqual(receipt, issuer.stage(root, op, source))
            with self.assertRaises(ValueError):
                issuer.stage(root, op, 'b' * 64)
            with self.assertRaises(ValueError):
                issuer.stage(root, 'smp-' + '2' * 32, source)
            self.assertTrue((root / op / 'stage.json').is_file())
            self.assertEqual((root / op / 'stage.json').stat().st_mode & 0o777, 0o600)
            self.assertEqual(len(list((root / op).glob('*.key'))), 3)
            staged = root / op / 'stage.json'
            value = json.loads(staged.read_text())
            value['receipt']['objects'].append('foreign')
            staged.write_text(json.dumps(value))
            with self.assertRaises(ValueError):
                issuer.stage(root, op, source)
            value['receipt'] = receipt
            packet = json.loads(base64.b64decode(value['packet']))
            packet['items'][0]['data']['tls.crt'] = base64.b64encode(b'not a certificate').decode()
            raw = json.dumps(packet, sort_keys=True, separators=(',', ':')).encode()
            value['packet'] = base64.b64encode(raw).decode()
            value['packet_sha256'] = hashlib.sha256(raw).hexdigest()
            staged.write_text(json.dumps(value))
            with self.assertRaises(ValueError):
                issuer.stage(root, op, source)

    def test_partial_stage_is_never_regenerated(self):
        op = 'smp-' + '3' * 32
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / 'state'
            root.mkdir(mode=0o700)
            partial = root / (op + '.building')
            partial.mkdir(mode=0o700)
            (partial / 'intent').write_text('retained')
            with self.assertRaises(ValueError):
                issuer.stage(root, op, hashlib.sha256(path.read_bytes()).hexdigest())
            self.assertEqual((partial / 'intent').read_text(), 'retained')

    def test_parent_directory_is_synced_before_key_creation(self):
        op = 'smp-' + '4' * 32
        source = hashlib.sha256(path.read_bytes()).hexdigest()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / 'state'
            root.mkdir(mode=0o700)
            calls = []
            def sync(directory):
                calls.append(Path(directory))
            def create(*args):
                self.assertEqual(calls[-2:], [root, root / (op + '.building')])
                raise ValueError('injected stop')
            with mock.patch.object(issuer, 'sync_dir', side_effect=sync), \
                    mock.patch.object(issuer, 'create_packet', side_effect=create):
                with self.assertRaisesRegex(ValueError, 'injected stop'):
                    issuer.stage(root, op, source)

    def test_concurrent_staging_refuses_without_generating_keys(self):
        op = 'smp-' + '6' * 32
        source = hashlib.sha256(path.read_bytes()).hexdigest()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / 'state'
            root.mkdir(mode=0o700)
            lock = root / '.lock'
            with lock.open('w') as stream:
                lock.chmod(0o600)
                fcntl.flock(stream, fcntl.LOCK_EX | fcntl.LOCK_NB)
                with mock.patch.object(issuer, 'create_packet') as create:
                    with self.assertRaisesRegex(ValueError, 'issuer busy'):
                        issuer.stage(root, op, source)
                    create.assert_not_called()
            self.assertFalse((root / (op + '.building')).exists())

    def test_exact_certificate_extensions_reject_extra_names_or_purposes(self):
        dns = issuer.SERVER_NAMES[0]
        good = {
            'basicConstraints': b'X509v3 Basic Constraints: critical\n    CA:FALSE\n',
            'keyUsage': b'X509v3 Key Usage: critical\n    Digital Signature, Key Encipherment\n',
            'extendedKeyUsage': b'X509v3 Extended Key Usage: \n    TLS Web Server Authentication\n',
            'subjectAltName': ('X509v3 Subject Alternative Name: \n    DNS:' + dns + '\n').encode(),
        }
        def check(values):
            with mock.patch.object(issuer, 'openssl', side_effect=lambda *args, **kwargs: values[args[-1]]):
                issuer.validate_leaf_extensions(b'certificate', dns, 'serverAuth')
        check(good)
        for field, suffix in (('subjectAltName', b', IP Address:127.0.0.1\n'),
                              ('subjectAltName', b', URI:spiffe://foreign\n'),
                              ('extendedKeyUsage', b', TLS Web Client Authentication\n')):
            bad = dict(good)
            bad[field] = good[field].rstrip() + suffix
            with self.assertRaises(ValueError):
                check(bad)

    def test_ca_constraints_reject_excluded_or_extra_names(self):
        domain = 'story-client'
        lines = ['X509v3 Name Constraints: critical', 'Permitted:', 'DNS:' + issuer.DOMAINS[domain][0]]
        issuer.validate_ca_constraints(lines, domain)
        for extra in ('Excluded:', 'IP:127.0.0.1', 'DNS:foreign.example'):
            with self.assertRaises(ValueError):
                issuer.validate_ca_constraints(lines + [extra], domain)

    def test_2048_bit_dsa_is_not_accepted_as_rsa_signer(self):
        with tempfile.TemporaryDirectory() as tmp:
            params = Path(tmp) / 'params.pem'
            key = Path(tmp) / 'key.pem'
            issuer.openssl('genpkey', '-genparam', '-algorithm', 'DSA',
                           '-pkeyopt', 'dsa_paramgen_bits:2048', '-out', params)
            issuer.openssl('genpkey', '-paramfile', params, '-out', key)
            with self.assertRaises(ValueError):
                issuer.validate_signer(key.read_bytes(), set())

    def test_malformed_staged_object_returns_sanitized_cli_error(self):
        op = 'smp-' + '5' * 32
        source = hashlib.sha256(path.read_bytes()).hexdigest()
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / 'state'
            root.mkdir(mode=0o700)
            folder = root / op
            folder.mkdir(mode=0o700)
            payload = b'private-sentinel'
            for domain in issuer.DOMAINS:
                p = folder / (domain + '.key')
                p.write_bytes(payload)
                p.chmod(0o600)
            raw = json.dumps({'apiVersion': 'v1', 'kind': 'List',
                              'items': [{'metadata': []}] * 10}).encode()
            value = {'operation': op, 'issuer_source_sha256': source,
                     'receipt': issuer.receipt_for(op, source),
                     'packet': base64.b64encode(raw).decode(), 'packet_sha256': hashlib.sha256(raw).hexdigest(),
                     'private_key_sha256': {domain: hashlib.sha256(payload).hexdigest() for domain in issuer.DOMAINS}}
            stage = folder / 'stage.json'
            stage.write_text(json.dumps(value))
            stage.chmod(0o600)
            result = subprocess.run([sys.executable, str(path), '--state-dir', str(root), '--operation', op,
                                     '--issuer-source-sha256', source], capture_output=True, text=True, check=False)
            self.assertNotEqual(result.returncode, 0)
            self.assertNotIn('Traceback', result.stderr)
            self.assertNotIn(payload.decode(), result.stdout + result.stderr)


if __name__ == '__main__':
    unittest.main()
