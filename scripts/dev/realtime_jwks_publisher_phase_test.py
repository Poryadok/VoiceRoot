"""Real Journal/Fence with synthetic Kubernetes, P3 and OCI boundaries."""
import copy
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest import mock

import media_backup_phase_test as fixtures
import media_operation_journal
import resource_fencing
import realtime_jwks_publisher as subject
import realtime_p3_create_only


class Client(fixtures.Client):
    def __init__(self, lock):
        super().__init__(lock); self.objects = {}; self.calls = []; self.lost = False
        self.node = {'metadata': {'name': 'node-a', 'uid': 'node-uid', 'resourceVersion': '5',
            'labels': {'kubernetes.io/hostname': 'node-a', 'kubernetes.io/os': 'linux', 'kubernetes.io/arch': 'amd64'}},
            'status': {'conditions': [{'type': 'Ready', 'status': 'True'}]}}
        self.public = {'apiVersion': 'v1', 'kind': 'ConfigMap', 'immutable': True,
            'metadata': {'name': subject.PUBLIC_NAME, 'namespace': 'voice-dev', 'uid': 'pub', 'resourceVersion': '9'},
            'data': {'realtime.json': '{"keys":[]}'}}
        self.foreign_pods = []
        self.replicasets = []; self.pods = []; self.mutate_pod = False

    def call(self, *args, body=None):
        if args[0] == 'get' and args[1] == 'node': return copy.deepcopy(self.node)
        if args[0] == 'get' and args[1] == 'nodes': return {'items': [copy.deepcopy(self.node)]}
        if args[0] == 'get' and args[1] == 'pods': return {'items': copy.deepcopy(self.foreign_pods + self.pods)}
        if args[0] == 'get' and args[1] == 'replicasets': return {'items': copy.deepcopy(self.replicasets)}
        if args[0] == 'get' and args[1] == 'configmap' and args[2] == subject.PUBLIC_NAME: return copy.deepcopy(self.public)
        if args[0] == 'get' and args[1] in ('configmap', 'deployment', 'service') and '--ignore-not-found=true' in args:
            return copy.deepcopy(self.objects.get((args[1], args[2]), {}))
        if args[0] == 'create':
            dry = '--dry-run=server' in args; self.calls.append(('dry' if dry else 'create', body['metadata']['name']))
            result = copy.deepcopy(body)
            if body['kind'] == 'Service': result['spec'].update(clusterIP='10.96.0.20', clusterIPs=['10.96.0.20'],
                                                                 ipFamilies=['IPv4'], ipFamilyPolicy='SingleStack', internalTrafficPolicy='Cluster')
            if not dry:
                result['metadata'].update(uid='uid-' + body['kind'], resourceVersion='10')
                if body['kind'] == 'Deployment':
                    result['metadata']['generation'] = 1
                    result['status'] = {'observedGeneration': 1, 'updatedReplicas': 1, 'availableReplicas': 1, 'readyReplicas': 1}
                    self.replicasets = [{'metadata': {'uid': 'rs', 'resourceVersion': '12', 'namespace': 'voice-dev', 'labels': {'app': subject.NAME},
                        'ownerReferences': [{'controller': True, 'kind': 'Deployment', 'uid': 'uid-Deployment'}]}}]
                    podspec = copy.deepcopy(body['spec']['template']['spec']); podspec['nodeName'] = 'node-a'
                    if self.mutate_pod: podspec['containers'].append({'name': 'intruder', 'image': 'bad'})
                    self.pods = [{'metadata': {'name': 'jwks-pod', 'uid': 'pod', 'resourceVersion': '14',
                        'namespace': 'voice-dev', 'labels': {'app': subject.NAME},
                        'ownerReferences': [{'controller': True, 'kind': 'ReplicaSet', 'uid': 'rs'}]},
                        'spec': podspec, 'status': {'conditions': [{'type': 'Ready', 'status': 'True'}],
                            'containerStatuses': [{'name': 'jwks', 'ready': True, 'restartCount': 0,
                                'containerID': 'containerd://abc', 'imageID': 'docker-pullable://nginx@' + subject.PLATFORM,
                                'state': {'running': {}}}]}}]
                self.objects[body['kind'].lower(), body['metadata']['name']] = copy.deepcopy(result)
                if self.lost: raise OSError('lost after commit')
            return result
        return super().call(*args)


