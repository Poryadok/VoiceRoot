"""Authenticated public JWKS tunnel contract."""
import socket
import copy
import ssl
import subprocess
import sys
import threading
from types import SimpleNamespace
import unittest
from unittest.mock import patch
from pathlib import Path

import realtime_jwks_publisher_health as health
import realtime_p3_create_only_packet_test as fixture
import realtime_jwks_publisher as publisher
import realtime_jwks_publisher_phase_test as phase_fixture
import media_operation_journal
import resource_fencing
import writer_cut
import full_cohort_package_phase


class TunnelTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        fixture.PacketTest.setUpClass(); cls.addClassCleanup(fixture.PacketTest.doClassCleanups)

    def test_real_tls_exact_hostname_and_public_path(self):
        import base64
        sock = socket.socket(); sock.bind(('127.0.0.1', 0)); sock.listen(1); sock.settimeout(8)
        port = sock.getsockname()[1]; requests = []; errors = []
        ctx = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
        ctx.load_cert_chain(fixture.PacketTest.temp.name + '/jwks.crt', fixture.PacketTest.temp.name + '/jwks.key')
        def server():
            try:
                raw, _ = sock.accept()
                with ctx.wrap_socket(raw, server_side=True) as tls:
                    requests.append(tls.recv(4096)); body = b'{"keys":[]}'
                    tls.sendall(b'HTTP/1.1 200 OK\r\nContent-Length: '+str(len(body)).encode()+b'\r\nConnection: close\r\n\r\n'+body)
            except Exception as error: errors.append(error)
            finally: sock.close()
        thread = threading.Thread(target=server); thread.start()
        original = subprocess.Popen; processes = []
        def start(*args, **kwargs):
            if args[0][0] != 'kubectl': return original(*args, **kwargs)
            proc = original([sys.executable,'-c',f'import time;print("Forwarding from 127.0.0.1:{port} -> 8443",flush=True);time.sleep(30)'], **kwargs)
            processes.append(proc); return proc
        cert = base64.b64decode(next(x for x in fixture.PacketTest.envelope['objects']['items'] if x['metadata']['name']=='voice-p3-principal-jwks-tls')['data']['tls.crt'])
        try:
            with patch.object(health.subprocess, 'Popen', side_effect=start):
                self.assertEqual(health._tunnel('jwks-pod', fixture.PacketTest.principal, cert, lambda:None), b'{"keys":[]}')
        finally: thread.join(timeout=5)
        self.assertFalse(thread.is_alive()); self.assertEqual(errors, [])
        self.assertIn(b'GET /realtime.json HTTP/1.1', requests[0])
        self.assertIn(b'Host: voice-p3-principal-jwks.voice-dev.svc', requests[0])
        self.assertIsNotNone(processes[0].poll())

    def test_invalid_pod_never_launches(self):
        with patch.object(health.subprocess, 'Popen') as process:
            with self.assertRaises(ValueError): health._tunnel('../other', b'', b'', lambda:None)
        process.assert_not_called()


