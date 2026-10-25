"""DEV reset scope comes from the explicit disposable-DEV task contract."""
import copy
import unittest

import environment as dev
import reset_bootstrap as reset
from types import SimpleNamespace
from datetime import datetime, timezone
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
from pathlib import Path
import shutil
import tempfile
import threading
from unittest.mock import patch


class DeleteTransportTest(unittest.TestCase):
    @unittest.skipUnless(shutil.which('kubectl'), 'requires kubectl private transport probe')
    def test_real_kubectl_sends_uid_preconditions_to_private_http_server(self):
        calls = []
        class Handler(BaseHTTPRequestHandler):
            def do_DELETE(self):
                if self.headers.get('Transfer-Encoding') == 'chunked':
                    chunks = []
                    while size := int(self.rfile.readline().strip(), 16):
                        chunks.append(self.rfile.read(size))
                        self.rfile.read(2)
                    self.rfile.read(2)
                    raw = b''.join(chunks)
                else:
                    raw = self.rfile.read(int(self.headers['Content-Length']))
                calls.append((self.path, json.loads(raw)))
                self.send_response(200)
                self.send_header('Content-Type', 'application/json')
                self.end_headers()
                self.wfile.write(b'{"kind":"Status","status":"Success"}')
            def log_message(self, *args):
                pass
        server = HTTPServer(('127.0.0.1', 0), Handler)
        thread = threading.Thread(target=server.serve_forever, daemon=True)
        thread.start()
        try:
            with tempfile.TemporaryDirectory() as tmp:
                config = Path(tmp) / 'config.json'
                config.write_text(json.dumps({'apiVersion': 'v1', 'kind': 'Config',
                    'clusters': [{'name': 'private', 'cluster': {'server': 'http://127.0.0.1:' + str(server.server_port)}}],
                    'contexts': [{'name': 'private', 'context': {'cluster': 'private'}}], 'current-context': 'private'}))
                reset.Kube(prefix=('kubectl', '--kubeconfig=' + str(config))).delete_old('Namespace', 'voice-dev', 'old-uid', '123')
            self.assertEqual(calls[0][0], '/api/v1/namespaces/voice-dev')
            self.assertEqual(calls[0][1]['preconditions'], {'uid': 'old-uid', 'resourceVersion': '123'})
        finally:
            server.shutdown()
            server.server_close()
            thread.join()

    def test_exact_uid_and_revision_are_in_delete_request(self):
        calls = []
        def run(args, **kwargs):
            calls.append((args, kwargs))
            return SimpleNamespace(stdout=b'{}')
        kube = reset.Kube(runner=run)
        kube.delete_old('Namespace', 'voice-dev', 'old-uid', '123')
        self.assertEqual(calls[0][0], ['kubectl', 'delete', '--raw', '/api/v1/namespaces/voice-dev', '-f', '-'])
        import json
        body = json.loads(calls[0][1]['input'])
        self.assertEqual(body['preconditions'], {'uid': 'old-uid', 'resourceVersion': '123'})
        for kind, name, uid, rv in [('Namespace', 'voice-staging', 'x', '1'),
                                    ('PersistentVolume', 'voice-staging-postgres', 'x', '1'),
                                    ('PersistentVolume', 'voice-dev-postgres', '', '1')]:
            with self.assertRaises(ValueError):
                kube.delete_old(kind, name, uid, rv)
        self.assertEqual(len(calls), 1)


class QuietWindowTest(unittest.TestCase):
    def test_conservative_midnight_not_randomized_next_trigger(self):
        now = datetime(2026, 10, 25, 20, 30, tzinfo=timezone.utc)
        original = {'observed_at': now.isoformat(), 'boot_id': 'boot', 'monotonic_seconds': 100,
                    'timezone': 'Europe/Moscow', 'service_active': 'inactive', 'main_pid': 0,
                    'timer_substate': 'waiting', 'pending_jobs': [], 'dev_updaters': [], 'catch_up_due': False,
                    'schedule': {'calendar': 'daily', 'randomized_seconds': 3600, 'persistent': True}}
        self.assertEqual(reset.quiet_window(original, original, now=now, remaining_seconds=600), '2026-10-26T00:00:00+03:00')
        with self.assertRaises(ValueError):
            reset.quiet_window(original, original, now=now, remaining_seconds=1800)
        for key, value in [('boot_id', 'different'), ('service_active', 'activating'), ('main_pid', 123),
                           ('pending_jobs', ['renew.service']), ('catch_up_due', True), ('dev_updaters', ['pid']),
                           ('monotonic_seconds', 90), ('observed_at', '2026-10-25T20:29:00+00:00')]:
            with self.subTest(key=key), self.assertRaises(ValueError):
                reset.quiet_window(dict(original, **{key: value}), original, now=now, remaining_seconds=600)


