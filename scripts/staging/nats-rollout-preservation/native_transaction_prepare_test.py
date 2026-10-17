"""Transaction-level native PREPARE proof with a typed fake Kube API."""
import base64
import copy
import hashlib
import json
import os
from pathlib import Path
import unittest
from contextlib import contextmanager
from unittest.mock import patch

import kube_json_api
import native_runtime_stage_test
import preserve
import rollout_census
import transaction
from closed_archive import Archive
from native_store import archive_closed_store


OP = 'a' * 12
ACCOUNT = 'A' + 'B' * 55
IMAGE = 'ghcr.io/poryadok/voice-nats-rollout-runner@sha256:' + 'a' * 64
SOURCE_UID = '00000000-0000-4000-8000-000000000104'
SOURCE_PV_UID = '00000000-0000-4000-8000-000000000105'
SCRATCH_UID = '00000000-0000-4000-8000-000000000204'
SCRATCH_PV_UID = '00000000-0000-4000-8000-000000000205'
ARCHIVE_POD_UID = '00000000-0000-4000-8000-000000000301'
CENSUS_POD_UID = '00000000-0000-4000-8000-000000000302'


@contextmanager
def _private_operation(operation):
    base = Path(Archive.PRIVATE_ANCHOR) / ('rollout-' + operation)
    base.mkdir(mode=0o700)
    yield base


def _stream_frames(archive, manifest, request, pod_uid, pvc_uid, pv_uid):
    import archive_stream
    raw = Path(archive).read_bytes()
    for sequence, offset in enumerate(range(0, len(raw), 48 << 10)):
        yield (json.dumps({'schema': archive_stream.SCHEMA, 'operation': request['operation'],
            'nonce': request['nonce'], 'pod_uid': pod_uid, 'source_pvc_uid': pvc_uid,
            'source_pv_uid': pv_uid, 'kind': 'data', 'sequence': sequence,
            'data': base64.b64encode(raw[offset:offset + (48 << 10)]).decode('ascii')},
            sort_keys=True, separators=(',', ':')) + '\n').encode()
    manifest_raw = json.dumps(manifest, sort_keys=True, separators=(',', ':')).encode()
    for sequence, offset in enumerate(range(0, len(manifest_raw), 48 << 10)):
        yield (json.dumps({'schema': archive_stream.SCHEMA, 'operation': request['operation'],
            'nonce': request['nonce'], 'pod_uid': pod_uid, 'source_pvc_uid': pvc_uid,
            'source_pv_uid': pv_uid, 'kind': 'manifest', 'sequence': sequence,
            'data': base64.b64encode(manifest_raw[offset:offset + (48 << 10)]).decode('ascii')},
            sort_keys=True, separators=(',', ':')) + '\n').encode()
    yield (json.dumps({'schema': archive_stream.SCHEMA, 'operation': request['operation'],
        'nonce': request['nonce'], 'pod_uid': pod_uid, 'source_pvc_uid': pvc_uid,
        'source_pv_uid': pv_uid, 'kind': 'complete',
        'frames': (len(raw) + (48 << 10) - 1) // (48 << 10),
        'manifest_frames': (len(manifest_raw) + (48 << 10) - 1) // (48 << 10),
        'archive_sha256': hashlib.sha256(raw).hexdigest(),
        'manifest_sha256': hashlib.sha256(manifest_raw).hexdigest()},
        sort_keys=True, separators=(',', ':')) + '\n').encode()


