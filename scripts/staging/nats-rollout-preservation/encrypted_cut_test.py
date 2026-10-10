import hashlib
import io
import json
from pathlib import Path
import shutil
import subprocess
import tarfile
import tempfile
import unittest
import encrypted_cut


class EncryptedCutTest(unittest.TestCase):
    def test_operation_restore_decrypts_bound_cipher_and_checks_every_payload_member(self):
        binding=encrypted_cut.encrypt_cut(self.base,self.keys)
        seen=[]
        proof=encrypted_cut.decrypt_verify(self.base,self.keys,binding,
            lambda archive:seen.append(archive.read_bytes()))
        self.assertEqual(seen,[(self.base/'rollout-before.tar').read_bytes()])
        self.assertEqual(proof['cipher_sha256'],binding['cipher_sha256'])
        self.assertTrue(proof['decrypted_members_verified'])
        self.assertFalse(any(p.name.startswith('decrypt-') for p in self.base.iterdir()))

    def test_operation_restore_rejects_changed_enclosing_cut_after_encrypt(self):
        binding=encrypted_cut.encrypt_cut(self.base,self.keys)
        (self.base/'copy-checkpoint.json').write_bytes(b'changed operation')
        with self.assertRaises(encrypted_cut.CryptoError):
            encrypted_cut.decrypt_verify(self.base,self.keys,binding,lambda archive:None)

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

    def test_postseal_cipher_contains_bound_full_closed_copy_and_observation(self):
        from preserved_upload import digest
        closed=b'full synthetic closed native archive'
        record={'stream':'chat_events','record':{'subject':'chat.x','seq':1,'time':'2026-10-10T00:00:00Z','data':'YQ=='}}
        raw=json.dumps(record,sort_keys=True,separators=(',',':')).encode()
        name=hashlib.sha256(b'chat_events\x001').hexdigest()+'.json'
        folder=self.base/'current-record-files';folder.mkdir(mode=0o700)
        (folder/name).write_bytes(raw);(folder/name).chmod(0o600)
        observed={'schema':'synthetic-private-full-observation','private_record_ledger':{
            'storage':'ROOT_IMMUTABLE_RECORD_FILES','streams':{'chat_events':{'records':{'1':{
                'file':name,'sha256':hashlib.sha256(raw).hexdigest(),'bytes':len(raw),
                'subject':'chat.x','seq':1,'time':'2026-10-10T00:00:00Z'}}}}}}
        (self.base/'current-copy-closed.tar').write_bytes(closed)
        (self.base/'current-copy-observed.json').write_text(json.dumps(observed))
        manifest={'schema':'voice-expired-finish-current-observation-v1',
            'postseal':{'schema':'voice-root-postseal-current-proof-v1',
                'copy_observation_sha256':digest(observed)},
            'native_disposition':{'closed_archive_sha256':hashlib.sha256(closed).hexdigest()}}
        (self.base/'rollout-before-manifest.json').write_text(json.dumps(manifest))
        for member_name in ('current-copy-closed.tar','current-copy-observed.json'):
            (self.base/member_name).chmod(0o600)
        binding=encrypted_cut.encrypt_cut(self.base,self.keys,postseal=manifest)
        payload=self.base/'postseal-recovered.tar'
        encrypted_cut.openssl(['cms','-decrypt','-binary','-inform','DER','-in',str(self.base/'rollout-backup.cms'),
            '-recip',str(self.keys/'recovery-cert.pem'),'-inkey',str(self.keys/'recovery-key.pem'),'-out',str(payload)])
        with tarfile.open(payload) as archive:
            self.assertEqual(set(archive.getnames()),{'rollout-before.tar','rollout-before-manifest.json',
                'copy-checkpoint.json','current-copy-closed.tar','current-copy-observed.json','current-record-files/'+name})
            for member_name in binding['postseal_members']:
                self.assertEqual(hashlib.sha256(archive.extractfile(member_name).read()).hexdigest(),binding['postseal_members'][member_name])
        (self.base/'rollout-backup.cms').unlink()
        record_path=folder/name
        for change in ('bytes','missing','group-readable','hard-link'):
            if change=='bytes':record_path.write_bytes(raw+b'drift')
            elif change=='missing':record_path.unlink()
            elif change=='group-readable':record_path.chmod(0o640)
            else:
                import os
                os.link(record_path,folder/'extra-link')
            with self.assertRaises((encrypted_cut.CryptoError,OSError)):
                encrypted_cut.encrypt_cut(self.base,self.keys,postseal=manifest)
            self.assertFalse((self.base/'rollout-backup.cms').exists())
            if (folder/'extra-link').exists():(folder/'extra-link').unlink()
            record_path.write_bytes(raw);record_path.chmod(0o600)
        (self.base/'current-copy-closed.tar').write_bytes(closed+b'drift')
        with self.assertRaises(encrypted_cut.CryptoError):
            encrypted_cut.encrypt_cut(self.base,self.keys,postseal=manifest)
        self.assertFalse((self.base/'rollout-backup.cms').exists())


if __name__ == '__main__':
    unittest.main()