class RecoveryJournalTest(unittest.TestCase):
    def test_second_operator_cannot_enter_same_package(self):
        with tempfile.TemporaryDirectory() as tmp:
            with reset.operator_lock(Path(tmp)):
                with self.assertRaises(OSError):
                    with reset.operator_lock(Path(tmp)):
                        self.fail('second process entered operation')

    def test_interruption_is_retained_and_never_replays_delete(self):
        with tempfile.TemporaryDirectory() as tmp:
            binding = {'operation': 'abcdef123456', 'namespace_uid': 'old-uid', 'package_sha256': 'x'}
            journal = reset.Journal(tmp, binding)
            journal.begin('delete-namespace')
            recovered = reset.Journal(tmp, binding)
            with self.assertRaises(ValueError):
                recovered.begin('delete-namespace')
            recovered.complete('delete-namespace', {'old_namespace_absent': True})
            with self.assertRaises(ValueError):
                reset.Journal(tmp, binding).begin('delete-namespace')
            with self.assertRaises(ValueError):
                reset.Journal(tmp, dict(binding, package_sha256='changed'))


def storage_fixture():
    namespace = {'metadata': {'name': dev.NS, 'uid': 'old-ns', 'labels': dev.LABEL}}
    claims, volumes = [], []
    for name, claim in reset.CLAIMS.items():
        uid = 'old-' + name
        claims.append({'metadata': {'name': claim, 'namespace': dev.NS, 'uid': uid},
                       'spec': {'volumeName': 'voice-dev-' + name}, 'status': {'phase': 'Bound'}})
        volumes.append({'metadata': {'name': 'voice-dev-' + name, 'uid': 'volume-' + name},
                        'spec': {'persistentVolumeReclaimPolicy': 'Retain', 'storageClassName': 'voice-dev-local',
                                 'claimRef': {'namespace': dev.NS, 'name': claim, 'uid': uid}}})
    return namespace, claims, volumes


