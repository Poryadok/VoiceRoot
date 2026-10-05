import hashlib
import io
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import unittest
import encrypted_cut


class EncryptedCutTest(unittest.TestCase):
    def setUp(self):
        root = Path('/var/lib/voice-nats-preservation')
        root.mkdir(exist_ok=True, mode=0o700)
        self.base = Path(tempfile.mkdtemp(prefix='crypto-test-', dir=root))
        self.addCleanup(lambda: shutil.rmtree(self.base))
        self.keys = self.base / 'recovery'
        self.keys.mkdir(mode=0o700)
        encrypted_cut.initialize_recovery_key(self.keys)
        for name in ('rollout-before.tar', 'rollout-before-manifest.json', 'copy-checkpoint.json'):
            (self.base / name).write_bytes(('private synthetic ' + name).encode())
            (self.base / name).chmod(0o600)

    def test_existing_recovery_key_is_retained_across_installs(self):
        before = {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in self.keys.iterdir()}
        encrypted_cut.initialize_recovery_key(self.keys)
        self.assertEqual(before, {p.name: hashlib.sha256(p.read_bytes()).hexdigest() for p in self.keys.iterdir()})

    def test_mismatched_existing_certificate_is_rejected(self):
        other=self.base/'other';other.mkdir(mode=0o700)
        encrypted_cut.initialize_recovery_key(other)
        shutil.copyfile(other/'recovery-cert.pem',self.keys/'recovery-cert.pem')
        with self.assertRaises(encrypted_cut.CryptoError):encrypted_cut.initialize_recovery_key(self.keys)

    def test_actual_authenticated_cms_roundtrip_contains_only_cut_evidence(self):
        binding = encrypted_cut.encrypt_cut(self.base, self.keys)
        ciphertext = self.base / 'rollout-backup.cms'
        self.assertEqual(binding['cipher_sha256'], hashlib.sha256(ciphertext.read_bytes()).hexdigest())
        self.assertNotIn(b'private synthetic', ciphertext.read_bytes())
        payload = self.base / 'private-recovered.tar'
        encrypted_cut.openssl(['cms', '-decrypt', '-binary', '-inform', 'DER', '-in', str(ciphertext),
            '-recip', str(self.keys / 'recovery-cert.pem'), '-inkey', str(self.keys / 'recovery-key.pem'), '-out', str(payload)])
        with tarfile.open(payload) as archive:
            self.assertEqual(set(archive.getnames()), {'rollout-before.tar', 'rollout-before-manifest.json', 'copy-checkpoint.json'})
            for member in archive:
                self.assertEqual(archive.extractfile(member).read(), (self.base / member.name).read_bytes())
        damaged = bytearray(ciphertext.read_bytes());damaged[-10] ^= 1
        ciphertext.write_bytes(damaged)
        with self.assertRaises(encrypted_cut.CryptoError):
            encrypted_cut.openssl(['cms', '-decrypt', '-binary', '-inform', 'DER', '-in', str(ciphertext),
                '-recip', str(self.keys / 'recovery-cert.pem'), '-inkey', str(self.keys / 'recovery-key.pem'), '-out', str(self.base / 'untrusted.partial')])

    def test_key_symlink_and_group_readable_key_are_rejected(self):
        key = self.keys / 'recovery-key.pem';key.chmod(0o640)
        with self.assertRaises(encrypted_cut.CryptoError):encrypted_cut.encrypt_cut(self.base, self.keys)
        key.chmod(0o600);key.unlink();key.symlink_to('/etc/hostname')
        with self.assertRaises((encrypted_cut.CryptoError, OSError)):encrypted_cut.encrypt_cut(self.base, self.keys)

    def test_immutable_ciphertext_is_never_replaced(self):
        encrypted_cut.encrypt_cut(self.base, self.keys)
        before = (self.base / 'rollout-backup.cms').read_bytes()
        with self.assertRaises(FileExistsError):encrypted_cut.encrypt_cut(self.base, self.keys)
        self.assertEqual((self.base / 'rollout-backup.cms').read_bytes(), before)


if __name__ == '__main__':
    unittest.main()
