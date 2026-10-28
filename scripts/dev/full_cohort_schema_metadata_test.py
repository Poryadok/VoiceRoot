import json
import hashlib
import sys
import subprocess
import shutil
from unittest import mock
from pathlib import Path
import unittest

import full_cohort_schema_metadata as subject


def bound(reader):
    def wrapped(*args):
        return reader(*args)
    wrapped.namespace = 'voice-dev'
    return wrapped


def runtime_bound(reader):
    wrapped = bound(reader)
    wrapped.workload = 'user'
    wrapped.pod_name = 'voice-user-abc'
    return wrapped


def dns():
    return runtime_bound(lambda _: ['10.1.0.1'])


class ResolveTest(unittest.TestCase):
    def setUp(self):
        self.pod = {'namespace': 'voice-dev', 'namespace_uid': 'namespace', 'deployment': 'voice-user',
                    'deployment_uid': 'dep', 'deployment_rv': '1', 'pod_name': 'voice-user-abc', 'pod_uid': 'pod',
                    'pod_rv': '2', 'container_id': 'container', 'restart_count': 0,
                    'container_name': 'user', 'envFrom': [{'configMapRef': {'name': 'voice-app-config'}}],
                    'env': [{'name': 'DATABASE_URL', 'valueFrom': {'secretKeyRef':
                            {'name': 'voice-app-secrets', 'key': 'USER_DATABASE_URL'}}}]}
        self.calls = []

    def read(self, kind, name, key):
        self.calls.append((kind, name, key))
        return {'uid': 'uid', 'resource_version': '1', 'value':
                'postgres://user_runtime:private-password@voice-postgres:5432/user_db?sslmode=disable'}

    def runtime(self, _name):
        return 'postgres://user_runtime:private-password@voice-postgres:5432/user_db?sslmode=disable'

    def test_explicit_overrides_envfrom_and_never_returns_secret(self):
        result = subject.resolve('user', lambda: self.pod.copy(), bound(self.read), runtime_bound(self.runtime), dns(), lambda: None)
        self.assertEqual(('user_db', 'voice-postgres', 'user_runtime'),
                         (result['database'], result['host'], result['role']))
        self.assertNotIn('private-password', json.dumps(result))
        self.assertEqual([('secret', 'voice-app-secrets', 'USER_DATABASE_URL')] * 2, self.calls)

    def test_bad_endpoint_and_stale_pod_rejected_without_value_leak(self):
        def remote(*args):
            return {'uid': 'u', 'resource_version': '1', 'value': 'postgres://u:secret@remote.example/user_db'}
        with self.assertRaises(subject.Hold) as raised:
            subject.resolve('user', lambda: self.pod.copy(), bound(remote), runtime_bound(lambda _: remote()['value']), dns(), lambda: None)
        self.assertNotIn('secret', str(raised.exception))
        calls = iter([self.pod, dict(self.pod, container_id='new')])
        with self.assertRaises(subject.Hold):
            subject.resolve('user', lambda: next(calls), bound(self.read), runtime_bound(self.runtime), dns(), lambda: None)

    def test_envfrom_last_source_wins_and_stale_reference_fails(self):
        self.pod['env'] = []
        self.pod['envFrom'] = [{'configMapRef': {'name': 'voice-app-config'}},
                               {'secretRef': {'name': 'voice-app-secrets'}}]
        def read(kind, name, key):
            self.assertEqual(key, 'DATABASE_URL')
            return {'uid': name, 'resource_version': '1', 'value':
                    'postgres://%s:password@voice-postgres/user_db' % ('second' if kind == 'secret' else 'first')}
        result = subject.resolve('user', lambda: self.pod.copy(), bound(read),
                                 runtime_bound(lambda _: read('secret', 'voice-app-secrets', 'DATABASE_URL')['value']), dns(), lambda: None)
        self.assertEqual(result['role'], 'second')
        count = 0
        def changed(*args):
            nonlocal count
            count += 1
            return {'uid': 'u', 'resource_version': str(count), 'value':
                    'postgres://user:secret@voice-postgres/user_db'}
        with self.assertRaises(subject.Hold):
            subject.resolve('user', lambda: self.pod.copy(), bound(changed),
                            runtime_bound(lambda _: 'postgres://user:secret@voice-postgres/user_db'), dns(), lambda: None)

    def test_other_namespace_endpoint_rejected_and_errors_redacted(self):
        def wrong(*args):
            return {'uid': 'u', 'resource_version': '1', 'value':
                    'postgres://user:secret@voice-postgres.voice-staging.svc/user_db'}
        with self.assertRaises(subject.Hold):
            subject.resolve('user', lambda: self.pod.copy(), bound(wrong), runtime_bound(lambda _: wrong()['value']), dns(), lambda: None)
        def broken(*args):
            raise RuntimeError('private-password')
        with self.assertRaises(subject.Hold) as error:
            subject.resolve('user', lambda: self.pod.copy(), bound(broken), runtime_bound(self.runtime), dns(), lambda: None)
        self.assertNotIn('private-password', str(error.exception))
        wrong_namespace = bound(self.read)
        wrong_namespace.namespace = 'voice-staging'
        with self.assertRaises(subject.Hold):
            subject.resolve('user', lambda: self.pod.copy(), wrong_namespace, runtime_bound(self.runtime), dns(), lambda: None)
        with self.assertRaises(subject.Hold):
            subject.resolve('user', lambda: self.pod.copy(), bound(self.read), runtime_bound(lambda _: 'stale'), dns(), lambda: None)

    def test_messaging_secondary_is_bound_to_documented_owner(self):
        self.pod.update(deployment='voice-messaging', container_name='messaging', pod_name='voice-messaging-abc')
        self.pod['env'] = [{'name': 'CHAT_DATABASE_URL', 'valueFrom': {'secretKeyRef':
                            {'name': 'voice-app-secrets', 'key': 'CHAT_DATABASE_URL'}}}]
        value = 'postgres://reader:private@voice-postgres/chat_db'
        def read(*_args):
            return {'uid': 'u', 'resource_version': '1', 'value': value}
        runtime = bound(lambda _: value)
        runtime.workload, runtime.pod_name = 'messaging', 'voice-messaging-abc'
        lookup = bound(lambda _: ['10.1.0.1'])
        lookup.workload, lookup.pod_name = 'messaging', 'voice-messaging-abc'
        result = subject.resolve('messaging', lambda: self.pod.copy(), bound(read), runtime, lookup,
                                 lambda: None, variable='CHAT_DATABASE_URL')
        self.assertEqual(result['database'], 'chat_db')

    def test_gateway_configured_database_requires_matching_source(self):
        self.pod.update(deployment='voice-gateway', container_name='gateway', pod_name='voice-gateway-abc')
        value = 'postgres://gateway:private@voice-postgres/gateway_db'
        self.pod['env'] = [{'name': 'GATEWAY_DATABASE_URL', 'valueFrom': {'secretKeyRef':
                            {'name': 'voice-app-secrets', 'key': 'GATEWAY_DATABASE_URL'}}}]
        def read(*_args):
            return {'uid': 'u', 'resource_version': '1', 'value': value}
        runtime, lookup = bound(lambda _: value), bound(lambda _: ['10.1.0.1'])
        runtime.workload = lookup.workload = 'gateway'
        runtime.pod_name = lookup.pod_name = 'voice-gateway-abc'
        binding = subject.resolve('gateway', lambda: self.pod.copy(), bound(read), runtime, lookup, lambda: None)
        self.assertEqual(binding['database'], 'gateway_db')
        with self.assertRaises(subject.Hold):
            subject.observe(subject.catalog(CatalogTest().snapshot()), binding,
                            lambda: self.fail('unexpected engine read'), 'voice',
                            lambda _: self.fail('unexpected query'), lambda: None)

    def test_auth_requires_known_spring_runtime_and_explicit_no_flyway(self):
        self.pod.update(deployment='voice-auth', container_name='auth', pod_name='voice-auth-abc', spring_config_safe=True)
        url = 'jdbc:postgresql://voice-postgres:5432/auth_db'
        self.pod['env'] = [{'name': 'SPRING_DATASOURCE_URL', 'value': url},
                           {'name': 'SPRING_DATASOURCE_USERNAME', 'value': 'auth_runtime'}]
        values = {'SPRING_DATASOURCE_URL': url, 'SPRING_DATASOURCE_USERNAME': 'auth_runtime',
                  'AUTH_FLYWAY_ENABLED': 'false'}
        runtime = bound(lambda key: values.get(key))
        runtime.workload, runtime.pod_name = 'auth', 'voice-auth-abc'
        runtime.auth_policy = lambda: True
        lookup = bound(lambda _: ['10.1.0.1'])
        lookup.workload, lookup.pod_name = 'auth', 'voice-auth-abc'
        result = subject.resolve('auth', lambda: self.pod.copy(), bound(self.read), runtime, lookup, lambda: None)
        self.assertEqual((result['database'], result['role']), ('auth_db', 'auth_runtime'))
        values['SPRING_APPLICATION_JSON'] = '{"spring":{}}'
        with self.assertRaises(subject.Hold):
            subject.resolve('auth', lambda: self.pod.copy(), bound(self.read), runtime, lookup, lambda: None)