class FreshStorageTest(unittest.TestCase):
    def test_retained_data_is_never_selected_for_new_claims(self):
        ns, claims, volumes = storage_fixture()
        retained = reset.old_storage(ns, claims, volumes, namespace_uid='old-ns')
        ns['metadata']['uid'] = 'new-ns'
        fresh = []
        for name, claim in reset.CLAIMS.items():
            uid = 'new-' + name
            fresh.append({'metadata': {'name': claim, 'namespace': dev.NS, 'uid': uid,
                                       'annotations': {'volume.kubernetes.io/selected-node': 'pmdebook'}},
                          'spec': {'storageClassName': 'local-path', 'volumeName': 'pvc-' + uid},
                          'status': {'phase': 'Bound'}})
            volumes.append({'metadata': {'name': 'pvc-' + uid, 'uid': 'new-volume-' + name,
                                         'annotations': {'pv.kubernetes.io/provisioned-by': 'rancher.io/local-path'}},
                            'spec': {'claimRef': {'namespace': dev.NS, 'name': claim, 'uid': uid},
                                     'storageClassName': 'local-path', 'persistentVolumeReclaimPolicy': 'Delete',
                                     'nodeAffinity': reset.PROVIDER_AFFINITY,
                                     'local': {'path': '/var/lib/rancher/k3s/storage/pvc-' + uid + '_' + dev.NS + '_' + claim}},
                            'status': {'phase': 'Bound'}})
        reset.verify_fresh(ns, fresh, volumes, old_namespace_uid='old-ns', retained=retained,
                           provider_contract='local-path-local-uid-v1')
        for change in (lambda: fresh[0]['spec'].update(volumeName='voice-dev-postgres'),
                       lambda: fresh[0]['spec'].update(dataSource={'name': 'old-snapshot'}),
                       lambda: volumes[0]['spec'].update(claimRef={})):
            before_claims, before_volumes = copy.deepcopy(fresh), copy.deepcopy(volumes)
            change()
            with self.assertRaises(ValueError):
                reset.verify_fresh(ns, fresh, volumes, old_namespace_uid='old-ns', retained=retained,
                                   provider_contract='local-path-local-uid-v1')
            fresh, volumes = before_claims, before_volumes

    def test_delete_policy_wrong_namespace_or_uid_fail_before_reset(self):
        for change in (lambda n, c, v: n['metadata'].update(uid='changed'),
                       lambda n, c, v: n['metadata'].update(name='voice-staging'),
                       lambda n, c, v: v[0]['spec'].update(persistentVolumeReclaimPolicy='Delete'),
                       lambda n, c, v: v[0]['spec']['claimRef'].update(uid='changed')):
            ns, claims, volumes = storage_fixture()
            change(ns, claims, volumes)
            with self.assertRaises(ValueError):
                reset.old_storage(ns, claims, volumes, namespace_uid='old-ns')

    def test_existing_class_and_capacity_are_explicit(self):
        sc = {'metadata': {'name': 'local-path', 'uid': 'class', 'resourceVersion': '1',
                          'annotations': {'defaultVolumeType': 'local'}},
              'provisioner': 'rancher.io/local-path', 'reclaimPolicy': 'Delete',
              'volumeBindingMode': 'WaitForFirstConsumer'}
        self.assertEqual(reset.storage_class_pin(sc, available_bytes=100 * 1024 ** 3)['uid'], 'class')
        with self.assertRaises(ValueError):
            reset.storage_class_pin(sc, available_bytes=1024)
        sc['provisioner'] = 'foreign'
        with self.assertRaises(ValueError):
            reset.storage_class_pin(sc, available_bytes=100 * 1024 ** 3)

    def test_provider_backend_mismatch_is_rejected_at_admission(self):
        sc = {'metadata': {'name': 'local-path', 'uid': 'class', 'resourceVersion': '1',
                          'annotations': {'defaultVolumeType': 'hostPath'}},
              'provisioner': 'rancher.io/local-path', 'reclaimPolicy': 'Delete',
              'volumeBindingMode': 'WaitForFirstConsumer'}
        with self.assertRaisesRegex(ValueError, 'local-path'):
            reset.storage_class_pin(sc, available_bytes=100 * 1024 ** 3)


