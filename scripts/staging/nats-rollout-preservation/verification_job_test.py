"""Tests for the source-owned, tokenless native census Job ABI."""
import unittest

import verification_job as jobs


REQUEST = {'operation': 'a' * 12, 'nonce': 'b' * 32, 'phase': 'census'}
BINDING = {
    'source_claim': 'voice-nats-jsdata-d20261017a',
    'source_uid': '11111111-1111-4111-8111-111111111111',
    'verification_claim': 'voice-nats-verify-d20261017a',
    'verification_uid': '22222222-2222-4222-8222-222222222222',
    'pv_name': 'pv-voice-nats-verify',
    'pv_uid': '33333333-3333-4333-8333-333333333333',
    'node_name': 'k3s-worker-1',
}
IMAGE = 'ghcr.io/poryadok/voice-nats-phase@sha256:' + 'a' * 64
POD_UID = '44444444-4444-4444-8444-444444444444'


class VerificationJobTests(unittest.TestCase):
    def test_fixed_job_has_closed_tokenless_nonroot_two_pvc_shape(self):
        row = jobs.document(REQUEST, BINDING, IMAGE)
        pod = row['spec']['template']['spec']
        container = pod['containers'][0]
        self.assertEqual(row['apiVersion'], 'batch/v1')
        self.assertEqual(row['kind'], 'Job')
        self.assertEqual(row['metadata']['namespace'], 'voice-staging')
        self.assertEqual(pod['automountServiceAccountToken'], False)
        self.assertEqual(pod['securityContext'], {
            'runAsNonRoot': True, 'runAsUser': 10000, 'runAsGroup': 10000,
            'seccompProfile': {'type': 'RuntimeDefault'}})
        self.assertNotIn('fsGroup', pod['securityContext'])
        self.assertEqual(container['image'], IMAGE)
        self.assertEqual(container['args'], ['--serve', '--operation', REQUEST['operation'], '--nonce', REQUEST['nonce']])
        self.assertEqual([mount['mountPath'] for mount in container['volumeMounts']],
            ['/data', '/verify', '/out', '/inputs'])
        source_mount = next(m for m in container['volumeMounts'] if m['name'] == 'source')
        self.assertTrue(source_mount['readOnly'])
        volumes = {volume['name']: volume for volume in pod['volumes']}
        self.assertEqual(volumes['source']['persistentVolumeClaim'], {
            'claimName': BINDING['source_claim'], 'readOnly': True})
        self.assertEqual(volumes['verification']['persistentVolumeClaim']['claimName'], BINDING['verification_claim'])
        self.assertNotIn('hostPath', str(pod))
        self.assertNotIn('secret', str(pod).lower())

    def test_request_cannot_select_image_command_or_another_phase(self):
        for value in (
            REQUEST | {'phase': 'restore'},
            REQUEST | {'command': ['/bin/sh', '-c', 'id']},
            REQUEST | {'image': 'attacker.invalid/image:latest'},
            REQUEST | {'extra': 'field'},
        ):
            with self.subTest(value=value), self.assertRaises(jobs.VerificationJobError):
                jobs.validate_request(value)
        for image in ('candidate:latest', IMAGE.replace('a' * 64, 'z' * 64)):
            with self.subTest(image=image), self.assertRaises(jobs.VerificationJobError):
                jobs.document(REQUEST, BINDING, image)

    def test_source_and_verification_claim_identity_must_be_distinct_and_valid(self):
        for mutation in (
            {'verification_claim': BINDING['source_claim']},
            {'verification_uid': BINDING['source_uid']},
            {'source_uid': '../admin'},
            {'verification_claim': 'untrusted'},
        ):
            value = BINDING | mutation
            with self.subTest(mutation=mutation), self.assertRaises(jobs.VerificationJobError):
                jobs.document(REQUEST, value, IMAGE)

    def test_exec_argv_is_fixed_and_binds_live_pod_and_both_pvc_uids(self):
        vectors = jobs.exec_vectors(REQUEST, POD_UID, BINDING)
        self.assertEqual(vectors['load_inputs'], [
            '/usr/bin/python3', '/voice-nats-runner/phase', '--load-inputs',
            '--operation', REQUEST['operation'], '--nonce', REQUEST['nonce'], '--pod-uid', POD_UID])
        self.assertEqual(vectors['census'], [
            '/usr/bin/python3', '/voice-nats-runner/phase', '--phase', 'census',
            '--operation', REQUEST['operation'], '--nonce', REQUEST['nonce'], '--pod-uid', POD_UID,
            '--source-pvc-uid', BINDING['source_uid'],
            '--verification-pvc-uid', BINDING['verification_uid']])
        with self.assertRaises(jobs.VerificationJobError):
            jobs.exec_vectors(REQUEST, 'not-a-pod-uid', BINDING)


if __name__ == '__main__':
    unittest.main()
