import copy
import json
import os
import threading
import unittest
from pathlib import Path
from unittest import mock

import frontend_runtime_adoption as adoption
import frontend_runtime_executor as executor
import media_backup_phase_test as fixture
import media_operation_journal as journal
import resource_fencing
import update_images

REAL_TRANSPORT = executor.KubectlTransport

class Cluster:
    def __init__(self, lock):
        self.lock = lock
        self.configs = {}
        self.service_account = {'metadata': {'name': 'default', 'namespace': 'voice-dev', 'uid': 'sa-uid', 'resourceVersion': '7'}}
        self.deployments = {}
        self.rs = []
        self.pods = []
        self.created = 0
        self.patched = 0
        self.lose_create = False
        self.lose_patch = False
        self.wrong_bytes = False
        self.swap_config = False
        self.swap_pod = False
        for name, container in adoption.APPS.values():
            self.deployments[name] = {'kind': 'Deployment', 'metadata': {'name': name, 'namespace': 'voice-dev',
                'uid': name + '-uid', 'resourceVersion': '1', 'generation': 1}, 'spec': {'replicas': 1,
                'template': {'metadata': {'labels': {'app': name}}, 'spec': {'containers': [{'name': container, 'image': 'old'}]}}}}

    def call(self, *args, body=None):
        if args[0] == 'create':
            self.created += 1
            obj = copy.deepcopy(body)
            obj['metadata'].update(uid='cm-uid', resourceVersion='2')
            self.configs[obj['metadata']['name']] = obj
            if self.swap_config:
                self.configs[obj['metadata']['name']] = copy.deepcopy(obj)
                self.configs[obj['metadata']['name']]['metadata']['uid'] = 'replacement'
            if self.lose_create:
                raise OSError('response lost after commit')
            return copy.deepcopy(obj)
        if args[1] == 'namespace':
            return {'metadata': {'name': 'voice-dev', 'uid': 'ns', 'labels': update_images.LABELS}}
        if args[1] == 'replicasets': return {'items': copy.deepcopy(self.rs)}
        if args[1] == 'pods': return {'items': copy.deepcopy(self.pods)}
        if args[1] == 'serviceaccount':
            return copy.deepcopy(self.service_account)
        if args[1] == 'configmap':
            return copy.deepcopy(self.lock if args[2] == update_images.LOCK else self.configs.get(args[2], {}))
        if args[1] == 'deployment': return copy.deepcopy(self.deployments.get(args[2], {}))
        raise AssertionError(args)

    def patch(self, name, patch):
        self.patched += 1
        old = self.deployments[name]
        if (patch[0]['value'] != old['metadata']['uid'] or patch[1]['value'] != old['metadata']['resourceVersion']
                or patch[2]['value'] != old['spec']):
            raise RuntimeError('CAS conflict')
        changed = old['spec'] != patch[3]['value']
        old['spec'] = copy.deepcopy(patch[3]['value'])
        old['metadata']['resourceVersion'] = str(int(old['metadata']['resourceVersion']) + 1)
        old['metadata']['generation'] += int(changed)
        old['status'] = {'observedGeneration': old['metadata']['generation'], 'updatedReplicas': 1, 'availableReplicas': 1}
        template = copy.deepcopy(old['spec']['template'])
        template['metadata']['labels']['pod-template-hash'] = 'abc'
        self.rs = [{'metadata': {'uid': 'rs-new', 'ownerReferences': [{'controller': True, 'kind': 'Deployment', 'uid': old['metadata']['uid']}]},
                    'spec': {'template': template}}]
        self.pods = [{'metadata': {'name': 'new-pod', 'namespace': 'voice-dev', 'uid': 'pod-new', 'resourceVersion': '4',
                     'ownerReferences': [{'controller': True, 'kind': 'ReplicaSet', 'uid': 'rs-new'}]},
                     'spec': copy.deepcopy(old['spec']['template']['spec']),
                     'status': {'containerStatuses': [{'name': old['spec']['template']['spec']['containers'][0]['name'], 'ready': True,
                                'imageID': 'containerd://sha256:' + 'a' * 64}],
                                'conditions': [{'type': 'Ready', 'status': 'True'}]}}]
        if self.lose_patch: raise OSError('response lost after commit')
        return copy.deepcopy(old)

    def read_pod(self, name):
        config = list(self.configs.values())[-1]
        if self.swap_pod:
            self.pods[0]['metadata']['resourceVersion'] = '5'
        return b'bad' if self.wrong_bytes else config['data'][adoption.KEY].encode()


