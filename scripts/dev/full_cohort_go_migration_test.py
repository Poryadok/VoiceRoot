import unittest
from unittest import mock
from types import SimpleNamespace
import json
import os
from pathlib import Path
import secrets
import subprocess
import tempfile
import time

import full_cohort_go_migration as migration


class RenderTest(unittest.TestCase):
    def test_job_uses_fixed_url_key_and_single_use_job(self):
        config, job = migration.render('media-c1-' + 'a' * 32, 'b' * 40, 'chat_db',
                                       {'000001_init.up.sql': b'SELECT 1;'},
                                       {'name': 'node', 'uid': 'node-uid', 'hostname': 'node', 'architecture': 'amd64'},
                                       {'host': 'voice-postgres', 'service_ip': '10.0.0.1'})
        self.assertTrue(config['immutable'])
        self.assertEqual(job['spec']['backoffLimit'], 0)
        pod = job['spec']['template']['spec']
        self.assertEqual(pod['restartPolicy'], 'Never')
        self.assertFalse(pod['automountServiceAccountToken'])
        self.assertEqual(pod['containers'][0]['env'][0]['valueFrom']['secretKeyRef'],
                         {'name': 'voice-app-secrets', 'key': 'CHAT_DATABASE_URL'})
        command = pod['containers'][0]['command'][-1]
        self.assertLess(command.index('getent ahostsv4'), command.index('exec migrate'))
        self.assertNotIn('password', str(job).lower())

    def test_special_database_and_unsafe_names_rejected(self):
        for db in ('space_db', 'auth_db', 'gateway_db', 'realtime_db', 'game_integration_db', 'postgres'):
            with self.subTest(db=db), self.assertRaises(migration.MigrationError):
                migration.render('media-c1-' + 'a' * 32, 'b' * 40, db,
                                 {'000001_init.up.sql': b'SELECT 1;'},
                                 {'name': 'node', 'uid': 'node-uid', 'hostname': 'node', 'architecture': 'amd64'},
                                 {'host': 'voice-postgres', 'service_ip': '10.0.0.1'})

    def test_url_rejects_wrong_target_and_session_selector(self):
        good = 'postgres://voice:private@voice-postgres:5432/chat_db?sslmode=disable'
        self.assertEqual(migration._parse_url('chat_db', good)[:2], ('voice', 'disable'))
        for bad in (good.replace('chat_db', 'user_db'), good + '&application_name=other',
                    good.replace('voice-postgres', 'outside.example'), good.replace(':private@', ':@')):
            with self.subTest(bad=bad.replace('private', 'redacted')), self.assertRaises(migration.MigrationError):
                migration._parse_url('chat_db', bad)

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

    def test_restored_target_must_match_source_plan(self):
        metadata_phase = object.__new__(migration.metadata.BackupMetadataPhase)
        cohort = object.__new__(migration.cohort_backup_phases.CohortPhases)
        cut = object.__new__(migration.writer_cut.WriterCut)
        row = {migration.metadata.LEDGER: {'receipt_sha256': 'f' * 64}}
        fence = SimpleNamespace(context={'operation_id': 'op'}, ledger=SimpleNamespace(read=lambda: (row, ('uid', 'rv'))))
        common = {'fence': fence, 'lock': {}, 'namespace_uid': 'ns'}
        for obj in (metadata_phase, cut): obj.__dict__.update(common)
        metadata_phase.source_root = '/fixture'
        cut.verify_drained = lambda: None
        cohort._guard = lambda: None
        target = {'engine': 'golang-migrate', 'from': 1, 'to': 3, 'files': {'000001_up.up.sql': 'a' * 64}}
        observed = {'metadata': {'migrations': {'auth_db': {'from': 26}, 'chat_db': target}},
                    'databases': ['auth_db', 'chat_db'], 'sources': {'chat_db': {'database_oid': 4}},
                    'restore_inputs': {}, 'full_catalog': {}}
        binding = {k: observed[k] for k in ('databases', 'sources', 'restore_inputs', 'full_catalog')}
        receipt = {'databases': {'chat_db': {'mode': 'migrate', 'from': 1, 'to': 3}}}
        cohort.phases = SimpleNamespace(**common, journal=SimpleNamespace(state={'steps': {'cohort-restore': {'receipt': receipt}}}))
        with mock.patch.object(migration.metadata, 'verify_completed', return_value=observed), \
             mock.patch.object(migration.auth, 'verified_sources', return_value=(binding, {})), \
             mock.patch.object(migration.restore, 'migration_inventory'):
            self.assertEqual(migration.qualified_plan(metadata_phase, cohort, cut, 'token', 'chat_db')['plan'], target)
            receipt['databases']['chat_db']['to'] = 4
            with self.assertRaises(migration.MigrationError):
                migration.qualified_plan(metadata_phase, cohort, cut, 'token', 'chat_db')


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
