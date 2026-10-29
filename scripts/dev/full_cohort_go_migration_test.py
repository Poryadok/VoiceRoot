import unittest
import copy
from unittest import mock
from types import SimpleNamespace
import json
import base64
import os
from pathlib import Path
import secrets
import subprocess
import tempfile
import time
import media_backup_phase_test as fixture
import media_operation_journal
import resource_fencing

import full_cohort_go_migration as migration


class RenderTest(unittest.TestCase):
    def test_server_start_compares_aware_instants_not_session_timezone(self):
        self.assertTrue(migration._same_start('2026-10-29T12:00:00+00:00', '2026-10-29T15:00:00+03:00'))
        self.assertFalse(migration._same_start('2026-10-29T12:00:00+00:00', '2026-10-29T12:00:01+00:00'))
        self.assertFalse(migration._same_start('2026-10-29T12:00:00', '2026-10-29T12:00:00+00:00'))

    def test_private_immutable_secret_contains_only_password_and_no_tls_downgrade(self):
        credential = {'role': 'voice', 'sslmode': 'disable', 'password': 'fixture-password'}
        obj = migration._secret('media-c1-' + 'a' * 32, 'chat_db', credential, '10.0.0.1')
        self.assertTrue(obj['immutable'])
        self.assertEqual(set(obj['data']), {'PGPASSWORD'})
        self.assertEqual(base64.b64decode(obj['data']['PGPASSWORD']), b'fixture-password')
        with self.assertRaises(migration.MigrationError):
            migration._secret('media-c1-' + 'a' * 32, 'chat_db', credential | {'sslmode': 'verify-full'}, '10.0.0.1')

    def test_private_kubectl_empty_result_only_for_fixed_absence_lookup(self):
        with tempfile.TemporaryDirectory() as directory:
            executable = Path(directory) / 'kubectl'; executable.write_text('#!/bin/sh\nexit 0\n'); executable.chmod(0o700)
            with mock.patch.dict(os.environ, {'PATH': directory + ':' + os.environ['PATH']}):
                name = 'voice-go-input-chat-' + 'a' * 32
                self.assertEqual(migration._private_kube(['get', 'secret', name, '--ignore-not-found=true', '-o', 'json']), {})
                with self.assertRaises(migration.MigrationError):
                    migration._private_kube(['get', 'secret', name, '-o', 'json'])

    def test_secret_readback_checks_uid_rv_type_immutable_and_exact_private_bytes(self):
        credential = {'role': 'voice', 'sslmode': 'disable', 'password': 'fixture-password'}
        intended = migration._secret('media-c1-' + 'a' * 32, 'chat_db', credential, '10.0.0.1')
        actual = dict(intended, metadata=intended['metadata'] | {'uid': 'uid', 'resourceVersion': '7'})
        self.assertEqual(migration._check_secret(intended, actual), {'name': intended['metadata']['name'],
                         'uid': 'uid', 'resourceVersion': '7'})
        for bad in (actual | {'immutable': False}, actual | {'data': {'PGPASSWORD': 'd3Jvbmc='}},
                    actual | {'type': 'kubernetes.io/tls'}, actual | {'metadata': actual['metadata'] | {'deletionTimestamp': 'now'}},
                    actual | {'metadata': actual['metadata'] | {'ownerReferences': [{'uid': 'foreign'}]}},
                    actual | {'metadata': actual['metadata'] | {'finalizers': ['foreign']}},
                    actual | {'metadata': actual['metadata'] | {'annotations': {'foreign': 'value'}}}):
            with self.subTest(), self.assertRaises(migration.MigrationError): migration._check_secret(intended, bad)

    def test_job_uses_fixed_url_key_and_single_use_job(self):
        config, job = migration.render('media-c1-' + 'a' * 32, 'b' * 40, 'chat_db',
                                       {'000001_init.up.sql': b'SELECT 1;'},
                                       {'name': 'node', 'uid': 'node-uid', 'hostname': 'node', 'architecture': 'amd64'},
                                       {'host': 'voice-postgres', 'service_ip': '10.0.0.1', 'role': 'voice', 'sslmode': 'disable'})
        self.assertTrue(config['immutable'])
        self.assertEqual(job['spec']['backoffLimit'], 0)
        pod = job['spec']['template']['spec']
        self.assertEqual(pod['restartPolicy'], 'Never')
        self.assertFalse(pod['automountServiceAccountToken'])
        self.assertEqual(pod['containers'][0]['env'][0]['valueFrom']['secretKeyRef'],
                         {'name': 'voice-go-input-chat-' + 'a' * 32, 'key': 'PGPASSWORD'})
        command = pod['containers'][0]['command'][-1]
        self.assertLess(command.index('VOICE_DB_SERVICE_IP'), command.index('exec migrate'))
        self.assertNotIn('fixture-password', str(job))
        actual = dict(config, metadata=config['metadata'] | {'uid': 'uid', 'resourceVersion': '1'})
        self.assertEqual(migration.verify_writer_configmap(config, actual)['uid'], 'uid')
        with self.assertRaises(migration.MigrationError):
            migration.verify_writer_configmap(config, actual | {'binaryData': {'000099_injected.up.sql': 'U0VMRUNUIDE7'}})

    def test_special_database_and_unsafe_names_rejected(self):
        for db in ('space_db', 'auth_db', 'gateway_db', 'realtime_db', 'game_integration_db', 'postgres'):
            with self.subTest(db=db), self.assertRaises(migration.MigrationError):
                migration.render('media-c1-' + 'a' * 32, 'b' * 40, db,
                                 {'000001_init.up.sql': b'SELECT 1;'},
                                 {'name': 'node', 'uid': 'node-uid', 'hostname': 'node', 'architecture': 'amd64'},
                                 {'host': 'voice-postgres', 'service_ip': '10.0.0.1', 'role': 'voice', 'sslmode': 'disable'})

    def test_url_rejects_wrong_target_and_session_selector(self):
        good = 'postgres://voice:private@voice-postgres:5432/chat_db?sslmode=disable'
        self.assertEqual(migration._parse_url('chat_db', good)[:2], ('voice', 'disable'))
        for bad in (good.replace('chat_db', 'user_db'), good + '&application_name=other',
                    good.replace('voice-postgres', 'outside.example'), good.replace(':private@', ':@')):
            with self.subTest(bad=bad.replace('private', 'redacted')), self.assertRaises(migration.MigrationError):
                migration._parse_url('chat_db', bad)

    def test_runtime_secret_change_cannot_redirect_job(self):
        _, job = migration.render('media-c1-' + 'a' * 32, 'b' * 40, 'chat_db',
                                  {'000001_init.up.sql': b'SELECT 1;'},
                                  {'name': 'node', 'uid': 'node-uid', 'hostname': 'node', 'architecture': 'amd64'},
                                  {'host': 'voice-postgres', 'service_ip': '10.0.0.1', 'role': 'voice', 'sslmode': 'disable'})
        command = job['spec']['template']['spec']['containers'][0]['command'][-1]
        with tempfile.TemporaryDirectory() as directory:
            for name, text in [('migrate', '#!/bin/sh\nexit 0\n')]:
                path = Path(directory) / name; path.write_text(text); path.chmod(0o700)
            base = dict(os.environ, PATH=directory + ':' + os.environ['PATH'], VOICE_DB_HOST='voice-postgres',
                        VOICE_DB_SERVICE_IP='10.0.0.1', VOICE_DB_ROLE='voice', VOICE_SSLMODE='disable',
                        VOICE_DATABASE='chat_db')
            good = 'postgres://voice@10.0.0.1:5432/chat_db?sslmode=disable'
            for value, expected in [(good, 0), (good.replace('chat_db', 'user_db'), 1),
                                    (good.replace('voice@', 'other@'), 1),
                                    (good + '&application_name=other', 1), (good.replace('10.0.0.1', '10.0.0.2'), 1)]:
                result = subprocess.run(['/bin/sh', '-ec', command], env=base | {'DATABASE_URL': value},
                                        stdout=subprocess.PIPE, stderr=subprocess.PIPE)
                self.assertEqual(result.returncode, expected)
                self.assertNotIn(b'private', result.stdout + result.stderr)

    def test_secret_rv_or_value_change_is_refused(self):
        value = {'uid': 'uid', 'resource_version': '1',
                 'value': 'postgres://voice:private@voice-postgres/chat_db?sslmode=disable'}
        calls = []
        def stable(kind, name, key):
            calls.append((kind, name, key))
            return dict(value)
        with mock.patch.object(migration.schema, 'kube_key_reader', return_value=stable):
            result = migration._credential('chat_db', lambda: None)
            self.assertEqual(result['references'], {'url': {'uid': 'uid', 'resource_version': '1'}})
            self.assertEqual(calls, [('secret', 'voice-app-secrets', 'CHAT_DATABASE_URL')] * 2)
        def changed(*args):
            result = dict(value)
            if calls: result['resource_version'] = '2'
            calls.append(args)
            return result
        calls.clear()
        with mock.patch.object(migration.schema, 'kube_key_reader', return_value=changed):
            with self.assertRaises(migration.MigrationError): migration._credential('chat_db', lambda: None)
        with mock.patch.object(migration.schema, 'kube_key_reader', return_value=stable):
            with self.assertRaises(migration.schema.GuardLost): migration._credential('chat_db', lambda: False)

    def test_job_owned_pod_identity_and_platform_are_required(self):
        _, authored = migration.render('media-c1-' + 'a' * 32, 'b' * 40, 'chat_db',
                                       {'000001_init.up.sql': b'SELECT 1;'},
                                       {'name': 'node', 'uid': 'node-uid', 'hostname': 'node', 'architecture': 'amd64'},
                                       {'host': 'voice-postgres', 'service_ip': '10.0.0.1', 'role': 'voice', 'sslmode': 'disable'})
        job = {'metadata': {'uid': 'job-uid'}}
        pod = {'apiVersion': 'v1', 'kind': 'Pod', 'metadata': {'namespace': 'voice-dev', 'uid': 'pod-uid',
               'resourceVersion': '4', 'labels': authored['spec']['template']['metadata']['labels'],
               'ownerReferences': [{'kind': 'Job', 'uid': 'job-uid', 'controller': True}]},
               'spec': copy.deepcopy(authored['spec']['template']['spec']) | {'nodeName': 'node'},
               'status': {'phase': 'Running', 'podIP': '10.0.0.4', 'containerStatuses': [{'name': 'migrate',
               'containerID': 'containerd://cid', 'restartCount': 0,
               'imageID': 'containerd://' + migration.realtime_migration_job.PLATFORMS['amd64']}]}}
        execution = {'database': 'chat_db', 'database_oid': 4, 'role': 'voice',
                     'application_name': authored['metadata']['name'], 'container_name': 'migrate',
                     'node_name': 'node', 'node_uid': 'node-uid',
                     'platform_digest': migration.realtime_migration_job.PLATFORMS['amd64']}
        with mock.patch.object(migration, 'verify_writer_job', return_value=authored['spec']), \
             mock.patch.object(migration.auth_user_candidate_probe, 'admitted_pod'):
            self.assertEqual(migration.verify_writer_pod(authored, job, pod, execution), {'uid': 'pod-uid', 'ip': '10.0.0.4'})
            pending = copy.deepcopy(pod); pending['spec'].pop('nodeName')
            pending['status'] = {'phase': 'Pending', 'containerStatuses': []}
            self.assertEqual(migration.verify_writer_pod(authored, job, pending, execution), {'uid': 'pod-uid', 'ip': None})
            pending['status']['containerStatuses'] = [{'name': 'migrate', 'restartCount': 0,
                                                       'state': {'running': {'startedAt': 'now'}}}]
            with self.assertRaises(migration.MigrationError): migration.verify_writer_pod(authored, job, pending, execution)
            bad = copy.deepcopy(pod); bad['metadata']['ownerReferences'][0]['uid'] = 'foreign'
            with self.assertRaises(migration.MigrationError): migration.verify_writer_pod(authored, job, bad, execution)
            bad = copy.deepcopy(pod); bad['status']['containerStatuses'][0]['imageID'] = 'sha256:' + 'f' * 64
            with self.assertRaises(migration.MigrationError): migration.verify_writer_pod(authored, job, bad, execution)

    def test_job_admission_rejects_injected_owner(self):
        _, authored = migration.render('media-c1-' + 'a' * 32, 'b' * 40, 'chat_db',
                                       {'000001_init.up.sql': b'SELECT 1;'},
                                       {'name': 'node', 'uid': 'node-uid', 'hostname': 'node', 'architecture': 'amd64'},
                                       {'host': 'voice-postgres', 'service_ip': '10.0.0.1', 'role': 'voice', 'sslmode': 'disable'})
        actual = copy.deepcopy(authored)
        actual['metadata'].update(uid='uid', ownerReferences=[{'uid': 'foreign'}])
        with self.assertRaises(migration.MigrationError): migration.verify_writer_job(authored, actual, dry=True)

    def test_metadata_role_is_source_login_and_secret_drift_is_rejected(self):
        plans = {'chat_db': {'from': 1, 'to': 2}, 'space_db': {'from': 25, 'to': 25}}
        catalog = {'engine': {'uid': 'engine'}, 'server_start': 'start', 'databases': {'chat_db': 4}}
        roles = {'roles': [{'name': 'chat_migrator', 'login': True}]}
        secret = {'role': 'chat_migrator', 'password': 'fixture', 'sslmode': 'disable', 'host': 'voice-postgres',
                  'references': {'url': {'uid': 'uid', 'resource_version': '1'}}}
        safe = {k:v for k,v in secret.items() if k != 'password'}
        phase = SimpleNamespace(_guard=lambda: None)
        with mock.patch.object(migration, '_credential', return_value=secret), \
             mock.patch.object(migration, '_login', return_value=safe):
            self.assertEqual(migration.metadata._migration_roles(phase, plans, catalog, roles), {'chat_db': safe})
            roles['roles'][0]['login'] = False
            with self.assertRaises(migration.metadata.MetadataError):
                migration.metadata._migration_roles(phase, plans, catalog, roles)
        roles['roles'][0]['login'] = True
        with mock.patch.object(migration, '_credential', side_effect=[secret, secret | {'password': 'changed'}]), \
             mock.patch.object(migration, '_login', return_value=safe):
            with self.assertRaises(migration.metadata.MetadataError):
                migration.metadata._migration_roles(phase, plans, catalog, roles)

    def test_restored_target_must_match_source_plan(self):
        metadata_phase = object.__new__(migration.metadata.BackupMetadataPhase)
        cohort = object.__new__(migration.cohort_backup_phases.CohortPhases)
        cut = object.__new__(migration.writer_cut.WriterCut)
        row = {migration.metadata.LEDGER: {'receipt_sha256': 'f' * 64}}
        fence = SimpleNamespace(context={'operation_id': 'op'}, ledger=SimpleNamespace(read=lambda: (row, ('uid', 'rv'))))
        common = {'fence': fence, 'lock': {}, 'namespace_uid': 'ns'}
        for obj in (metadata_phase, cut): obj.__dict__.update(common)
        metadata_phase.source_root = '/fixture'
        cut.verify_drained = lambda: None; cut.migration_history = []
        cohort._guard = lambda: None
        target = {'engine': 'golang-migrate', 'from': 1, 'to': 3, 'files': {'000001_up.up.sql': 'a' * 64}}
        observed = {'metadata': {'migrations': {'auth_db': {'from': 26}, 'chat_db': target}},
                    'databases': ['auth_db', 'chat_db'], 'sources': {'chat_db': {'database_oid': 4}},
                    'restore_inputs': {'postgres_image': 'pg', 'migrate_image': 'migrate'},
                    'full_catalog': {}, 'migration_roles': {'chat_db': {'role': 'voice'}}}
        binding = {k: observed[k] for k in ('databases', 'sources', 'restore_inputs', 'full_catalog')}
        binding.update(source_sha='b' * 40, snapshot_id='capture-' + 'a' * 32)
        marker = {'manifest_sha256': 'c' * 64}
        clone = {'database': 'chat_db', 'role': 'voice', 'from': 1, 'to': 3, 'source_sha': binding['source_sha'],
                 'snapshot_id': binding['snapshot_id'], 'manifest_sha256': marker['manifest_sha256'],
                 'migration_roles_sha256': 'd' * 64, 'scope': 'isolated-clone-migration-login-only'}
        receipt = {'databases': {'chat_db': {'mode': 'migrate', 'from': 1, 'to': 3}}}
        cohort.phases = SimpleNamespace(**common, journal=SimpleNamespace(state={'steps': {'cohort-restore': {'receipt': receipt}}}))
        with mock.patch.object(migration.metadata, 'verify_completed', return_value=observed), \
             mock.patch.object(migration.readback, '_snapshot', return_value=(binding, {}, marker, {})), \
             mock.patch.object(migration.cohort_restore_journal, 'verify_go_role', return_value=clone), \
             mock.patch.object(migration.restore, '_source'), \
             mock.patch.object(migration.restore, 'migration_inventory'):
            self.assertEqual(migration.qualified_plan(metadata_phase, cohort, cut, 'token', 'chat_db')['plan'], target)
            receipt['databases']['chat_db']['to'] = 4
            with self.assertRaises(migration.MigrationError):
                migration.qualified_plan(metadata_phase, cohort, cut, 'token', 'chat_db')

    def test_live_go_order_matches_single_clone_order(self):
        observed = {'databases': ['chat_db', 'user_db'], 'metadata': {'migrations': {
            'chat_db': {'from': 1, 'to': 2}, 'user_db': {'from': 19, 'to': 22}}}}
        cut = SimpleNamespace(migration_history=[])
        migration._check_order(cut, observed, 'chat_db')
        with self.assertRaises(migration.MigrationError): migration._check_order(cut, observed, 'user_db')
        prior = object.__new__(migration.Migration); prior.cut = cut; prior.database = 'chat_db'
        cut.migration_history.append({'kind': 'go', 'phase': prior, 'finished': True})
        with mock.patch.object(migration, 'verify_completed') as verified:
            migration._check_order(cut, observed, 'user_db')
            verified.assert_called_once_with(prior)
        cut.migration_history[0]['finished'] = False
        with self.assertRaises(migration.MigrationError): migration._check_order(cut, observed, 'user_db')