class CatalogTest(unittest.TestCase):
    def snapshot(self):
        files = {}
        for db in subject.OWNERS.values():
            if db == 'auth_db':
                files['src/backend/auth/src/main/resources/db/migration/V1__first.sql'] = 'a' * 64
                files['src/backend/migrations/auth_db/000001_first.up.sql'] = 'a' * 64
            else:
                for direction in ('up', 'down'):
                    files['src/backend/migrations/%s/000001_first.%s.sql' % (db, direction)] = 'a' * 64
        return {'source_sha': 'b' * 40, 'git_tree_sha256': 'c' * 64, 'files': files}

    def test_catalog_requires_complete_fixed_owners_and_source_inventory(self):
        snapshot = self.snapshot()
        catalog = subject.catalog(snapshot)
        self.assertEqual(set(catalog['databases']), set(subject.OWNERS.values()))
        self.assertEqual(catalog['databases']['auth_db']['kind'], 'flyway')
        self.assertEqual(catalog['databases']['space_db']['kind'], 'space-native')
        self.assertFalse(catalog['live_schema_qualified'])
        snapshot['files'].pop('src/backend/migrations/user_db/000001_first.up.sql')
        with self.assertRaises(subject.Hold):
            subject.catalog(snapshot)

    def test_repository_inventory_shape(self):
        root = Path(__file__).resolve().parents[2]
        files = {}
        for db in subject.OWNERS.values():
            base = (root / 'src/backend/auth/src/main/resources/db/migration' if db == 'auth_db'
                    else root / 'src/backend/migrations' / db)
            for path in base.glob('*.sql'):
                files[path.relative_to(root).as_posix()] = hashlib.sha256(path.read_bytes()).hexdigest()
        for path in (root / 'src/backend/migrations/auth_db').glob('*.sql'):
            files[path.relative_to(root).as_posix()] = hashlib.sha256(path.read_bytes()).hexdigest()
        for path in (root / 'src/backend/migrations/gateway_db').glob('*.sql'):
            files[path.relative_to(root).as_posix()] = hashlib.sha256(path.read_bytes()).hexdigest()
        result = subject.catalog({'source_sha': 'b' * 40, 'git_tree_sha256': 'c' * 64, 'files': files})
        self.assertEqual(len(result['databases']), 19)

    def test_source_authority_checked_before_and_after_snapshot(self):
        calls = []
        def authority(sha):
            calls.append(sha)
            if len(calls) == 2:
                raise RuntimeError('private authority detail')
        with mock.patch('publish_candidate.source_snapshot', return_value=self.snapshot()):
            with self.assertRaises(subject.Hold) as error:
                subject.catalog_from_source('.', 'b' * 40, authority)
        self.assertEqual(len(calls), 2)
        self.assertNotIn('private authority detail', str(error.exception))


