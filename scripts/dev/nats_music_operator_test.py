import base64
import copy
import json
import sys
import tempfile
import types
import unittest
from pathlib import Path
from unittest.mock import patch

import nats_music_operator as operator

STAGING = Path(__file__).resolve().parents[1] / 'staging/nats-rollout-preservation'
sys.path.insert(0, str(STAGING.resolve()))
import actor_permissions


def meta(name, uid, rv='1', ns=True, labels=True):
    result = {'name': name, 'uid': uid, 'resourceVersion': rv}
    if ns:
        result['namespace'] = 'voice-dev'
    if labels:
        result['labels'] = dict(operator.LABELS)
    return result


def secret(name, uid):
    return {'apiVersion': 'v1', 'kind': 'Secret', 'metadata': meta(name, uid, labels=False),
            'type': 'Opaque', 'data': {'messaging.creds': base64.b64encode(b'fixture').decode()}}


def fixture():
    import base64
    deployment = {
        'metadata': dict(meta('voice-messaging', 'messaging-deployment'), generation=1),
        'spec': {'replicas': 1, 'selector': {'matchLabels': {'app': 'voice-messaging'}},
                 'template': {'spec': {'containers': [{'name': 'nats-leaf', 'volumeMounts': [
                     {'name': 'nats-service-creds', 'mountPath': '/var/run/nats/creds/messaging.creds',
                      'subPath': 'messaging.creds', 'readOnly': True}]}],
                     'volumes': [{'name': 'nats-service-creds', 'secret': {
                         'secretName': 'voice-nats-service-credentials',
                         'defaultMode': 256, 'items': [{'key': 'messaging.creds', 'path': 'messaging.creds'}]}}]}}},
        'status': {'observedGeneration': 1, 'readyReplicas': 1, 'updatedReplicas': 1, 'unavailableReplicas': 0}}
    service = {'metadata': meta('voice-nats', 'nats-service', labels=False),
               'spec': {'type': 'ClusterIP', 'selector': dict(operator.NATS_SELECTOR),
                        'ports': [{'name': 'client', 'port': 4222, 'targetPort': 4222}]}}
    image = 'nats:2.12.12-alpine@sha256:' + 'a' * 64
    nats_deployment = {'metadata': dict(meta(operator.NATS_DEPLOYMENT, 'nats-deployment', labels=False), generation=3),
        'spec': {'replicas': 1, 'selector': {'matchLabels': dict(operator.NATS_SELECTOR)},
                 'template': {'metadata': {'labels': dict(operator.NATS_SELECTOR)},
                              'spec': {'containers': [{'name': 'nats', 'image': image}]}}},
        'status': {'observedGeneration': 3, 'readyReplicas': 1, 'updatedReplicas': 1, 'unavailableReplicas': 0}}
    replica_set = {'metadata': dict(meta(operator.NATS_DEPLOYMENT + '-abc', 'nats-rs', labels=False),
                                    ownerReferences=[{'kind': 'Deployment', 'name': operator.NATS_DEPLOYMENT,
                                                      'uid': 'nats-deployment', 'controller': True}]),
        'spec': {'replicas': 1, 'selector': {'matchLabels': dict(operator.NATS_SELECTOR)},
                 'template': {'metadata': {'labels': dict(operator.NATS_SELECTOR)},
                              'spec': {'containers': [{'name': 'nats', 'image': image}]}}},
        'status': {'readyReplicas': 1}}
    pod = {'metadata': dict(meta('nats-pod', 'nats-pod'), labels=dict(operator.NATS_SELECTOR),
                            ownerReferences=[{'kind': 'ReplicaSet', 'name': operator.NATS_DEPLOYMENT + '-abc',
                                              'uid': 'nats-rs', 'controller': True}]),
           'status': {'phase': 'Running', 'containerStatuses': [{'name': 'nats', 'ready': True,
                      'image': image, 'imageID': 'docker-pullable://nats@sha256:' + 'a' * 64}]},
           'spec': {'nodeName': 'dev-node', 'containers': [{'name': 'nats', 'image': image}]}}
    return {
        ('namespace', 'voice-dev'): {'metadata': meta('voice-dev', '15950fc8-0392-4053-9156-e95e78b596f9', ns=False)},
        ('secret', 'voice-nats-operator'): {'metadata': meta('voice-nats-operator', 'operator', labels=False),
            'data': {'account.public': base64.b64encode(b'PUBLICKEY').decode()}},
        ('secret', 'voice-nats-service-credentials'): secret('voice-nats-service-credentials', 'service'),
        ('deployment', 'voice-messaging'): deployment,
        ('service', 'voice-nats'): service,
        ('deployment', operator.NATS_DEPLOYMENT): nats_deployment,
        ('replicaset', operator.NATS_DEPLOYMENT + '-abc'): replica_set,
        ('configmap', 'voice-nats-hub-config'): {'metadata': meta('voice-nats-hub-config', 'hub-config', labels=False),
            'data': {'nats.conf': 'server_name: voice-nats-hub\nleafnodes { listen: 0.0.0.0:7422 }'}},
        ('configmap', 'voice-nats-analytics-chat-bootstrap'): {'metadata': meta(
            'voice-nats-analytics-chat-bootstrap', 'bootstrap-config', labels=False),
            'data': {'bootstrap.sh': 'current source'}} ,
        ('pods', ','.join(f'{k}={v}' for k, v in operator.NATS_SELECTOR.items())): [pod],
    }


class FakeKube:
    def __init__(self, objects):
        self.objects = objects

    def get(self, kind, name, optional=False):
        value = self.objects.get((kind, name))
        if value is None and optional:
            return None
        if value is None:
            raise KeyError((kind, name))
        return copy.deepcopy(value)

    def list(self, kind, selector=None):
        return copy.deepcopy(self.objects.get((kind, selector), []))


class MusicOperatorTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.grant = {'publish': ['message.sent', *operator.SERVICE_ADDITIONS],
                     'subscribe': ['_INBOX.voice.messaging.>'], 'no_response': True}
        cls.grants = {'messaging': cls.grant,
                      'bootstrap': {'publish': list(operator.BOOTSTRAP_ADDITIONS),
                                    'subscribe': ['_INBOX.voice.bootstrap.reply.>'], 'no_response': False}}

    def setUp(self):
        self.journal_dir = tempfile.TemporaryDirectory(prefix='nats-music-journal-')
        self.addCleanup(self.journal_dir.cleanup)
        patcher = patch.object(operator, 'BOOTSTRAP_ENROLLMENT_JOURNAL', Path(self.journal_dir.name))
        patcher.start()
        self.addCleanup(patcher.stop)
        acl_path = Path(self.journal_dir.name) / 'acl-intent.yaml'
        acl_path.write_text('fixture ACL intent\n', encoding='utf-8')
        acl_patcher = patch.object(operator, 'ACL', acl_path)
        acl_patcher.start()
        self.addCleanup(acl_patcher.stop)
        bootstrap = types.ModuleType('bootstrap')
        bootstrap.canonical_bootstrap_data = lambda: {
            'voice-nats-analytics-chat-bootstrap': {
                'bootstrap.sh': 'canonical source fixture\n' + operator.MUSIC_BOOTSTRAP_CONTRACT}}
        bootstrap_patcher = patch.dict(sys.modules, {'bootstrap': bootstrap})
        bootstrap_patcher.start()
        self.addCleanup(bootstrap_patcher.stop)

    def full(self):
        return {'pub': {'allow': list(self.grant['publish']), 'deny': []},
                'sub': {'allow': list(self.grant['subscribe']), 'deny': []}, 'response': None}

    def full_bootstrap(self):
        grant = self.grants['bootstrap']
        return {'pub': {'allow': list(grant['publish']), 'deny': []},
                'sub': {'allow': list(grant['subscribe']), 'deny': []}, 'response': None}

    def configmap_enrollment(self):
        generation = 'dev-aaaaaaaaaaaaaaaaaaaaaaaa'
        base = {'metadata': meta('voice-nats-analytics-chat-bootstrap', 'base-config', '7', labels=False),
                'data': {'bootstrap.sh': 'old bootstrap declaration'}}
        target = {'bootstrap.sh': 'new canonical source\n' + operator.MUSIC_BOOTSTRAP_CONTRACT}
        target_sha = operator.sha(operator.canonical(target))
        name = operator.bootstrap_configmap_name(generation, target_sha)
        plan = 'b' * 64
        annotations = {
            'voice.io/dev-music-configmap-enrollment': 'voice-dev-music-configmap-enrollment-v1',
            'voice.io/dev-music-binding-id': generation,
            'voice.io/dev-music-namespace-uid': operator.NS_UID,
            'voice.io/dev-music-base-configmap-uid': 'base-config',
            'voice.io/dev-music-base-configmap-rv': '7',
            'voice.io/dev-music-base-configmap-sha256': operator.sha(operator.canonical(base['data'])),
            'voice.io/dev-music-target-source-sha256': target_sha,
            'voice.io/dev-music-plan-sha256': plan,
        }
        enrolled = {'apiVersion': 'v1', 'kind': 'ConfigMap',
                    'metadata': {'name': name, 'namespace': operator.NS,
                                 'uid': 'enrolled-config', 'resourceVersion': '8',
                                 'labels': {**operator.LABELS, 'voice.io/dev-music-binding-id': generation},
                                 'annotations': annotations},
                    'immutable': True, 'data': target}
        journal = {
            'schema': 'voice-dev-music-configmap-enrollment-v1', 'name': name,
            'baseline_binding_id': generation, 'namespace_uid': operator.NS_UID,
            'base_uid': 'base-config', 'base_resource_version': '7',
            'base_data_sha256': operator.sha(operator.canonical(base['data'])),
            'target_source_sha256': target_sha, 'configmap_uid': 'enrolled-config',
            'configmap_resource_version': '8', 'plan_sha256': plan}
        return base, target, enrolled, journal

    def test_immutable_configmap_enrollment_binds_base_source_generation_and_target(self):
        base, target, enrolled, journal = self.configmap_enrollment()
        with patch.object(operator, 'private_file', return_value=operator.canonical(journal)):
            self.assertTrue(operator.validate_bootstrap_configmap_enrollment(
                enrolled, base, target, 'dev-aaaaaaaaaaaaaaaaaaaaaaaa'))
        altered_base = copy.deepcopy(base)
        altered_base['metadata']['resourceVersion'] = '9'
        with patch.object(operator, 'private_file', return_value=operator.canonical(journal)):
            with self.assertRaisesRegex(ValueError, 'bootstrap_configmap_enrollment_binding_invalid'):
                operator.validate_bootstrap_configmap_enrollment(
                    enrolled, altered_base, target, 'dev-aaaaaaaaaaaaaaaaaaaaaaaa')
        altered_target = dict(target, **{'extra.sh': 'not canonical'})
        with patch.object(operator, 'private_file', return_value=operator.canonical(journal)):
            with self.assertRaisesRegex(ValueError, 'bootstrap_configmap_enrollment_binding_invalid'):
                operator.validate_bootstrap_configmap_enrollment(
                    enrolled, base, altered_target, 'dev-aaaaaaaaaaaaaaaaaaaaaaaa')

    def test_configmap_enrollment_is_create_only_and_readback_checked(self):
        base, target, enrolled, journal_data = self.configmap_enrollment()
        state = {'bootstrap_config': base, 'bootstrap_config_data': target,
                 'bootstrap_config_target_available': True, 'baseline_binding_id': 'dev-aaaaaaaaaaaaaaaaaaaaaaaa'}
        commands = []

        class CreateOnlyKube:
            def run(self, *args, body=None, timeout=20):
                commands.append(args[0])
                if args[0] != 'create' or len(commands) != 1:
                    raise ValueError('already_exists')
                created = json.loads(body)
                created['metadata'].update(uid='enrolled-config', resourceVersion='8')
                self.row = created

            def get(self, kind, name, optional=False):
                self.asserted = (kind, name)
                if name == 'voice-nats-analytics-chat-bootstrap':
                    return copy.deepcopy(base)
                return copy.deepcopy(self.row)

        fake = CreateOnlyKube()
        with patch.object(operator, 'Kube', return_value=fake), \
             patch.object(operator, 'write_bootstrap_configmap_enrollment_journal') as journal, \
             patch.object(operator, 'private_file', return_value=operator.canonical(journal_data)):
            result = operator.create_bootstrap_configmap_enrollment(state, 'b' * 64)
        self.assertEqual(commands, ['create'])
        self.assertEqual(fake.asserted, ('configmap', enrolled['metadata']['name']))
        self.assertEqual(result['data'], target)
        self.assertTrue(result['immutable'])
        journal.assert_called_once()

        class ConflictKube(CreateOnlyKube):
            def run(self, *args, body=None, timeout=20):
                commands.append(args[0])
                raise ValueError('already_exists')

        with patch.object(operator, 'Kube', return_value=ConflictKube()), \
             patch.object(operator, 'write_bootstrap_configmap_enrollment_journal') as journal:
            with self.assertRaisesRegex(ValueError, 'already_exists'):
                operator.create_bootstrap_configmap_enrollment(state, 'b' * 64)
            journal.assert_not_called()

    def test_configmap_enrollment_revalidates_before_create(self):
        base, target, _, _ = self.configmap_enrollment()
        state = {'bootstrap_config': base, 'bootstrap_config_data': target,
                 'bootstrap_config_target_available': True, 'baseline_binding_id': 'dev-aaaaaaaaaaaaaaaaaaaaaaaa'}
        events = []

        class Kube:
            def get(self, kind, name, optional=False):
                events.append(('get', kind, name))
                return copy.deepcopy(base)

            def run(self, *args, **kwargs):
                events.append(('create', *args))

        def revalidate():
            events.append(('revalidate',))
            if events.count(('revalidate',)) == 2:
                operator.fail('operator_snapshot_changed')

        with patch.object(operator, 'Kube', return_value=Kube()):
            with self.assertRaisesRegex(ValueError, 'operator_snapshot_changed'):
                operator.create_bootstrap_configmap_enrollment(state, 'b' * 64, revalidate=revalidate)
        self.assertEqual(events, [
            ('revalidate',),
            ('get', 'configmap', 'voice-nats-analytics-chat-bootstrap'),
            ('revalidate',),
        ])

    def test_configmap_transition_allows_only_exact_enrollment(self):
        base, target, enrolled, journal = self.configmap_enrollment()
        before = operator.capture(FakeKube(fixture()))
        before.update(bootstrap_config=base, bootstrap_config_data=target,
                      bootstrap_config_target_available=True,
                      baseline_binding_id='dev-aaaaaaaaaaaaaaaaaaaaaaaa',
                      bootstrap_config_enrollment=None, bootstrap_config_enrollment_name=None)
        after = copy.deepcopy(before)
        after['bootstrap_config_enrollment'] = enrolled
        after['bootstrap_config_enrollment_name'] = enrolled['metadata']['name']
        with patch.object(operator, 'private_file', return_value=operator.canonical(journal)):
            self.assertTrue(operator.assert_bootstrap_configmap_transition(before, after, enrolled))
            drifted = copy.deepcopy(after)
            drifted['nats_pod']['metadata']['uid'] = 'replacement-pod'
            with self.assertRaisesRegex(ValueError, 'bootstrap_configmap_unrelated_state_changed'):
                operator.assert_bootstrap_configmap_transition(before, drifted, enrolled)

    def test_replay_uses_only_enrolled_configmap_and_preserves_static_source(self):
        base, target, enrolled, _ = self.configmap_enrollment()
        bootstrap = {'metadata': meta(operator.BOOTSTRAP_SECRET, 'bootstrap-cred')}
        source_job = self.bootstrap_job()
        state = {'bootstrap_job': None, 'bootstrap_job_source': source_job,
                 'bootstrap': bootstrap, 'bootstrap_config': base,
                 'bootstrap_config_data': target, 'bootstrap_config_target_available': True,
                 'bootstrap_config_enrollment': enrolled,
                 'baseline_binding_id': 'dev-aaaaaaaaaaaaaaaaaaaaaaaa'}
        objects = {}
        commands = []

        class ReplayKube:
            def get(self, kind, name, optional=False):
                commands.append(('get', kind, name))
                return copy.deepcopy(objects.get((kind, name)))

            def run(self, *args, body=None, timeout=20):
                commands.append((args[0], *args[1:]))
                if args[0] == 'create':
                    job = json.loads(body)
                    job['metadata'].update(uid='replay-job', resourceVersion='22')
                    job['status'] = {'succeeded': 1}
                    objects[('job', job['metadata']['name'])] = job

        fake = ReplayKube()
        static_before = copy.deepcopy(base)
        with patch.object(operator, 'Kube', return_value=fake), \
             patch.object(operator, 'validate_bootstrap_configmap_enrollment', return_value=True), \
             patch.object(operator, 'validate_bootstrap_job', return_value=True), \
             patch.object(operator, 'private_file', return_value=b'prepared-source'):
            name = operator.replay_bootstrap_job(state, 'c' * 64, 'natsio/nats-box:0.18.0',
                                                revalidate=lambda: None)
        replay = objects[('job', name)]
        pod = replay['spec']['template']['spec']
        config = next(volume['configMap']['name'] for volume in pod['volumes'] if volume['name'] == 'config')
        secret_name = next(volume['secret']['secretName'] for volume in pod['volumes']
                           if volume['name'] == 'nats-bootstrap-creds')
        self.assertEqual(config, enrolled['metadata']['name'])
        self.assertEqual(secret_name, operator.BOOTSTRAP_SECRET)
        self.assertEqual(base, static_before)
        self.assertEqual(replay['metadata']['annotations']['voice.io/dev-music-enrolled-configmap-uid'],
                         'enrolled-config')

    def test_probe_uses_only_fixed_role_contracts(self):
        messaging = operator.probe_command('messaging', 4222, Path('/private/messaging.creds'))
        bootstrap = operator.probe_command('bootstrap', 4222, Path('/private/bootstrap.creds'))
        self.assertEqual(messaging[1:3], ['--actor-role', 'messaging'])
        self.assertEqual(bootstrap[1:3], ['--actor-role', 'bootstrap'])
        with self.assertRaisesRegex(ValueError, 'broker_probe_actor_role_invalid'):
            operator.probe_command('other', 4222, Path('/private/other.creds'))

    def test_renewal_payload_builders_keep_service_and_bootstrap_contracts_separate(self):
        operator_secret = {'data': {
            'account.jwt': base64.b64encode(b'fixture-account-jwt').decode(),
            'operator.jwt': base64.b64encode(b'fixture-operator-jwt').decode(),
        }}
        receipt = {'actor_sha256': 'a' * 64, 'account_sha256': 'b' * 64}
        seed = b'fixture-signer-seed'
        actor = b'fixture-actor-creds'
        grant = {'publish': ['message.sent', *operator.SERVICE_ADDITIONS],
                 'subscribe': ['_INBOX.voice.messaging.>'], 'no_response': True}
        messaging = operator.service_renewal_request(actor, operator_secret, seed, receipt, grant)
        bootstrap = operator.bootstrap_renewal_request(actor, operator_secret, seed, receipt)
        self.assertEqual(messaging, {
            'schema': 'voice-nats-existing-service-renewal-v1', 'old_creds': actor.decode(),
            'account_jwt': 'fixture-account-jwt', 'operator_jwt': 'fixture-operator-jwt',
            'signer_seed': seed.decode(), 'actor_sha256': receipt['actor_sha256'],
            'account_sha256': receipt['account_sha256'], 'service': 'messaging',
            'publish': grant['publish'], 'subscribe': grant['subscribe'], 'no_response': True})
        self.assertEqual(bootstrap, {
            'schema': 'voice-nats-existing-bootstrap-renewal-v1', 'old_creds': actor.decode(),
            'account_jwt': 'fixture-account-jwt', 'operator_jwt': 'fixture-operator-jwt',
            'signer_seed': seed.decode(), 'actor_sha256': receipt['actor_sha256'],
            'account_sha256': receipt['account_sha256']})

    def bootstrap_job(self):
        return {'metadata': meta('voice-nats-analytics-chat-bootstrap', 'job-uid'),
                'spec': {'backoffLimit': 12, 'ttlSecondsAfterFinished': 86400, 'template': {'spec': {
                    'automountServiceAccountToken': False, 'restartPolicy': 'OnFailure',
                    'containers': [{'name': 'bootstrap', 'image': 'natsio/nats-box:0.18.0',
                                    'command': ['/bin/sh', '/config/bootstrap.sh'],
                                    'env': [{'name': 'NATS_URL', 'value': 'nats://voice-nats:4222'},
                                            {'name': 'NATS_CREDS', 'value': '/var/run/nats/bootstrap/bootstrap.creds'}],
                                    'resources': {'requests': {'cpu': '20m', 'memory': '64Mi'},
                                                  'limits': {'cpu': '500m', 'memory': '384Mi'}},
                                    'volumeMounts': [{'name': 'config', 'mountPath': '/config', 'readOnly': True},
                                                     {'name': 'nats-bootstrap-creds',
                                                      'mountPath': '/var/run/nats/bootstrap/bootstrap.creds',
                                                      'subPath': 'bootstrap.creds', 'readOnly': True}]}],
                    'volumes': [{'name': 'config', 'configMap': {
                        'name': 'voice-nats-analytics-chat-bootstrap', 'defaultMode': 365}},
                                {'name': 'nats-bootstrap-creds', 'secret': {
                                    'secretName': 'voice-nats-bootstrap-credentials',
                                    'defaultMode': 256,
                                    'items': [{'key': 'bootstrap.creds', 'path': 'bootstrap.creds'}]}}]}}},
                'status': {'succeeded': 1, 'active': 0}}

    def enrollment(self):
        generation = 'dev-aaaaaaaaaaaaaaaaaaaaaaaa'
        source = {'metadata': meta(operator.BOOTSTRAP_SECRET, 'bootstrap-source', '11', labels=False),
                  'data': {'bootstrap.creds': base64.b64encode(b'old-private-fixture').decode()}}
        candidate = b'new-private-fixture'
        actor_sha = 'a' * 64
        plan_sha = 'b' * 64
        enrolled = {'metadata': meta(operator.BOOTSTRAP_SECRET + '-music-' + generation, 'enrolled-uid', '12', labels=False),
                    'immutable': True, 'type': 'Opaque',
                    'data': {'bootstrap.creds': base64.b64encode(candidate).decode()}}
        enrolled['metadata']['labels'] = {**operator.LABELS, 'voice.io/dev-music-binding-id': generation}
        enrolled['metadata']['annotations'] = {
            'voice.io/dev-music-enrollment': 'voice-dev-music-bootstrap-enrollment-v1',
            'voice.io/dev-music-binding-id': generation,
            'voice.io/dev-music-namespace-uid': operator.NS_UID,
            'voice.io/dev-music-source-secret-uid': 'bootstrap-source',
            'voice.io/dev-music-source-secret-rv': '11',
            'voice.io/dev-music-actor-sha256': actor_sha,
            'voice.io/dev-music-plan-sha256': plan_sha,
        }
        journal = {'schema': 'voice-dev-music-bootstrap-enrollment-v1',
                   'baseline_binding_id': generation, 'namespace_uid': operator.NS_UID,
                   'source_secret_uid': 'bootstrap-source', 'source_secret_rv': '11',
                    'secret_name': operator.BOOTSTRAP_SECRET + '-music-' + generation,
                   'secret_uid': 'enrolled-uid', 'secret_rv': '12',
                   'actor_sha256': actor_sha, 'plan_sha256': plan_sha,
                   'credential_sha256': operator.sha(candidate)}
        return source, enrolled, candidate, journal

    def prepared_bootstrap_bundle(self, config_data):
        job = self.bootstrap_job()
        job.update(apiVersion='batch/v1', kind='Job')
        labels = {'app': 'voice-nats-analytics-chat-bootstrap', **operator.LABELS}
        job['metadata'] = {'name': job['metadata']['name'], 'namespace': 'voice-dev', 'labels': labels}
        job['spec']['suspend'] = True
        job['spec']['template']['metadata'] = {'labels': labels}
        config = {'apiVersion': 'v1', 'kind': 'ConfigMap',
                  'metadata': {'name': 'voice-nats-analytics-chat-bootstrap', 'namespace': 'voice-dev'},
                  'data': config_data}
        return {'apiVersion': 'v1', 'kind': 'List', 'items': [job, config]}, job

    def test_signed_grants_and_live_info_allow_noop_even_without_consumer(self):
        broker = {'broker_authenticated': True, 'consumer_info_authorized': True,
                  'consumer_exists': False, 'consumer_matches': False, 'outcome': 'authorized_absent'}
        self.assertEqual(operator.decision(self.full(), self.grant, broker), 'current')

    def test_only_music_info_and_ack_additions_trigger_renewal(self):
        effective = self.full()
        effective['pub']['allow'] = [x for x in effective['pub']['allow'] if x not in operator.SERVICE_ADDITIONS]
        broker = {'broker_authenticated': True, 'consumer_info_authorized': False, 'consumer_exists': False,
                  'consumer_matches': False, 'outcome': 'no_response'}
        self.assertEqual(operator.decision(effective, self.grant, broker), 'renew_music_grants')

    def test_unrelated_missing_permission_vetoes_renewal(self):
        effective = self.full()
        effective['pub']['allow'].remove('message.sent')
        broker = {'broker_authenticated': True, 'consumer_info_authorized': False, 'consumer_exists': False,
                  'consumer_matches': False, 'outcome': 'no_response'}
        with self.assertRaises(ValueError):
            operator.decision(effective, self.grant, broker)

    def test_claims_current_but_live_info_denied_is_not_renewed(self):
        broker = {'broker_authenticated': True, 'consumer_info_authorized': False, 'consumer_exists': False,
                  'consumer_matches': False, 'outcome': 'no_response'}
        with self.assertRaises(ValueError):
            operator.decision(self.full(), self.grant, broker)

    def test_bootstrap_explicitly_replays_only_when_authorized_info_proves_absence(self):
        broker = {'broker_authenticated': True, 'consumer_info_authorized': True,
                  'consumer_exists': False, 'consumer_matches': False, 'outcome': 'authorized_absent'}
        job = self.bootstrap_job()
        self.assertEqual(operator.bootstrap_action(self.full_bootstrap(), self.grants['bootstrap'], broker,
                                                    config_matches=True, job=job, prepared_available=True),
                         'replay_exact_bootstrap_job')
        self.assertTrue(operator.validate_bootstrap_job(job, 'dev-aaaaaaaaaaaaaaaaaaaaaaaa',
                                                       {'metadata': meta('voice-nats-bootstrap-credentials', 'bootstrap-uid')},
                                                       True))

    def test_bootstrap_adds_only_missing_create_grant_before_replay(self):
        broker = {'broker_authenticated': True, 'consumer_info_authorized': True,
                  'consumer_exists': False, 'consumer_matches': False, 'outcome': 'authorized_absent'}
        effective = self.full_bootstrap()
        effective['pub']['allow'] = []
        self.assertEqual(operator.bootstrap_action(effective, self.grants['bootstrap'], broker,
                                                   config_matches=True, job=self.bootstrap_job(), prepared_available=True),
                         'renew_bootstrap_then_replay')

    def test_bootstrap_mode_noops_when_consumer_exists(self):
        broker = {'broker_authenticated': True, 'consumer_info_authorized': True,
                  'consumer_exists': True, 'consumer_matches': True, 'outcome': 'authorized_present'}
        self.assertEqual(operator.bootstrap_action(self.full_bootstrap(), self.grants['bootstrap'], broker,
                                                    config_matches=True, job=self.bootstrap_job(), prepared_available=True),
                         'consumer_already_present')

    def test_bootstrap_can_replay_after_completed_job_ttl_removed_it(self):
        broker = {'broker_authenticated': True, 'consumer_info_authorized': True,
                  'consumer_exists': False, 'consumer_matches': False, 'outcome': 'authorized_absent'}
        self.assertEqual(operator.bootstrap_action(self.full_bootstrap(), self.grants['bootstrap'], broker,
                                                   config_matches=True, job=None, prepared_available=True),
                         'replay_exact_bootstrap_job')
        with self.assertRaisesRegex(ValueError, 'bootstrap_prepared_job_missing'):
            operator.bootstrap_action(self.full_bootstrap(), self.grants['bootstrap'], broker,
                                      config_matches=True, job=None, prepared_available=False)

    def test_existing_mismatched_consumer_is_never_overwritten(self):
        broker = {'broker_authenticated': True, 'consumer_info_authorized': True,
                  'consumer_exists': True, 'consumer_matches': False, 'outcome': 'authorized_present'}
        with self.assertRaisesRegex(ValueError, 'music_consumer_config_mismatch'):
            operator.bootstrap_action(self.full_bootstrap(), self.grants['bootstrap'], broker,
                                      config_matches=True, job=self.bootstrap_job(), prepared_available=True)

    def test_bootstrap_mode_fails_closed_on_config_drift_or_unknown_absence(self):
        broker = {'broker_authenticated': True, 'consumer_info_authorized': True,
                  'consumer_exists': False, 'consumer_matches': False, 'outcome': 'authorized_absent'}
        with self.assertRaisesRegex(ValueError, 'bootstrap_source_config_mismatch'):
            operator.bootstrap_action(self.full_bootstrap(), self.grants['bootstrap'], broker,
                                      config_matches=False, job=self.bootstrap_job(), prepared_available=True)
        broker['consumer_info_authorized'] = False
        broker['outcome'] = 'no_response'
        with self.assertRaisesRegex(ValueError, 'bootstrap_live_info_not_proven'):
            operator.bootstrap_action(self.full_bootstrap(), self.grants['bootstrap'], broker,
                                      config_matches=True, job=self.bootstrap_job(), prepared_available=True)

    def test_bootstrap_mode_refuses_running_job(self):
        broker = {'broker_authenticated': True, 'consumer_info_authorized': True,
                  'consumer_exists': False, 'consumer_matches': False, 'outcome': 'authorized_absent'}
        job = self.bootstrap_job()
        job['status'] = {'active': 1}
        with self.assertRaisesRegex(ValueError, 'bootstrap_job_already_active'):
            operator.bootstrap_action(self.full_bootstrap(), self.grants['bootstrap'], broker,
                                      config_matches=True, job=job, prepared_available=True)

    def test_bootstrap_replay_rejects_mutated_command_and_wrong_secret(self):
        job = self.bootstrap_job()
        job['spec']['template']['spec']['containers'][0]['command'] = ['/bin/sh', '-c', 'arbitrary']
        bootstrap = {'metadata': meta('voice-nats-bootstrap-credentials', 'bootstrap-uid')}
        with self.assertRaisesRegex(ValueError, 'bootstrap_job_command_contract_changed'):
            operator.validate_bootstrap_job(job, 'dev-aaaaaaaaaaaaaaaaaaaaaaaa', bootstrap, True)

    def test_bootstrap_replay_revalidates_after_name_lookup_before_create(self):
        import stat
        from types import SimpleNamespace

        source = {'metadata': meta(operator.BOOTSTRAP_SECRET, 'bootstrap-source')}
        state = {
            'namespace': {'metadata': meta('voice-dev', operator.NS_UID, ns=False)},
            'marker': None,
            'operator': {'metadata': meta('voice-nats-operator', 'operator')},
            'service': {'metadata': meta('voice-nats-service-credentials', 'service')},
            'bootstrap_source': source,
            'bootstrap': copy.deepcopy(source),
            'deployment': {'metadata': meta('voice-messaging', 'deployment')},
            'nats_service': {'metadata': meta('voice-nats', 'service')},
            'hub_config': {'metadata': meta('voice-nats-hub-config', 'hub-config')},
            'nats_workload': {'metadata': meta(operator.NATS_DEPLOYMENT, 'nats-deployment')},
            'nats_replica_set': {'metadata': meta(operator.NATS_DEPLOYMENT + '-abc', 'nats-rs')},
            'bootstrap_job': None,
            'bootstrap_config': {'metadata': meta('voice-nats-analytics-chat-bootstrap', 'config')},
            'bootstrap_config_data': {'bootstrap.sh': operator.MUSIC_BOOTSTRAP_CONTRACT},
            'bootstrap_config_enrollment': {'metadata': meta(
                operator.bootstrap_configmap_name('dev-aaaaaaaaaaaaaaaaaaaaaaaa', 'a' * 64), 'enrolled-config')},
            'nats_pod': {'metadata': meta('voice-nats-0', 'pod')},
            'profile': operator.PROFILE,
            'baseline_binding_id': 'dev-aaaaaaaaaaaaaaaaaaaaaaaa',
            'bootstrap_job_source': self.bootstrap_job(),
            'bootstrap_config_target_available': True,
        }
        acl = b'canonical fixture ACL'
        issuer_sha = 'a' * 64
        probe_sha = operator.sha(b'probe')
        prepared_sha = operator.sha(b'prepared')
        digest, _ = operator.snapshot_hash(state, acl, issuer_sha)

        class File:
            def __init__(self, data):
                self.data = data

            def read_bytes(self):
                return self.data

            def lstat(self):
                return SimpleNamespace(st_mode=stat.S_IFREG | 0o600, st_uid=0, st_nlink=1)

        drifts = {
            'namespace_uid': lambda row: row['namespace']['metadata'].__setitem__('uid', 'other-namespace'),
            'baseline_binding_id': lambda row: row.__setitem__('baseline_binding_id', 'dev-bbbbbbbbbbbbbbbbbbbbbbbb'),
            'generation_marker_appeared': lambda row: row.__setitem__(
                'marker', {'metadata': meta('voice-nats-generation', 'unexpected-marker')}),
            'source_secret_uid': lambda row: row['bootstrap_source']['metadata'].__setitem__('uid', 'other-source'),
            'source_secret_rv': lambda row: row['bootstrap_source']['metadata'].__setitem__('resourceVersion', '2'),
            'selected_secret_uid': lambda row: row['bootstrap']['metadata'].__setitem__('uid', 'other-selected'),
            'selected_secret_rv': lambda row: row['bootstrap']['metadata'].__setitem__('resourceVersion', '2'),
            'configmap_uid': lambda row: row['bootstrap_config']['metadata'].__setitem__('uid', 'other-config'),
            'configmap_rv': lambda row: row['bootstrap_config']['metadata'].__setitem__('resourceVersion', '2'),
        }

        for name, change in drifts.items():
            with self.subTest(drift=name):
                events = []
                changed = copy.deepcopy(state)
                change(changed)

                class Kube:
                    def get(self, kind, resource_name, optional=False):
                        events.append(('get', kind, resource_name))
                        return None

                    def run(self, action, *args, **kwargs):
                        events.append((action, *args))
                        return ''

                def revalidate():
                    return operator.require_current(digest, state, issuer_sha, probe_sha, prepared_sha)

                with patch.object(operator, 'capture', return_value=changed), \
                     patch.object(operator, 'ACL', File(acl)), \
                     patch.object(operator, 'PROBE', File(b'probe')), \
                     patch.object(operator, 'private_file', return_value=b'prepared'), \
                     patch.object(operator, 'Kube', return_value=Kube()), \
                     patch.object(operator, 'validate_bootstrap_configmap_enrollment', return_value=True), \
                     patch.object(operator, 'validate_bootstrap_job', return_value=True):
                    with self.assertRaisesRegex(ValueError, 'operator_snapshot_changed'):
                        operator.replay_bootstrap_job(
                            state, 'c' * 64, 'nats-box:0.18.0', revalidate)

                self.assertEqual(events, [
                    ('get', 'job', 'voice-nats-analytics-chat-music-' + 'c' * 12),
                ])

    def test_bootstrap_enrollment_requires_immutable_generation_and_private_journal_binding(self):
        source, enrolled, _, journal = self.enrollment()
        with patch.object(operator, 'private_file', return_value=operator.canonical(journal)):
            self.assertTrue(operator.validate_bootstrap_enrollment(enrolled, source, 'dev-aaaaaaaaaaaaaaaaaaaaaaaa'))
        wrong = copy.deepcopy(enrolled)
        wrong['metadata']['annotations']['voice.io/dev-music-source-secret-rv'] = '10'
        with patch.object(operator, 'private_file', return_value=operator.canonical(journal)):
            with self.assertRaisesRegex(ValueError, 'bootstrap_enrollment_binding_invalid'):
                operator.validate_bootstrap_enrollment(wrong, source, 'dev-aaaaaaaaaaaaaaaaaaaaaaaa')

    def test_bootstrap_enrollment_rejects_digest_or_identity_change_in_private_journal(self):
        source, enrolled, _, journal = self.enrollment()
        journal['credential_sha256'] = 'c' * 64
        with patch.object(operator, 'private_file', return_value=operator.canonical(journal)):
            with self.assertRaisesRegex(ValueError, 'bootstrap_enrollment_binding_invalid'):
                operator.validate_bootstrap_enrollment(enrolled, source, 'dev-aaaaaaaaaaaaaaaaaaaaaaaa')

    def test_bootstrap_enrollment_transition_preserves_source_and_other_state(self):
        source, enrolled, candidate, journal = self.enrollment()
        before = {key: None for key in ('namespace', 'marker', 'operator', 'service', 'deployment',
                                        'nats_service', 'hub_config', 'nats_workload', 'nats_replica_set',
                                        'bootstrap_job', 'bootstrap_config', 'bootstrap_config_enrollment', 'nats_pod')}
        before.update(profile=operator.PROFILE, baseline_binding_id='dev-aaaaaaaaaaaaaaaaaaaaaaaa',
                      nats_replica_set=None, bootstrap_source=source, bootstrap=source)
        after = dict(before, bootstrap=enrolled)
        with patch.object(operator, 'private_file', return_value=operator.canonical(journal)):
            operator.assert_bootstrap_transition(before, after, candidate, 'a' * 64, 'b' * 64)
        changed = dict(after, service={'uid': 'unexpected'})
        with patch.object(operator, 'private_file', return_value=operator.canonical(journal)):
            with self.assertRaisesRegex(ValueError, 'bootstrap_unrelated_state_changed'):
                operator.assert_bootstrap_transition(before, changed, candidate, 'a' * 64, 'b' * 64)

    def test_bootstrap_enrollment_uses_create_only_and_checks_readback_before_journal(self):
        source, _, candidate, _ = self.enrollment()
        state = {'bootstrap_source': source, 'bootstrap': source, 'baseline_binding_id': 'dev-aaaaaaaaaaaaaaaaaaaaaaaa'}
        created = {}
        commands = []

        class CreateOnlyKube:
            def run(self, *args, body=None, timeout=20):
                commands.append(args[0])
                if args[0] != 'create' or created:
                    raise ValueError('already_exists')
                row = json.loads(body)
                row['metadata'].update(uid='enrolled-uid', resourceVersion='12')
                created['secret'] = row

            def get(self, kind, name, optional=False):
                return copy.deepcopy(created['secret'])

        with patch.object(operator, 'Kube', CreateOnlyKube), \
             patch.object(operator, 'write_bootstrap_backup', return_value=Path('backup.json')), \
             patch.object(operator, 'write_bootstrap_enrollment_journal', return_value={}) as journal, \
             patch.object(operator, 'validate_bootstrap_enrollment', return_value=True):
            backup = operator.create_bootstrap_enrollment(state, candidate, 'a' * 64, 'b' * 64)
        self.assertEqual(backup, Path('backup.json'))
        self.assertEqual(commands, ['create'])
        journal.assert_called_once()
        enrolled = journal.call_args.args[1]
        self.assertEqual(enrolled['metadata']['name'], operator.BOOTSTRAP_SECRET + '-music-dev-aaaaaaaaaaaaaaaaaaaaaaaa')
        self.assertEqual(base64.b64decode(enrolled['data']['bootstrap.creds']), candidate)
        self.assertTrue(enrolled['immutable'])

        class CorruptReadbackKube(CreateOnlyKube):
            def run(self, *args, body=None, timeout=20):
                row = json.loads(body)
                row['metadata'].update(uid='enrolled-uid', resourceVersion='12')
                created['secret'] = row

            def get(self, kind, name, optional=False):
                row = copy.deepcopy(created['secret'])
                row['data']['bootstrap.creds'] = base64.b64encode(b'wrong').decode()
                return row

        with patch.object(operator, 'Kube', CorruptReadbackKube), \
             patch.object(operator, 'write_bootstrap_backup', return_value=Path('backup.json')), \
             patch.object(operator, 'write_bootstrap_enrollment_journal') as journal:
            with self.assertRaisesRegex(ValueError, 'bootstrap_enrollment_readback_mismatch'):
                operator.create_bootstrap_enrollment(state, candidate, 'a' * 64, 'b' * 64)
            journal.assert_not_called()
        source['metadata']['resourceVersion'] = '12'
        _, enrolled, _, journal = self.enrollment()
        with patch.object(operator, 'private_file', return_value=operator.canonical(journal)):
            with self.assertRaisesRegex(ValueError, 'bootstrap_enrollment_binding_invalid'):
                operator.validate_bootstrap_enrollment(enrolled, source, 'dev-aaaaaaaaaaaaaaaaaaaaaaaa')

    def test_prepared_bootstrap_is_exact_source_config_and_job_contract(self):
        base_config = {'bootstrap.sh': 'previous deployed bootstrap source'}
        target_config = {'bootstrap.sh': 'new canonical source\n' + operator.MUSIC_BOOTSTRAP_CONTRACT}
        bundle, job = self.prepared_bootstrap_bundle(base_config)
        raw = operator.canonical(bundle)
        with patch.object(operator, 'private_file', return_value=raw):
            prepared, image, digest, target = operator.prepared_bootstrap_job(
                base_config, 'dev-aaaaaaaaaaaaaaaaaaaaaaaa', target_config)
        self.assertEqual(prepared, job)
        self.assertEqual(image, 'natsio/nats-box:0.18.0')
        self.assertEqual(digest, operator.sha(raw))
        self.assertEqual(target, target_config)
        changed = copy.deepcopy(bundle)
        changed['items'][0]['spec']['template']['spec']['volumes'][1]['secret']['secretName'] = 'foreign-secret'
        with patch.object(operator, 'private_file', return_value=operator.canonical(changed)):
            with self.assertRaisesRegex(ValueError, 'bootstrap_prepared_job_contract_invalid'):
                operator.prepared_bootstrap_job(base_config, 'dev-aaaaaaaaaaaaaaaaaaaaaaaa', target_config)
        extra = copy.deepcopy(bundle)
        extra['items'].append({'kind': 'Secret', 'metadata': {'name': 'unexpected'}})
        with patch.object(operator, 'private_file', return_value=operator.canonical(extra)):
            with self.assertRaisesRegex(ValueError, 'bootstrap_prepared_source_mismatch'):
                operator.prepared_bootstrap_job(base_config, 'dev-aaaaaaaaaaaaaaaaaaaaaaaa', target_config)

    def test_capture_binds_exact_dev_namespace_generation_and_messaging_mount(self):
        objects = fixture()
        state = operator.capture(FakeKube(objects))
        self.assertEqual(state['profile'], operator.PROFILE)
        self.assertIsNone(state['marker'])
        self.assertTrue(state['baseline_binding_id'].startswith('dev-'))
        self.assertIsNone(state['bootstrap'])
        self.assertIsNone(state['bootstrap_job'])

    def test_wrong_namespace_uid_is_refused(self):
        objects = fixture()
        objects[('namespace', 'voice-dev')]['metadata']['uid'] = 'foreign'
        with self.assertRaisesRegex(ValueError, 'voice_dev_namespace_uid_changed'):
            operator.capture(FakeKube(objects))

    def test_pending_generation_marker_is_refused(self):
        objects = fixture()
        objects[('configmap', 'voice-nats-generation')] = {
            'metadata': meta('voice-nats-generation', 'unexpected-marker'),
            'data': {'generation': 'not-adopted', 'phase': 'active'}}
        with self.assertRaisesRegex(ValueError, 'dev_profile_generation_marker_present'):
            operator.capture(FakeKube(objects))

    def test_current_dev_profile_rejects_changed_service_selector(self):
        objects = fixture()
        objects[('service', 'voice-nats')]['spec']['selector'] = {'app': 'voice-nats'}
        with self.assertRaisesRegex(ValueError, 'dev_profile_nats_service_selector_changed'):
            operator.capture(FakeKube(objects))

    def test_current_dev_profile_rejects_unowned_replica_set_and_unpinned_pod(self):
        objects = fixture()
        rs = objects[('replicaset', operator.NATS_DEPLOYMENT + '-abc')]
        rs['metadata']['ownerReferences'][0]['uid'] = 'foreign-deployment'
        with self.assertRaisesRegex(ValueError, 'dev_profile_nats_replica_set_owner_changed'):
            operator.capture(FakeKube(objects))

        objects = fixture()
        selector = ','.join(f'{k}={v}' for k, v in operator.NATS_SELECTOR.items())
        objects[('pods', selector)][0]['status']['containerStatuses'][0]['imageID'] = 'sha256:' + 'b' * 64
        with self.assertRaisesRegex(ValueError, 'dev_profile_nats_pod_image_changed'):
            operator.capture(FakeKube(objects))

    def test_baseline_binding_changes_with_stable_owner_or_account_identity(self):
        original = fixture()
        baseline = operator.capture(FakeKube(original))['baseline_binding_id']
        changed = copy.deepcopy(original)
        changed[('service', 'voice-nats')]['metadata']['uid'] = 'replacement-service'
        self.assertNotEqual(baseline, operator.capture(FakeKube(changed))['baseline_binding_id'])
        changed = copy.deepcopy(original)
        changed[('deployment', operator.NATS_DEPLOYMENT)]['metadata']['uid'] = 'replacement-deployment'
        changed[('replicaset', operator.NATS_DEPLOYMENT + '-abc')]['metadata']['ownerReferences'][0]['uid'] = 'replacement-deployment'
        self.assertNotEqual(baseline, operator.capture(FakeKube(changed))['baseline_binding_id'])
        changed = copy.deepcopy(original)
        changed[('secret', 'voice-nats-operator')]['data']['account.public'] = base64.b64encode(b'OTHERKEY').decode()
        self.assertNotEqual(baseline, operator.capture(FakeKube(changed))['baseline_binding_id'])

    def test_wrong_service_secret_key_mount_is_refused(self):
        objects = fixture()
        deployment = objects[('deployment', 'voice-messaging')]
        deployment['spec']['template']['spec']['volumes'][0]['secret']['secretName'] = 'foreign'
        with self.assertRaisesRegex(ValueError, 'messaging_leaf_mount_changed'):
            operator.capture(FakeKube(objects))

    def test_snapshot_changes_on_secret_rv_or_acl_intent(self):
        objects = fixture()
        state = operator.capture(FakeKube(objects))
        acl = operator.ACL.read_bytes()
        before, _ = operator.snapshot_hash(state, acl, 'a' * 64)
        objects[('secret', 'voice-nats-service-credentials')]['metadata']['resourceVersion'] = '2'
        changed_state = operator.capture(FakeKube(objects))
        changed, _ = operator.snapshot_hash(changed_state, acl, 'a' * 64)
        self.assertNotEqual(before, changed)
        changed, _ = operator.snapshot_hash(state, acl + b'\n# changed', 'a' * 64)
        self.assertNotEqual(before, changed)

    def test_stable_binding_allows_only_replica_scale_transition(self):
        objects = fixture()
        state = operator.capture(FakeKube(objects))
        acl = operator.ACL.read_bytes()
        before = operator.stable_hash(state, acl, 'b' * 64)
        scaled = copy.deepcopy(state)
        scaled['deployment']['spec']['replicas'] = 0
        scaled['deployment']['metadata']['resourceVersion'] = '2'
        self.assertEqual(before, operator.stable_hash(scaled, acl, 'b' * 64))
        scaled['deployment']['spec']['template']['spec']['volumes'][0]['secret']['secretName'] = 'foreign'
        self.assertNotEqual(before, operator.stable_hash(scaled, acl, 'b' * 64))


if __name__ == '__main__':
    unittest.main()