class ProviderQualificationTest(unittest.TestCase):
    def fixture(self):
        namespace = {'metadata': {'name': 'voice-dev-provider-probe-fixture', 'uid': 'probe-ns',
                                 'labels': {'voice.dev/provider-probe': 'true'}}}
        claim = {'metadata': {'namespace': namespace['metadata']['name'], 'name': 'probe', 'uid': 'probe-claim',
                             'annotations': {'volume.kubernetes.io/selected-node': 'pmdebook'}},
                 'spec': {'storageClassName': 'local-path', 'volumeName': 'pvc-probe-claim'},
                 'status': {'phase': 'Bound'}}
        volume = {'metadata': {'name': 'pvc-probe-claim', 'uid': 'probe-pv',
                               'annotations': {'pv.kubernetes.io/provisioned-by': 'rancher.io/local-path'}},
                  'spec': {'storageClassName': 'local-path', 'persistentVolumeReclaimPolicy': 'Delete',
                           'claimRef': {k: claim['metadata'][k] for k in ('namespace', 'name', 'uid')},
                           'local': {'path': '/var/lib/rancher/k3s/storage/pvc-probe-claim_' + namespace['metadata']['name'] + '_probe'},
                           'nodeAffinity': {'required': {'nodeSelectorTerms': [{'matchExpressions': [
                               {'key': 'kubernetes.io/hostname', 'operator': 'In', 'values': ['pmdebook']}]}]}}},
                  'status': {'phase': 'Bound'}}
        provider, storage_class = {'config_uid': 'provider'}, {'uid': 'class'}
        receipt = {'schema': 'voice-dev-provider-qualification-v1', 'contract': 'local-path-local-uid-v1',
                   'provider': provider, 'storage_class': storage_class,
                   'namespace': {k: namespace['metadata'][k] for k in ('name', 'uid')},
                   'claim': {k: claim['metadata'][k] for k in ('name', 'uid')},
                   'volume': {k: volume['metadata'][k] for k in ('name', 'uid')}}
        class Fake:
            volumes = [volume]
            def command(self, args):
                if 'namespace' in args:
                    return namespace
                if 'pvc' in args:
                    return claim
                if 'pv' in args:
                    return {'items': self.volumes}
                raise AssertionError(args)
        return Fake(), receipt, provider, storage_class, namespace, claim, volume

    def test_live_supported_output_is_bound_to_actual_provider(self):
        kube, receipt, provider, sc, _, _, volume = self.fixture()
        pin = reset.provider_qualification(kube, receipt, provider=provider, storage_class=sc)
        self.assertEqual(pin['contract'], 'local-path-local-uid-v1')
        self.assertEqual(pin['volume_spec_sha256'], reset.digest(volume['spec']))
        self.assertEqual(pin['receipt'], receipt)

    def test_backend_claim_node_path_and_provider_drift_fail(self):
        changes = [lambda r, n, c, v: r['provider'].update(config_uid='different'),
                   lambda r, n, c, v: r['storage_class'].update(uid='different'),
                   lambda r, n, c, v: n['metadata'].update(uid='replacement'),
                   lambda r, n, c, v: c['metadata'].update(uid='replacement'),
                   lambda r, n, c, v: c['metadata']['annotations'].update({'volume.kubernetes.io/selected-node': 'elsewhere'}),
                   lambda r, n, c, v: v['spec']['claimRef'].update(uid='foreign'),
                   lambda r, n, c, v: v['spec'].update(hostPath=v['spec'].pop('local')),
                   lambda r, n, c, v: v['spec']['nodeAffinity']['required']['nodeSelectorTerms'].append({}),
                   lambda r, n, c, v: v['spec']['local'].update(path='/var/lib/rancher/k3s/storage/reused')]
        for change in changes:
            kube, receipt, provider, sc, namespace, claim, volume = self.fixture()
            provider = copy.deepcopy(provider)
            sc = copy.deepcopy(sc)
            change(receipt, namespace, claim, volume)
            with self.subTest(change=changes.index(change)), self.assertRaises(ValueError):
                reset.provider_qualification(kube, receipt, provider=provider, storage_class=sc)
        kube, receipt, provider, sc, _, _, volume = self.fixture()
        kube.volumes = [volume, {'metadata': {'name': 'protected', 'uid': 'protected'},
                                'spec': {'local': {'path': '/var/lib/rancher/k3s/storage'}}}]
        with self.assertRaises(ValueError):
            reset.provider_qualification(kube, receipt, provider=provider, storage_class=sc)
        for suffix in ('', '/child', '/..', '//child'):
            kube, receipt, provider, sc, _, _, volume = self.fixture()
            kube.volumes = [volume, {'metadata': {'name': 'protected', 'uid': 'protected'},
                                    'spec': {'hostPath': {'path': volume['spec']['local']['path'] + suffix}}}]
            with self.subTest(overlap=suffix), self.assertRaises(ValueError):
                reset.provider_qualification(kube, receipt, provider=provider, storage_class=sc)

    def test_missing_qualification_holds_plan_before_any_cluster_submission(self):
        class NoSubmission:
            def command(self, *args, **kwargs):
                raise AssertionError('missing qualification must not submit')
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaisesRegex(ValueError, 'provider qualification'):
                reset.prepare_plan(NoSubmission(), Path(tmp), Path(tmp), namespace_uid='old', staging_uid='stg',
                    actor='operator', claim='claim', budget_seconds=7200, budget_evidence='reviewed fixture budget evidence for provider admission')
            self.assertFalse((Path(tmp) / 'reset-plan.json').exists())

    def test_provider_pin_requires_ready_current_generation_and_node_hostname(self):
        config = {'metadata': {'uid': 'config', 'resourceVersion': '1'}, 'data': {'config.json': json.dumps(
            {'nodePathMap': [{'node': 'DEFAULT_PATH_FOR_NON_LISTED_NODES', 'paths': ['/var/lib/rancher/k3s/storage']}]})}}
        deployment = {'metadata': {'uid': 'deployment', 'generation': 2}, 'spec': {'replicas': 1},
                      'status': {'observedGeneration': 2, 'availableReplicas': 1, 'readyReplicas': 1,
                                 'updatedReplicas': 1, 'replicas': 1}}
        node = {'metadata': {'name': 'pmdebook', 'uid': 'node', 'labels': {'kubernetes.io/hostname': 'pmdebook'}},
                'status': {'conditions': [{'type': 'Ready', 'status': 'True'}]}}
        class Fake:
            def command(self, args):
                return config if 'configmap' in args else deployment if 'deployment' in args else node
        self.assertEqual(reset.provider_pin(Fake())['node_uid'], 'node')
        deployment['status']['observedGeneration'] = 1
        with self.assertRaises(ValueError):
            reset.provider_pin(Fake())
        deployment['status']['observedGeneration'] = 2
        node['metadata']['labels']['kubernetes.io/hostname'] = 'different'
        with self.assertRaises(ValueError):
            reset.provider_pin(Fake())


