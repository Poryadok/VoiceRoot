import copy
import hashlib
import json
import unittest
from unittest import mock

import media_live_baseline as collector


def meta(name, uid=None, owner=None):
    value = {'name': name, 'uid': uid or name + '-uid', 'resourceVersion': '1'}
    if owner:
        value['ownerReferences'] = [{'kind': owner[0], 'name': owner[1], 'uid': owner[2], 'controller': True}]
    return value


def fixture():
    images = 'ghcr.io/test/image@sha256:' + 'a' * 64
    workloads = []
    for i in range(29):
        name = 'voice-svc-' + str(i)
        containers = [{'name': 'main', 'image': images}]
        if i < 18:
            containers.append({'name': 'side', 'image': images})
        workloads.append({'kind': 'Deployment', 'metadata': meta(name),
                          'spec': {'template': {'spec': {'containers': containers}}}})
    workloads.append({'kind': 'Deployment', 'metadata': meta('voice-nats-pvc-candidate'),
                      'spec': {'template': {'spec': {'containers': [{'name': 'nats', 'image': images}],
                      'initContainers': [{'name': 'nats-config-renderer', 'image': images}]}}}})
    workloads.append({'kind': 'Deployment', 'metadata': meta('voice-dev-mail'),
                      'spec': {'template': {'spec': {'containers': [{'name': 'mail', 'image': 'node:24-alpine'}]}}}})
    deployment = workloads[-1]
    rs = {'kind': 'ReplicaSet', 'metadata': meta('mail-rs', owner=('Deployment', 'voice-dev-mail', deployment['metadata']['uid']))}
    pod = {'kind': 'Pod', 'metadata': meta('mail-pod', owner=('ReplicaSet', 'mail-rs', rs['metadata']['uid'])),
           'spec': {'nodeName': 'node-a'}, 'status': {'phase': 'Running', 'containerStatuses': [
               {'name': 'mail', 'ready': True, 'started': True, 'restartCount': 0,
                'containerID': 'containerd://' + 'b' * 64,
                'imageID': 'docker.io/library/node@sha256:' + 'c' * 64}]}}
    pvcs = [{'kind': 'PersistentVolumeClaim', 'metadata': meta(n), 'status': {'phase': 'Bound'}}
            for n in ('pgdata-voice-postgres-0', 'data-voice-minio-0', 'chdata-voice-clickhouse-0', 'voice-nats-jsdata')]
    def listed(items): return {'metadata': {'resourceVersion': '100'}, 'items': items}
    return {'namespace': {'kind': 'Namespace', 'metadata': meta('voice-dev')},
            'deployments': listed(workloads), 'statefulsets': listed([]), 'pods': listed([pod]),
            'replicasets': listed([rs]), 'pvcs': listed(pvcs), 'jobs': listed([])}


class Reader:
    def __init__(self, rounds):
        self.rounds = rounds
        self.calls = 0

    def __call__(self, context, namespace, resource):
        n = self.calls // 7
        self.calls += 1
        return copy.deepcopy(self.rounds[min(n, len(self.rounds) - 1)][resource])


