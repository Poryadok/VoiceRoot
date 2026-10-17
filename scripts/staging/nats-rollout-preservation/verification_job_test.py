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
SESSION_REQUEST = REQUEST | {'phase': 'verification-session'}
SESSION_BINDING = BINDING | {'session_attempt': '1234abcd',
    'input_pod_uid': '55555555-5555-4555-8555-555555555555',
    'input_bundle_sha256': 'e' * 64, 'input_bundle_bytes': 512,
    'kernel_bundle_sha256': 'f' * 64, 'server_config_sha256': '1' * 64,
    'archive_sha256': 'd' * 64, 'archive_bytes': 1234}


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
            {'sha256': 'd' * 64, 'bytes': 1234},
            session_inputs={'attempt': '1234abcd', 'sha256': 'e' * 64, 'bytes': 512})
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

    def test_selected_store_replacement_and_promotable_restore_are_not_supported(self):
        for phase in ('restore', 'bind-restore'):
            with self.subTest(phase=phase), self.assertRaises(jobs.VerificationJobError):
                jobs.validate_request(REQUEST | {'phase': phase})

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

    def test_selected_store_job_has_native_nats_sidecar_and_disjoint_container_mounts(self):
        row = jobs.document(SESSION_REQUEST, SESSION_BINDING, IMAGE)
        pod = row['spec']['template']['spec']
        labels = row['spec']['template']['metadata']['labels']
        self.assertEqual(row['metadata']['labels'][jobs.LABEL_PHASE], 'verification-session')
        self.assertEqual(row['spec']['selector']['matchLabels'], {jobs.LABEL_JOB_SELECTOR: row['metadata']['name']})
        self.assertEqual(labels[jobs.LABEL_JOB_SELECTOR], row['metadata']['name'])
        self.assertEqual(row['spec']['backoffLimit'], 0)
        self.assertEqual(row['spec']['podReplacementPolicy'], 'Never')
        self.assertEqual(pod['restartPolicy'], 'Never')
        self.assertEqual(pod['automountServiceAccountToken'], False)
        self.assertEqual(pod['securityContext']['fsGroup'], 65532)
        self.assertEqual(pod['securityContext']['fsGroupChangePolicy'], 'OnRootMismatch')
        self.assertEqual(pod['initContainers'][0]['name'], 'nats')
        self.assertEqual(pod['initContainers'][0]['restartPolicy'], 'Always')
        self.assertEqual(pod['initContainers'][0]['image'], IMAGE)
        self.assertEqual(pod['initContainers'][0]['command'],
            ['/usr/bin/python3', '/voice-nats-runner/nats-supervisor'])
        self.assertEqual(pod['initContainers'][0]['args'], [])
        nats = pod['initContainers'][0]
        self.assertEqual(nats['securityContext']['runAsUser'], 65532)
        self.assertEqual(nats['securityContext']['runAsGroup'], 65532)
        self.assertTrue(nats['securityContext']['readOnlyRootFilesystem'])
        self.assertEqual(nats['securityContext']['capabilities'], {'drop': ['ALL']})
        self.assertEqual(nats['volumeMounts'], [
            {'name': 'scratch', 'mountPath': '/data', 'subPath': 'store'},
            {'name': 'scratch', 'mountPath': '/server.conf',
                'subPath': 'inputs/public/server.conf', 'readOnly': True},
        ])
        control = pod['containers'][0]
        self.assertEqual(control['name'], 'control')
        self.assertEqual(control['image'], IMAGE)
        self.assertEqual(pod['securityContext']['runAsUser'], 65532)
        self.assertEqual(pod['securityContext']['runAsGroup'], 65532)
        self.assertNotIn('runAsUser', control['securityContext'])
        self.assertNotIn('runAsGroup', control['securityContext'])
        self.assertEqual(control['args'], ['--serve-selected-session', '--operation', SESSION_REQUEST['operation'],
            '--nonce', SESSION_REQUEST['nonce'], '--session-attempt', SESSION_BINDING['session_attempt'],
            '--source-pvc-uid', SESSION_BINDING['source_uid'], '--source-pv-uid', SESSION_BINDING['source_pv_uid'],
            '--verification-pvc-uid', SESSION_BINDING['verification_uid'],
            '--verification-pv-uid', SESSION_BINDING['verification_pv_uid'],
            '--input-pod-uid', SESSION_BINDING['input_pod_uid'],
            '--input-bundle-sha256', SESSION_BINDING['input_bundle_sha256'],
            '--input-bundle-bytes', str(SESSION_BINDING['input_bundle_bytes']),
            '--kernel-bundle-sha256', SESSION_BINDING['kernel_bundle_sha256'],
            '--server-config-sha256', SESSION_BINDING['server_config_sha256'],
            '--archive-sha256', SESSION_BINDING['archive_sha256'],
            '--archive-bytes', str(SESSION_BINDING['archive_bytes'])])
        self.assertEqual(control['volumeMounts'], [
            {'name': 'scratch', 'mountPath': '/inputs', 'subPath': 'kernelinputbundle', 'readOnly': True},
            {'name': 'scratch', 'mountPath': '/state',
                'subPath': 'state/' + SESSION_REQUEST['operation'] + '/' + SESSION_BINDING['session_attempt']},
            {'name': 'output', 'mountPath': '/out'},
        ])
        volumes = {v['name']: v for v in pod['volumes']}
        self.assertEqual(volumes['scratch']['persistentVolumeClaim'], {'claimName': BINDING['verification_claim']})
        self.assertEqual(volumes['output']['emptyDir'], {'sizeLimit': '256Mi'})
        self.assertEqual(set(volumes), {'scratch', 'output'})
        self.assertNotIn('nodeName', pod)
        self.assertNotIn('hostPath', str(pod))
        self.assertNotIn('secret', str(pod).lower())

    def test_selected_store_vectors_are_closed_and_bind_same_session_identity(self):
        vectors = jobs.exec_vectors(SESSION_REQUEST, POD_UID, SESSION_BINDING)
        names = {'bootstrap-realtime', 'bootstrap-notification', 'bootstrap-analytics-chat',
            'bootstrap-search', 'kernel-census', 'kernel-seed', 'kernel-verify-closed', 'kernel-drain',
            'broker-ready', 'broker-restart-prepare', 'broker-restart-complete', 'kernel-verify-drained',
            'session-close'}
        self.assertEqual(set(vectors['actions']), names)
        self.assertEqual(set(vectors['readbacks']), names)
        self.assertNotIn('nats_restart_signal', vectors)
        self.assertEqual(vectors['nats_health'], ['/bin/busybox', 'wget', '-q', '-O', '-',
            'http://127.0.0.1:8222/healthz'])
        self.assertEqual(vectors['session_close'][2:4], ['--session-action', 'session-close'])
        self.assertEqual(vectors['session_stop'][2:4], ['--session-action', 'session-stop'])
        for action in names:
            command = vectors['actions'][action]
            self.assertEqual(command[:3], ['/usr/bin/python3', '/voice-nats-runner/phase', '--session-action'])
            self.assertEqual(command[3], action)
            for flag, value in (('--operation', SESSION_REQUEST['operation']),
                ('--nonce', SESSION_REQUEST['nonce']), ('--pod-uid', POD_UID),
                ('--session-attempt', SESSION_BINDING['session_attempt']),
                ('--input-pod-uid', SESSION_BINDING['input_pod_uid']),
                ('--input-bundle-sha256', SESSION_BINDING['input_bundle_sha256']),
                ('--input-bundle-bytes', str(SESSION_BINDING['input_bundle_bytes'])),
                ('--source-pvc-uid', BINDING['source_uid']), ('--source-pv-uid', BINDING['source_pv_uid']),
                ('--verification-pvc-uid', BINDING['verification_uid']),
                ('--verification-pv-uid', BINDING['verification_pv_uid'])):
                self.assertEqual(command[command.index(flag) + 1], value)
            self.assertEqual(vectors['readbacks'][action][2], '--session-readback')
            self.assertEqual(vectors['readbacks'][action][4], action)
        self.assertEqual(vectors['actions']['kernel-seed'][3], 'kernel-seed')
        with self.assertRaisesRegex(jobs.VerificationJobError, 'selected_action_invalid'):
            jobs.exec_vectors(SESSION_REQUEST, POD_UID, SESSION_BINDING, action='../../bin/sh')
        contracts = jobs.selected_exec_contracts(SESSION_REQUEST, POD_UID, SESSION_BINDING)
        self.assertEqual(contracts['nats-health'], {'container': 'nats', 'command': vectors['nats_health'],
            'stdin': False, 'stdout': True, 'stderr': False, 'tty': False})
        self.assertEqual(contracts['action:kernel-seed']['container'], 'control')
        self.assertEqual(contracts['action:kernel-seed']['stdin'], False)
        self.assertEqual(contracts['readback:kernel-seed']['stdout'], True)
        for invalid in (SESSION_BINDING | {'session_attempt': '1234abcdx'},
            SESSION_BINDING | {'extra': 'ignored'},
            SESSION_BINDING | {'source_uid': '../pvc'}):
            with self.subTest(invalid=invalid), self.assertRaises(jobs.VerificationJobError):
                jobs.document(SESSION_REQUEST, invalid, IMAGE)


if __name__ == '__main__':
    unittest.main()
