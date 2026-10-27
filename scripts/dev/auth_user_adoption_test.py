import json
import copy
import os
import pathlib
import tempfile
import unittest
from unittest.mock import Mock
from types import SimpleNamespace

from auth_user_adoption import (Journal, Operator, STEPS, RecoveryRequired,
                               deployment_plan, operation_secret, sql_phases)
from auth_user_adoption import Kube, terminal_job, old_incarnations_drained


class Fixture(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.path = pathlib.Path(self.tmp.name) / 'operation'
        self.intent = {'operation': 'owned-1', 'namespace_uid': 'namespace-original',
                       'resource_claim': 'claim-original', 'generation': 26,
                       'source': 'f' * 40, 'candidate': {'auth': 'repo/auth@sha256:' + 'a' * 64}}

    def open(self, intent=None):
        j = Journal(self.path, self.intent if intent is None else intent)
        self.addCleanup(j.close)
        return j


class JournalTest(Fixture):

    def test_resume_keeps_exact_credentials_and_intent(self):
        first = self.open()
        saved = first.state['credentials'].copy()
        first.close()
        second = self.open()
        self.assertTrue(saved == second.state['credentials'], 'resume must reuse credentials')
        self.assertTrue(saved['auth']!=saved['user'],'runtime credentials must be independent')

    def test_private_modes(self):
        self.open()
        self.assertEqual(0o700, self.path.stat().st_mode & 0o777)
        self.assertEqual(0o600, (self.path / 'state.json').stat().st_mode & 0o777)

    def test_drift_refuses_without_rewriting_private_state(self):
        j = self.open()
        j.close()
        before = (self.path / 'state.json').read_bytes()
        changed = dict(self.intent, namespace_uid='foreign')
        with self.assertRaises(RecoveryRequired):
            self.open(changed)
        self.assertEqual(before, (self.path / 'state.json').read_bytes())

    def test_symlink_journal_does_not_touch_target(self):
        target = pathlib.Path(self.tmp.name) / 'target'
        target.mkdir()
        self.path.symlink_to(target, target_is_directory=True)
        with self.assertRaises(RecoveryRequired):
            self.open()
        self.assertEqual([], list(target.iterdir()))

    def test_concurrent_operator_is_refused(self):
        self.open()
        with self.assertRaises(RecoveryRequired):
            self.open()


class FakeBackend:
    def __init__(self):
        self.actual = {}
        self.effects = []
        self.crash = None
        self.drift = False

    def guard(self, state):
        if self.drift:
            raise RecoveryRequired('baseline changed')

    def reconcile(self, step, state):
        return self.actual.get(step, {'status': 'not_started'})

    def execute(self, step, state):
        self.effects.append(step)
        self.actual[step] = {'status': 'complete', 'receipt': {'identity': step + '-owned'}}
        if self.crash == step:
            raise RuntimeError('simulated lost response containing private values')


class OperatorTest(Fixture):
    def test_aborted_journal_cannot_start_any_more_effect(self):
        journal=self.open();journal.state['aborted']=True;journal.save()
        backend=FakeBackend()
        with self.assertRaises(RecoveryRequired): Operator(journal,backend).run()
        self.assertEqual([],backend.effects)

    def test_crash_after_each_effect_resume_never_repeats_effect(self):
        for step in STEPS:
            with self.subTest(step=step), tempfile.TemporaryDirectory() as tmp:
                j = Journal(pathlib.Path(tmp) / 'op', self.intent)
                backend = FakeBackend()
                backend.crash = step
                with self.assertRaises(RecoveryRequired):
                    Operator(j, backend).run()
                self.assertEqual('pending', j.state['steps'][step]['status'])
                passwords = j.state['credentials'].copy()
                j.close()
                j = Journal(pathlib.Path(tmp) / 'op', self.intent)
                self.addCleanup(j.close)
                backend.crash = None
                Operator(j, backend).run()
                self.assertEqual(list(STEPS), backend.effects)
                self.assertTrue(passwords == j.state['credentials'])
                self.assertTrue(all(s['status'] == 'done' for s in j.state['steps'].values()))

    def test_no_effect_on_guard_drift(self):
        j = self.open()
        backend = FakeBackend()
        backend.drift = True
        with self.assertRaises(RecoveryRequired):
            Operator(j, backend).run()
        self.assertEqual([], backend.effects)

    def test_uncertain_absent_effect_is_not_retried(self):
        j = self.open()
        j.state['steps']['lock'] = {'status': 'pending'}
        j.save()
        backend = FakeBackend()
        with self.assertRaises(RecoveryRequired):
            Operator(j, backend).run()
        self.assertEqual([], backend.effects)

    def test_running_or_foreign_complete_is_not_adopted(self):
        for status in ['running', 'complete', 'conflict']:
            with self.subTest(status=status):
                j = self.open()
                backend = FakeBackend()
                backend.actual['lock'] = {'status': status, 'receipt': {'identity': 'foreign'}}
                with self.assertRaises(RecoveryRequired):
                    Operator(j, backend).run()
                self.assertEqual([], backend.effects)
                j.close()

    def test_completed_receipt_drift_prevents_more_effects(self):
        j = self.open()
        backend = FakeBackend()
        backend.crash = 'migrate_auth'
        with self.assertRaises(RecoveryRequired):
            Operator(j, backend).run()
        backend.actual['backup']['receipt'] = {'identity': 'changed'}
        backend.crash = None
        effects = list(backend.effects)
        with self.assertRaises(RecoveryRequired):
            Operator(j, backend).run()
        self.assertEqual(effects, backend.effects)


class MutationPlanTest(unittest.TestCase):
    def deployment(self, service):
        env = ([{'name': 'SPRING_DATASOURCE_USERNAME', 'value': 'voice'},
                {'name': 'SPRING_DATASOURCE_PASSWORD', 'valueFrom': {'secretKeyRef': {'name': 'old', 'key': 'POSTGRES_PASSWORD'}}},
                {'name': 'AUTH_FLYWAY_ENABLED', 'value': 'false'}] if service == 'auth' else
               [{'name': 'DATABASE_URL', 'valueFrom': {'secretKeyRef': {'name': 'old', 'key': 'USER_DATABASE_URL'}}}])
        env += [{'name': 'SPACE_GRPC_ADDR', 'value': 'voice-space:9090'}]
        if service=='auth': env.append({'name':'SPRING_DATASOURCE_URL','value':'jdbc:postgresql://voice-postgres:5432/auth_db'})
        return {'kind': 'Deployment', 'metadata': {'namespace': 'voice-dev', 'name': 'voice-' + service,
                'uid': service + '-uid', 'resourceVersion': 'original', 'labels': {'voice.environment': 'dev', 'app.kubernetes.io/managed-by': 'voice-dev'}},
                'spec': {'replicas': 1, 'template': {'spec': {'volumes': [{'name': 'keys', 'secret': {'secretName': 'old', 'items': [{'key': 'jwt.key', 'path': 'jwt.key'}]}}],
                 'containers': [{'name': 'nats-leaf', 'image': 'nats:old'}, {'name': service, 'image': 'old/' + service, 'env': env}]}}}}

    def test_patch_is_fullspec_cas_and_changes_only_own_fields(self):
        for service in ['auth', 'user']:
            with self.subTest(service=service):
                before = self.deployment(service)
                patch, after = deployment_plan(before, service, 'repo/' + service + '@sha256:' + 'a' * 64, 'owned-secret')
                self.assertIn({'op': 'test', 'path': '/spec', 'value': before['spec']}, patch)
                self.assertIn({'op': 'test', 'path': '/metadata/uid', 'value': before['metadata']['uid']}, patch)
                self.assertIn({'op': 'test', 'path': '/metadata/resourceVersion', 'value': 'original'}, patch)
                self.assertEqual(before['spec']['template']['spec']['volumes'], after['spec']['template']['spec']['volumes'])
                self.assertEqual(before['spec']['template']['spec']['containers'][0], after['spec']['template']['spec']['containers'][0])
                main = after['spec']['template']['spec']['containers'][1]
                refs = [e['valueFrom']['secretKeyRef'] for e in main['env'] if e.get('valueFrom', {}).get('secretKeyRef')]
                self.assertEqual({'AUTH_DB_USERNAME', 'AUTH_DB_PASSWORD'} if service == 'auth' else {'USER_DATABASE_URL'}, {e['key'] for e in refs})
                self.assertTrue(all(e['name'] == 'owned-secret' for e in refs))
                self.assertIn({'name': 'SPACE_GRPC_ADDR', 'value': 'voice-space:9090'}, main['env'])

    def test_foreign_or_broad_secret_or_flyway_on_refused(self):
        for mutation in ['namespace', 'envFrom', 'flyway', 'image','user_db','shared_secret_mount']:
            before = self.deployment('auth')
            image = 'repo/auth@sha256:' + 'a' * 64
            if mutation == 'namespace': before['metadata']['namespace'] = 'voice-staging'
            if mutation == 'envFrom': before['spec']['template']['spec']['containers'][1]['envFrom'] = [{'secretRef': {'name': 'old'}}]
            if mutation == 'flyway': before['spec']['template']['spec']['containers'][1]['env'][2]['value'] = 'true'
            if mutation == 'image': image = 'repo/auth:mutable'
            if mutation == 'user_db': before['spec']['template']['spec']['containers'][1]['env'].append({'name':'USER_DATABASE_URL','valueFrom':{'secretKeyRef':{'name':'old','key':'USER_DATABASE_URL'}}})
            if mutation == 'shared_secret_mount': before['spec']['template']['spec']['volumes'][0]['secret']={'secretName':'voice-app-secrets'}
            with self.subTest(mutation=mutation), self.assertRaises(RecoveryRequired):
                deployment_plan(before, 'auth', image, 'owned-secret')

    def test_secret_is_once_immutable_runtime_only(self):
        secret = operation_secret('owned-1', {'auth': 'a' * 40, 'user': 'b' * 40})
        self.assertTrue(secret['immutable'])
        self.assertEqual({'AUTH_DB_USERNAME', 'AUTH_DB_PASSWORD', 'USER_DATABASE_URL', 'USER_DB_PASSWORD'}, set(secret['data']))
        self.assertNotIn('POSTGRES_PASSWORD', secret['data'])

    def test_strict_roles_have_no_password_alter_and_grants_keep_barrier(self):
        phases = sql_phases(pathlib.Path(__file__).resolve().parents[2])
        self.assertNotIn('ALTER ROLE', phases['roles'])
        self.assertNotIn('WHERE NOT EXISTS', phases['roles'])
        self.assertIn('CREATE ROLE', phases['roles'])
        self.assertIn('auth_identity_ddl_barrier', phases['grants_auth'])
        self.assertIn('REVOKE ALL ON TABLE public.auth_identity_ddl_barrier FROM auth_runtime', phases['grants_auth'])
        self.assertNotIn('user_runtime', phases['grants_auth'])
        self.assertNotIn('auth_runtime', phases['grants_user'])


class TransportTest(unittest.TestCase):
    def test_context_namespace_deadline_and_secret_stdin(self):
        runner = Mock(return_value=SimpleNamespace(returncode=0, stdout='{}', stderr=''))
        Kube(runner).call('create', '-f', '-', '-o', 'json', body={'secret': 'private-marker'})
        args, kwargs = runner.call_args
        self.assertEqual(['kubectl', '--kubeconfig', '/home/pmd/.kube/config', '--context', 'default',
                          '--namespace', 'voice-dev', '--request-timeout=15s'], args[0][:8])
        self.assertNotIn('private-marker', ' '.join(args[0]))
        self.assertIn('private-marker', kwargs['input'])
        self.assertEqual(20, kwargs['timeout'])

    def test_error_never_propagates_raw_stderr(self):
        runner = Mock(return_value=SimpleNamespace(returncode=1, stdout='', stderr='private-marker'))
        with self.assertRaises(RecoveryRequired) as error:
            Kube(runner).call('get', 'secret', 'owned')
        self.assertNotIn('private-marker', str(error.exception))

    def test_patch_payload_uses_private_file_not_arguments(self):
        def run(command, **kwargs):
            patch = next(arg.split('=', 1)[1] for arg in command if arg.startswith('--patch-file='))
            self.assertEqual(0o600, os.stat(patch).st_mode & 0o777)
            self.assertIn('private-marker', pathlib.Path(patch).read_text())
            self.assertNotIn('private-marker', ' '.join(command))
            self.path = pathlib.Path(patch)
            return SimpleNamespace(returncode=0, stdout='{}', stderr='')
        Kube(run).call('patch', 'deployment', 'voice-auth', '--type=json', body=[{'value': 'private-marker'}])
        self.assertFalse(self.path.exists())


class TerminalEvidenceTest(unittest.TestCase):
    def job(self):
        return {'metadata': {'uid': 'owned-job', 'annotations': {'voice.dev/adoption-operation': 'op'}},
                'spec': {'template': 'exact'}, 'status': {'succeeded': 1, 'active': 0, 'conditions': [{'type': 'Complete', 'status': 'True'}]}}

    def test_successful_job_with_server_session_is_not_terminal(self):
        job = self.job()
        result = terminal_job(job, job['spec'], 'op', [{'application': 'owned-op'}], False)
        self.assertEqual('running', result['status'])

    def test_failed_or_dirty_or_foreign_job_never_completes(self):
        for fault in ['failed', 'dirty', 'spec', 'owner', 'condition']:
            job = self.job()
            dirty = fault == 'dirty'
            if fault == 'failed': job['status'] = {'failed': 1, 'conditions': [{'type': 'Failed', 'status': 'True'}]}
            if fault == 'spec': job['spec'] = {'template': 'drifted'}
            if fault == 'owner': job['metadata']['annotations']['voice.dev/adoption-operation'] = 'foreign'
            if fault == 'condition': job['status']['conditions'] = []
            with self.subTest(fault=fault):
                self.assertNotEqual('complete', terminal_job(job, {'template': 'exact'}, 'op', [], dirty)['status'])

    def test_terminal_receipt_binds_job_uid_and_expected_spec(self):
        job = self.job()
        result = terminal_job(job, job['spec'], 'op', [], False)
        self.assertEqual('complete', result['status'])
        self.assertEqual('owned-job', result['receipt']['uid'])

    def test_old_admin_pods_replicas_and_newly_opened_old_ip_sessions_must_drain(self):
        original = {'pods': [{'uid': 'old-pod', 'ip': '10.0.0.1', 'rs_uid': 'old-rs'}],
                    'sessions': [{'pid': 17, 'backend_start': 'before', 'client_address': '10.0.0.1/32'}]}
        cases = [([{'uid': 'old-pod'}], [], []),
                 ([], [{'uid': 'old-rs', 'replicas': 1}], []),
                 ([], [], [{'pid': 17, 'backend_start': 'before', 'client_address': '10.0.0.1/32', 'role': 'voice'}]),
                 ([], [], [{'pid': 99, 'backend_start': 'later', 'client_address': '10.0.0.1/32', 'role': 'voice'}])]
        for pods, rs, sessions in cases:
            with self.subTest(pods=bool(pods), rs=bool(rs), sessions=bool(sessions)):
                self.assertFalse(old_incarnations_drained(original, pods, rs, sessions))
        self.assertTrue(old_incarnations_drained(original, [], [{'uid': 'old-rs', 'replicas': 0}],
                        [{'pid': 200, 'backend_start': 'new', 'client_address': '10.0.0.2/32', 'role': 'user_runtime'}]))


if __name__ == '__main__':
    unittest.main()