class View:
    def __init__(self, ledger, client): self.ledger, self.client = ledger, client
    def read(self): return self.ledger.read()
    def write(self, revision, row): return self.ledger.write(revision, row)


class PhaseTest(unittest.TestCase):
    def setUp(self):
        self.base = fixtures.PhaseTest('test_exact_terminal_receipt_and_no_replay'); self.base.setUp()
        self.addCleanup(self.base.doCleanups)
        self.client = Client(self.base.lock)
        intent = {'resource_context': self.base.context, 'namespace_uid': 'ns', 'realtime_jwks_publisher': {
            'source_repo': '/source', 'source_sha': self.base.context['commit'],
            'node': {'name': 'node-a', 'uid': 'node-uid', 'resourceVersion': '5', 'hostname': 'node-a'}}}
        self.journal = media_operation_journal.Journal(Path(self.base.temp.name) / 'publisher', intent)
        self.addCleanup(self.journal.close); self.client.journal = self.journal
        self.fence = resource_fencing.Fence(self.base.context, lambda: copy.deepcopy(self.base.card), View(self.base.ledger, self.client))
        other_client = Client(self.base.lock)
        other = media_operation_journal.Journal(Path(self.base.temp.name) / 'p3', {'resource_context': self.base.context, 'namespace_uid': 'ns'})
        self.addCleanup(other.close); other_client.journal = other
        p3phase = object.__new__(realtime_p3_create_only.Session)
        p3phase.fence = resource_fencing.Fence(self.base.context, lambda: copy.deepcopy(self.base.card), View(self.base.ledger, other_client))
        p3phase.namespace_uid = 'ns'; p3phase.lock = copy.deepcopy(self.base.lock); p3phase.journal = other; p3phase.completed = True
        objects = {
            'configmap-' + subject.PUBLIC_NAME: {'status':'done','kind':'ConfigMap','name':subject.PUBLIC_NAME,
                'keys':['realtime.json'],'uid':'pub','resourceVersion':'9'},
            'secret-' + subject.TLS_NAME: {'status':'done','kind':'Secret','name':subject.TLS_NAME,
                'keys':['tls.crt','tls.key'],'uid':'tls','resourceVersion':'7'}}
        p3phase.handoff = {'status':'done','receipt': {'operation_id':self.base.context['operation_id'],
            'candidate_source_sha':self.base.context['commit'],'namespace_uid':'ns','objects':objects}}
        p3phase.roots = {('voice-principal-ca','ca.crt'): {'certificate':b'public-ca'}}
        other.state['steps']['realtime-p3-handoff'] = copy.deepcopy(p3phase.handoff); other.save()
        self.base.ledger.row['realtime_p3_handoff'] = copy.deepcopy(p3phase.handoff)
        p3phase._guard = lambda: None; p3phase._check_roots = lambda: None
        self.patches = [mock.patch.object(subject.frontend_runtime_executor, 'KubectlTransport', Client),
                        mock.patch.object(subject, 'verify_source'),
                        mock.patch.object(subject, 'qualify_oci', return_value={'index':subject.INDEX,'platform':subject.PLATFORM,'config':subject.CONFIG}),
                        mock.patch.object(subject, '_public_tls', return_value=({'uid':'tls','resourceVersion':'7'},b'certificate')),
                        mock.patch.object(subject, '_verify_cert', return_value='c'*64)]
        for patch in self.patches: patch.start(); self.addCleanup(patch.stop)
        self.phase = subject.PublisherPhase(self.journal, self.fence, self.client, self.base.lock, 'ns', p3phase)

    def test_create_only_real_journal_and_recheck(self):
        proof = self.phase.run(b'index', b'child', b'config')
        self.assertEqual(proof['scope'], 'create-only')
        self.assertEqual([kind for kind, _ in self.client.calls], ['dry'] * 3 + ['create'] * 3)
        self.assertEqual(self.base.ledger.row['realtime_jwks_publisher']['status'], 'done')
        self.assertEqual(subject.verify_publisher(self.phase), proof)
        self.client.objects['service', subject.NAME]['spec']['selector']['app'] = 'foreign'
        with self.assertRaises(subject.PublisherError): subject.verify_publisher(self.phase)
        self.assertEqual(self.base.ledger.row['status'], 'uncertain')

    def test_collision_is_never_adopted(self):
        self.client.objects['service', subject.NAME] = {'foreign': True}
        with self.assertRaises(subject.PublisherError): self.phase.run(b'', b'', b'')
        self.assertEqual(self.client.calls, [])

    def test_existing_foreign_selector_pod_prevents_publisher_creation(self):
        self.client.foreign_pods = [{'metadata': {'uid': 'foreign'}}]
        with self.assertRaises(subject.PublisherError): self.phase.run(b'', b'', b'')
        self.assertEqual(self.client.calls, [])

    def test_lost_response_retains_pending_and_never_replays(self):
        self.client.lost = True
        with self.assertRaises(subject.PublisherError): self.phase.run(b'', b'', b'')
        self.assertEqual(len(self.client.objects), 1)
        self.assertEqual(self.base.ledger.row['status'], 'uncertain')
        self.assertEqual(self.base.ledger.row['realtime_jwks_publisher']['objects']['configmap']['status'], 'pending')
        with self.assertRaises(subject.PublisherError): self.phase.run(b'', b'', b'')
        self.assertEqual(len(self.client.objects), 1)

    def test_source_drift_after_first_effect_stops_before_second(self):
        def source_check(*args):
            if self.client.objects: raise subject.PublisherError('source changed')
        with mock.patch.object(subject, 'verify_source', side_effect=source_check):
            with self.assertRaises(subject.PublisherError): self.phase.run(b'', b'', b'')
        self.assertEqual(len(self.client.objects), 1)
        self.assertEqual(self.base.ledger.row['status'], 'uncertain')

    def test_mutated_actual_pod_cannot_produce_a_terminal_proof(self):
        self.client.mutate_pod = True
        with self.assertRaises(subject.PublisherError): self.phase.run(b'', b'', b'')
        self.assertEqual(self.base.ledger.row['status'], 'uncertain')

    def test_consistent_ledger_and_local_tampering_cannot_change_pinned_image(self):
        self.phase.run(b'', b'', b'')
        forged = copy.deepcopy(self.phase.record); forged['image']['platform'] = 'sha256:' + 'f' * 64
        self.phase.record = forged
        self.journal.state['steps']['realtime-jwks-publisher'] = copy.deepcopy(forged); self.journal.save()
        self.base.ledger.row['realtime_jwks_publisher'] = copy.deepcopy(forged)
        with self.assertRaises(subject.PublisherError): subject.verify_publisher(self.phase)

    def test_single_replicaset_without_pod_is_pending_not_corruption(self):
        self.phase.run(b'', b'', b'')
        replicasets = copy.deepcopy(self.client.replicasets); pods = copy.deepcopy(self.client.pods)
        self.client.replicasets = []; self.client.pods = []
        expected = dict(zip(('configmap', 'deployment', 'service'), subject.desired('node-a')))
        with self.assertRaises(subject._NotReady): self.phase._runtime(expected)
        self.client.replicasets = replicasets
        self.client.pods = []
        with self.assertRaises(subject._NotReady): self.phase._runtime(expected)
        self.client.pods = pods
        deploy = self.client.objects['deployment', subject.NAME]
        deploy['status']['readyReplicas'] = 0
        with self.assertRaises(subject._NotReady): self.phase._runtime(expected)
        deploy['status']['readyReplicas'] = 1
        self.assertEqual(self.phase._runtime(expected)['pod_uid'], 'pod')

    def test_real_p3_journal_and_public_certificate_binding(self):
        import base64
        import realtime_p3_create_only_packet as packet
        import realtime_p3_create_only_packet_test as packet_fixture
        packet_fixture.PacketTest.setUpClass(); self.addCleanup(packet_fixture.PacketTest.doClassCleanups)
        envelope = copy.deepcopy(packet_fixture.PacketTest.envelope)
        envelope['operation_id'] = self.base.context['operation_id']
        issuer = envelope['source_sha']
        validated = packet.validate(envelope, self.base.context['operation_id'], issuer,
                                    packet_fixture.PacketTest.principal, packet_fixture.PacketTest.media, packet_fixture.PacketTest.space)
        p3_client = Client(self.base.lock)
        intent = {'resource_context': self.base.context, 'namespace_uid': 'ns', 'realtime_p3_create': {
            'issuer_source_sha': issuer, 'source_repo': '/source', 'issuer_suffix': self.base.context['operation_id'].rsplit('-', 1)[1]}}
        p3_journal = media_operation_journal.Journal(Path(self.base.temp.name) / 'real-p3', intent)
        self.addCleanup(p3_journal.close); p3_client.journal = p3_journal
        p3_fence = resource_fencing.Fence(self.base.context, lambda: copy.deepcopy(self.base.card), View(self.base.ledger, p3_client))
        p3phase = realtime_p3_create_only.Session(p3_journal, p3_fence, p3_client, self.base.lock, 'ns')
        effects = {}
        for (kind, name), keys in packet.MEMBERS.items():
            step = kind.lower() + '-' + name
            effects[step] = {'status':'done','kind':kind,'name':name,'keys':sorted(keys),
                             'uid':'uid-'+name,'resourceVersion':'11','nonce':'d'*32,'operation_id':self.base.context['operation_id']}
            p3_journal.state['steps']['realtime-p3-object-' + step] = copy.deepcopy(effects[step])
        handoff = {'status':'done','receipt':{'operation_id':self.base.context['operation_id'],
            'candidate_source_sha':self.base.context['commit'],'namespace_uid':'ns','objects':copy.deepcopy(effects)}}
        p3_journal.state['steps']['realtime-p3-handoff'] = copy.deepcopy(handoff); p3_journal.save()
        self.base.ledger.row['realtime_p3_handoff'] = copy.deepcopy(handoff)
        self.base.ledger.row['realtime_p3_objects'] = copy.deepcopy(effects)
        p3phase.handoff = copy.deepcopy(handoff); p3phase.effects = copy.deepcopy(effects); p3phase.completed = True
        roots = {'voice-principal-ca':packet_fixture.PacketTest.principal, 'voice-media-ca':packet_fixture.PacketTest.media,
                 'voice-space-media-server-tls':packet_fixture.PacketTest.space}
        p3phase.roots = {(name,key): {'uid':'uid-'+name, 'resourceVersion':'9', 'certificate':roots[name]}
                         for name,key in realtime_p3_create_only.PUBLIC}
        public = validated['ConfigMap', subject.PUBLIC_NAME]
        self.client.public = copy.deepcopy(public); self.client.public['metadata'].update(uid='uid-'+subject.PUBLIC_NAME, resourceVersion='11')
        cert = base64.b64decode(validated['Secret', subject.TLS_NAME]['data']['tls.crt'])
        self.phase = subject.PublisherPhase(self.journal, self.fence, self.client, self.base.lock, 'ns', p3phase)
        def public_certificate(name,key):
            return ('uid-'+name+'\n9\n'+base64.b64encode(roots[name]).decode()).encode()
        self.patches[-1].stop()
        with mock.patch.object(realtime_p3_create_only.transport, 'public_certificate', side_effect=public_certificate), \
             mock.patch.object(subject, '_public_tls', return_value=({'uid':'uid-'+subject.TLS_NAME,'resourceVersion':'11'}, cert)):
            proof = self.phase.run(b'', b'', b'')
        self.assertEqual(proof['public']['uid'], 'uid-'+subject.PUBLIC_NAME)
        self.assertNotIn('PRIVATE KEY', str(proof)); self.assertNotIn('PRIVATE KEY', str(self.journal.state))


if __name__ == '__main__': unittest.main()