class PrepareAPI(native_runtime_stage_test.FakeAPI):
    """One fake authority surface for stage CAS and fixed producer Jobs."""

    def __init__(self, base):
        super().__init__()
        self.base = base
        self.calls = []
        self.job_rows = {}
        self.pod_rows = {}
        self.archive = None
        self.archive_manifest = None
        self.census = None
        self.source_claim = copy.deepcopy(self.claim)
        self.source_claim['metadata']['uid'] = SOURCE_UID
        self.source_claim['status']['capacity'] = {'storage': '100Gi'}
        self.source_pv = copy.deepcopy(self.pv)
        self.source_pv['metadata']['uid'] = SOURCE_PV_UID
        self.source_pv['spec']['claimRef']['uid'] = SOURCE_UID
        self.scratch_claim = None
        self.scratch_pv = {
            'metadata': {'name': 'scratch-pv', 'uid': SCRATCH_PV_UID},
            'status': {'phase': 'Bound'},
            'spec': {'claimRef': {'namespace': 'voice-staging',
                    'name': 'voice-nats-verify-' + OP, 'uid': SCRATCH_UID},
                'storageClassName': 'fast', 'accessModes': ['ReadWriteOnce'],
                'volumeMode': 'Filesystem', 'capacity': {'storage': '48Gi'},
                'nodeAffinity': {'required': {'nodeSelectorTerms': [{'matchExpressions': [
                    {'key': 'kubernetes.io/hostname', 'operator': 'In',
                     'values': ['host-label']}]}]}}}}
        self.scratch_node = {'metadata': {'name': 'node-object-name',
                'uid': '00000000-0000-4000-8000-000000000206',
                'resourceVersion': '1',
                'labels': {'kubernetes.io/hostname': 'host-label'}},
            'spec': {}, 'status': {'conditions': [{'type': 'Ready', 'status': 'True'}]}}
        tree = {'account_details': [{'id': ACCOUNT, 'stream_detail': [{
            'name': 'chat_events', 'created': '2026-10-05T10:00:00Z',
            'config': {'name': 'chat_events', 'storage': 'file'},
            'state': {'messages': 0, 'bytes': 0, 'first_seq': 0, 'last_seq': 0,
                'consumer_count': 0}, 'consumer_detail': []}]}],
            'total': 1, 'streams': 1, 'consumers': 0, 'messages': 0, 'bytes': 0}
        self.live_jsz_tree = tree
        class Snapshot:
            def monitor_jsz(self, _broker): return tree
        self.census = rollout_census.census(Snapshot(), 'fixture-broker', ACCOUNT)

    def exec_hub_jsz(self, *, namespace, pod_name, pod_uid, container, timeout_seconds):
        assert namespace == 'voice-staging' and container == 'nats'
        assert pod_uid == self.pod['metadata']['uid'] and timeout_seconds == 30
        return json.dumps(self.live_jsz_tree, separators=(',', ':')).encode()

    def get_pvc(self, namespace, name):
        if name == self.source_claim['metadata']['name']:
            return copy.deepcopy(self.source_claim)
        if self.scratch_claim is not None and name == self.scratch_claim['metadata']['name']:
            return copy.deepcopy(self.scratch_claim)
        raise kube_json_api.KubeNotFound('missing')

    def get_configmap(self, namespace, name):
        if name == 'voice-nats-generation': return copy.deepcopy(self.marker)
        return super().get_configmap(namespace=namespace, name=name)

    def get_deployment(self, namespace, name):
        return copy.deepcopy(next(row for row in self.deployments
            if row['metadata']['name'] == name))

    def list_hub_pods(self, namespace, labels):
        assert namespace == 'voice-staging' and labels == {'app': 'voice-nats-pvc-candidate'}
        return copy.deepcopy([row for row in self.pods
            if row.get('metadata', {}).get('labels', {}).get('app') == 'voice-nats-pvc-candidate'])

    def get_pv(self, name):
        if name == self.source_pv['metadata']['name']:
            return copy.deepcopy(self.source_pv)
        if name == self.scratch_pv['metadata']['name']:
            return copy.deepcopy(self.scratch_pv)
        raise kube_json_api.KubeNotFound('missing')

    def get_node(self, namespace, name=None):
        if name is None:
            name = namespace
            namespace = 'voice-staging'
        if name == self.scratch_node['metadata']['name']:
            return copy.deepcopy(self.scratch_node)
        return super().get_node(namespace, name)

    def list_nodes_by_hostname(self, hostname):
        if hostname == 'host-label': return [copy.deepcopy(self.scratch_node)]
        return super().list_nodes_by_hostname(hostname)

    def get_storage_class(self, name):
        self.calls.append(('storage-class', name))
        return {'metadata': {'name': name, 'uid': '00000000-0000-4000-8000-000000000207',
                'resourceVersion': '1'}, 'provisioner': 'example.test/csi',
            'volumeBindingMode': 'Immediate'}

    def create_verification_pvc(self, operation, nonce, size_bytes, storage_class, *, attempt):
        self.calls.append(('create-verification-pvc', operation, nonce, attempt))
        claim_name = 'voice-nats-verify-' + operation + '-' + attempt
        self.scratch_pv['spec']['claimRef']['name'] = claim_name
        self.scratch_claim = {'apiVersion': 'v1', 'kind': 'PersistentVolumeClaim',
            'metadata': {'name': claim_name, 'namespace': 'voice-staging',
                'uid': SCRATCH_UID, 'resourceVersion': '1', 'labels': {
                    'voice.io/nats-operation': operation, 'voice.io/nats-job-nonce': nonce,
                    'voice.io/nats-pvc-role': 'verification',
                    'voice.io/nats-verification-attempt':attempt}},
            'spec': {'accessModes': ['ReadWriteOnce'], 'volumeMode': 'Filesystem',
                'storageClassName': storage_class,
                'resources': {'requests': {'storage': str(size_bytes)}},
                'volumeName': self.scratch_pv['metadata']['name']},
            'status': {'phase': 'Bound', 'capacity': {'storage': '48Gi'}}}
        return copy.deepcopy(self.scratch_claim)

    def list_operation_jobs(self, namespace, operation):
        self.calls.append(('list-operation-jobs', operation))
        return []

    def create_job(self, namespace, wanted):
        self.calls.append(('create-job', wanted['metadata']['name']))
        row = copy.deepcopy(wanted)
        row['metadata'].update(uid=f'00000000-0000-4000-8000-{len(self.job_rows)+401:012d}',
            resourceVersion='1', generation=1)
        row['status'] = {'active': 1}
        self.job_rows[row['metadata']['name']] = row
        return copy.deepcopy(row)

    def get_job(self, namespace, name):
        if name not in self.job_rows: raise kube_json_api.KubeNotFound('missing')
        return copy.deepcopy(self.job_rows[name])

    def wait_job_pod(self, namespace, name, uid, timeout):
        job = self.job_rows[name]
        pod_uid = ARCHIVE_POD_UID if 'archive' in name else CENSUS_POD_UID
        template = job['spec']['template']
        labels = copy.deepcopy(template['metadata']['labels']) | {
            'batch.kubernetes.io/controller-uid': uid, 'controller-uid': uid,
            'batch.kubernetes.io/job-name': name, 'job-name': name}
        pod = {'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {
                **copy.deepcopy(template['metadata']), 'name': name + '-abcde',
                'namespace': namespace, 'uid': pod_uid, 'labels': labels,
                'ownerReferences': [{'apiVersion': 'batch/v1', 'kind': 'Job',
                    'name': name, 'uid': uid, 'controller': True, 'blockOwnerDeletion': True}]},
            'spec': copy.deepcopy(template['spec']),
            'status': {'phase': 'Running', 'containerStatuses': [{
                'name': 'nats-phase', 'containerID': 'containerd://' + ('a' if 'archive' in name else 'c') * 64,
                'ready': True, 'restartCount': 0}]}}
        pod['spec']['nodeName'] = 'node-object-name'
        self.pod_rows[name] = pod
        return copy.deepcopy(pod)

    def get_job_pod(self, namespace, name): return copy.deepcopy(self.pod_rows[name])

    def get_pod(self, namespace, name):
        for row in self.pod_rows.values():
            if row.get('metadata', {}).get('name') == name:
                return copy.deepcopy(row)
        return super().get_pod(namespace=namespace, name=name)

    def cas_rollout_deployment(self, namespace, name, expected_uid, expected_rv,
            expected_replicas, expected_template_sha256, *, new_replicas=None,
            new_template=None, expected_jsdata_claim=None, new_jsdata_claim=None):
        result = super().cas_rollout_deployment(namespace, name, expected_uid, expected_rv,
            expected_replicas, expected_template_sha256, new_replicas=new_replicas,
            new_template=new_template, expected_jsdata_claim=expected_jsdata_claim,
            new_jsdata_claim=new_jsdata_claim)
        if new_replicas == 0 and name != native_runtime_stage_test.HUB:
            self.pods = [pod for pod in self.pods
                if pod.get('metadata', {}).get('labels', {}).get('app') != name]
        elif new_replicas == 0:
            status = self.pod['status']['containerStatuses'][0]
            status['state'] = {'terminated': {'exitCode': 0, 'reason': 'Completed'}}
            self.pods = [copy.deepcopy(self.pod) if row['metadata']['uid'] ==
                self.pod['metadata']['uid'] else row for row in self.pods]
        return result

    def add_owned_pod_finalizer(self, *, namespace, name, pod_uid, expected_rv,
            expected_finalizers, finalizer):
        self.calls.append(('pod-finalizer-add', pod_uid))
        pod = self.get_pod(namespace=namespace, name=name)
        if pod['metadata']['uid'] != pod_uid or pod['metadata']['resourceVersion'] != expected_rv:
            raise kube_json_api.KubeAPIError('kube_api_conflict')
        if pod['metadata'].get('finalizers', []) != expected_finalizers:
            raise kube_json_api.KubeAPIError('kube_api_conflict')
        pod['metadata']['finalizers'] = [*expected_finalizers, finalizer]
        pod['metadata']['resourceVersion'] = str(int(expected_rv) + 1)
        self.pod = copy.deepcopy(pod)
        self.pods = [pod if row['metadata']['uid'] == pod_uid else row for row in self.pods]
        return copy.deepcopy(pod)

    def remove_owned_pod_finalizer(self, *, namespace, name, pod_uid, expected_rv,
            expected_finalizers, finalizer):
        self.calls.append(('pod-finalizer-remove', pod_uid))
        pod = self.get_pod(namespace=namespace, name=name)
        if pod['metadata']['uid'] != pod_uid or pod['metadata']['resourceVersion'] != expected_rv:
            raise kube_json_api.KubeAPIError('kube_api_conflict')
        if pod['metadata'].get('finalizers', []) != expected_finalizers:
            raise kube_json_api.KubeAPIError('kube_api_conflict')
        pod['metadata']['finalizers'] = [item for item in expected_finalizers if item != finalizer]
        pod['metadata']['resourceVersion'] = str(int(expected_rv) + 1)
        self.pods = [row for row in self.pods if row['metadata']['uid'] != pod_uid]
        return copy.deepcopy(pod)

    def exec_pod_verification(self, namespace, pod_name, pod_uid, request, binding, action):
        self.calls.append(('exec-verification', action))
        if action == 'archive_create_and_stream':
            return _stream_frames(self.archive, self.archive_manifest, request, pod_uid,
                binding['source_uid'], binding['source_pv_uid'])
        raise AssertionError(action)

    def exec_pod_verification_input(self, namespace, pod_name, pod_uid,
            request, binding, archive, bundle, bundle_info):
        self.calls.append(('load-census-inputs', pod_uid))
        self._input_receipt = self._input_receipt_for(request, binding, archive,
            bundle_info, pod_uid)
        return (json.dumps(self._input_receipt, sort_keys=True, separators=(',', ':'),
            ensure_ascii=False, allow_nan=False).encode() + b'\n')

    def _input_receipt_for(self, request, binding, archive, info, pod_uid):
        return {'schema': 'voice-nats-census-input-receipt-v1', 'phase': 'census-inputs',
            'purpose': 'verification', 'operation': request['operation'], 'nonce': request['nonce'],
            'pod_uid': pod_uid, 'source_pvc_uid': binding['source_uid'],
            'source_pv_uid': binding['source_pv_uid'],
            'verification_pvc_uid': binding['verification_uid'],
            'verification_pv_uid': binding['verification_pv_uid'],
            'archive_sha256': archive['sha256'], 'archive_bytes': archive['bytes'],
            'session_attempt': info['attempt'], 'input_bundle_sha256': info['sha256'],
            'input_bundle_bytes': info['bytes'], 'status': 'COMPLETE', 'file_count': 2,
            'kernel_bundle_sha256': 'c' * 64, 'server_config_sha256': 'd' * 64}

    def read_census_input_receipt(self, namespace, pod_name, pod_uid,
            request, binding, archive, info, *, input_pod_uid):
        return copy.deepcopy(self._input_receipt)

    def execute_census_input_consumption(self, namespace, pod_name, pod_uid,
            request, binding, archive, info, *, input_pod_uid):
        self.calls.append(('consume-census-inputs', pod_uid))
        return {'schema': 'voice-nats-census-consumption-receipt-v1',
            'phase': 'census-consumption', 'purpose': 'verification',
            'operation': request['operation'], 'nonce': request['nonce'],
            'pod_uid': pod_uid, 'input_pod_uid': input_pod_uid,
            'source_pvc_uid': binding['source_uid'], 'source_pv_uid': binding['source_pv_uid'],
            'verification_pvc_uid': binding['verification_uid'],
            'verification_pv_uid': binding['verification_pv_uid'],
            'archive_sha256': archive['sha256'], 'archive_bytes': archive['bytes'],
            'session_attempt': info['attempt'], 'input_bundle_sha256': info['sha256'],
            'input_bundle_bytes': info['bytes'], 'kernel_bundle_sha256': 'c' * 64,
            'server_config_sha256': 'd' * 64, 'status': 'COMPLETE'}

    def exec_pod_census_input(self, namespace, pod_name, pod_uid,
            request, binding, archive, manifest_bytes, archive_sha256, archive_bytes, info):
        self.calls.append(('receive-census-archive', pod_uid))
        assert Path(archive).stat().st_size == archive_bytes

    def read_verification_census(self, namespace, pod_name, pod_uid,
            request, binding, archive, info):
        self.calls.append(('read-census', pod_uid))
        return {'receipt': {'census_sha256': preserve.canonical(self.census)},
            'census': copy.deepcopy(self.census)}

    def delete_job(self, namespace, name, uid):
        self.calls.append(('delete-job', name, uid))
        return True