class ObserveTest(unittest.TestCase):
    def test_wrong_database_or_restarted_container_is_refused(self):
        identity = {'namespace': 'voice-dev', 'namespace_uid': 'namespace', 'pod_name': 'voice-postgres-0',
                    'pod_uid': 'pod', 'pod_rv': '1', 'container_name': 'postgres',
                    'container_id': 'container', 'restart_count': 0, 'pvc_uid': 'pvc', 'pvc_rv': '1', 'pv_uid': 'pv', 'pv_rv': '1',
                    'service_uid': 's', 'service_rv': '1', 'service_ip': '10.1.0.1', 'pod_ip': '10.0.0.1', 'endpoints_uid': 'e', 'endpoints_rv': '1', 'slices_identity': 'slice:1'}
        response = {'database': 'user_db', 'database_oid': 12, 'role': 'voice',
                    'pid': 123, 'server_start': 'start', 'read_only': 'on', 'data_directory': '/var/lib/postgresql/data',
                    'version': 1, 'dirty': False, 'ledger_rows': 1}
        source = CatalogTest().snapshot()
        binding = {'database': 'user_db', 'state': 'resolved', 'addresses': ['10.1.0.1'], 'binding': {'namespace': 'voice-dev', 'namespace_uid': 'namespace'}}
        def run(_args):
            return json.dumps(response)
        result = subject.observe(subject.catalog(source), binding, lambda: identity.copy(), 'voice', run, lambda: None)
        self.assertEqual(result['ledger']['version'], 1)
        self.assertFalse(result['startup_qualified'])
        response['database'] = 'other_db'
        with self.assertRaises(subject.Hold):
            subject.observe(subject.catalog(source), binding, lambda: identity.copy(), 'voice', run, lambda: None)
        response['database'] = 'user_db'
        states = iter([identity, dict(identity, container_id='replacement')])
        with self.assertRaises(subject.Hold):
            subject.observe(subject.catalog(source), binding, lambda: next(states), 'voice', run, lambda: None)
        for changes in ({'dirty': True}, {'version': 99}, {'role': 'wrong'}, {'pid': 0}, {'ledger_rows': 0}):
            with self.subTest(changes=changes), self.assertRaises(subject.Hold):
                subject.observe(subject.catalog(source), binding, lambda: identity.copy(), 'voice',
                                lambda _: json.dumps(dict(response, **changes)), lambda: None)
        with self.assertRaises(subject.Hold):
            subject.observe(subject.catalog(source), dict(binding, addresses=['10.2.0.1']),
                            lambda: identity.copy(), 'voice', run, lambda: None)
        checks = 0
        def lost():
            nonlocal checks
            checks += 1
            if checks == 3:
                raise RuntimeError('lease lost')
        with self.assertRaises(subject.Hold):
            subject.observe(subject.catalog(source), binding, lambda: identity.copy(), 'voice', run, lost)

    def test_flyway_rejects_unrepresented_version_and_mixed_ledger(self):
        identity = {'namespace': 'voice-dev', 'namespace_uid': 'n', 'pod_name': 'voice-postgres-0',
                    'pod_uid': 'p', 'pod_rv': '1', 'container_name': 'postgres', 'container_id': 'c',
                    'restart_count': 0, 'pvc_uid': 'v', 'pvc_rv': '1', 'pv_uid': 'pv', 'pv_rv': '1', 'service_uid': 's',
                    'service_rv': '1', 'service_ip': '10.1.0.1', 'pod_ip': '10.0.0.1', 'endpoints_uid': 'e', 'endpoints_rv': '1', 'slices_identity': 'slice:1'}
        binding = {'database': 'auth_db', 'state': 'resolved', 'addresses': ['10.1.0.1'], 'binding': {'namespace': 'voice-dev', 'namespace_uid': 'n'}}
        row = {'database': 'auth_db', 'database_oid': 1, 'role': 'voice', 'pid': 1,
               'server_start': 'start', 'read_only': 'on', 'data_directory': '/var/lib/postgresql/data', 'version': 1, 'dirty': False,
               'ledger_rows': 1, 'bad_versions': 0, 'ledger_kind': 'flyway'}
        for field, value in [('bad_versions', 1), ('ledger_kind', 'ambiguous')]:
            with self.subTest(field=field), self.assertRaises(subject.Hold):
                subject.observe(subject.catalog(CatalogTest().snapshot()), binding, lambda: identity,
                                'voice', lambda _: json.dumps(dict(row, **{field: value})), lambda: None)
        self.assertIn("version !~ ''^[0-9]+$''", subject.AUTH_SQL)
        good = subject.observe(subject.catalog(CatalogTest().snapshot()), binding, lambda: identity,
                               'voice', lambda _: json.dumps(dict(row, ledger_kind='canonical')), lambda: None)
        self.assertEqual(good['ledger_kind'], 'canonical')
        with self.assertRaises(subject.Hold):
            subject.observe(subject.catalog(CatalogTest().snapshot()), binding, lambda: identity, 'voice',
                            lambda _: json.dumps(dict(row, ledger_kind='canonical', version=999)), lambda: None)

    def test_voice_source_backed_readiness_table_check(self):
        snapshot = CatalogTest().snapshot()
        snapshot['files'].update(subject.VOICE_LIFECYCLE_SOURCE)
        source = subject.catalog(snapshot)
        identity = {'namespace': 'voice-dev', 'namespace_uid': 'n', 'pod_name': 'voice-postgres-0',
                    'pod_uid': 'p', 'pod_rv': '1', 'container_name': 'postgres', 'container_id': 'c',
                    'restart_count': 0, 'pvc_uid': 'v', 'pvc_rv': '1', 'pv_uid': 'pv', 'pv_rv': '1', 'service_uid': 's', 'service_rv': '1',
                    'service_ip': '10.1.0.1', 'pod_ip': '10.0.0.1', 'endpoints_uid': 'e', 'endpoints_rv': '1',
                    'slices_identity': 'slice:1'}
        binding = {'database': 'voice_db', 'state': 'resolved', 'addresses': ['10.1.0.1'],
                   'binding': {'namespace': 'voice-dev', 'namespace_uid': 'n'}}
        row = {'database': 'voice_db', 'database_oid': 1, 'role': 'voice', 'pid': 1,
               'server_start': 'start', 'read_only': 'on', 'data_directory': '/var/lib/postgresql/data',
               'version': 1, 'dirty': False, 'ledger_rows': 1, 'voice_lifecycle_tables_present': True}
        result = subject.observe(source, binding, lambda: identity, 'voice', lambda _: json.dumps(row), lambda: None)
        self.assertEqual(result['source_checks']['voice_lifecycle_tables'], 'present')
        self.assertFalse(result['startup_qualified'])
        row['voice_lifecycle_tables_present'] = False
        with self.assertRaises(subject.Hold):
            subject.observe(source, binding, lambda: identity, 'voice', lambda _: json.dumps(row), lambda: None)


