import base64
import subprocess
import tempfile
import unittest
from pathlib import Path

import space_role_peer_material as role


def key(bits):
    return subprocess.run(['/usr/bin/openssl', 'genpkey', '-algorithm', 'RSA', '-pkeyopt',
                           'rsa_keygen_bits:' + str(bits)], check=True, stdout=subprocess.PIPE,
                          stderr=subprocess.DEVNULL).stdout


def secret(current, next_key, active=b'current'):
    values = {'current.pem': current, 'next.pem': next_key, 'active-kid': active}
    return {'apiVersion': 'v1', 'kind': 'Secret', 'type': 'Opaque',
            'metadata': {'name': 'voice-role-principal-signing', 'namespace': 'voice-dev'},
            'data': {name: base64.b64encode(value).decode('ascii') for name, value in values.items()}}


class RoleMaterialTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.current, cls.next_key, cls.short = key(2048), key(2048), key(1024)

    def test_public_jwks_for_distinct_2048_bit_keys(self):
        result = role.qualify('voice-role-principal-signing', secret(self.current, self.next_key, b'next'), b'')
        self.assertEqual([key['kid'] for key in result['keys']], ['current', 'next'])
        self.assertNotEqual(result['keys'][0]['n'], result['keys'][1]['n'])

    def test_short_duplicate_and_appended_private_material_hold(self):
        for current, other in ((self.short, self.next_key), (self.current, self.current),
                               (self.current + b'extra', self.next_key)):
            with self.subTest(current=current[:30], duplicate=current == other):
                with self.assertRaises(ValueError):
                    role.qualify('voice-role-principal-signing', secret(current, other), b'')

    def test_partial_and_wrong_active_hold(self):
        obj = secret(self.current, self.next_key, b'unknown')
        with self.assertRaises(ValueError): role.qualify('voice-role-principal-signing', obj, b'')
        del obj['data']['next.pem']
        with self.assertRaises(ValueError): role.qualify('voice-role-principal-signing', obj, b'')

    def test_role_tls_checks_ca_identity_and_client_eku(self):
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory)
            def run(*args):
                subprocess.run(['/usr/bin/openssl', *args], cwd=path, check=True,
                               stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            run('req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-keyout', 'ca.key', '-out', 'ca.crt',
                '-subj', '/CN=test-ca', '-days', '4', '-addext', 'basicConstraints=critical,CA:TRUE')
            run('req', '-new', '-newkey', 'rsa:2048', '-nodes', '-keyout', 'role.key', '-out', 'role.csr', '-subj', '/CN=voice-role')
            (path / 'extensions').write_text('basicConstraints=critical,CA:FALSE\nextendedKeyUsage=clientAuth\nsubjectAltName=DNS:voice-role,DNS:voice-role.voice-dev.svc\n')
            run('x509', '-req', '-in', 'role.csr', '-CA', 'ca.crt', '-CAkey', 'ca.key', '-CAcreateserial',
                '-out', 'role.crt', '-days', '2', '-extfile', 'extensions')
            values = {'tls.crt': (path / 'role.crt').read_bytes(), 'tls.key': (path / 'role.key').read_bytes()}
            obj = {'apiVersion': 'v1', 'kind': 'Secret', 'type': 'Opaque',
                   'metadata': {'name': 'voice-role-media-client-tls', 'namespace': 'voice-dev'},
                   'data': {name: base64.b64encode(value).decode('ascii') for name, value in values.items()}}
            self.assertIsNone(role.qualify('voice-role-media-client-tls', obj, (path / 'ca.crt').read_bytes()))
            obj['data']['tls.key'] = base64.b64encode(self.current).decode('ascii')
            with self.assertRaises(ValueError):
                role.qualify('voice-role-media-client-tls', obj, (path / 'ca.crt').read_bytes())
if __name__ == '__main__':
    unittest.main()