class NativeTransactionPrepareTests(unittest.TestCase):
    def test_prepare_reaches_real_stage_fence_archive_and_census_before_waiting(self):
        if (os.geteuid() != 65532 or os.getegid() != 65532
            or not os.environ.get('NATIVE_PRIVATE_TEST_ANCHOR')):
            self.skipTest('transaction PREPARE requires private Linux UID/GID 65532 fixture')
        with _private_operation(OP) as base:
            manifests = base / 'apply-manifests.json'
            manifests.write_text('[]', encoding='utf-8')
            code = base / 'trusted-source-fixture'
            code.mkdir(mode=0o700)
            (code / 'kernel').write_bytes(b'pinned helper image kernel digest fixture\n')
            source = base / 'fixture-source'
            stream = source / 'jetstream' / ACCOUNT / 'streams' / 'chat_events'
            stream.mkdir(parents=True, mode=0o700)
            (stream / 'meta.inf').write_text(json.dumps({'name': 'chat_events',
                'storage': 'file'}), encoding='utf-8')
            (stream / 'meta.sum').write_bytes(b'fixture metadata')
            bootstrap = base / 'inputs' / 'bootstrap.creds'
            bootstrap.parent.mkdir(mode=0o700)
            bootstrap.write_bytes(b'fixture bootstrap credentials')
            bootstrap.chmod(0o440)
            server = base / 'server.conf'
            server.write_bytes(b'accounts { SYS { users = [] } }')
            server.chmod(0o440)
            api = PrepareAPI(base)
            source_tar = base / 'source.tar'
            api.archive_manifest = archive_closed_store(source, source_tar)
            api.archive = source_tar

            target = {'mode': 'full', 'migration_sha256': transaction.canonical([]),
                'nonnats_sha256': transaction.canonical({'checks': [], 'actions': []})}
            nonnats = {'checks': [], 'actions': []}
            capture = {'generation_bound': True}

            def captured_inputs(reader, root, contract, generation, *,
                    bootstrap_enrollment, public_account_callback, native_private):
                self.assertTrue(native_private)
                self.assertEqual(generation, 'r20261017a')
                public_account_callback(ACCOUNT)
                return {'schema': 'voice-nats-captured-inputs-v1', 'generation': generation}

            with patch.object(transaction.guard, 'frontend_image_only', return_value=False), \
                 patch.object(transaction, 'capture_inputs', side_effect=captured_inputs), \
                 patch.object(transaction, 'normalize_target', side_effect=lambda _k, _s, _m, t, **_kw: t), \
                 patch.object(transaction.migrations, 'preflight', return_value={}), \
                 patch.object(transaction.space_safeguards, 'preflight', return_value=None), \
                 patch.object(transaction.nonnats_runtime, 'preflight', return_value={}), \
                 patch.object(transaction.nonnats_plan, 'revalidate'), \
                 patch('source_plan.verify_db_init'):
                state = transaction.prepare(api, base, code, target, {'scripts': []}, OP, capture,
                    [], nonnats, native_api=api, native_config={
                        'phase_image': IMAGE, 'verification_pvc_size_bytes': 48 << 30,
                        'verification_storage_class': 'fast'})

            self.assertEqual((state['phase'], state['status']), ('AWAITING_OFF_NODE', 'WAITING'))
            self.assertTrue(state['cut']['manifest']['archive_sha256'])
            self.assertEqual(state['cut']['census'], api.census)
            self.assertEqual(state['cut']['native_catalog_sha256'], preserve.canonical({
                'account': ACCOUNT, 'streams': ['chat_events'], 'consumers': []}))
            self.assertEqual(state['cut']['closed_durable_states'], {})
            self.assertEqual(state['native_source_binding']['binding']['source_uid'], SOURCE_UID)
            self.assertIn(('consume-census-inputs', CENSUS_POD_UID), api.calls)
            self.assertIn(('receive-census-archive', CENSUS_POD_UID), api.calls)
            self.assertEqual(sum(call[0] == 'create-job' for call in api.calls), 2)
            self.assertEqual(sum(call[0] == 'delete-job' for call in api.calls), 2)
            self.assertNotIn(('delete-pvc',), api.calls)
            self.assertEqual(api.marker['data']['phase'], 'rollout-capturing')


if __name__ == '__main__':
    unittest.main()