class PrivateReaderTest(unittest.TestCase):
    def test_absent_empty_and_secret_are_distinct_and_bounded(self):
        for payload, expected in [(b'M', None), (b'P', ''), (b'Pc2VjcmV0', 'secret')]:
            reader = subject.kube_key_reader(lambda _: b'uid\n7\n' + payload)
            self.assertEqual(reader('secret', 'voice-app-secrets', 'USER_DATABASE_URL')['value'], expected)
        with self.assertRaises(subject.Hold):
            subject.kube_key_reader(lambda _: b'u\n1\nP')('secret', 'voice-app-secrets', 'POSTGRES_PASSWORD')
        with self.assertRaises(subject.Hold) as error:
            subject.kube_key_reader(lambda _: (_ for _ in ()).throw(RuntimeError('secret')))('secret', 'voice-app-secrets', 'USER_DATABASE_URL')
        self.assertNotIn('secret', str(error.exception))

    def test_bounded_linux_process_and_guard_takeover(self):
        if sys.platform != 'linux':
            self.skipTest('Linux-only process transport')
        self.assertEqual(subject._bounded_process([sys.executable, '-c', 'print("ok")'], 10), b'ok\n')
        with self.assertRaises(subject.Hold):
            subject._bounded_process([sys.executable, '-c', 'print("x" * 100)'], 10)
        state = ResolveTest()
        state.setUp()
        calls = 0
        def guard():
            nonlocal calls
            calls += 1
            if calls == 2:
                raise RuntimeError('private guard token')
        with self.assertRaises(subject.Hold) as error:
            subject.resolve('user', lambda: state.pod.copy(), bound(state.read), runtime_bound(state.runtime), dns(), guard)
        self.assertNotIn('private guard token', str(error.exception))

    def test_linux_proc_parser_selects_exactly_one_variable(self):
        if sys.platform != 'linux':
            self.skipTest('Linux-only shell')
        script = subject.ENV_SCRIPT.replace('/proc/1/environ', '/dev/stdin')
        shell = shutil.which('bash') or 'sh'
        result = subprocess.run([shell, '-ec', script, '--', 'DATABASE_URL'],
                                input=b'OTHER=hidden\0DATABASE_URL=expected\0', capture_output=True)
        self.assertEqual(result.returncode, 0)
        self.assertEqual(result.stdout, b'Pexpected')

    def test_auth_env_policy_rejects_relaxed_property_names(self):
        if sys.platform != 'linux' or not shutil.which('bash'):
            self.skipTest('requires bash')
        script = subject.AUTH_ENV_POLICY.replace('/proc/1/environ', '/dev/stdin')
        for key in ('spring_datasource_url', 'spring_flyway_enabled', 'spring_config_location',
                    'voice.auth.jdbc.url', 'spring.datasource.url'):
            with self.subTest(key=key):
                result = subprocess.run(['bash', '-ec', script], input=(key + '=private\0').encode(), capture_output=True)
                self.assertNotEqual(result.returncode, 0)
                self.assertNotIn(b'private', result.stdout + result.stderr)
        result = subprocess.run(['bash', '-ec', script], input=b'SPRING_DATASOURCE_URL=private\0', capture_output=True)
        self.assertEqual(result.stdout, b'ok')