class RoutingTest(unittest.TestCase):
    def setUp(self):
        svc = publisher.desired('node1')[2]
        svc['metadata'].update(uid='service-uid', resourceVersion='4')
        svc['spec']['clusterIP'] = '10.0.0.1'
        meta = lambda name, uid: {'name': name, 'namespace': 'voice-dev', 'uid': uid, 'resourceVersion': '7'}
        self.pod = {'metadata': meta('jwks-pod', 'pod-uid'), 'status': {'podIP': '10.0.0.2'}}
        target = {'kind': 'Pod', 'name': 'jwks-pod', 'uid': 'pod-uid'}
        port = {'name': 'https', 'port': 8443, 'protocol': 'TCP'}
        self.endpoints = {'metadata': meta(publisher.NAME, 'ep-uid'), 'subsets': [
            {'addresses': [{'ip': '10.0.0.2', 'targetRef': target}], 'ports': [port]}]}
        self.slice = {'metadata': meta('jwks-slice', 'slice-uid') | {'labels': {'kubernetes.io/service-name': publisher.NAME},
                      'ownerReferences': [{'kind': 'Service', 'uid': 'service-uid', 'controller': True}]}, 'addressType': 'IPv4',
                      'endpoints': [{'addresses': ['10.0.0.2'], 'conditions': {'ready': True}, 'targetRef': target}],
                      'ports': [port]}
        self.service = svc
        self.proof = {'runtime': {'pod_name': 'jwks-pod', 'pod_uid': 'pod-uid', 'pod_resourceVersion': '7'},
                      'node': {'hostname': 'node1'}, 'objects': {'service': {'uid': 'service-uid', 'resourceVersion': '4',
                                 'spec_sha256': publisher.digest(svc['spec'])}}}
        self.client = self

    def call(self, *args):
        if args[1] == 'pod': return copy.deepcopy(self.pod)
        if args[1] == 'service': return copy.deepcopy(self.service)
        if args[1] == 'endpoints': return copy.deepcopy(self.endpoints)
        if args[1] == 'endpointslices': return {'items': [copy.deepcopy(self.slice)]}
        raise AssertionError(args)

    def test_exact_ready_pod_and_service(self):
        self.assertEqual(health._routing(self.client, self.proof)['pod']['uid'], 'pod-uid')

    def test_wrong_endpoint_and_drift_are_rejected(self):
        self.slice['endpoints'][0]['targetRef']['uid'] = 'foreign'
        with self.assertRaises(ValueError): health._routing(self.client, self.proof)
        self.slice['endpoints'][0]['targetRef']['uid'] = 'pod-uid'
        self.endpoints['subsets'][0]['notReadyAddresses'] = [{'ip': '10.0.0.3'}]
        with self.assertRaises(ValueError): health._routing(self.client, self.proof)


