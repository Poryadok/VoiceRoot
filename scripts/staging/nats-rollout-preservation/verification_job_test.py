"""Tests for the source-owned, tokenless native phase Job ABI."""
import copy
import unittest

import verification_job as jobs


REQUEST = {'operation': 'a' * 12, 'nonce': 'b' * 32, 'phase': 'census'}
BINDING = {
    'source_claim': 'voice-nats-jsdata-d20261017a',
    'source_uid': '11111111-1111-4111-8111-111111111111',
    'source_pv_name': 'pv-voice-nats-source',
    'source_pv_uid': '55555555-5555-4555-8555-555555555555',
    'source_node_name': 'k3s-worker-1',
    'verification_claim': 'voice-nats-verify-' + 'a' * 12,
    'verification_uid': '22222222-2222-4222-8222-222222222222',
    'verification_pv_name': 'pv-voice-nats-verify',
    'verification_pv_uid': '33333333-3333-4333-8333-333333333333',
    'verification_node_name': 'k3s-worker-1',
}
IMAGE = 'ghcr.io/poryadok/voice-nats-phase@sha256:' + 'a' * 64
POD_UID = '44444444-4444-4444-8444-444444444444'
ARCHIVE_REQUEST = REQUEST | {'phase': 'archive'}
ARCHIVE_BINDING = {key: BINDING[key] for key in (
    'source_claim', 'source_uid', 'source_pv_name', 'source_pv_uid', 'source_node_name')}


