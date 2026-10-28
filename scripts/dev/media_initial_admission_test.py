"""Offline qualification for the bounded DEV initial-media cohort."""
import copy
import hashlib
import json
import unittest
from unittest import mock

import media_initial_admission as admission


class InitialAdmissionTest(unittest.TestCase):
    def setUp(self):
        self.product = 'a' * 40
        self.claim = '11111111-1111-4111-8111-111111111111'
        self.window = '22222222-2222-4222-8222-222222222222'
        self.selected = {'voice-file/file', 'voice-story/story', 'voice-messaging/messaging'}
        self.images = {f'voice-extra-{n}/main': f'example/extra@sha256:{n:064x}' for n in range(26)}
        self.images['voice-nats-pvc-candidate/nats'] = 'example/nats@sha256:' + 'a' * 64
        self.images['voice-dev-mail/mail'] = 'node:24-alpine'
        self.images.update({f'voice-extra-{n}/sidecar': f'example/sidecar@sha256:{n:064x}' for n in range(18)})
        self.images.update({name: 'example/old@sha256:' + 'b' * 64 for name in self.selected})
        workload_names = ['voice-file', 'voice-story', 'voice-messaging', 'voice-dev-mail',
                          'voice-nats-pvc-candidate'] + [f'voice-extra-{n}' for n in range(26)]
        mail_runtime = {'pod_uid': 'pod', 'replicaset_uid': 'rs', 'deployment_uid': 'uid-3',
                        'node': 'pmdebook', 'container_id': 'containerd://' + 'a' * 64,
                        'image_id': 'docker.io/library/node@sha256:' + 'b' * 64,
                        'restart_count': 7, 'ready': True, 'started': True,
                        'pod_count': 1, 'terminating_count': 0, 'owner_chain_verified': True}
        self.targets = {name: 'ghcr.io/poryadok/voiceroot/' + name.split('/')[1] + '@sha256:' + str(index + 1) * 64
                        for index, name in enumerate(sorted(self.selected))}
        self.plan = {'schema': 'voice-dev-initial-media-v1', 'product_sha': self.product,
                     'namespace_uid': 'ns-uid', 'baseline_images': self.images,
                     'init_images': {'voice-nats-pvc-candidate/nats-config-renderer': 'example/init@sha256:' + 'f' * 64},
                     'workload_uids': {f'Deployment/{name}': f'uid-{n}' for n, name in enumerate(workload_names)},
                     'workload_spec_sha256': {f'Deployment/{name}': 'a' * 64 for name in workload_names},
                     'mail_runtime': mail_runtime,
                     'pvc_uids': {'pgdata-voice-postgres-0': 'p1', 'data-voice-minio-0': 'p2',
                                  'chdata-voice-clickhouse-0': 'p3', 'voice-nats-jsdata': 'p4'},
                     'targets': self.targets, 'sources': {name: {'state': 'unattributed'} for name in self.selected},
                     'evidence': {name: 'c' * 64 for name in admission.REQUIRED_EVIDENCE}}
        self.observed = {'state': 'known', 'claim': self.claim, 'window': self.window,
                         'operation': 7, 'images': copy.deepcopy(self.images)}
        self.provenance = {name: {'image': self.images[name], 'source': {'state': 'unattributed'}}
                           for name in self.selected}
        self.live = {'namespace_uid': 'ns-uid', 'images': copy.deepcopy(self.images),
                     'pvc_uids': copy.deepcopy(self.plan['pvc_uids']),
                     'init_images': copy.deepcopy(self.plan['init_images']),
                     'workload_uids': copy.deepcopy(self.plan['workload_uids']),
                     'workload_spec_sha256': copy.deepcopy(self.plan['workload_spec_sha256']),
                     'mail_runtime': copy.deepcopy(mail_runtime)}
        self.context = {'project': 'voice', 'resource': 'DEV', 'actor': 'worker', 'claim': self.claim,
                        'window': self.window, 'generation': 2, 'revision': 9,
                        'operation_id': 'fixture-1', 'commit': self.product, 'images': copy.deepcopy(self.targets)}
        self.card = {'code': 'DEV', 'environment': 'dev', 'owner': 'worker',
                     'claim': self.claim, 'window': self.window, 'operation': 8,
                     'generation': 2, 'revision': 9, 'state': 'held', 'phase': 'deploy',
                     'observed': self.observed, 'candidate': {'state': 'known',
                     'commit': self.product, 'images': copy.deepcopy(self.targets)}}

    def validate(self):
        raw = json.dumps(self.plan, sort_keys=True, separators=(',', ':')).encode()
        provenance = json.dumps(self.provenance, sort_keys=True, separators=(',', ':')).encode()
        with mock.patch.object(admission, 'PINNED_PRODUCT', self.product), \
                mock.patch.object(admission, 'PINNED_PACKET_SHA256', hashlib.sha256(raw).hexdigest()), \
                mock.patch.object(admission, 'PINNED_PROVENANCE_SHA256', hashlib.sha256(provenance).hexdigest()):
            return admission.validate_pinned(raw, provenance, self.live, self.card, self.context,
                                             lambda old, new: old == 'd' * 40)

    def test_exact_unattributed_baseline_and_three_targets(self):
        self.assertTrue(self.validate())

    def test_wrong_product_baseline_claim_and_evidence_are_denied(self):
        cases = [lambda: self.plan.update(product_sha='b' * 40),
                 lambda: self.live['images'].pop(next(iter(self.live['images']))),
                 lambda: self.card.update(claim='other'),
                 lambda: self.card.update(revision=10),
                 lambda: self.card.update(code='OTHER'),
                 lambda: self.card.update(environment='stg'),
                 lambda: self.live['workload_uids'].update({'Deployment/voice-extra-4': 'replaced'}),
                 lambda: self.live['mail_runtime'].update(restart_count=8),
                 lambda: self.live['init_images'].clear(),
                 lambda: self.card['observed'].update(commit='d' * 40),
                 lambda: self.plan['evidence'].pop('backup_restore'),
                 lambda: self.plan['targets'].update({'voice-user/user': 'example/new@sha256:' + 'd' * 64})]
        for change in cases:
            with self.subTest(change=change):
                self.setUp()
                change()
                with self.assertRaises(ValueError):
                    self.validate()

    def test_known_source_must_be_ancestor_and_cannot_be_relabelled(self):
        name = sorted(self.selected)[0]
        self.plan['sources'][name] = {'state': 'known', 'commit': 'e' * 40}
        self.provenance[name]['source'] = {'state': 'known', 'commit': 'e' * 40}
        with self.assertRaises(ValueError):
            self.validate()
        self.plan['sources'][name] = {'state': 'known', 'commit': 'd' * 40}
        self.provenance[name]['source'] = {'state': 'known', 'commit': 'd' * 40}
        self.assertTrue(self.validate())
        self.plan['sources'][name] = {'state': 'unattributed'}
        with self.assertRaises(ValueError):
            self.validate()

    def test_no_runtime_packet_is_admitted_before_exact_p_and_hash_are_pinned(self):
        raw = json.dumps(self.plan, sort_keys=True, separators=(',', ':')).encode()
        with self.assertRaises(ValueError):
            admission.load_pinned(raw)
        with self.assertRaises(ValueError):
            admission.validate_pinned(raw, b'{}', self.live, self.card, self.context, lambda old, new: True)
        with mock.patch.object(admission, 'PINNED_PRODUCT', self.product), \
                mock.patch.object(admission, 'PINNED_PACKET_SHA256', hashlib.sha256(raw).hexdigest()):
            self.assertEqual(admission.load_pinned(raw), self.plan)
            with self.assertRaises(ValueError):
                admission.load_pinned(raw + b' ')

    def test_independent_provenance_bytes_cannot_drift(self):
        raw = json.dumps(self.plan, sort_keys=True, separators=(',', ':')).encode()
        provenance = json.dumps(self.provenance, sort_keys=True, separators=(',', ':')).encode()
        with mock.patch.object(admission, 'PINNED_PRODUCT', self.product), \
                mock.patch.object(admission, 'PINNED_PACKET_SHA256', hashlib.sha256(raw).hexdigest()), \
                mock.patch.object(admission, 'PINNED_PROVENANCE_SHA256', hashlib.sha256(provenance).hexdigest()):
            with self.assertRaises(ValueError):
                admission.validate_pinned(raw, provenance + b' ', self.live, self.card, self.context,
                                          lambda old, new: True)

    def test_mail_runtime_rejects_foreign_image_or_boolean_counts(self):
        for key, value in (('image_id', 'foreign/node@sha256:' + 'b' * 64),
                           ('pod_count', True), ('terminating_count', False)):
            with self.subTest(key=key):
                self.setUp()
                self.plan['mail_runtime'][key] = value
                self.live['mail_runtime'][key] = value
                with self.assertRaises(ValueError):
                    self.validate()

    def test_init_image_owner_cannot_be_missing_from_other_inventories(self):
        old, new = 'voice-nats-pvc-candidate', 'voice-foreign'
        for mapping in (self.plan['baseline_images'], self.live['images'], self.card['observed']['images']):
            mapping[new + '/nats'] = mapping.pop(old + '/nats')
        for mapping in (self.plan['workload_uids'], self.plan['workload_spec_sha256'],
                        self.live['workload_uids'], self.live['workload_spec_sha256']):
            mapping['Deployment/' + new] = mapping.pop('Deployment/' + old)
        with self.assertRaises(ValueError):
            self.validate()

    def test_additional_init_container_is_not_in_the_fixed_packet(self):
        name = 'voice-file/foreign-init'
        for mapping in (self.plan['init_images'], self.live['init_images']):
            mapping[name] = 'example/init@sha256:' + 'a' * 64
        with self.assertRaises(ValueError):
            self.validate()


if __name__ == '__main__':
    unittest.main()