class ExecutorTest(unittest.TestCase):
    def setUp(self):
        fixture.PhaseTest.setUp(self)
        self.context['images'].update({'voice-web/web': 'ghcr.io/x/web@sha256:' + 'a' * 64,
          'voice-admin/admin': 'ghcr.io/x/admin@sha256:' + 'b' * 64,
          'voice-developer-portal/portal': 'ghcr.io/x/portal@sha256:' + 'c' * 64})
        self.fence.context = copy.deepcopy(self.context)
        self.card['candidate']['images'] = copy.deepcopy(self.context['images'])
        self.ledger.row = self.context | {'status': 'active', 'step': 'admitted'}
        self.journal.intent['resource_context'] = copy.deepcopy(self.context)
        self.platform_digests = {key: 'sha256:' + 'a' * 64 for key in ('voice-web/web', 'voice-admin/admin', 'voice-developer-portal/portal')}
        self.journal.intent['frontend_platform_digests'] = copy.deepcopy(self.platform_digests)
        self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
        self.lock['data']['resource_context'] = json.dumps(self.context, sort_keys=True, separators=(',', ':'))
        self.images = {key: self.context['images'][key] for key in ('voice-web/web', 'voice-admin/admin', 'voice-developer-portal/portal')}
        self.cluster = Cluster(self.lock)
        self.journal.intent['frontend_deployment_baseline'] = {
            app: {'uid': self.cluster.deployments[name]['metadata']['uid'],
                  'generation': self.cluster.deployments[name]['metadata']['generation'],
                  'spec': copy.deepcopy(self.cluster.deployments[name]['spec'])}
            for app, (name, _) in adoption.APPS.items()}
        self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
        self.cluster.journal = self.journal
        self.ledger.client = self.cluster
        self.client_type_patch = mock.patch.object(executor, 'KubectlTransport', Cluster)
        self.client_type_patch.start()
        self.addCleanup(self.client_type_patch.stop)
        self.phase = executor.FrontendExecutor(self.journal, self.fence, self.cluster, self.lock, 'ns', self.images, self.platform_digests, self.cluster)

    def test_invalid_app_never_mutates(self):
        with self.assertRaises(executor.RecoveryRequired): self.phase.run('staging')
        self.assertEqual(self.journal.state['steps'], {})
        self.assertEqual(self.cluster.created, 0)

    def test_success_commits_two_receipts_and_checks_served_bytes(self):
        receipt = self.phase.run('web')
        self.assertEqual(receipt['config']['uid'], 'cm-uid')
        self.assertEqual(receipt['pods'][0][1], 'pod-new')
        self.assertEqual(self.ledger.row['frontend_effects']['frontend-web-config']['status'], 'done')
        self.assertEqual(self.ledger.row['frontend_effects']['frontend-web-deployment']['status'], 'done')
        self.assertEqual((self.cluster.created, self.cluster.patched), (1, 1))

    def test_first_adoption_unqualified_template_drift_holds_before_create(self):
        self.cluster.deployments['voice-web']['spec']['template']['spec']['containers'][0]['command'] = ['foreign']
        with self.assertRaises(executor.RecoveryRequired): self.phase.run('web')
        self.assertEqual((self.cluster.created, self.cluster.patched), (0, 0))

    def test_status_only_baseline_rv_uses_fresh_exact_cas(self):
        self.cluster.deployments['voice-web']['metadata']['resourceVersion'] = '9'
        self.phase.run('web')
        self.assertEqual(self.cluster.deployments['voice-web']['metadata']['resourceVersion'], '10')

    def test_previous_effect_removed_blocks_next_app(self):
        self.phase.run('web')
        del self.ledger.row['frontend_effects']['frontend-web-deployment']
        self.ledger.version += 1
        with self.assertRaises(executor.RecoveryRequired): self.phase.run('admin')
        self.assertEqual((self.cluster.created, self.cluster.patched), (1, 1))

    def test_ledger_drift_after_final_guard_prevents_patch(self):
        guard = self.phase._guard
        count = 0
        def drift():
            nonlocal count
            guard()
            pending = self.ledger.row.get('frontend_effects', {}).get('frontend-web-deployment')
            if pending and pending.get('status') == 'pending':
                count += 1
                if count == 2:
                    del self.ledger.row['frontend_effects']['frontend-web-config']
                    self.ledger.version += 1
        with mock.patch.object(self.phase, '_guard', side_effect=drift):
            with self.assertRaises(executor.RecoveryRequired): self.phase.run('web')
        self.assertEqual(self.cluster.patched, 0)

    def test_three_apps_use_one_invocation(self):
        for app in ('web', 'admin', 'portal'):
            self.phase.run(app)
        self.assertEqual(self.cluster.created, 3)
        self.assertEqual(self.cluster.patched, 3)
        self.assertEqual(len(self.ledger.row['frontend_effects']), 6)

    def test_lost_create_marks_uncertain_and_forbids_replay(self):
        self.cluster.lose_create = True
        with self.assertRaises(executor.RecoveryRequired): self.phase.run('web')
        self.assertEqual(self.ledger.row['status'], 'uncertain')
        self.assertIn('voice-web-runtime-config', self.cluster.configs)
        with self.assertRaises(executor.RecoveryRequired): self.phase.run('web')
        self.assertEqual(self.cluster.created, 1)
        self.assertEqual(self.cluster.patched, 0)

    def test_lost_patch_and_wrong_served_bytes_never_mark_done(self):
        for fault in ('lose_patch', 'wrong_bytes'):
            with self.subTest(fault=fault):
                if fault == 'wrong_bytes':
                    self.setUp()
                setattr(self.cluster, fault, True)
                with self.assertRaises(executor.RecoveryRequired): self.phase.run('web')
                self.assertEqual(self.ledger.row['status'], 'uncertain')
                self.assertEqual(self.cluster.patched, 1)
                self.assertEqual(self.ledger.row['frontend_effects']['frontend-web-deployment']['status'], 'pending')

    def test_changed_candidate_refuses_before_effect(self):
        self.images['voice-web/web'] = 'ghcr.io/wrong@sha256:' + 'f' * 64
        with self.assertRaises(executor.RecoveryRequired):
            executor.FrontendExecutor(self.journal, self.fence, self.cluster, self.lock, 'ns', self.images, self.platform_digests, self.cluster)
        self.assertEqual(self.cluster.created, 0)

    def test_single_client_and_bound_platform_digest_required(self):
        with self.assertRaises(executor.RecoveryRequired):
            executor.FrontendExecutor(self.journal, self.fence, self.cluster, self.lock, 'ns', self.images,
                                      self.platform_digests, Cluster(self.lock))
        other = self.platform_digests | {'voice-web/web': 'sha256:' + 'b' * 64}
        with self.assertRaises(executor.RecoveryRequired):
            executor.FrontendExecutor(self.journal, self.fence, self.cluster, self.lock, 'ns', self.images, other, self.cluster)
        self.assertEqual(self.cluster.created, 0)

    def test_explicit_same_nonfixed_transport_is_rejected(self):
        self.client_type_patch.stop()
        with self.assertRaises(executor.RecoveryRequired):
            executor.FrontendExecutor(self.journal, self.fence, self.cluster, self.lock, 'ns', self.images,
                                      self.platform_digests, self.cluster)
        self.client_type_patch.start()

    def test_config_recreation_and_pod_change_cannot_complete(self):
        self.cluster.swap_config = True
        with self.assertRaises(executor.RecoveryRequired): self.phase.run('web')
        self.assertEqual(self.cluster.patched, 0)
        self.setUp()
        self.cluster.swap_pod = True
        with self.assertRaises(executor.RecoveryRequired): self.phase.run('web')
        self.assertEqual(self.ledger.row['frontend_effects']['frontend-web-deployment']['status'], 'pending')

    def test_mutated_pod_image_or_mount_fails_closed(self):
        for field in ('image', 'mount', 'imageID', 'command', 'env', 'extra_mount', 'sidecar', 'dns', 'service_account', 'volume', 'extra_volume'):
            with self.subTest(field=field):
                self.setUp()
                if field == 'volume':
                    pod_spec = self.cluster.deployments['voice-web']['spec']['template']['spec']
                    pod_spec['volumes'] = [{'name': 'existing', 'configMap': {'name': 'expected', 'defaultMode': 420}}]
                    pod_spec['containers'][0]['volumeMounts'] = [{'name': 'existing', 'mountPath': '/etc/existing'}]
                    self.journal.intent['frontend_deployment_baseline']['web']['spec'] = copy.deepcopy(
                        self.cluster.deployments['voice-web']['spec'])
                    self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
                    self.phase.intent = copy.deepcopy(self.journal.intent)
                original = self.cluster.patch
                def patch(name, body):
                    result = original(name, body)
                    pod = self.cluster.pods[0]
                    if field == 'image': pod['spec']['containers'][0]['image'] = 'foreign'
                    if field == 'mount': pod['spec']['containers'][0]['volumeMounts'][-1]['readOnly'] = False
                    if field == 'imageID': pod['status']['containerStatuses'][0]['imageID'] = 'containerd://sha256:' + 'f' * 64
                    if field == 'command': pod['spec']['containers'][0]['command'] = ['hostile']
                    if field == 'env': pod['spec']['containers'][0]['env'] = [{'name': 'X', 'value': 'changed'}]
                    if field == 'extra_mount': pod['spec']['containers'][0]['volumeMounts'].append({'name': 'evil', 'mountPath': '/usr'})
                    if field == 'sidecar': pod['spec']['containers'].append({'name': 'evil'})
                    if field == 'dns': pod['spec']['dnsPolicy'] = 'None'
                    if field == 'service_account': pod['spec']['serviceAccountName'] = 'privileged'
                    if field == 'volume': pod['spec']['volumes'][0]['configMap']['name'] = 'foreign'
                    if field == 'extra_volume': pod['spec']['volumes'].append({'name': 'foreign', 'hostPath': {'path': '/'}})
                    return result
                self.cluster.patch = patch
                with mock.patch.object(executor.time, 'monotonic', side_effect=[0, 0, 181]), \
                     mock.patch.object(executor.time, 'sleep'):
                    with self.assertRaises(executor.RecoveryRequired): self.phase.run('web')
                self.assertEqual(self.ledger.row['frontend_effects']['frontend-web-deployment']['status'], 'pending')

    def test_known_kubernetes_defaulted_pod_is_accepted(self):
        original = self.cluster.patch
        def patch(name, body):
            result = original(name, body)
            pod = self.cluster.pods[0]['spec']
            pod.update(serviceAccountName='default', serviceAccount='default', dnsPolicy='ClusterFirst',
                       restartPolicy='Always', schedulerName='default-scheduler', terminationGracePeriodSeconds=30,
                       enableServiceLinks=True, preemptionPolicy='PreemptLowerPriority', priority=0,
                       securityContext={}, nodeName='node-1', tolerations=[
                           {'key': 'node.kubernetes.io/not-ready', 'operator': 'Exists', 'effect': 'NoExecute', 'tolerationSeconds': 300},
                           {'key': 'node.kubernetes.io/unreachable', 'operator': 'Exists', 'effect': 'NoExecute', 'tolerationSeconds': 300}])
            pod['containers'][0]['volumeMounts'].append({'name': 'kube-api-access-abc',
                'mountPath': '/var/run/secrets/kubernetes.io/serviceaccount', 'readOnly': True})
            pod['volumes'].append({'name': 'kube-api-access-abc', 'projected': {'defaultMode': 420, 'sources': [
                {'serviceAccountToken': {'expirationSeconds': 3607, 'path': 'token'}},
                {'configMap': {'name': 'kube-root-ca.crt', 'items': [{'key': 'ca.crt', 'path': 'ca.crt'}]}},
                {'downwardAPI': {'items': [{'path': 'namespace', 'fieldRef': {'apiVersion': 'v1', 'fieldPath': 'metadata.namespace'}}]}}]}})
            return result
        self.cluster.patch = patch
        self.assertEqual(self.phase.run('web')['pods'][0][1], 'pod-new')

    def test_disabled_automount_refuses_injected_token(self):
        self.cluster.deployments['voice-web']['spec']['template']['spec']['automountServiceAccountToken'] = False
        self.journal.intent['frontend_deployment_baseline']['web']['spec'] = copy.deepcopy(
            self.cluster.deployments['voice-web']['spec'])
        self.journal.state['intent'] = copy.deepcopy(self.journal.intent)
        self.phase.intent = copy.deepcopy(self.journal.intent)
        original = self.cluster.patch
        def patch(name, body):
            result = original(name, body)
            pod = self.cluster.pods[0]['spec']
            pod['containers'][0]['volumeMounts'].append({'name': 'kube-api-access-abc',
                'mountPath': '/var/run/secrets/kubernetes.io/serviceaccount', 'readOnly': True})
            pod['volumes'].append({'name': 'kube-api-access-abc', 'projected': {'defaultMode': 420, 'sources': [
                {'serviceAccountToken': {'expirationSeconds': 3607, 'path': 'token'}},
                {'configMap': {'name': 'kube-root-ca.crt', 'items': [{'key': 'ca.crt', 'path': 'ca.crt'}]}},
                {'downwardAPI': {'items': [{'path': 'namespace', 'fieldRef': {'apiVersion': 'v1', 'fieldPath': 'metadata.namespace'}}]}}]}})
            return result
        self.cluster.patch = patch
        with mock.patch.object(executor.time, 'monotonic', side_effect=[0, 0, 181]), mock.patch.object(executor.time, 'sleep'):
            with self.assertRaises(executor.RecoveryRequired): self.phase.run('web')
        self.assertEqual(self.ledger.row['frontend_effects']['frontend-web-deployment']['status'], 'pending')

    def test_service_account_change_across_patch_cannot_complete(self):
        original = self.cluster.patch
        def patch(name, body):
            result = original(name, body)
            self.cluster.service_account['metadata']['resourceVersion'] = '8'
            return result
        self.cluster.patch = patch
        with self.assertRaises(executor.RecoveryRequired): self.phase.run('web')
        self.assertEqual(self.ledger.row['frontend_effects']['frontend-web-deployment']['status'], 'pending')

    def test_real_linux_transport_uses_private_file_and_fixed_context(self):
        script = Path(self.temp.name) / 'kubectl'
        script.write_text('#!/usr/bin/env python3\n'
            'import json, os, stat, sys\n'
            'a=sys.argv[1:]\n'
            'assert a[:6]==["--kubeconfig","/home/pmd/.kube/config","--context","default","--namespace","voice-dev"]\n'
            'assert not any("private-marker" in x for x in a)\n'
            'if "patch" in a:\n'
            ' p=next(x.split("=",1)[1] for x in a if x.startswith("--patch-file="))\n'
            ' assert stat.S_IMODE(os.stat(p).st_mode)==0o600\n'
            ' assert json.load(open(p))[0]["value"]=="private-marker"\n'
            ' print("{}")\n'
            'else: sys.stdout.buffer.write(b"public-config\\n")\n', encoding='utf-8')
        script.chmod(0o700)
        transport = REAL_TRANSPORT(self.journal)
        with mock.patch.dict(os.environ, {'PATH': str(self.temp.name) + os.pathsep + os.environ['PATH']}):
            self.assertEqual(transport.patch('voice-web', [{'op': 'test', 'value': 'private-marker'}]), {})
            self.assertEqual(transport.read_pod('new-pod'), b'public-config\n')
        self.assertEqual(set(os.listdir(self.journal.dirfd)), {'lock', 'state.json'})

    def test_two_journals_compete_for_one_invocation(self):
        other = journal.Journal(Path(self.temp.name) / 'other', self.journal.intent)
        self.addCleanup(other.close)
        second_client = copy.copy(self.cluster)
        second_client.journal = other
        class LedgerView:
            client = second_client
            def read(_): return self.ledger.read()
            def write(_, revision, row): return self.ledger.write(revision, row)
        other_fence = resource_fencing.Fence(self.context, lambda: copy.deepcopy(self.card), LedgerView())
        second = executor.FrontendExecutor(other, other_fence, second_client, self.lock, 'ns', self.images, self.platform_digests, second_client)
        barrier = threading.Barrier(2, timeout=5)
        write = self.ledger.write
        def race(revision, row):
            if row.get('frontend_invocation') and not row.get('frontend_effects'):
                barrier.wait()
            write(revision, row)
        outcomes = []
        def run(phase):
            try:
                phase.run('web')
                outcomes.append('done')
            except executor.RecoveryRequired: outcomes.append('refused')
        with mock.patch.object(self.ledger, 'write', side_effect=race):
            threads = [threading.Thread(target=run, args=(p,)) for p in (self.phase, second)]
            for thread in threads: thread.start()
            for thread in threads: thread.join(10)
        self.assertTrue(all(not t.is_alive() for t in threads))
        self.assertEqual(len(outcomes), 2)
        self.assertLessEqual(self.cluster.created + second_client.created, 1)


if __name__ == '__main__': unittest.main()