class SnapshotTest(unittest.TestCase):
    def test_incomplete_inventory_does_not_invoke_transport(self):
        with self.assertRaises(subject.Hold):
            subject.inspect_all({}, {'user': ['voice-user-a']}, 'voice', lambda: None,
                                lambda _: self.fail('unexpected read'), lambda _: self.fail('unexpected query'))

    def test_one_hold_does_not_hide_other_apps_and_guard_loss_aborts(self):
        pods = {name: ['voice-' + name + '-one'] for name in subject.CONTAINERS}
        def inspect(_source, workload, *_args):
            if workload == 'auth':
                raise subject.Hold('auth_effective_configuration_unverified')
            return {'startup_qualified': False}
        with mock.patch.object(subject, 'inspect', side_effect=inspect):
            result = subject.inspect_all({'source_sha': 'a' * 40}, pods, 'voice', lambda: None)
        self.assertEqual(result['status'], 'HOLD')
        self.assertEqual(result['workloads']['auth'][0]['status'], 'hold')
        self.assertEqual(result['workloads']['user'][0]['status'], 'observed')
        with mock.patch.object(subject, 'inspect', side_effect=inspect), self.assertRaises(subject.Hold):
            subject.inspect_all({'source_sha': 'a' * 40}, pods, 'voice',
                                lambda: (_ for _ in ()).throw(RuntimeError('claim lost')))
        count = 0
        def intermittent_guard():
            nonlocal count
            count += 1
            if count == 2:
                raise RuntimeError('transient claim failure')
        def invoke_guard(_source, _workload, _name, _role, guard, *_args):
            subject._fence(guard)
        with mock.patch.object(subject, 'inspect', side_effect=invoke_guard), self.assertRaises(subject.GuardLost):
            subject.inspect_all({'source_sha': 'a' * 40}, pods, 'voice', intermittent_guard)
        self.assertEqual(count, 2)

    def test_secondary_owner_must_have_same_database_oid(self):
        pods = {name: ['voice-' + name + '-one'] for name in subject.CONTAINERS}
        def meta(oid):
            return {'database': 'space_db', 'engine': {'container_id': 'one'},
                    'session': {'server_start': 'time', 'database_oid': oid}}
        def inspect(_source, workload, *_args):
            if workload == 'chat':
                return {'secondary': [{'metadata': meta(1), 'owner_consistency': 'unknown'}]}
            if workload == 'space':
                return {'metadata': meta(2)}
            return {}
        with mock.patch.object(subject, 'inspect', side_effect=inspect):
            result = subject.inspect_all({'source_sha': 'a' * 40}, pods, 'voice', lambda: None)
        self.assertEqual(result['workloads']['chat'][0]['result']['secondary'][0]['owner_consistency'], 'hold')
        pods['space'].append('voice-space-two')
        for variant, expected in [('matched', 'matched'), ('hold', 'hold'), ('missing', 'hold'),
                                  ('start', 'hold'), ('engine', 'hold'), ('other_pod', 'hold')]:
            def check(_source, workload, name, *_args):
                if workload == 'chat':
                    return {'secondary': [{'metadata': meta(1), 'owner_consistency': 'unknown'}]}
                if workload != 'space':
                    return {}
                if variant == 'hold':
                    raise subject.Hold('owner_unavailable')
                if variant == 'missing':
                    return {}
                row = meta(1)
                if variant == 'start':
                    row['session']['server_start'] = 'changed'
                if variant == 'engine':
                    row['engine']['container_id'] = 'changed'
                if variant == 'other_pod' and name.endswith('-two'):
                    row['session']['database_oid'] = 2
                return {'metadata': row}
            with self.subTest(variant=variant), mock.patch.object(subject, 'inspect', side_effect=check):
                result = subject.inspect_all({'source_sha': 'a' * 40}, pods, 'voice', lambda: None)
                self.assertEqual(result['workloads']['chat'][0]['result']['secondary'][0]['owner_consistency'], expected)

    def test_owner_chain_and_container_restart(self):
        dep = {'metadata': {'namespace': 'voice-dev', 'name': 'voice-user', 'uid': 'd', 'resourceVersion': '1'}}
        rs = {'metadata': {'namespace': 'voice-dev', 'name': 'voice-user-rs', 'uid': 'r',
                           'ownerReferences': [{'controller': True, 'kind': 'Deployment', 'uid': 'd'}]}}
        pod = {'metadata': {'namespace': 'voice-dev', 'name': 'voice-user-abc', 'uid': 'p', 'resourceVersion': '2',
                            'ownerReferences': [{'controller': True, 'kind': 'ReplicaSet', 'name': 'voice-user-rs', 'uid': 'r'}]},
               'spec': {'containers': [{'name': 'user', 'env': [], 'envFrom': []}]},
               'status': {'containerStatuses': [{'name': 'user', 'containerID': 'id', 'restartCount': 0,
                                                 'state': {'running': {}}}]}}
        objects = {'deployment': dep, 'replicaset': rs, 'pod': pod,
                   'namespace': {'metadata': {'name': 'voice-dev', 'uid': 'namespace'}}}
        def runner(args):
            return json.dumps(objects[args[9]]).encode()
        result = subject.kube_workload_snapshot('user', 'voice-user-abc', runner, lambda: None)()
        self.assertEqual(result['container_id'], 'id')
        rs['metadata']['ownerReferences'][0]['uid'] = 'other'
        with self.assertRaises(subject.Hold):
            subject.kube_workload_snapshot('user', 'voice-user-abc', runner, lambda: None)()

    def test_engine_service_target_and_pvc_identity(self):
        objects = {
            ('namespace', 'voice-dev'): {'metadata': {'name': 'voice-dev', 'uid': 'n'}},
            ('statefulset', 'voice-postgres'): {'metadata': {'name': 'voice-postgres', 'namespace': 'voice-dev', 'uid': 's'}},
            ('pod', 'voice-postgres-0'): {'metadata': {'name': 'voice-postgres-0', 'namespace': 'voice-dev', 'uid': 'p', 'resourceVersion': '2', 'labels': {'app': 'voice-postgres'},
                 'ownerReferences': [{'controller': True, 'kind': 'StatefulSet', 'uid': 's'}]},
                 'spec': {'containers': [{'name': 'postgres', 'volumeMounts': [{'name': 'pgdata', 'mountPath': '/var/lib/postgresql/data'}]}],
                          'volumes': [{'name': 'pgdata', 'persistentVolumeClaim': {'claimName': 'pgdata-voice-postgres-0'}}]},
                 'status': {'podIP': '10.0.0.1', 'containerStatuses': [{'name': 'postgres', 'containerID': 'c', 'restartCount': 0, 'state': {'running': {}}}]}},
            ('persistentvolumeclaim', 'pgdata-voice-postgres-0'): {'metadata': {'name': 'pgdata-voice-postgres-0', 'namespace': 'voice-dev', 'uid': 'v', 'resourceVersion': '3'}, 'spec': {'volumeName': 'pv-one'}, 'status': {'phase': 'Bound'}},
            ('persistentvolume', 'pv-one'): {'metadata': {'name': 'pv-one', 'uid': 'pv', 'resourceVersion': '3'}, 'spec': {'claimRef': {'name': 'pgdata-voice-postgres-0', 'namespace': 'voice-dev', 'uid': 'v'}}, 'status': {'phase': 'Bound'}},
            ('service', 'voice-postgres'): {'metadata': {'uid': 'svc', 'resourceVersion': '4'},
                                            'spec': {'type': 'ClusterIP', 'clusterIP': '10.1.0.1', 'selector': {'app': 'voice-postgres'}, 'ports': [{'port': 5432, 'targetPort': 5432}]}},
            ('endpoints', 'voice-postgres'): {'metadata': {'uid': 'ep', 'resourceVersion': '5'}, 'subsets': [
                {'ports': [{'port': 5432}], 'addresses': [{'ip': '10.0.0.1', 'targetRef': {'kind': 'Pod', 'uid': 'p'}}]}]},
            ('endpointslices', '-l'): {'items': [{'metadata': {'uid': 'slice', 'resourceVersion': '6'},
                'ports': [{'port': 5432}], 'endpoints': [{'addresses': ['10.0.0.1'], 'conditions': {'ready': True},
                'targetRef': {'kind': 'Pod', 'uid': 'p'}}]}]},
        }
        def runner(args):
            return json.dumps(objects[(args[9], args[10])]).encode()
        snapshot = subject.kube_engine_snapshot(runner, lambda: None)
        self.assertEqual(snapshot()['pvc_uid'], 'v')
        objects[('persistentvolume', 'pv-one')]['spec']['claimRef']['uid'] = 'other'
        with self.assertRaises(subject.Hold):
            snapshot()
        objects[('persistentvolume', 'pv-one')]['spec']['claimRef']['uid'] = 'v'
        spec = objects[('service', 'voice-postgres')]['spec']
        spec['type'] = 'ExternalName'
        with self.assertRaises(subject.Hold):
            snapshot()
        spec['type'] = 'ClusterIP'
        spec['ports'][0]['targetPort'] = 9999
        with self.assertRaises(subject.Hold):
            snapshot()
        spec['ports'][0]['targetPort'] = 5432
        slice_obj = objects[('endpointslices', '-l')]['items'][0]
        slice_obj['ports'][0]['name'] = ''
        self.assertEqual(snapshot()['pvc_uid'], 'v')
        slice_obj['endpoints'][0]['targetRef']['uid'] = 'wrong'
        with self.assertRaises(subject.Hold):
            snapshot()
        slice_obj['endpoints'][0]['targetRef']['uid'] = 'p'
        objects[('endpoints', 'voice-postgres')]['subsets'][0]['addresses'][0]['targetRef']['uid'] = 'wrong'
        with self.assertRaises(subject.Hold):
            snapshot()


if __name__ == '__main__':
    unittest.main()