class NamespaceExecutionTest(unittest.TestCase):
    def test_namespace_drift_refuses_all_mutations(self):
        ns, _, volumes = storage_fixture()
        class Fake:
            def get(self, *args):
                return dict(ns, metadata=dict(ns['metadata'], uid='changed'))
            def pause(self, *args):
                raise AssertionError('must not mutate')
        with tempfile.TemporaryDirectory() as tmp:
            journal = reset.Journal(tmp, {'operation': 'x'})
            with self.assertRaises(ValueError):
                reset.stop_writers(Fake(), {'namespace_uid': 'old-ns', 'writers': []}, journal, check_window=lambda: None)
            self.assertEqual(journal.state['steps'], {})

    def test_completed_pods_do_not_block_and_delete_keeps_retained_volumes(self):
        ns, claims, volumes = storage_fixture()
        ns['metadata']['resourceVersion'] = '1'
        writer = {'kind': 'Deployment', 'metadata': {'name': 'voice-app', 'namespace': dev.NS, 'uid': 'writer', 'resourceVersion': '2'},
                  'spec': {'replicas': 1}}
        class Fake:
            deleted = False
            paused = False
            calls = []
            def get(self, *args):
                return None if self.deleted else ns
            def objects(self, kind):
                if kind == reset.WRITERS:
                    return [dict(writer, spec={'replicas': 0})] if self.paused else [writer]
                if kind == 'pods':
                    return [{'metadata': {'namespace': dev.NS}, 'status': {'phase': 'Succeeded',
                             'containerStatuses': [{'state': {'terminated': {'exitCode': 0}}}]}}]
                return []
            def pause(self, obj):
                self.calls.append(('pause', obj['metadata']['uid']))
                self.paused = True
            def command(self, args):
                self.calls.append(('read', args))
                return {'items': volumes}
            def delete_old(self, *args):
                self.calls.append(('delete', args))
                self.deleted = True
        fake = Fake()
        expected = {'namespace_uid': 'old-ns', 'writers': [writer],
                    'retained': reset.old_storage(ns, claims, volumes, namespace_uid='old-ns')}
        with tempfile.TemporaryDirectory() as tmp:
            journal = reset.Journal(tmp, {'operation': 'x'})
            checks = []
            reset.stop_writers(fake, expected, journal, check_window=lambda: checks.append('check'))
            reset.delete_namespace(fake, expected, journal, check_window=lambda: checks.append('check'))
            self.assertEqual(journal.state['steps']['delete-namespace']['status'], 'complete')
            self.assertEqual(checks, ['check', 'check'])
            self.assertEqual([c for c in fake.calls if c[0] == 'delete'], [('delete', ('Namespace', 'voice-dev', 'old-ns', '1'))])


class TlsPreservationTest(unittest.TestCase):
    def test_exact_bytes_preserved_with_fresh_metadata_and_private_key_file(self):
        old = reset.signal_tls.secret(b'public fixture certificate', b'private fixture key')
        old['metadata'].update(uid='old-secret', resourceVersion='1', ownerReferences=[{'uid': 'old-owner'}])
        checks = []
        def validate(cert, key):
            checks.append((Path(cert).read_bytes(), Path(key).read_bytes()))
        with tempfile.TemporaryDirectory() as tmp:
            fresh = reset.tls_identity(old, tmp, validate=validate)
            self.assertEqual(list(Path(tmp).iterdir()), [])
        self.assertEqual(fresh['data'], old['data'])
        self.assertNotIn('uid', fresh['metadata'])
        self.assertNotIn('ownerReferences', fresh['metadata'])
        self.assertEqual(checks, [(b'public fixture certificate', b'private fixture key')])

    def test_existing_renewed_identity_is_never_overwritten(self):
        old = reset.signal_tls.secret(b'old', b'old-key')
        renewed = reset.signal_tls.secret(b'new', b'new-key')
        class Fake:
            def get(self, *args):
                return renewed
            def call(self, *args, **kwargs):
                raise AssertionError('must not overwrite renewed identity')
        with patch.object(reset, 'tls_identity', side_effect=lambda obj, directory: obj):
            self.assertEqual(reset.restore_tls(Fake(), old, 'private'), renewed)