class HealthPhaseTest(unittest.TestCase):
    def setUp(self):
        self.base = phase_fixture.PhaseTest('test_create_only_real_journal_and_recheck'); self.base.setUp()
        self.addCleanup(self.base.doCleanups)
        self.base.phase.run(b'', b'', b'')
        self.client = phase_fixture.Client(self.base.base.lock)
        intent = {'resource_context': self.base.base.context, 'namespace_uid': 'ns'}
        self.journal = media_operation_journal.Journal(Path(self.base.base.temp.name) / 'health', intent)
        self.addCleanup(self.journal.close); self.client.journal = self.journal
        self.fence = resource_fencing.Fence(self.base.base.context, lambda: copy.deepcopy(self.base.base.card),
                                             phase_fixture.View(self.base.base.ledger, self.client))
        self.cut = object.__new__(writer_cut.WriterCut)
        self.cut.fence = self.fence; self.cut.lock = self.base.base.lock; self.cut.namespace_uid = 'ns'
        self.cut.invocation = 'a' * 32; self.cut.activation = {'stage': 'publisher_health'}; self.cut._guard = lambda: None
        self.cut.verify_activation_stage = lambda: {'operation_id':self.fence.context['operation_id'],
            'cut_invocation':self.cut.invocation, 'plan_sha256':'b'*64, 'stage':self.cut.activation['stage']}
        self.package = object.__new__(full_cohort_package_phase.PackagePhase)
        self.package.fence = self.fence; self.package.lock = self.base.base.lock; self.package.namespace_uid = 'ns'
        self.package.verify_current = lambda token: None
        self.phase = health.PublisherHealthPhase(self.journal, self.fence, self.client, self.base.base.lock,
                                                 'ns', self.base.phase, self.package)
        self.receipt = {'scope': 'fixture', 'cut_invocation': self.cut.invocation}

    def test_real_journal_and_fence_store_once_and_verify(self):
        with patch.object(self.phase, '_observe', return_value=self.receipt):
            self.assertEqual(self.phase.run('token', self.cut), self.receipt)
            self.assertEqual(health.verify_completed(self.phase, self.cut, 'token'), self.receipt)
            with self.assertRaises(health.PublisherHealthError): self.phase.run('token', self.cut)
        self.assertEqual(self.base.base.ledger.row[health.LEDGER]['status'], 'done')
        self.assertNotIn('receipt', self.base.base.ledger.row[health.LEDGER])

    def test_observation_failure_is_uncertain_and_never_replayed(self):
        with patch.object(self.phase, '_observe', side_effect=OSError('lost')):
            with self.assertRaises(health.PublisherHealthError): self.phase.run('token', self.cut)
        self.assertEqual(self.base.base.ledger.row['status'], 'uncertain')
        self.assertEqual(self.base.base.ledger.row[health.LEDGER]['status'], 'pending')
        with self.assertRaises(health.PublisherHealthError): self.phase.run('token', self.cut)

    def test_wrong_served_jwks_never_completes(self):
        with patch.object(health, '_routing', return_value={'fixture': True}), \
             patch.object(health, '_materials', return_value=(b'expected', b'ca', b'cert')), \
             patch.object(health, '_tunnel', return_value=b'wrong'):
            with self.assertRaises(health.PublisherHealthError): self.phase.run('token', self.cut)
        self.assertEqual(self.base.base.ledger.row['status'], 'uncertain')
        self.assertEqual(self.base.base.ledger.row[health.LEDGER]['status'], 'pending')

    def test_stage_loss_during_observation_never_completes(self):
        def changed(cut, token, stage):
            cut.activation['stage'] = 'chat'
            return self.receipt
        with patch.object(self.phase, '_observe', side_effect=changed):
            with self.assertRaises(health.PublisherHealthError): self.phase.run('token', self.cut)
        self.assertEqual(self.base.base.ledger.row['status'], 'uncertain')
        self.assertEqual(self.base.base.ledger.row[health.LEDGER]['status'], 'pending')

    def test_failed_done_recheck_marks_uncertain(self):
        with patch.object(self.phase, '_observe', return_value=self.receipt): self.phase.run('token', self.cut)
        with patch.object(self.phase, '_observe', side_effect=OSError('drift')):
            with self.assertRaises(health.PublisherHealthError): health.verify_completed(self.phase, self.cut, 'token')
        self.assertEqual(self.base.base.ledger.row['status'], 'uncertain')

    def test_later_stage_requires_exact_durable_cut_barrier(self):
        with patch.object(self.phase, '_observe', return_value=self.receipt): self.phase.run('token', self.cut)
        self.cut.activation['stage'] = 'chat'
        with self.assertRaises(health.PublisherHealthError): health.verify_completed(self.phase, self.cut, 'token')
        self.assertEqual(self.base.base.ledger.row['status'], 'uncertain')

    def test_later_stage_accepts_only_exact_barrier_receipt(self):
        with patch.object(self.phase, '_observe', return_value=self.receipt): self.phase.run('token', self.cut)
        details = {'operation_id':self.fence.context['operation_id'], 'cut_invocation':self.cut.invocation,
                   'activation_plan_sha256':'b'*64, 'health_receipt_sha256':self.phase.record['receipt_sha256']}
        step = {'status':'done', 'nonce':'d'*32, 'details':details, 'details_sha256':publisher.digest(details), 'receipt':details}
        self.cut.journal = SimpleNamespace(state={'steps':{'writer-cut-publisher-health':step}})
        self.base.base.ledger.row['writer_cut_effects'] = {'writer-cut-publisher-health':{'status':'done',
            'nonce':step['nonce'], 'details_sha256':step['details_sha256'], 'receipt_sha256':publisher.digest(details)}}
        self.cut.activation['stage'] = 'chat'
        with patch.object(self.phase, '_observe', return_value=self.receipt):
            self.assertEqual(health.verify_completed(self.phase, self.cut, 'token'), self.receipt)
            step['details'] = dict(details, cut_invocation='different')
            with self.assertRaises(health.PublisherHealthError): health.verify_completed(self.phase, self.cut, 'token')
        self.assertEqual(self.base.base.ledger.row['status'], 'uncertain')


if __name__ == '__main__': unittest.main()