@unittest.skipUnless(os.environ.get('COHORT_METADATA_PG_TEST') == '1', 'disposable PostgreSQL fixture')
class LoginTest(unittest.TestCase):
    def test_private_stdin_actual_login_and_wrong_password(self):
        name = 'voice-go-login-test-' + secrets.token_hex(8)
        subprocess.run(['docker', 'run', '-d', '--name', name, '--network', 'none',
                        '--add-host', 'voice-postgres:127.0.0.1', '-e', 'POSTGRES_PASSWORD=fixture-only',
                        '-e', 'POSTGRES_HOST_AUTH_METHOD=scram-sha-256',
                        '-e', 'POSTGRES_INITDB_ARGS=--auth-host=scram-sha-256',
                        'postgres:16-alpine'], check=True, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        self.addCleanup(subprocess.run, ['docker', 'rm', '-f', '-v', name], check=True,
                        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        for _ in range(60):
            logs = subprocess.run(['docker', 'logs', name], stdout=subprocess.PIPE, stderr=subprocess.STDOUT, check=True).stdout
            ready = subprocess.run(['docker', 'exec', name, 'pg_isready', '-U', 'postgres'],
                                   stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
            if b'PostgreSQL init process complete; ready for start up.' in logs and ready.returncode == 0:
                break
            time.sleep(.5)
        else: self.fail('disposable PostgreSQL failed to start')
        raw = subprocess.run(['docker', 'exec', name, 'psql', '-X', '-qAt', '-U', 'postgres', '-d', 'postgres', '-c',
                              "SELECT json_build_object('database_oid',oid::bigint,'server_start',pg_postmaster_start_time()) "
                              "FROM pg_database WHERE datname=current_database()"], check=True, stdout=subprocess.PIPE).stdout
        source = json.loads(raw) | {'engine': {'uid': 'fixture-engine'}}
        with tempfile.TemporaryDirectory() as temp:
            shim = Path(temp) / 'kubectl'
            shim.write_text('#!/usr/bin/env python3\nimport os,sys\ni=sys.argv.index("sh")\n'
                            'os.execvp("docker",["docker","exec","-i",' + repr(name) + ']+sys.argv[i:])\n')
            shim.chmod(0o700)
            credential = {'role': 'postgres', 'sslmode': 'disable', 'host': 'voice-postgres',
                          'password': 'fixture-only', 'references': {'url': {'uid': 'fixture', 'resource_version': '1'}}}
            with mock.patch.dict(os.environ, {'PATH': temp + ':' + os.environ['PATH']}), \
                 mock.patch.object(migration.schema, 'kube_engine_snapshot', return_value=lambda: source['engine']):
                safe = migration._login('postgres', credential, source, lambda: None)
                self.assertNotIn('password', safe)
                credential['password'] = 'incorrect'
                with self.assertRaises(migration.MigrationError):
                    migration._login('postgres', credential, source, lambda: None)
                subprocess.run(['docker', 'exec', name, 'psql', '-X', '-q', '-v', 'ON_ERROR_STOP=1', '-U', 'postgres',
                                '-d', 'postgres', '-c', "CREATE ROLE chat_login LOGIN PASSWORD 'fixture-only'; "
                                "ALTER ROLE chat_login SET timezone='Europe/Moscow'"], check=True,
                               stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
                credential['role'] = 'chat_login'; credential['password'] = 'fixture-only'
                self.assertEqual(migration._login('postgres', credential, source, lambda: None)['role'], 'chat_login')
        subprocess.run(['docker', 'exec', name, 'psql', '-X', '-q', '-v', 'ON_ERROR_STOP=1', '-U', 'postgres',
                        '-d', 'postgres', '-c', 'CREATE TABLE public.schema_migrations(version bigint,dirty boolean); '
                        'INSERT INTO public.schema_migrations VALUES(1,false)'], check=True,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        args = ['docker', 'run', '--rm', '--network', 'container:' + name, '-e', 'PGPASSWORD=fixture-only',
                migration.realtime_migration_job.MIGRATE_IMAGE, '-path=/tmp',
                '-database=postgres://postgres@127.0.0.1:5432/postgres?sslmode=disable', 'version']
        tool = subprocess.run(args, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=True)
        self.assertIn(b'1', tool.stdout + tool.stderr)
        self.assertNotIn(b'fixture-only', tool.stdout + tool.stderr)
        bad = list(args); bad[6] = 'PGPASSWORD=incorrect'
        refused = subprocess.run(bad, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.assertNotEqual(refused.returncode, 0)
        subprocess.run(['docker', 'exec', name, 'psql', '-X', '-q', '-v', 'ON_ERROR_STOP=1', '-U', 'postgres',
                        '-d', 'postgres', '-c', 'CREATE DATABASE chat_db'], check=True,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        subprocess.run(['docker', 'exec', name, 'psql', '-X', '-q', '-v', 'ON_ERROR_STOP=1', '-U', 'postgres',
                        '-d', 'chat_db', '-c', 'CREATE TABLE public.schema_migrations(version bigint,dirty boolean); '
                        'INSERT INTO public.schema_migrations VALUES(1,false)'], check=True,
                       stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        node = {'name': 'node', 'uid': 'uid', 'hostname': 'node', 'architecture': 'amd64'}
        route = {'host': 'voice-postgres', 'service_ip': '127.0.0.1', 'role': 'postgres', 'sslmode': 'disable'}
        files = {'000001_initial.up.sql': b'SELECT 1;',
                 '000002_next.up.sql': b'CREATE TABLE public.go_migration_fixture(id integer);'}
        _, job = migration.render('media-c1-' + 'a' * 32, 'b' * 40, 'chat_db', files, node, route)
        container = job['spec']['template']['spec']['containers'][0]
        command = container['command'][-1]
        environment = {'PGPASSWORD': 'fixture-only', 'DATABASE_URL': 'postgres://postgres@127.0.0.1:5432/chat_db?sslmode=disable',
                       'VOICE_DB_SERVICE_IP': '127.0.0.1', 'VOICE_DB_ROLE': 'postgres', 'VOICE_DATABASE': 'chat_db',
                       'PGAPPNAME': 'voice-go-fixture'}
        create = ['docker', 'create', '--network', 'container:' + name, '--entrypoint', '/bin/sh']
        for key, value in environment.items(): create.extend(['-e', key + '=' + value])
        create.extend([migration.realtime_migration_job.MIGRATE_IMAGE, '-ec', command])
        cid = subprocess.run(create, check=True, stdout=subprocess.PIPE, stderr=subprocess.DEVNULL).stdout.decode().strip()
        self.addCleanup(subprocess.run, ['docker', 'rm', '-f', '-v', cid], check=True,
                        stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        with tempfile.TemporaryDirectory() as temporary:
            folder = Path(temporary) / 'migrations'; folder.mkdir()
            for filename, value in files.items(): (folder / filename).write_bytes(value)
            subprocess.run(['docker', 'cp', str(folder), cid + ':/'], check=True,
                           stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
        run = subprocess.run(['docker', 'start', '-a', cid], check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        self.assertNotIn(b'fixture-only', run.stdout + run.stderr)
        result = subprocess.run(['docker', 'exec', name, 'psql', '-X', '-qAt', '-U', 'postgres', '-d', 'chat_db', '-c',
                                 "SELECT version || ':' || dirty || ':' || (to_regclass('public.go_migration_fixture') IS NOT NULL) "
                                 'FROM public.schema_migrations'], check=True, stdout=subprocess.PIPE).stdout.strip()
        self.assertEqual(result, b'2:false:true')


class SecretSnapshotTest(unittest.TestCase):
    def setUp(self):
        self.base = fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.base.setUp(); self.addCleanup(self.base.doCleanups)
        self.journal = media_operation_journal.Journal(Path(self.base.temp.name) / 'go-secret',
                            {'resource_context': self.base.context, 'namespace_uid': 'ns'})
        self.addCleanup(self.journal.close)
        client = object.__new__(migration.frontend_runtime_executor.KubectlTransport)
        client.journal = self.journal; self.base.ledger.client = client
        self.cut = SimpleNamespace(verify_drained=lambda: None)
        self.phase = migration.SecretSnapshot(self.journal, self.base.fence, client, self.base.lock, 'ns',
                                              SimpleNamespace(source_root=Path('/fixture')), object(), self.cut, 'token', 'chat_db')
        self.credential = {'role': 'voice', 'sslmode': 'disable', 'host': 'voice-postgres',
                           'password': 'fixture-secret', 'references': {'url': {'uid': 's', 'resource_version': '1'}}}
        self.safe = {k:v for k,v in self.credential.items() if k != 'password'}
        self.proof = {'source': {'engine': {'service_ip': '10.0.0.1'}, 'database_oid': 1, 'server_start': 'start'},
                      'credential': self.safe, 'metadata_receipt_sha256': 'd' * 64, 'plan': {'from': 1, 'to': 2}}
        self.clone = {'database': 'chat_db', 'role': 'voice', 'from': 1, 'to': 2,
                      'source_sha': self.base.context['commit'], 'snapshot_id': 'capture-' + 'a' * 32,
                      'manifest_sha256': 'b' * 64, 'migration_roles_sha256': 'c' * 64,
                      'scope': 'isolated-clone-migration-login-only'}
        self.created = None
        self.patches = [mock.patch.object(migration.media_operation_guard, 'verify'),
                        mock.patch.object(migration, 'qualified_plan', return_value=self.proof),
                        mock.patch.object(migration.cohort_restore_journal, 'verify_go_role', create=True,
                                          return_value=self.clone),
                        mock.patch.object(migration, '_credential', return_value=self.credential),
                        mock.patch.object(migration, '_login', return_value=self.safe)]
        for patch in self.patches: patch.start(); self.addCleanup(patch.stop)

    def transport(self, args, body=None):
        if '--dry-run=server' in args:
            return body
        if args[0] == 'create':
            self.created = dict(body, metadata=body['metadata'] | {'uid': 'secret-uid', 'resourceVersion': '7'})
        return self.created

    def test_create_readback_exact_and_no_secret_in_journal_or_ledger(self):
        with mock.patch.object(migration, '_private_kube', side_effect=self.transport), \
             mock.patch.object(migration.schema, 'kube_engine_snapshot', return_value=lambda:self.proof['source']['engine']):
            receipt = self.phase.create()
            self.assertEqual(receipt['uid'], 'secret-uid')
            self.assertEqual(self.phase.verify(), receipt)
            self.assertNotIn('fixture-secret', str(self.journal.state) + str(self.base.ledger.row))
            self.assertNotIn(base64.b64encode(b'fixture-secret').decode(), str(self.journal.state) + str(self.base.ledger.row))
            self.created['metadata']['uid'] = 'recreated'
            with self.assertRaises(migration.MigrationError): self.phase.verify()
            with self.assertRaises(migration.MigrationError): self.phase.create()

    def test_lost_create_reply_marks_fence_uncertain_and_never_retries(self):
        def lost(args, body=None):
            if args[0] == 'create' and '--dry-run=server' not in args:
                self.transport(args, body)
                raise OSError('response lost')
            return self.transport(args, body)
        with mock.patch.object(migration, '_private_kube', side_effect=lost):
            with self.assertRaises(migration.MigrationError): self.phase.create()
            self.assertEqual(self.base.ledger.row['status'], 'uncertain')
            with self.assertRaises(migration.MigrationError): self.phase.create()

    def test_admission_injected_owner_refused_before_create(self):
        def injected(args, body=None):
            if '--dry-run=server' in args:
                return dict(body, metadata=body['metadata'] | {'ownerReferences': [{'uid': 'foreign'}]})
            self.fail('create must not occur')
        with mock.patch.object(migration, '_private_kube', side_effect=injected):
            with self.assertRaises(migration.MigrationError): self.phase.create()
        self.assertNotIn('go_migration_secrets', self.base.ledger.row)

    def test_mismatched_clone_role_refused_before_kubernetes_effect(self):
        clone = self.clone | {'role': 'other'}
        with mock.patch.object(migration.cohort_restore_journal, 'verify_go_role', return_value=clone, create=True), \
             mock.patch.object(migration, '_private_kube') as transport:
            with self.assertRaises(migration.MigrationError): self.phase.create()
            transport.assert_not_called()
        self.assertNotIn('go_migration_secrets', self.base.ledger.row)

    def test_source_credential_drift_after_pending_prevents_post(self):
        changed = self.credential | {'password': 'changed'}
        with mock.patch.object(migration, '_credential', side_effect=[self.credential, self.credential, changed]), \
             mock.patch.object(migration, '_private_kube', side_effect=self.transport):
            with self.assertRaises(migration.MigrationError): self.phase.create()
        self.assertIsNone(self.created)
        self.assertEqual(self.base.ledger.row['status'], 'uncertain')

    def test_guard_lost_after_pending_aborts_before_post(self):
        def guard(*args):
            if self.base.ledger.row.get('go_migration_secrets'):
                raise RuntimeError('claim lost')
        with mock.patch.object(migration.media_operation_guard, 'verify', side_effect=guard), \
             mock.patch.object(migration, '_private_kube', side_effect=self.transport):
            with self.assertRaises(migration.schema.GuardLost): self.phase.create()
        self.assertIsNone(self.created)
        self.assertEqual(self.base.ledger.row['status'], 'uncertain')

    def test_job_journal_rejects_changed_input_after_reservation(self):
        with mock.patch.object(self.phase, 'verify', return_value={'uid': 'fixture'}):
            self.phase._bound = ({}, {}, {})
            job = migration.Migration(self.phase)
            job.input = {'files': {'000001_init.up.sql': b'SELECT 1;'}, 'node': {'name': 'node'}}
            marker = {'status': 'active', 'operation_id': self.base.context['operation_id'],
                      'source_sha': self.base.context['commit'], 'nonce': job.invocation,
                      'input_sha256': job._input_hash(), 'steps': {}}
            job._update(None, marker)
            self.assertEqual(self.base.ledger.row['go_migrations']['chat_db'], marker)
            job.input['files']['000001_init.up.sql'] = b'SELECT 2;'
            with self.assertRaises(migration.MigrationError): job._guard()

    def _job_flow(self, lost_reply, dirty=False, pending_drift=False):
        files = {'000001_init.up.sql': b'SELECT 1;', '000002_next.up.sql': b'SELECT 2;'}
        source = self.proof['source']
        secret_receipt = {'name': 'voice-go-input-chat-' + 'a' * 32, 'uid': 'secret-uid', 'resourceVersion': '7'}
        secret_done = {'status': 'done', 'receipt': secret_receipt, 'clone_receipt_sha256': migration._hash(self.clone)}
        self.phase._bound = (secret_done, self.credential, source)
        self.journal.state['steps']['go-migration-secret-chat_db'] = copy.deepcopy(secret_done); self.journal.save()
        self.base.ledger.row['go_migration_secrets'] = {'chat_db': copy.deepcopy(secret_done)}
        plan = {'engine': 'golang-migrate', 'from': 1, 'to': 2, 'files': {'fixture': 'a' * 64}}
        proof = self.proof | {'plan': plan, 'database': 'chat_db'}
        clone = self.clone
        self.phase.metadata_phase.inventory = SimpleNamespace(_admin_role=lambda: 'postgres')
        self.phase.metadata_phase.package = SimpleNamespace(verify_current=lambda token: {'nodes': [
            {'name': 'node', 'uid': 'node-uid', 'hostname': 'node', 'architecture': 'amd64'}]})
        values = iter([{'history': [{'version': 1, 'dirty': False}]}, {'history': [{'version': 2, 'dirty': dirty}]}])
        self.phase.metadata_phase._read = lambda *args: next(values)
        before = {'history': [{'version': 1, 'dirty': False}]}
        observed = {'databases': ['chat_db'], 'metadata': {'catalog': {'chat_db': before},
                    'migrations': {'chat_db': plan}}}
        state = {'cm': None, 'job': None, 'pod': None, 'bound': False, 'finished': False, 'posts': 0, 'pending_checks': 0}
        def call(*args, body=None):
            if args[:2] in (('get', 'configmaps'), ('get', 'jobs')): return {'items': []}
            if args[:2] == ('get', 'configmap'): return state['cm']
            if args[:2] == ('get', 'job'): return state['job']
            if args[:2] == ('get', 'pods'): return {'items': [state['pod']] if state['pod'] else []}
            if args[0] == 'delete':
                if '/jobs/' in args[1]: state['job'] = state['pod'] = None
                if '/configmaps/' in args[1]: state['cm'] = None
                return {}
            if args[0] == 'create' and '--dry-run=server' in args:
                if body['kind'] == 'Pod': return copy.deepcopy(body)
                return dict(body, metadata=body['metadata'] | {'uid': 'dry-uid', 'resourceVersion': '1'})
            if args[0] == 'create' and body['kind'] == 'ConfigMap':
                state['cm'] = dict(body, metadata=body['metadata'] | {'uid': 'cm-uid', 'resourceVersion': '2'})
                return state['cm']
            if args[0] == 'create' and body['kind'] == 'Job':
                state['posts'] += 1
                state['job'] = dict(body, metadata=body['metadata'] | {'uid': 'job-uid', 'resourceVersion': '3'},
                                    status={'conditions': [{'type': 'Complete', 'status': 'True'}]})
                state['pod'] = {'metadata': {'uid': 'pod-uid', 'ownerReferences': [{'kind': 'Job', 'uid': 'job-uid', 'controller': True}]},
                                'status': {'phase': 'Succeeded', 'containerStatuses': [{'containerID': 'cid',
                                'imageID': 'containerd://' + migration.realtime_migration_job.PLATFORMS['amd64'],
                                'state': {'terminated': {'exitCode': 0}}}]}}
                if lost_reply: raise OSError('lost committed Job response')
                return state['job']
            raise AssertionError(args)
        self.phase.client.call = call
        def bind(kind, phase, root):
            self.assertEqual(kind, 'go')
            self.assertEqual(migration.verify_writer_successor(phase, root)['job']['state'], 'absent')
            state['bound'] = True
        self.cut.bind_migration = bind
        self.cut.migration_history = []
        def pending_check():
            self.assertTrue(state['bound']); state['pending_checks'] += 1
            if pending_drift and state['pending_checks'] == 2: raise RuntimeError('writer appeared')
        self.cut.verify_migration_pending = pending_check
        self.cut.verify_migration_running = lambda: self.assertTrue(state['bound'])
        self.cut.finish_migration = lambda: state.update(finished=True)
        def accept_cleanup():
            self.cut.migration_history.append({'kind': 'go', 'phase': phase, 'finished': True})
            state.update(bound=False)
        self.cut.accept_migration_cleanup = accept_cleanup
        with mock.patch.object(self.phase, 'verify', return_value=secret_receipt), \
             mock.patch.object(migration, 'qualified_plan', return_value=proof), \
             mock.patch.object(migration.cohort_restore_journal, 'verify_go_role', return_value=clone, create=True), \
             mock.patch.object(migration.metadata, 'verify_completed', return_value=observed), \
             mock.patch.object(migration.restore, 'migration_inventory', return_value={'chat_db': files}), \
             mock.patch.object(migration.schema, 'kube_engine_snapshot', return_value=lambda: source['engine']), \
             mock.patch.object(migration.realtime_migration_job, 'admitted_job', side_effect=lambda expected, actual: expected['spec']), \
             mock.patch.object(migration.auth_user_candidate_probe, 'admitted_pod'), \
             mock.patch.object(migration, 'verify_writer_job', side_effect=lambda expected, actual, **kwargs: expected['spec']), \
             mock.patch.object(migration, 'verify_writer_pod', return_value={'uid': 'pod-uid', 'ip': '10.0.0.4'}), \
             mock.patch.object(migration, '_private_kube', return_value={}):
            phase = migration.Migration(self.phase)
            if lost_reply or dirty or pending_drift:
                with self.assertRaises(migration.MigrationError): phase.run()
                self.assertEqual(self.base.ledger.row['status'], 'uncertain')
                with self.assertRaises(migration.MigrationError): phase.run()
                self.assertEqual(state['posts'], 0 if pending_drift else 1)
                return
            result = phase.run()
            self.assertTrue(state['bound'] and state['finished'])
            self.assertEqual((result['from'], result['to'], result['dirty']), (1, 2, False))
            self.assertEqual(self.base.ledger.row['go_migrations']['chat_db']['status'], 'done')
            self.assertNotIn('fixture-secret', str(self.journal.state) + str(self.base.ledger.row))
            self.assertEqual(phase.cleanup(), secret_receipt)
            self.assertEqual(self.base.ledger.row['go_migrations']['chat_db']['steps']['job']['status'], 'removed')
            self.assertEqual(self.base.ledger.row['go_migration_secrets']['chat_db']['status'], 'removed')
            self.assertEqual(migration.verify_completed(phase), result)
            historic = migration.verify_writer_successor(phase, phase.root)
            self.assertEqual((historic['job']['state'], historic['configmap']['state']), ('removed', 'removed'))
            ordered = {'databases': ['chat_db', 'user_db'], 'metadata': {'migrations': {
                'chat_db': {'from': 1, 'to': 2}, 'user_db': {'from': 19, 'to': 22}}}}
            migration._check_order(self.cut, ordered, 'user_db')
            self.base.ledger.row['go_migrations']['chat_db']['steps']['job']['receipt']['to'] = 99
            with self.assertRaises(migration.MigrationError): migration.verify_completed(phase)

    def test_job_flow_real_journal_fence_synthetic_boundaries(self):
        self._job_flow(False)

    def test_committed_job_reply_lost_is_uncertain_and_never_retried(self):
        self._job_flow(True)

    def test_dirty_terminal_ledger_holds_and_never_retries(self):
        self._job_flow(False, dirty=True)

    def test_second_pending_check_refuses_new_writer_before_post(self):
        self._job_flow(False, pending_drift=True)
