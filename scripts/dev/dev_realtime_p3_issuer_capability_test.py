import copy
import hashlib
import json
import os
from pathlib import Path
import shutil
import socket
import tempfile
import threading
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import dev_peer_issuer_preflight as peer
import dev_realtime_p3_issuer_capability as subject
import dev_realtime_p3_issuer_service as root_service
import dev_peer_issuer_service as peer_service
import media_backup_phase_test as fixture
import media_operation_journal as journal
import realtime_jwks_publisher_phase_test as shared
import resource_fencing
import reset_credentials as reset


class Package:
    def verify_current(self, token):
        if token != 'token': raise ValueError()


@unittest.skipUnless(os.name == 'posix' and os.geteuid() == 0, 'isolated root Linux fixture')
class CapabilityTest(unittest.TestCase):
    def setUp(self):
        self.base = fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.base.setUp()
        self.addCleanup(self.base.doCleanups)
        client = fixture.Client(self.base.lock)
        context = copy.deepcopy(self.base.context)
        intent = {'resource_context': context, 'namespace_uid': 'ns',
                  'dev_peer_issuer_preflight': {'source_root': '/source', 'source_sha': context['commit'], 'profile': 'creator'}}
        self.journal = journal.Journal(Path(self.base.temp.name) / 'creator-journal', intent)
        self.addCleanup(self.journal.close)
        client.journal = self.journal
        def fence(c):
            return resource_fencing.Fence(context, lambda: copy.deepcopy(self.base.card), shared.View(self.base.ledger, c))
        own_fence = fence(client)
        cut_client = fixture.Client(self.base.lock)
        cut_client.journal = self.base.journal
        self.cut = SimpleNamespace(fence=fence(cut_client), client=cut_client, lock=self.base.lock,
                                   namespace_uid='ns', _guard=lambda: None, journal=self.base.journal)
        self.package = Package()
        self.package.journal = journal.Journal(Path(self.base.temp.name) / 'package-journal', intent)
        self.addCleanup(self.package.journal.close)
        self.package.client = fixture.Client(self.base.lock)
        self.package.client.journal = self.package.journal
        self.package.fence = fence(self.package.client)
        self.package.lock, self.package.namespace_uid = self.base.lock, 'ns'
        self.media = {'uid': 'media', 'resourceVersion': '1', 'der_sha256': 'a' * 64}
        self.principal = {'uid': 'principal', 'resourceVersion': '2', 'der_sha256': 'b' * 64}
        self.nine = {name: {'state': 'absent'} for name in peer.metadata.MEMBERS}
        self.seven = {kind + '/' + name: {'state': 'absent'} for kind, name in subject.metadata.MEMBERS}
        self.directory = Path(self.base.temp.name) / 'sockets'
        self.directory.mkdir(mode=0o700)
        suffix = context['operation_id'][-32:]
        (self.directory / suffix).mkdir(mode=0o700)
        for name in ('creator', 'realtime'):
            (self.directory / suffix / name).mkdir(mode=0o700)
        patches = [patch.object(peer.frontend_runtime_executor, 'KubectlTransport', fixture.Client),
                   patch.object(peer.full_cohort_package_phase, 'PackagePhase', Package),
                   patch.object(peer.writer_cut, 'WriterCut', SimpleNamespace),
                   patch.object(peer.media_operation_guard, 'verify'), patch.object(peer.source, 'verify'),
                   patch.object(peer, 'SOCKET_ROOT', self.directory), patch.object(subject, 'SOCKET_ROOT', self.directory),
                   patch.object(peer.PeerIssuerPreflightPhase, '_ca', side_effect=lambda: copy.deepcopy(self.media)),
                   patch.object(peer.PeerIssuerPreflightPhase, '_creator', side_effect=lambda: copy.deepcopy(self.nine)),
                   patch.object(subject.RealtimeP3Capability, '_principal', side_effect=lambda: copy.deepcopy(self.principal)),
                   patch.object(subject.RealtimeP3Capability, '_seven', side_effect=lambda: copy.deepcopy(self.seven))]
        for item in patches:
            item.start()
            self.addCleanup(item.stop)
        self.creator = peer.PeerIssuerPreflightPhase(self.journal, own_fence, client, self.base.lock, 'ns',
                                                    self.package, self.cut, '/source', 'token', 'creator')
        self.addCleanup(self.creator.close)
        record = self.creator.prepare()
        def creator_reply():
            with socket.socket(socket.AF_UNIX, socket.SOCK_SEQPACKET) as channel:
                channel.connect(str(self.directory / suffix / 'creator' / 'custody.sock'))
                request = json.loads(channel.recv(65536))
                response = {'schema': 'voice-dev-peer-custody-v1', 'operation_id': record['operation_id'],
                            'source_sha': record['source_sha'], 'namespace_uid': 'ns', 'nonce': record['nonce'],
                            'context_sha256': subject.writer_cut.digest(context),
                            'lock': {key: self.base.lock['metadata'][key] for key in ('uid', 'resourceVersion')},
                            'ca': request['ca'], 'creator_secrets': self.nine,
                            'scope': 'root-peer-custody-available-not-issued', 'profile': 'creator'}
                channel.send(json.dumps(response).encode())
        thread = threading.Thread(target=creator_reply)
        thread.start()
        try: self.creator.finish()
        finally: thread.join(10)
        self.assertFalse(thread.is_alive())
        self.cap = subject.RealtimeP3Capability(self.creator, 'token')
        self.addCleanup(self.cap.close)

    def _complete(self):
        record = self.cap.prepare()
        def reply():
            with socket.socket(socket.AF_UNIX, socket.SOCK_SEQPACKET) as channel:
                channel.connect(str(self.directory / self.creator.suffix / 'realtime' / 'custody.sock'))
                request = json.loads(channel.recv(65536))
                response = {'schema': 'voice-dev-realtime-p3-capability-v1', 'operation_id': record['operation_id'],
                            'source_sha': record['source_sha'], 'namespace_uid': 'ns', 'nonce': record['nonce'],
                            'context_sha256': subject.writer_cut.digest(request['context']), 'principal': request['principal'],
                            'media': request['media'], 'objects': request['objects'],
                            'scope': 'root-principal-custody-available-not-issued'}
                channel.send(json.dumps(response).encode())
        thread = threading.Thread(target=reply)
        thread.start()
        try: receipt = self.cap.finish()
        finally: thread.join(10)
        self.assertFalse(thread.is_alive())
        return receipt

    def test_protected_capability_survives_legitimate_nine_successor_but_not_ca_or_seven_drift(self):
        receipt = self._complete()
        self.assertEqual(subject.verify_current(self.cap, 'token'), receipt)
        self.nine['voice-space-media-server-tls'] = {'state': 'present', 'uid': 'space', 'resourceVersion': '3', 'keys': ['tls.crt', 'tls.key']}
        self.assertEqual(subject.verify_current(self.cap, 'token'), receipt)
        self.principal['resourceVersion'] = 'changed'
        with self.assertRaises(subject.CapabilityError): subject.verify_current(self.cap, 'token')
        self.principal['resourceVersion'] = '2'
        next(iter(self.seven.values()))['state'] = 'changed'
        with self.assertRaises(subject.CapabilityError): subject.verify_current(self.cap, 'token')

    def test_closed_journal_cannot_authorize_and_completed_slot_cannot_replay(self):
        self._complete()
        second = subject.RealtimeP3Capability(self.creator, 'token')
        with self.assertRaises(subject.CapabilityError): second.prepare()
        self.journal.close()
        with self.assertRaises(subject.CapabilityError): subject.verify_current(self.cap, 'token')

    def test_actual_principal_archive_keymatch(self):
        with tempfile.TemporaryDirectory() as temp:
            root = Path(temp)
            archive = root / 'archive'
            archive.mkdir(mode=0o700)
            material = reset.ResetMaterial(root)
            key, cert = material.ca('principal')
            shutil.copyfile(key, archive / 'principal-ca.key')
            shutil.copyfile(cert, archive / 'principal-ca.crt')
            (archive / 'principal-ca.key').chmod(0o600)
            digest = hashlib.sha256(reset._openssl('x509', '-in', cert, '-outform', 'DER')).hexdigest()
            with patch.object(subject.metadata.custody, 'ARCHIVE', archive), \
                 patch.object(subject.metadata.custody, '_check_ancestors'):
                subject.metadata.check_principal({'der_sha256': digest})
                with self.assertRaises(ValueError):
                    subject.metadata.check_principal({'der_sha256': 'f' * 64})

    def test_actual_capability_service_handshake_without_space_leaf(self):
        record = self.cap.prepare()
        observed = []
        def observe(namespace_uid, context, lock, media, principal, space=None):
            observed.append(space)
            return copy.deepcopy(self.seven), {'principal': b'fixture', 'media': b'fixture'}
        endpoint = self.directory / self.creator.suffix / 'realtime' / 'custody.sock'
        result = []
        def root():
            try:
                root_service.capability_check(peer_service, record['operation_id'], record['source_sha'], os.getpid(), 7)
                result.append('ok')
            except Exception as error:
                result.append(type(error).__name__)
        with patch.object(peer_service, '_path', return_value=endpoint), \
             patch.object(peer_service.root_export, '_pmd_uid', return_value=0), \
             patch.object(peer_service.root_export, '_start_ticks', return_value=7), \
             patch.object(root_service.metadata, 'observe', side_effect=observe):
            thread = threading.Thread(target=root)
            thread.start()
            try: receipt = self.cap.finish()
            finally: thread.join(10)
        self.assertFalse(thread.is_alive())
        self.assertEqual(result, ['ok'])
        self.assertEqual(observed, [None])
        self.assertEqual(receipt, subject.verify_current(self.cap, 'token'))


if __name__ == '__main__':
    unittest.main()
