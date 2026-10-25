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
            fresh.append({'metadata': {'name': claim, 'namespace': dev.NS, 'uid': uid},
                          'spec': {'storageClassName': 'local-path', 'volumeName': 'pvc-' + uid},
                          'status': {'phase': 'Bound'}})
            volumes.append({'metadata': {'name': 'pvc-' + uid, 'uid': 'new-volume-' + name},
                            'spec': {'claimRef': {'namespace': dev.NS, 'name': claim, 'uid': uid},
                                     'storageClassName': 'local-path', 'persistentVolumeReclaimPolicy': 'Delete',
                                     'local': {'path': '/var/lib/rancher/k3s/storage/pvc-' + uid + '_' + dev.NS + '_' + claim}}})
        reset.verify_fresh(ns, fresh, volumes, old_namespace_uid='old-ns', retained=retained)
        for change in (lambda: fresh[0]['spec'].update(volumeName='voice-dev-postgres'),
                       lambda: fresh[0]['spec'].update(dataSource={'name': 'old-snapshot'}),
                       lambda: volumes[0]['spec'].update(claimRef={})):
            before_claims, before_volumes = copy.deepcopy(fresh), copy.deepcopy(volumes)
            change()
            with self.assertRaises(ValueError):
                reset.verify_fresh(ns, fresh, volumes, old_namespace_uid='old-ns', retained=retained)
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
        sc = {'metadata': {'name': 'local-path', 'uid': 'class', 'resourceVersion': '1'},
              'provisioner': 'rancher.io/local-path', 'reclaimPolicy': 'Delete',
              'volumeBindingMode': 'WaitForFirstConsumer'}
        self.assertEqual(reset.storage_class_pin(sc, available_bytes=100 * 1024 ** 3)['uid'], 'class')
        with self.assertRaises(ValueError):
            reset.storage_class_pin(sc, available_bytes=1024)
        sc['provisioner'] = 'foreign'
        with self.assertRaises(ValueError):
            reset.storage_class_pin(sc, available_bytes=100 * 1024 ** 3)


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
    def test_interrupted_bootstrap_resumes_without_redeleting_or_mutating_package(self):
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
            sc = {'metadata': {'name': 'local-path', 'uid': 'class', 'resourceVersion': '1'},
                  'provisioner': 'rancher.io/local-path', 'reclaimPolicy': 'Delete', 'volumeBindingMode': 'WaitForFirstConsumer'}
            class Fake:
                current = ns
                deletes = 0
                def get(self, kind, name):
                    return tls if kind == 'secret' else self.current
                def objects(self, kinds):
                    return claims if kinds == 'pvc' else []
                def command(self, args, body=None):
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
                    patch.object(reset, 'provider_pin', return_value={'node_uid': 'fixture-node', 'config_uid': 'fixture-provider'}), \
                    patch.object(reset, 'quiet_window', return_value='deadline'), \
                    patch.object(reset, 'shutil_disk_free', return_value=100 * 1024 ** 3), \
                    patch.object(reset, 'tls_identity', side_effect=lambda obj, directory: obj):
                reset.prepare_plan(kube, directory, root, namespace_uid='old-ns', staging_uid='staging', actor='actual-operator',
                    claim='actual-claim', budget_seconds=7200, budget_evidence='Fixture observes bounded phase ordering; this is not a live timing claim.')
                before = (directory / 'secrets.json').read_bytes()
                def bootstrap(args, **kwargs):
                    # Runtime mutation is legitimate; immutable source bundle remains unchanged.
                    (directory / 'runtime/secrets.json').write_text('{"runtime":"updated"}')
                    kube.current = {'metadata': {'uid': 'fresh-namespace'}}
                    return SimpleNamespace(returncode=1, stdout=b'interrupted', stderr=b'')
                with patch.object(reset.subprocess, 'run', side_effect=bootstrap):
                    with self.assertRaisesRegex(ValueError, 'forward bootstrap incomplete'):
                        reset.execute_plan(kube, directory, root, actor='actual-operator', claim='actual-claim')
                self.assertEqual(kube.deletes, 1)
                with patch.object(reset.subprocess, 'run', return_value=SimpleNamespace(returncode=0, stdout=b'pass', stderr=b'')) as run:
                    state = reset.execute_plan(kube, directory, root, actor='actual-operator', claim='actual-claim')
                    self.assertEqual(run.call_count, 1)
                self.assertEqual(kube.deletes, 1)
                self.assertEqual(state['steps']['bootstrap']['status'], 'complete')
                self.assertEqual((directory / 'secrets.json').read_bytes(), before)



if __name__ == '__main__':
    unittest.main()