class BaselineTest(unittest.TestCase):
    def test_raw_list_reader_uses_explicit_context_and_never_reads_secret(self):
        response = mock.Mock(stdout=json.dumps({'kind': 'DeploymentList', 'metadata': {'resourceVersion': '19'}, 'items': [{'metadata': meta('voice-a')}]}).encode())
        with mock.patch.object(collector.subprocess, 'run', return_value=response) as run:
            data = collector._read('ctx', 'voice-dev', 'deployments')
        args = run.call_args.args[0]
        self.assertEqual(args[:4], ['kubectl', '--context', 'ctx', 'get'])
        self.assertEqual(args[4:], ['--raw', '/apis/apps/v1/namespaces/voice-dev/deployments?limit=500'])
        self.assertNotIn('secret', ' '.join(args).lower())
        self.assertEqual(data['items'][0]['kind'], 'Deployment')

    def test_complete_safe_snapshot_and_spec_hash(self):
        data = fixture()
        result = collector.collect('ctx', 'voice-dev', reader=Reader([data, data]))
        self.assertEqual(set(result), {'namespace_uid', 'images', 'init_images', 'pvc_uids', 'workload_uids', 'workload_spec_sha256', 'mail_runtime'})
        self.assertEqual(len(result['images']), 49)
        self.assertEqual(result['mail_runtime']['pod_count'], 1)
        spec = data['deployments']['items'][0]['spec']
        digest = hashlib.sha256(json.dumps(spec, sort_keys=True, separators=(',', ':')).encode()).hexdigest()
        self.assertEqual(result['workload_spec_sha256']['Deployment/voice-svc-0'], digest)
        self.assertNotIn('template', json.dumps(result))

    def test_mutation_retries_then_fails_closed(self):
        a = fixture(); b = copy.deepcopy(a)
        b['deployments']['items'][0]['metadata']['resourceVersion'] = '2'
        with self.assertRaises(ValueError):
            collector.collect('ctx', 'voice-dev', retries=2, reader=Reader([a, b, a, b]))

    def test_bad_mail_and_duplicate_inventory_fail(self):
        for mutate in (
            lambda x: x['pods']['items'][0]['metadata']['ownerReferences'][0].update(uid='stale'),
            lambda x: x['pods']['items'].append(copy.deepcopy(x['pods']['items'][0])),
            lambda x: x['pods']['items'][0]['status']['containerStatuses'][0].update(ready=False),
            lambda x: x['pods']['items'][0]['status']['containerStatuses'][0].update(imageID='wrong'),
            lambda x: x['deployments']['items'][-1]['spec']['template']['spec']['containers'][0].update(image='node:25-alpine'),
            lambda x: x['pvcs']['items'][0]['status'].update(phase='Pending'),
            lambda x: x['namespace']['metadata'].update(deletionTimestamp='2026-01-01T00:00:00Z'),
        ):
            data = fixture(); mutate(data)
            with self.subTest(mutate=mutate), self.assertRaises(ValueError):
                collector.collect('ctx', 'voice-dev', reader=Reader([data, data]))

    def test_unstable_pod_set_and_unknown_init_are_not_hidden(self):
        a = fixture(); b = copy.deepcopy(a)
        b['pods']['items'][0]['metadata']['uid'] = 'different'
        with self.assertRaises(ValueError):
            collector.collect('ctx', 'voice-dev', retries=1, reader=Reader([a, b]))
        a['deployments']['items'][0]['spec']['template']['spec']['initContainers'] = [{'name': 'surprise', 'image': 'example@sha256:' + 'd' * 64}]
        result = collector.collect('ctx', 'voice-dev', reader=Reader([a, a]))
        self.assertIn('voice-svc-0/surprise', result['init_images'])

    def test_list_version_is_required_but_global_churn_is_allowed(self):
        a = fixture(); b = copy.deepcopy(a)
        b['pods']['metadata']['resourceVersion'] = '101'
        collector.collect('ctx', 'voice-dev', retries=1, reader=Reader([a, b]))
        b['pods']['metadata']['resourceVersion'] = ''
        with self.assertRaises(ValueError): collector.collect('ctx', 'voice-dev', retries=1, reader=Reader([a, b]))
        a['pods']['metadata']['continue'] = 'next'
        with self.assertRaises(ValueError): collector.collect('ctx', 'voice-dev', retries=1, reader=Reader([a, a]))

    def test_terminal_job_is_counted_but_active_or_unknown_fails(self):
        data = fixture()
        job = {'kind': 'Job', 'metadata': meta('old-job'), 'status': {'conditions': [{'type': 'Complete', 'status': 'True'}]}}
        pod = {'kind': 'Pod', 'metadata': meta('old-job-pod', owner=('Job', 'old-job', job['metadata']['uid'])), 'status': {'phase': 'Succeeded'}}
        data['jobs']['items'].append(job); data['pods']['items'].append(pod)
        collector.collect('ctx', 'voice-dev', reader=Reader([data, data]))
        job['status']['active'] = 1
        with self.assertRaises(ValueError): collector.collect('ctx', 'voice-dev', reader=Reader([data, data]))
        job['status']['active'] = 0; pod['metadata']['ownerReferences'][0]['uid'] = 'stale'
        with self.assertRaises(ValueError): collector.collect('ctx', 'voice-dev', reader=Reader([data, data]))

if __name__ == '__main__':
    unittest.main()
