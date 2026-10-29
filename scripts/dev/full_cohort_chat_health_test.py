import os
import base64
from pathlib import Path
import socket
import ssl
import subprocess
import sys
import tempfile
import threading
import time
import signal
import unittest
from unittest.mock import patch
from types import SimpleNamespace

import full_cohort_chat_health as health


class ResponseTests(unittest.TestCase):
    def test_private_key_selection_rejects_uid_drift_and_lost_guard(self):
        import full_cohort_schema_metadata as common
        import realtime_p3_create_only_transport as transport
        values = (b'client-certificate', b'private-secret', b'public-ca')
        raw = b'client-uid\n17\n' + base64.b64encode(values[0]) + b'\n' + base64.b64encode(values[1])
        ca = b'ca-uid\n23\n' + base64.b64encode(values[2])
        with patch.object(transport, '_run', return_value=raw) as read, patch.object(transport, 'public_certificate', return_value=ca):
            self.assertEqual(health._private_material({'uid':'client-uid','resourceVersion':'17'},
                {'uid':'ca-uid','resourceVersion':'23'}, lambda:None), values)
            arguments = read.call_args.args[0]
            self.assertEqual(arguments[:3], ['get', 'secret', 'voice-realtime-p3-tls'])
            self.assertNotIn('private-secret', repr(arguments))
            with self.assertRaises(health.HealthError) as error:
                health._private_material({'uid':'other','resourceVersion':'17'}, {'uid':'ca-uid','resourceVersion':'23'}, lambda:None)
            self.assertNotIn('private-secret', str(error.exception))
            with self.assertRaises(common.GuardLost):
                health._private_material({'uid':'client-uid','resourceVersion':'17'},
                    {'uid':'ca-uid','resourceVersion':'23'}, lambda:False)

    def test_dependency_error_is_never_downgraded_to_health_failure(self):
        import full_cohort_schema_metadata as common
        def unavailable(): raise RuntimeError('producer masked a lost guard')
        with self.assertRaises(common.GuardLost): health._authority_call(unavailable)

    def test_overall_deadline_interrupts_and_restores_timer(self):
        original = signal.getsignal(signal.SIGALRM)
        with self.assertRaises(health._Deadline):
            with health._bounded_observation(0.05): time.sleep(1)
        self.assertEqual(signal.getitimer(signal.ITIMER_REAL), (0.0, 0.0))
        self.assertEqual(signal.getsignal(signal.SIGALRM), original)

    def test_completed_guard_loss_is_not_downgraded(self):
        import full_cohort_schema_metadata as common
        phase = object.__new__(health.ChatHealthPhase)
        with patch.object(health.ChatHealthPhase, '_guard', side_effect=common.GuardLost('lost')):
            with self.assertRaises(common.GuardLost): health.verify_completed(phase, None, None)

    def test_real_guard_exception_is_classified_as_lost_authority(self):
        import full_cohort_schema_metadata as common
        import media_operation_guard
        import writer_cut
        phase = object.__new__(health.ChatHealthPhase)
        phase.poisoned = False; phase.intent = {'intent':'fixed'}
        phase.journal = SimpleNamespace(poisoned=False, intent=phase.intent, state={'intent':phase.intent})
        phase.lock = {'lock':'fixed'}; phase.namespace_uid = 'namespace'
        phase.fence = SimpleNamespace(context={'operation_id':'op'})
        phase.client = object()
        cut = object.__new__(writer_cut.WriterCut)
        for item in (cut,):
            item.client = object(); item.fence = SimpleNamespace(context=phase.fence.context,
                ledger=SimpleNamespace(client=item.client)); item.lock = phase.lock; item.namespace_uid = phase.namespace_uid
        phase.image_phase = phase.package_phase = cut
        with patch.object(media_operation_guard, 'verify', side_effect=ValueError('stale claim')):
            with self.assertRaises(common.GuardLost): health.verify_completed(phase, cut, None)

    def test_real_journal_and_ledger_terminal_and_lost_reply(self):
        import copy
        import json
        import media_operation_journal
        import resource_fencing
        context = dict(project='voice', resource='DEV', actor='scheduled_devops',
            claim='11111111-1111-4111-8111-111111111111', window='22222222-2222-4222-8222-222222222222',
            generation=1, revision=1, operation_id='media-c1-'+'a'*32, commit='b'*40,
            images={'voice-chat/chat':'registry/chat@sha256:'+'c'*64})
        receipt = {'operation_id':context['operation_id'], 'scope':'test-observation'}
        for lost in (False, True):
            with self.subTest(lost=lost), tempfile.TemporaryDirectory() as root:
                with media_operation_journal.Journal(root+'/journal', {'context':context}) as journal:
                    document = {'metadata':{'name':resource_fencing.LEDGER, 'namespace':'voice-dev', 'uid':'ledger', 'resourceVersion':'1'},
                                'data':{'executor':json.dumps(context | {'status':'active'})}}
                    class Client:
                        def call(self, *args, **kwargs):
                            if args[0] == 'patch':
                                patch_ = kwargs['body']
                                if patch_[1]['value'] != document['metadata']['resourceVersion']: raise ValueError()
                                document['data'] = copy.deepcopy(patch_[2]['value'])
                                document['metadata']['resourceVersion'] = str(int(document['metadata']['resourceVersion'])+1)
                                if lost: raise OSError('committed reply lost')
                            return copy.deepcopy(document)
                    ledger = resource_fencing.KubeLedger(Client())
                    fence = resource_fencing.Fence(context, lambda:{}, ledger)
                    phase = object.__new__(health.ChatHealthPhase)
                    phase.journal, phase.fence, phase.poisoned = journal, fence, False
                    cut = SimpleNamespace(invocation='d'*32)
                    with patch.object(health.ChatHealthPhase, '_guard'), \
                         patch.object(health.ChatHealthPhase, '_observe', return_value=receipt), \
                         patch.object(fence, 'uncertain') as uncertain:
                        if lost:
                            with self.assertRaises(health.HealthError): phase.run('token', cut)
                            self.assertTrue(phase.poisoned); uncertain.assert_called_once()
                            self.assertEqual(json.loads(document['data']['executor'])[health.LEDGER_KEY]['status'], 'pending')
                        else:
                            self.assertEqual(phase.run('token', cut), receipt)
                            self.assertEqual(json.loads(document['data']['executor'])[health.LEDGER_KEY]['status'], 'done')
                            with self.assertRaises(health.HealthError): phase.run('token', cut)

    def test_only_documented_chat_ready_response(self):
        self.assertIsNone(health.check_health(b'{"service":"chat","status":"ok"}\n'))
        for value in (b'', b'{"service":"chat","status":"degraded"}', b'{"service":"other","status":"ok"}',
                      b'{"service":"chat","status":"ok","extra":1}', b'{"service":"chat","status":"ok","status":"ok"}'):
            with self.subTest(value=value), self.assertRaises(health.HealthError):
                health.check_health(value)

    def test_tls_http2_settings_frame_only(self):
        health.check_h2_settings(b'\x00\x00\x00\x04\x00\x00\x00\x00\x00')
        for frame in (b'', b'\x00\x00\x00\x00\x00\x00\x00\x00\x00',
                      b'\x00\x00\x00\x04\x01\x00\x00\x00\x00', b'\x00\x00\x06\x04\x00\x00\x00\x00\x00'):
            with self.subTest(frame=frame), self.assertRaises(health.HealthError): health.check_h2_settings(frame)

    def test_real_mutual_tls_and_h2_with_owned_forward_cleanup(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            def openssl(*args):
                subprocess.run(['openssl', *args], cwd=root, check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            openssl('req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-keyout', 'ca.key', '-out', 'ca.crt',
                    '-subj', '/CN=test-ca', '-days', '1')
            for name, extension in [('server', 'subjectAltName=DNS:voice-chat.voice-dev.svc\nextendedKeyUsage=serverAuth\n'),
                                    ('client', 'extendedKeyUsage=clientAuth\n')]:
                openssl('req', '-newkey', 'rsa:2048', '-nodes', '-keyout', name+'.key', '-out', name+'.csr', '-subj', '/CN='+name)
                (root/(name+'.ext')).write_text(extension)
                openssl('x509', '-req', '-in', name+'.csr', '-CA', 'ca.crt', '-CAkey', 'ca.key', '-CAcreateserial',
                        '-out', name+'.crt', '-days', '1', '-extfile', name+'.ext')
            server = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER); server.verify_mode = ssl.CERT_REQUIRED
            server.load_cert_chain(root/'server.crt', root/'server.key'); server.load_verify_locations(root/'ca.crt')
            server.set_alpn_protocols(['h2'])
            listener = socket.socket(); listener.bind(('127.0.0.1', 0)); listener.listen(1)
            port = listener.getsockname()[1]; failures = []; finished = threading.Event()
            def accept():
                try:
                    raw, _ = listener.accept()
                    try:
                        with server.wrap_socket(raw, server_side=True):
                            failures.append('client_certificate_not_required')
                    except ssl.SSLError:
                        pass
                    raw, _ = listener.accept()
                    with server.wrap_socket(raw, server_side=True) as tls:
                        received = b''
                        while len(received) < 33: received += tls.recv(33-len(received))
                        if not received.startswith(b'PRI * HTTP/2.0'): raise ValueError()
                        tls.sendall(b'\x00\x00\x00\x04\x00\x00\x00\x00\x00')
                        finished.wait(3)
                except Exception as exc: failures.append(type(exc).__name__)
                finally: listener.close()
            worker = threading.Thread(target=accept, daemon=True); worker.start()
            real_popen = subprocess.Popen; children = []
            def forward(*args, **kwargs):
                code = 'import time;print("Forwarding from 127.0.0.1:'+str(port)+' -> 9099",flush=True);time.sleep(30)'
                child = real_popen([sys.executable, '-u', '-c', code], stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
                children.append(child); return child
            with patch.object(health.subprocess, 'Popen', side_effect=forward):
                health._protected_listener('voice-chat-test-123', ((root/'client.crt').read_bytes(),
                    (root/'client.key').read_bytes(), (root/'ca.crt').read_bytes()))
            finished.set()
            worker.join(timeout=3)
            self.assertFalse(worker.is_alive()); self.assertEqual(failures, [])
            self.assertIsNotNone(children[0].poll())
            server.verify_mode = ssl.CERT_NONE
            listener = socket.socket(); listener.bind(('127.0.0.1', 0)); listener.listen(1)
            port = listener.getsockname()[1]
            finished.clear()
            def no_client_auth():
                try:
                    raw, _ = listener.accept()
                    with server.wrap_socket(raw, server_side=True) as tls:
                        tls.sendall(b'\x00\x00\x00\x04\x00\x00\x00\x00\x00'); finished.wait(3)
                finally: listener.close()
            worker = threading.Thread(target=no_client_auth, daemon=True); worker.start()
            with patch.object(health.subprocess, 'Popen', side_effect=forward):
                with self.assertRaises(health.HealthError):
                    health._protected_listener('voice-chat-test-123', ((root/'client.crt').read_bytes(),
                        (root/'client.key').read_bytes(), (root/'ca.crt').read_bytes()))
            finished.set()
            worker.join(timeout=3)
            self.assertFalse(worker.is_alive()); self.assertIsNotNone(children[-1].poll())

    def test_pod_ownership_and_endpoint_slice_are_bound(self):
        target = {'kind':'Deployment', 'name':'voice-chat', 'uid':'deployment', 'generation':2,
                  'replicas':1, 'spec':{'replicas':1}}
        cut = SimpleNamespace(_guard=lambda:None, current_resumed_proof=lambda:{'controllers':[target]})
        owner = lambda kind,uid: [{'kind':kind, 'uid':uid, 'controller':True}]
        rs = {'metadata':{'namespace':'voice-dev', 'uid':'rs', 'resourceVersion':'3', 'ownerReferences':owner('Deployment','deployment')}}
        pod = {'metadata':{'name':'voice-chat-a-1','namespace':'voice-dev','uid':'pod','resourceVersion':'4',
                           'ownerReferences':owner('ReplicaSet','rs')},
               'status':{'podIP':'10.1.2.3','containerStatuses':[{'name':'chat','containerID':'container','restartCount':0,
                                                                 'imageID':'image','state':{'running':{}}}]}}
        service = {'metadata':{'name':'voice-chat','namespace':'voice-dev','uid':'service','resourceVersion':'5'},
                   'spec':{'clusterIP':'10.0.0.1','selector':{'app':'voice-chat'},'ports':[{'name':'realtime-lifecycle','port':9099}]}}
        slice_ = {'metadata':{'namespace':'voice-dev','uid':'slice','resourceVersion':'6',
                              'labels':{'kubernetes.io/service-name':'voice-chat'}, 'ownerReferences':owner('Service','service')},
                  'ports':[{'name':'realtime-lifecycle','port':9099}],
                  'endpoints':[{'targetRef':{'kind':'Pod','namespace':'voice-dev','uid':'pod','name':'voice-chat-a-1'},
                                'addresses':['10.1.2.3'],'conditions':{'ready':True}}]}
        deployment = {'metadata':{'uid':'deployment','generation':2}, 'spec':target['spec'],
                      'status':{'observedGeneration':2,'updatedReplicas':1,'availableReplicas':1}}
        class Client:
            def call(self, *args):
                return {'deployment':deployment,'replicasets':{'items':[rs]},'pods':{'items':[pod]},
                        'service':service,'endpointslices':{'items':[slice_]}}[args[1]]
        ledger_row = {'status':'active','full_cohort_invocation':{'nonce':'nonce','operation_id':'op'}}
        image = SimpleNamespace(verify_activation_lineage=lambda:None, _valid_pod=lambda *a:True, _accounts=lambda:None,
            _records=lambda row:None, _target=lambda name:target['spec'], started=True, invocation='nonce',
            journal=SimpleNamespace(state={'steps':{'full-cohort-image-voice-chat':
                {'status':'done','receipt':{'uid':'deployment','spec':target['spec'],'resourceVersion':'7'}}}}),
            _controller=lambda m,k: next((o['uid'] for o in m.get('ownerReferences',[]) if o['kind']==k and o['controller']),None),
            fence=SimpleNamespace(context={'images':{'voice-chat/chat':'image'},'operation_id':'op'},
                owned=lambda row:True, ledger=SimpleNamespace(read=lambda:(ledger_row,('ledger','1')))),
            intent={'full_cohort':{'platform_digests':{'voice-chat/chat':'platform'}}})
        package = {'images':{'voice-chat/chat':'image'}, 'platform_digests':{'voice-chat/chat':'platform'}}
        self.assertEqual(health._workloads(Client(), image, cut, package)['pods'][0]['uid'], 'pod')
        image.journal.state['steps']['full-cohort-image-voice-chat']['status'] = 'pending'
        with self.assertRaises(health.HealthError): health._workloads(Client(), image, cut, package)
        image.journal.state['steps']['full-cohort-image-voice-chat']['status'] = 'done'
        import full_cohort_schema_metadata as common
        cut._guard = lambda: (_ for _ in ()).throw(ValueError('lost claim'))
        with self.assertRaises(common.GuardLost): health._workloads(Client(), image, cut, package)
        cut._guard = lambda: None
        slice_['endpoints'][0]['targetRef']['uid'] = 'foreign'
        with self.assertRaises(health.HealthError): health._workloads(Client(), image, cut, package)


if __name__ == '__main__': unittest.main()