class ForwardExecutionTest(unittest.TestCase):
    def test_provider_backend_drift_prevents_writer_stop_and_namespace_delete(self):
        self.interrupted_bootstrap_case(provider_drift=True)

    def test_interrupted_bootstrap_resumes_without_redeleting_or_mutating_package(self):
        self.interrupted_bootstrap_case()

    def test_started_bootstrap_without_interruption_receipt_holds_after_supervisor_loss(self):
        self.interrupted_bootstrap_case(lost_supervisor=True)

    def interrupted_bootstrap_case(self, *, lost_supervisor=False, provider_drift=False):
        with tempfile.TemporaryDirectory() as tmp:
            root = Path(tmp) / 'source'
            directory = Path(tmp) / 'private'
            directory.mkdir(mode=0o700)
            source_rows = []
            for name in ('services.yaml', 'infra.yaml', 'configmap-app.yaml'):
                path = root / 'deploy/staging' / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text('source fixture')
                source_rows.append({'path': 'deploy/staging/' + name, 'sha256': reset.hashlib.sha256(path.read_bytes()).hexdigest()})
            image = 'example/tool@sha256:' + 'a' * 64
            tools = {name: image for name in ('node:24-alpine', 'migrate/migrate:v4.18.1', 'flyway/flyway:11.3.0', 'space-native-installer')}
            tls = reset.signal_tls.secret(b'fixture-cert', b'fixture-key')
            objects = [{'kind': 'Deployment', 'metadata': dev.metadata('voice-' + name)}
                       for name in reset.APPS | {'redis', 'nats-pvc-candidate', 'livekit', 'clamav', 'p3-principal-jwks'}]
            objects += [{'kind': 'StatefulSet', 'metadata': dev.metadata('voice-' + name)} for name in ('postgres', 'minio', 'clickhouse')]
            objects += [{'kind': 'Job', 'metadata': dev.metadata('voice-nats-' + role + '-bootstrap')} for role in ('realtime', 'notification', 'analytics-chat', 'search')]
            objects += [{'kind': 'Job', 'metadata': dev.metadata('voice-minio-create-' + bucket + '-bucket')} for bucket in ('avatars', 'files')]
            objects += [{'kind': 'ConfigMap', 'metadata': dev.metadata('voice-app-config')}, {'kind': 'PersistentVolumeClaim', 'metadata': dev.metadata('voice-nats-jsdata')}]
            for obj in objects:
                if obj['kind'] in ('Deployment', 'StatefulSet', 'Job'):
                    obj['spec'] = {'template': {'spec': {'containers': [{'name': 'fixture', 'image': image}],
                                                        'nodeSelector': {'kubernetes.io/hostname': 'pmdebook'}}}}
            values = {'prepared.json': {'items': objects}, 'secrets.json': {'items': [tls]},
                      'foundation.json': {'items': [dev.foundation('pmdebook')[0]]},
                      'package-receipt.json': {'schema': 'voice-dev-reset-package-v1', 'source_sha': 'a' * 40,
                          'storage_class': 'local-path', 'storage_mode': 'fresh-dynamic-no-retained-pv-binding',
                          'node': 'pmdebook', 'sources': source_rows, 'images': [{'reference': image}], 'tool_images': tools}}
            for name, value in values.items():
                reset.write_private(directory / name, value)
            descriptor = reset.os.open(directory / 'source-revision', reset.os.O_WRONLY | reset.os.O_CREAT, 0o600)
            with reset.os.fdopen(descriptor, 'w') as stream:
                stream.write('a' * 40)
            reset.validate_package(directory, root)
            objects[0]['spec']['template']['spec']['nodeSelector'] = {'kubernetes.io/hostname': 'foreign-node'}
            (directory / 'prepared.json').write_text(json.dumps({'items': objects}))
            with self.assertRaisesRegex(ValueError, 'placement'):
                reset.validate_package(directory, root)
            objects[0]['spec']['template']['spec']['nodeSelector'] = {'kubernetes.io/hostname': 'pmdebook'}
            (directory / 'prepared.json').write_text(json.dumps({'items': objects}))
            ns, claims, volumes = storage_fixture()
            ns['metadata']['resourceVersion'] = '1'
            sc = {'metadata': {'name': 'local-path', 'uid': 'class', 'resourceVersion': '1',
                              'annotations': {'defaultVolumeType': 'local'}},
                  'provisioner': 'rancher.io/local-path', 'reclaimPolicy': 'Delete', 'volumeBindingMode': 'WaitForFirstConsumer'}
            probe, qualification, _, _, _, _, probe_volume = ProviderQualificationTest().fixture()
            provider = {'node_uid': 'fixture-node', 'config_uid': 'fixture-provider'}
            qualification['provider'] = provider
            qualification['storage_class'] = reset.storage_class_pin(sc, available_bytes=100 * 1024 ** 3)
            qualification_path = directory / 'provider-qualification.json'
            reset.write_private(qualification_path, qualification)
            volumes.append(probe_volume)
            class Fake:
                current = ns
                deletes = 0
                def get(self, kind, name):
                    return tls if kind == 'secret' else self.current
                def objects(self, kinds):
                    return claims if kinds == 'pvc' else []
                def command(self, args, body=None):
                    if 'voice-dev-provider-probe-fixture' in args:
                        return probe.command(args)
                    if 'voice-staging' in args and 'namespace' in args:
                        return {'metadata': {'uid': 'staging'}}
                    if 'voice-staging' in args:
                        return {'items': []}
                    if 'storageclass' in args:
                        return sc
                    return {'items': volumes}
                def delete_old(self, *args):
                    self.deletes += 1
                    self.current = None
            kube = Fake()
            host = {'fixture': 'quiet-window tested separately'}
            with patch.object(reset, 'observe_host', return_value=host), \
                    patch.object(reset, 'provider_pin', return_value=provider), \
                    patch.object(reset, 'quiet_window', return_value='deadline'), \
                    patch.object(reset, 'shutil_disk_free', return_value=100 * 1024 ** 3), \
                    patch.object(reset, 'tls_identity', side_effect=lambda obj, directory: obj):
                reset.prepare_plan(kube, directory, root, namespace_uid='old-ns', staging_uid='staging', actor='actual-operator',
                    claim='actual-claim', budget_seconds=7200, budget_evidence='Fixture observes bounded phase ordering; this is not a live timing claim.',
                    qualification=qualification_path)
                if provider_drift:
                    probe_volume['spec']['hostPath'] = probe_volume['spec'].pop('local')
                    with patch.object(reset, 'run_bootstrap') as run:
                        with self.assertRaisesRegex(ValueError, 'supported local-path'):
                            reset.execute_plan(kube, directory, root, actor='actual-operator', claim='actual-claim')
                        run.assert_not_called()
                    self.assertEqual(kube.deletes, 0)
                    self.assertEqual(json.loads((directory / 'reset-state.json').read_text())['steps'], {})
                    return
                before = (directory / 'secrets.json').read_bytes()
                def bootstrap(args, **kwargs):
                    # Runtime mutation is legitimate; immutable source bundle remains unchanged.
                    (directory / 'runtime/secrets.json').write_text('{"runtime":"updated"}')
                    kube.current = {'metadata': {'uid': 'fresh-namespace'}}
                    return SimpleNamespace(returncode=1, stdout=b'interrupted', stderr=b'')
                with patch.object(reset, 'run_bootstrap', side_effect=bootstrap):
                    with self.assertRaisesRegex(ValueError, 'forward bootstrap incomplete'):
                        reset.execute_plan(kube, directory, root, actor='actual-operator', claim='actual-claim')
                self.assertEqual(kube.deletes, 1)
                interrupted = json.loads((directory / 'runtime/bootstrap-interrupted-000000.json').read_text())
                self.assertFalse(interrupted['accepted_work_cancelled'])
                if lost_supervisor:
                    # SIGKILL/host loss cannot run the exception handler. Durable
                    # started state is sufficient even with unchanged authority.
                    (directory / 'runtime/bootstrap-interrupted-000000.json').unlink()
                    operation = json.loads((directory / 'reset-plan.json').read_text())['operation']
                    kube.current = {'metadata': {'uid': 'fresh-namespace', 'annotations': {
                        'voice.dev/reset-operation': operation}}}
                    with patch.object(reset, 'run_bootstrap') as run:
                        with self.assertRaisesRegex(ValueError, 'explicit operator recovery'):
                            reset.execute_plan(kube, directory, root, actor='actual-operator', claim='actual-claim')
                        run.assert_not_called()
                kube.current = {'metadata': {'uid': 'replaced-after-interruption'}}
                with patch.object(reset, 'run_bootstrap') as run:
                    with self.assertRaisesRegex(ValueError, 'explicit operator recovery'):
                        reset.execute_plan(kube, directory, root, actor='actual-operator', claim='actual-claim')
                    run.assert_not_called()
                kube.current = {'metadata': {'uid': 'fresh-namespace'}}
                with patch.object(reset, 'run_bootstrap') as run:
                    with self.assertRaisesRegex(ValueError, 'explicit operator recovery'):
                        reset.execute_plan(kube, directory, root, actor='actual-operator', claim='actual-claim')
                    run.assert_not_called()
                self.assertEqual(kube.deletes, 1)
                state = json.loads((directory / 'reset-state.json').read_text())
                self.assertEqual(state['steps']['bootstrap']['status'], 'started')
                self.assertEqual((directory / 'secrets.json').read_bytes(), before)

    @unittest.skipUnless(reset.os.name == 'posix', 'actual executor requires POSIX process groups')
    def test_unsafe_window_stops_owned_bootstrap_process(self):
        checks = []
        def check():
            checks.append(True)
            if len(checks) > 1:
                raise ValueError('writer activated')
        with tempfile.TemporaryDirectory() as tmp:
            with self.assertRaisesRegex(ValueError, 'writer activated'):
                reset.run_bootstrap([reset.sys.executable, '-c', 'import time; time.sleep(30)'],
                    env=dict(reset.os.environ), timeout=10, check_window=check,
                    log=Path(tmp) / 'bootstrap.log', interval=0.01)
        self.assertGreater(len(checks), 1)

    @unittest.skipUnless(reset.os.name == 'posix', 'actual POSIX descendants')
    def test_normal_leader_exit_cannot_leave_term_ignoring_writer(self):
        import time
        with tempfile.TemporaryDirectory() as tmp:
            marker = Path(tmp) / 'late-write'
            child = 'import signal,time,pathlib; signal.signal(signal.SIGTERM,signal.SIG_IGN); time.sleep(.3); pathlib.Path(' + repr(str(marker)) + ').write_text("bad")'
            parent = 'import subprocess,sys,time; subprocess.Popen([sys.executable,"-c",' + repr(child) + ']); time.sleep(.1)'
            unrelated = reset.subprocess.Popen([reset.sys.executable, '-c', 'import time; time.sleep(30)'], start_new_session=True)
            try:
                result = reset.run_bootstrap([reset.sys.executable, '-c', parent], env=dict(reset.os.environ), timeout=5,
                    check_window=lambda: None, log=Path(tmp) / 'log', interval=.01, grace=.05)
                self.assertEqual(result.returncode, 0)
                time.sleep(.35)
                self.assertFalse(marker.exists())
                self.assertIsNone(unrelated.poll())
            finally:
                unrelated.terminate()
                unrelated.wait(timeout=2)

    @unittest.skipUnless(reset.os.name == 'posix', 'actual POSIX supervisor')
    def test_observer_hang_and_absolute_budget_are_bounded(self):
        import time
        for hanging in (True, False):
            checks = []
            def check():
                checks.append(True)
                if hanging and len(checks) > 1:
                    time.sleep(.4)
            with self.subTest(observer_hang=hanging), tempfile.TemporaryDirectory() as tmp:
                started = time.monotonic()
                with self.assertRaisesRegex(ValueError, 'observer timeout|deadline exhausted'):
                    reset.run_bootstrap([reset.sys.executable, '-c', 'import time; time.sleep(30)'],
                        env=dict(reset.os.environ), timeout=.1, check_window=check, log=Path(tmp) / 'log',
                        interval=.01, observer_timeout=.02, grace=.02)
                self.assertLess(time.monotonic() - started, 1)



if __name__ == '__main__':
    unittest.main()