class VerificationJobTests(unittest.TestCase):
    def test_job_readback_allows_only_kubernetes_defaults_and_exact_uid(self):
        wanted = jobs.document(ARCHIVE_REQUEST, ARCHIVE_BINDING, IMAGE)
        actual = copy.deepcopy(wanted)
        uid = '77777777-7777-4777-8777-777777777777'
        actual['metadata'].update(uid=uid, resourceVersion='19', generation=1,
            creationTimestamp='2026-10-17T00:00:00Z', managedFields=[])
        actual['spec'].update(parallelism=1, completions=1, completionMode='NonIndexed',
            suspend=False, podReplacementPolicy='TerminatingOrFailed')
        pod_spec = actual['spec']['template']['spec']
        pod_spec.update(dnsPolicy='ClusterFirst', schedulerName='default-scheduler',
            terminationGracePeriodSeconds=30, enableServiceLinks=True,
            preemptionPolicy='PreemptLowerPriority', priority=0)
        container = pod_spec['containers'][0]
        container.update(terminationMessagePath='/dev/termination-log', terminationMessagePolicy='File')
        actual['status'] = {'active': 1}
        self.assertEqual(jobs.verify_job(actual, wanted, uid), actual)
        with self.assertRaisesRegex(jobs.VerificationJobError, 'api_job_uid'):
            jobs.verify_job(actual, wanted, '88888888-8888-4888-8888-888888888888')
        extra = copy.deepcopy(actual)
        extra['spec']['template']['spec']['containers'].append({'name': 'injected'})
        with self.assertRaisesRegex(jobs.VerificationJobError, 'api_object_array'):
            jobs.verify_job(extra, wanted, uid)

    def test_pod_readback_binds_owner_template_and_affinity_node(self):
        job = jobs.document(ARCHIVE_REQUEST, ARCHIVE_BINDING, IMAGE)
        job_uid = '77777777-7777-4777-8777-777777777777'
        pod_uid = '88888888-8888-4888-8888-888888888888'
        template = job['spec']['template']
        name = job['metadata']['name'] + '-abcde'
        actual = {'apiVersion': 'v1', 'kind': 'Pod',
            'metadata': copy.deepcopy(template['metadata']), 'spec': copy.deepcopy(template['spec'])}
        actual['metadata'].update(name=name, namespace=jobs.NAMESPACE, uid=pod_uid,
            resourceVersion='20', creationTimestamp='2026-10-17T00:00:00Z', managedFields=[],
            ownerReferences=[{'apiVersion': 'batch/v1', 'kind': 'Job',
                'name': job['metadata']['name'], 'uid': job_uid, 'controller': True,
                'blockOwnerDeletion': True}])
        actual['metadata']['labels'].update({
            'batch.kubernetes.io/controller-uid': job_uid,
            'batch.kubernetes.io/job-name': job['metadata']['name']})
        actual['spec'].update(nodeName=ARCHIVE_BINDING['source_node_name'], dnsPolicy='ClusterFirst',
            schedulerName='default-scheduler', terminationGracePeriodSeconds=30,
            enableServiceLinks=True, preemptionPolicy='PreemptLowerPriority', priority=0)
        actual['spec']['containers'][0].update(terminationMessagePath='/dev/termination-log',
            terminationMessagePolicy='File')
        self.assertEqual(jobs.verify_pod(actual, template, job_uid, 'archive',
            expected_node_name=ARCHIVE_BINDING['source_node_name'], expected_uid=pod_uid), actual)
        wrong_node = copy.deepcopy(actual)
        wrong_node['spec']['nodeName'] = 'other-node'
        with self.assertRaisesRegex(jobs.VerificationJobError, 'api_pod_node'):
            jobs.verify_pod(wrong_node, template, job_uid, 'archive',
                expected_node_name=ARCHIVE_BINDING['source_node_name'])
        injected = copy.deepcopy(actual)
        injected['metadata']['ownerReferences'][0]['uid'] = pod_uid
        with self.assertRaisesRegex(jobs.VerificationJobError, 'api_pod_owner'):
            jobs.verify_pod(injected, template, job_uid, 'archive',
                expected_node_name=ARCHIVE_BINDING['source_node_name'])
        injected = copy.deepcopy(actual)
        injected['spec']['containers'][0]['env'].append({'name': 'EXTRA', 'value': '1'})
        with self.assertRaises(jobs.VerificationJobError):
            jobs.verify_pod(injected, template, job_uid, 'archive',
                expected_node_name=ARCHIVE_BINDING['source_node_name'])

    def test_archive_job_is_source_only_readonly_and_node_affinity_bound(self):
        row = jobs.document(ARCHIVE_REQUEST, ARCHIVE_BINDING, IMAGE)
        pod = row['spec']['template']['spec']
        container = pod['containers'][0]
        self.assertEqual(row['apiVersion'], 'batch/v1')
        self.assertEqual(row['kind'], 'Job')
        self.assertEqual(row['metadata']['namespace'], 'voice-staging')
        pod_labels = row['spec']['template']['metadata']['labels']
        selector_labels = row['spec']['selector']['matchLabels']
        self.assertTrue(all(pod_labels.get(key) == value for key, value in selector_labels.items()))
        self.assertEqual(selector_labels, {jobs.LABEL_JOB_SELECTOR: row['metadata']['name']})
        self.assertEqual(pod_labels[jobs.LABEL_JOB_SELECTOR], selector_labels[jobs.LABEL_JOB_SELECTOR])
        self.assertEqual(pod['automountServiceAccountToken'], False)
        self.assertEqual(pod['securityContext'], {
            'runAsNonRoot': True, 'runAsUser': 10000, 'runAsGroup': 10000,
            'seccompProfile': {'type': 'RuntimeDefault'}})
        self.assertNotIn('fsGroup', pod['securityContext'])
        self.assertEqual(container['image'], IMAGE)
        self.assertEqual(container['args'], ['--serve', '--operation', ARCHIVE_REQUEST['operation'],
            '--nonce', ARCHIVE_REQUEST['nonce'], '--source-pvc-uid', ARCHIVE_BINDING['source_uid'],
            '--source-pv-uid', ARCHIVE_BINDING['source_pv_uid']])
        self.assertEqual([mount['mountPath'] for mount in container['volumeMounts']], ['/data', '/out'])
        source_mount = next(m for m in container['volumeMounts'] if m['name'] == 'source')
        self.assertTrue(source_mount['readOnly'])
        volumes = {volume['name']: volume for volume in pod['volumes']}
        self.assertEqual(volumes['source']['persistentVolumeClaim'], {
            'claimName': ARCHIVE_BINDING['source_claim'], 'readOnly': True})
        self.assertEqual(set(volumes), {'source', 'output'})
        self.assertNotIn('nodeName', pod)
        self.assertEqual(pod['affinity']['nodeAffinity']['requiredDuringSchedulingIgnoredDuringExecution']
            ['nodeSelectorTerms'][0]['matchExpressions'], [{'key': 'kubernetes.io/hostname',
                'operator': 'In', 'values': [ARCHIVE_BINDING['source_node_name']]}])
        self.assertNotIn('hostPath', str(pod))
        self.assertNotIn('secret', str(pod).lower())

    def test_request_cannot_select_image_command_or_another_phase(self):
        for value in (
            REQUEST | {'command': ['/bin/sh', '-c', 'id']},
            REQUEST | {'image': 'attacker.invalid/image:latest'},
            REQUEST | {'extra': 'field'},
        ):
            with self.subTest(value=value), self.assertRaises(jobs.VerificationJobError):
                jobs.validate_request(value)
        for image in ('candidate:latest', IMAGE.replace('a' * 64, 'z' * 64)):
            with self.subTest(image=image), self.assertRaises(jobs.VerificationJobError):
                jobs.document(ARCHIVE_REQUEST, ARCHIVE_BINDING, image)
        census = jobs.document(REQUEST, BINDING, IMAGE)
        pod = census['spec']['template']['spec']
        self.assertEqual(census['metadata']['labels'][jobs.LABEL_PHASE], 'census')
        self.assertEqual(pod['securityContext'], {
            'runAsNonRoot': True, 'runAsUser': 65532, 'runAsGroup': 65532,
            'fsGroup': 65532, 'fsGroupChangePolicy': 'OnRootMismatch',
            'seccompProfile': {'type': 'RuntimeDefault'}})
        self.assertEqual({m['mountPath']: m for m in pod['containers'][0]['volumeMounts']}, {
            '/verify': {'name': 'target', 'mountPath': '/verify'},
            '/out': {'name': 'output', 'mountPath': '/out'},
            '/inputs': {'name': 'inputs', 'mountPath': '/inputs'}})
        self.assertEqual({v['name']: v for v in pod['volumes']}, {
            'target': {'name': 'target', 'persistentVolumeClaim': {
                'claimName': BINDING['verification_claim']}},
            'output': {'name': 'output', 'emptyDir': {'sizeLimit': '49Gi'}},
            'inputs': {'name': 'inputs', 'emptyDir': {'sizeLimit': '16Mi'}}})
        self.assertNotIn('source', {volume['name'] for volume in pod['volumes']})
        self.assertNotIn('source', {mount['name'] for mount in pod['containers'][0]['volumeMounts']})
        self.assertNotIn('nodeName', pod)

    def test_census_vectors_are_fixed_target_only_upload_then_same_pod_readback(self):
        vectors = jobs.exec_vectors(REQUEST, POD_UID, BINDING,
            {'sha256': 'd' * 64, 'bytes': 1234})
        upload = vectors['census_receive_archive']
        self.assertEqual(upload[:3], ['/usr/bin/python3', '/voice-nats-runner/phase', '--receive-archive'])
        for pair in (('--source-pvc-uid', BINDING['source_uid']),
            ('--source-pv-uid', BINDING['source_pv_uid']),
            ('--verification-pvc-uid', BINDING['verification_uid']),
            ('--verification-pv-uid', BINDING['verification_pv_uid'])):
            index = upload.index(pair[0])
            self.assertEqual(upload[index + 1], pair[1])
        self.assertEqual(upload[upload.index('--archive-sha256') + 1], 'd' * 64)
        self.assertEqual(upload[upload.index('--archive-bytes') + 1], '1234')
        self.assertEqual(vectors['census_same_pod_readback'][2], '--read-receipt')
        self.assertEqual(vectors['census_load_inputs'][2], '--load-inputs')
        self.assertEqual(vectors['census_load_inputs'][3:9], ['--operation', REQUEST['operation'],
            '--nonce', REQUEST['nonce'], '--pod-uid', POD_UID])
        self.assertEqual(vectors['census_read_input_receipt'][2], '--read-input-receipt')
        self.assertIn('--phase', vectors['census_same_pod_readback'])
        self.assertEqual(vectors['census_same_pod_readback'][-1], 'census')
        self.assertEqual(set(vectors), {'census_load_inputs', 'census_read_input_receipt', 'census_receive_archive',
            'census_same_pod_readback'})
        with self.assertRaises(jobs.VerificationJobError):
            jobs.exec_vectors(REQUEST, POD_UID, BINDING)

    def test_promotable_restore_is_target_only_hub_owned_and_attempt_bound(self):
        request = REQUEST | {'phase': 'restore'}
        target = {'restore_claim': 'voice-nats-restore-' + request['operation'] + '-1234abcd',
            'restore_attempt': '1234abcd', 'restore_uid': '66666666-6666-4666-8666-666666666666',
            'restore_pv_name': 'pv-voice-nats-restore',
            'restore_pv_uid': '77777777-7777-4777-8777-777777777777',
            'restore_node_name': 'k3s-worker-1'}
        row = jobs.document(request, target, IMAGE)
        pod = row['spec']['template']['spec']
        container = pod['containers'][0]
        labels = row['spec']['template']['metadata']['labels']
        self.assertEqual(row['metadata']['labels'][jobs.LABEL_PHASE], 'restore')
        self.assertEqual(row['spec']['selector']['matchLabels'], {jobs.LABEL_JOB_SELECTOR: row['metadata']['name']})
        self.assertEqual(labels[jobs.LABEL_JOB_SELECTOR], row['metadata']['name'])
        self.assertEqual(pod['securityContext'], {'runAsNonRoot': True, 'runAsUser': 10000,
            'runAsGroup': 10000, 'fsGroup': 10000, 'fsGroupChangePolicy': 'OnRootMismatch',
            'seccompProfile': {'type': 'RuntimeDefault'}})
        self.assertEqual(container['args'], ['--serve-restore', '--operation', request['operation'],
            '--nonce', request['nonce'], '--restore-attempt', target['restore_attempt'],
            '--restore-pvc-uid', target['restore_uid'], '--restore-pv-uid', target['restore_pv_uid']])
        self.assertEqual(container['volumeMounts'], [
            {'name': 'target', 'mountPath': '/restore'}, {'name': 'output', 'mountPath': '/out'}])
        self.assertEqual({volume['name']: volume for volume in pod['volumes']}, {
            'target': {'name': 'target', 'persistentVolumeClaim': {'claimName': target['restore_claim']}},
            'output': {'name': 'output', 'emptyDir': {'sizeLimit': '49Gi'}}})
        self.assertNotIn('source', {volume['name'] for volume in pod['volumes']})
        vectors = jobs.exec_vectors(request, POD_UID, target, {'sha256': 'd' * 64, 'bytes': 1234})
        self.assertEqual(vectors['restore_archive_once'][:3],
            ['/usr/bin/python3', '/voice-nats-runner/phase', '--restore-archive'])
        self.assertEqual(vectors['restore_archive_once'][-4:], ['--archive-sha256', 'd' * 64,
            '--archive-bytes', '1234'])
        self.assertEqual(vectors['restore_same_pod_readback'][2], '--restore-readback')
        self.assertNotIn('source-pvc-uid', ' '.join(vectors['restore_archive_once']))
        for invalid in (target | {'restore_attempt': '1234abcdx'},
            target | {'restore_claim': 'voice-nats-restore-' + request['operation'] + '-deadbeef'},
            target | {'restore_uid': '../uid'}, target | {'restore_node_name': '../node'}):
            with self.subTest(invalid=invalid), self.assertRaises(jobs.VerificationJobError):
                jobs.document(request, invalid, IMAGE)

    def test_source_and_verification_claim_identity_must_be_distinct_and_valid(self):
        for mutation in (
            {'source_claim': 'untrusted'},
            {'source_claim': 'voice-nats-jsdata-d20261017a/extra'},
            {'source_uid': '../admin'},
            {'source_pv_uid': '../pv'},
            {'source_pv_name': '../pv'},
            {'source_node_name': '../node'},
        ):
            value = ARCHIVE_BINDING | mutation
            with self.subTest(mutation=mutation), self.assertRaises(jobs.VerificationJobError):
                jobs.document(ARCHIVE_REQUEST, value, IMAGE)

    def test_exec_argv_is_fixed_and_binds_live_pod_and_both_pvc_uids(self):
        archive = jobs.exec_vectors(ARCHIVE_REQUEST, POD_UID, ARCHIVE_BINDING)
        self.assertEqual(archive['archive_create_and_stream'][2], '--create-and-stream')
        self.assertEqual(archive['archive_create_and_stream'][-4:], [
            '--source-pvc-uid', ARCHIVE_BINDING['source_uid'], '--source-pv-uid', ARCHIVE_BINDING['source_pv_uid']])
        self.assertEqual(archive['archive_same_pod_readback'][2], '--stream-existing')
        with self.assertRaises(jobs.VerificationJobError):
            jobs.exec_vectors(ARCHIVE_REQUEST, 'not-a-pod-uid', ARCHIVE_BINDING)

    def test_wffc_verification_binder_is_target_only_readonly_and_node_affinity_bound(self):
        request = REQUEST | {'phase': 'bind-verification'}
        target = {'role': 'verification', 'claim_name': 'voice-nats-verify-' + request['operation'],
            'target_uid': '22222222-2222-4222-8222-222222222222', 'node_name': 'k3s-worker-1'}
        row = jobs.document(request, target, IMAGE)
        pod = row['spec']['template']['spec']
        container = pod['containers'][0]
        labels = row['spec']['template']['metadata']['labels']
        self.assertEqual(row['metadata']['labels'][jobs.LABEL_PHASE], 'bind-verification')
        self.assertEqual(row['spec']['selector']['matchLabels'], {jobs.LABEL_JOB_SELECTOR: row['metadata']['name']})
        self.assertEqual(labels[jobs.LABEL_JOB_SELECTOR], row['metadata']['name'])
        self.assertEqual(pod['automountServiceAccountToken'], False)
        self.assertEqual(pod['securityContext'], {'runAsNonRoot': True, 'runAsUser': 65532,
            'runAsGroup': 65532, 'seccompProfile': {'type': 'RuntimeDefault'}})
        self.assertNotIn('nodeName', pod)
        self.assertEqual(pod['affinity']['nodeAffinity']['requiredDuringSchedulingIgnoredDuringExecution']
            ['nodeSelectorTerms'][0]['matchExpressions'], [{'key': 'kubernetes.io/hostname',
                'operator': 'In', 'values': ['k3s-worker-1']}])
        self.assertEqual(container['command'], ['/usr/bin/python3', '/voice-nats-runner/phase'])
        self.assertEqual(container['args'], ['--bind-only', '--operation', request['operation'],
            '--nonce', request['nonce'], '--role', 'verification'])
        self.assertEqual(container['volumeMounts'], [{'name': 'target', 'mountPath': '/verify', 'readOnly': True}])
        self.assertEqual(pod['volumes'], [{'name': 'target', 'persistentVolumeClaim': {
            'claimName': target['claim_name'], 'readOnly': True}}])
        self.assertNotIn('fsGroup', pod['securityContext'])
        self.assertNotIn('hostPath', str(pod))
        self.assertNotIn('secret', str(pod).lower())
        with self.assertRaisesRegex(jobs.VerificationJobError, 'binder_has_no_exec'):
            jobs.exec_vectors(request, POD_UID, target)

    def test_wffc_restore_binder_is_separate_role_and_pins_journaled_attempt_name(self):
        request = REQUEST | {'phase': 'bind-restore'}
        target = {'role': 'restore', 'claim_name': 'voice-nats-restore-' + request['operation'] + '-1234abcd',
            'target_uid': '33333333-3333-4333-8333-333333333333', 'node_name': 'k3s-worker-1'}
        row = jobs.document(request, target, IMAGE)
        pod = row['spec']['template']['spec']
        self.assertEqual(pod['containers'][0]['args'][-1], 'restore')
        self.assertEqual(pod['volumes'][0]['persistentVolumeClaim']['claimName'], target['claim_name'])
        for invalid in (target | {'role': 'verification'},
            target | {'claim_name': 'voice-nats-restore-' + request['operation'] + '-1234abcdx'},
            target | {'node_name': '../worker'},
            target | {'target_uid': '../uid'}):
            with self.subTest(invalid=invalid), self.assertRaises(jobs.VerificationJobError):
                jobs.document(request, invalid, IMAGE)


if __name__ == '__main__':
    unittest.main()
