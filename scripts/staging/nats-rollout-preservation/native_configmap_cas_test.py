import copy
import hashlib
import unittest

from native_configmap_cas import (
    ANNOTATION,
    ConfigMapCASError,
    build_request,
    verify_readback,
)


UID = '12345678-1234-4abc-8def-1234567890ab'
RV = '17'
OP = 'abcdef012345'
NAME = 'voice-nats-realtime-bootstrap'
NAMESPACE = 'voice-staging'
OLD = '# old bootstrap\n'
TARGET = '# new bootstrap\n'
OLD_SHA = hashlib.sha256(OLD.encode()).hexdigest()


class NativeConfigMapCASTest(unittest.TestCase):
    def test_patch_tests_identity_and_preserves_existing_annotations(self):
        annotations = {'owner': 'staging', 'unrelated': 'keep'}
        original = copy.deepcopy(annotations)
        patch = build_request(NAMESPACE, NAME, UID, RV, OLD_SHA, OP, TARGET, annotations)
        self.assertEqual(annotations, original)
        self.assertEqual(patch, [
            {'op': 'test', 'path': '/metadata/uid', 'value': UID},
            {'op': 'test', 'path': '/metadata/resourceVersion', 'value': RV},
            {'op': 'replace', 'path': '/data/bootstrap.sh', 'value': TARGET},
            {'op': 'replace', 'path': '/metadata/annotations', 'value': {
                'owner': 'staging', 'unrelated': 'keep', ANNOTATION: OP,
            }},
        ])

    def test_missing_annotations_uses_add_without_losing_fields(self):
        patch = build_request(NAMESPACE, NAME, UID, RV, OLD_SHA, OP, TARGET, None)
        self.assertEqual(patch[-1], {
            'op': 'add', 'path': '/metadata/annotations', 'value': {ANNOTATION: OP},
        })

    def test_rejects_unbound_namespace_name_and_identity(self):
        bad = [
            (('default', NAME, UID, RV, OLD_SHA, OP, TARGET, {}), 'namespace_invalid'),
            ((NAMESPACE, 'voice-auth-bootstrap', UID, RV, OLD_SHA, OP, TARGET, {}), 'configmap_name_invalid'),
            ((NAMESPACE, NAME, 'not-a-uid', RV, OLD_SHA, OP, TARGET, {}), 'uid_invalid'),
            ((NAMESPACE, NAME, UID, '0', OLD_SHA, OP, TARGET, {}), 'resource_version_invalid'),
            ((NAMESPACE, NAME, UID, RV, 'x' * 64, OP, TARGET, {}), 'sha256_invalid'),
            ((NAMESPACE, NAME, UID, RV, OLD_SHA, 'A' * 12, TARGET, {}), 'operation_invalid'),
        ]
        for args, reason in bad:
            with self.subTest(reason=reason), self.assertRaisesRegex(ConfigMapCASError, reason):
                build_request(*args)

    def test_readback_binds_identity_version_script_digest_and_operation(self):
        actual = {
            'metadata': {'namespace': NAMESPACE, 'name': NAME, 'uid': UID,
                         'resourceVersion': '18', 'annotations': {ANNOTATION: OP}},
            'data': {'bootstrap.sh': TARGET, 'unrelated': 'preserved'},
        }
        projection = verify_readback(actual, NAMESPACE, NAME, UID, RV, TARGET, OP)
        self.assertEqual(projection, {
            'name': NAME, 'uid': UID, 'resource_version': '18',
            'script_sha256': hashlib.sha256(TARGET.encode()).hexdigest(), 'operation': OP,
        })
        for mutate, reason in (
            (lambda x: x['metadata'].__setitem__('uid', 'aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa'), 'readback_uid_invalid'),
            (lambda x: x['metadata'].__setitem__('resourceVersion', RV), 'readback_resource_version_invalid'),
            (lambda x: x['data'].__setitem__('bootstrap.sh', OLD), 'readback_script_invalid'),
            (lambda x: x['metadata']['annotations'].__setitem__(ANNOTATION, '111111111111'), 'readback_operation_invalid'),
        ):
            changed = copy.deepcopy(actual)
            mutate(changed)
            with self.subTest(reason=reason), self.assertRaisesRegex(ConfigMapCASError, reason):
                verify_readback(changed, NAMESPACE, NAME, UID, RV, TARGET, OP)


if __name__ == '__main__':
    unittest.main()
