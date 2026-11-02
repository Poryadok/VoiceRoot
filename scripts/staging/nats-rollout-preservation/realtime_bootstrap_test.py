"""Source-bound create-only Realtime bootstrap contracts."""
import unittest
import copy
import tempfile
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import realtime_bootstrap as bootstrap


class CatalogTests(unittest.TestCase):
    @unittest.skipUnless(__import__('os').name == 'posix' and __import__('os').geteuid() == 0,
                         'private custody requires isolated Linux root')
    def test_source_authority_uses_bounded_large_custody_without_expanding_journal_limit(self):
        import json, os, guard, resource_claim
        from unittest.mock import Mock
        with tempfile.TemporaryDirectory(dir='/root') as folder:
            root=Path(folder);base=root/'installed'/'realtime-bootstrap'/('a'*12)
            base.mkdir(parents=True,mode=0o700)
            approved={'source_files':{'path/%05d'%i:'a'*64 for i in range(9000)}}
            execution={'head_sha':'b'*40};authority={'approved':approved,'execution':execution}
            path=base/'authority.json';path.write_text(json.dumps(authority));path.chmod(0o600)
            self.assertGreater(path.stat().st_size,65536)
            with self.assertRaises(RuntimeError):resource_claim.trusted_json(path)
            self.assertEqual(resource_claim.trusted_json(path,limit=2<<20),authority)
            session=Mock(operation='a'*12);view={'fixture':True}
            plan={'source_authority_sha256':bootstrap.digest(approved),
                  'source_files_sha256':bootstrap.digest(approved['source_files']),
                  'execution_sha256':bootstrap.digest(execution),'namespace_uid':'ns',
                  'postgres':{'system_identifier':'123'}}
            with patch.object(guard,'ROOT',root),patch.object(bootstrap,'_verify_source_capture'), \
                 patch.object(bootstrap,'_source_contract'),patch.object(bootstrap,'engine_snapshot',return_value=('ns',{})), \
                 patch.object(bootstrap,'_system_identifier',return_value='123'),patch.object(bootstrap,'_check_stage'):
                bootstrap._check_live(session,object(),plan,view)
                path.chmod(0o622)
                with self.assertRaises(RuntimeError):bootstrap._check_live(session,object(),plan,view)
            path.chmod(0o600);path.write_bytes(b' '*(2<<20)+b' ')
            with self.assertRaises(RuntimeError):resource_claim.trusted_json(path,limit=2<<20)
            with self.assertRaises(ValueError):resource_claim.trusted_json(path,limit=3<<20)

    def test_parent_drift_blocks_each_effect_after_pending_before_submit(self):
        import sys
        from unittest.mock import Mock
        parent={'fixture':'protected'};module=Mock();module.load_capture.return_value={'changed':True}
        for name,prior in (('role',()),('database',('role',)),('secret',('role','database'))):
            state={'status':'RUNNING','steps':{k:{'status':'DONE'} for k in prior},
                'plan':{'schema':'voice-staging-realtime-bootstrap-v2','actual_images':{'a':'b'},
                        'prerequisites_sha256':bootstrap.digest(parent)}}
            submit=Mock();observe=Mock(return_value={'before':True});session=Mock()
            with self.subTest(effect=name),patch.dict(sys.modules,{'staging_prerequisites':module}),patch.object(bootstrap,'save'):
                with self.assertRaisesRegex(bootstrap.Blocked,'parent_changed'):
                    bootstrap._effect(session,Path('/unused'),state,name,{'before':True},'a'*64,submit,observe,lambda _:True)
            self.assertEqual(state['steps'][name]['status'],'PENDING');submit.assert_not_called()
    def test_captured_parent_readonly_transport_is_fixed_and_one_use(self):
        import commands, resource_claim, resource_claim_test, sys
        from unittest.mock import Mock
        context=resource_claim_test.context();card=resource_claim_test.card(context)
        session=resource_claim.Session(context,'a'*12,'b'*64,lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal())
        parent={'protected':'fixture'}
        session.admit(start=True);session.journal.row.update(phase='prerequisites-captured',target_sha256=bootstrap.digest(parent))
        session.expected_target=bootstrap.digest(parent)
        calls=[]
        def capture(argv,*,body,**kwargs):
            session.check(argv,launch=commands.launch_snapshot(argv,body,kwargs['timeout'],kwargs['limit'],kwargs['env'],()))
            calls.append((argv,body));return b'42\n'
        module=Mock();module.load_capture.return_value=parent
        with patch.dict(sys.modules,{'staging_prerequisites':module}),patch.object(commands,'authority_guard',session.check), \
             patch.object(commands,'capture',side_effect=capture):
            self.assertEqual(bootstrap._capture_readonly('system_identifier'),b'42\n')
            self.assertEqual(calls[0][1],b'SELECT system_identifier FROM pg_control_system();\n')
            self.assertIsNone(session._prerequisite_readonly_permit)
            with self.assertRaisesRegex(RuntimeError,'prerequisite_command_forbidden'):session.check(bootstrap._COMMAND)
            with self.assertRaises(bootstrap.Blocked):bootstrap._capture_readonly('CREATE ROLE realtime')
            with self.assertRaises(bootstrap.Blocked):bootstrap._capture_readonly('runtime_auth',b'x'*32+b'\n')
            commands.capture.side_effect=RuntimeError('lost')
            with self.assertRaisesRegex(RuntimeError,'lost'):bootstrap._capture_readonly('catalog')
            self.assertIsNone(session._prerequisite_readonly_permit)
            self.assertEqual(module.load_capture.call_count,4)
            module.load_capture.side_effect=[{'protected':'fixture'},bootstrap.Blocked('parent_changed')]
            commands.capture.side_effect=capture
            with self.assertRaisesRegex(bootstrap.Blocked,'parent_changed'):bootstrap._capture_readonly('catalog')
            self.assertIsNone(session._prerequisite_readonly_permit)
    def test_existing_database_or_role_is_never_adopted(self):
        for catalog in ({'role': None, 'database': {'oid': 42}},
                        {'role': {'oid': 41}, 'database': None},
                        {'role': {'oid': 41}, 'database': {'oid': 42}}):
            with self.subTest(catalog=catalog), self.assertRaises(bootstrap.Blocked):
                bootstrap.require_absent(catalog)
        bootstrap.require_absent({'role': None, 'database': None})

    def test_role_requires_restricted_attributes_and_no_memberships(self):
        role = {'oid': 41, 'login': True, 'superuser': False, 'createdb': False,
                'createrole': False, 'inherit': False, 'replication': False,
                'bypassrls': False, 'memberships': 0, 'connection_limit': -1,
                'valid_until': None, 'config': None}
        self.assertTrue(bootstrap.valid_role(role))
        for field in ('superuser', 'createdb', 'createrole', 'inherit', 'replication', 'bypassrls'):
            with self.subTest(field=field):
                self.assertFalse(bootstrap.valid_role({**role, field: True}))
        self.assertFalse(bootstrap.valid_role({**role, 'memberships': 1}))

    def test_plan_rejects_changed_operation_or_unclosed_fields(self):
        session = SimpleNamespace(operation='012345abcdef', context={'commit': 'b' * 40}, helper='a' * 64)
        plan = {'schema': 'voice-staging-realtime-bootstrap-v1', 'operation': session.operation,
                'context': session.context, 'helper_sha256': session.helper,
                'source_sha': 'b' * 40, 'source_files_sha256': 'c' * 64,
                'source_authority_sha256': 'd' * 64, 'execution_sha256': 'e' * 64, 'stage_sha256': '3' * 64,
                'credential_sha256': 'f' * 64, 'actual_images': {'voice-realtime/realtime': 'example/image@sha256:' + '1' * 64},
                'namespace_uid': 'namespace-uid', 'postgres': {
                    'statefulset': {'uid': 'sts', 'resourceVersion': '1'},
                    'pod': {'uid': 'pod', 'resourceVersion': '2', 'container_id': 'containerd://id', 'restart_count': 0},
                    'pvc': {'name': 'pgdata-voice-postgres-0', 'uid': 'pvc', 'resourceVersion': '3'},
                    'pv': {'name': 'pv', 'uid': 'pv-uid', 'resourceVersion': '4'},
                    'service': {'uid': 'svc', 'resourceVersion': '5', 'cluster_ip': '10.0.0.1'},
                    'endpoints': {'uid': 'ep', 'resourceVersion': '6', 'pod_ip': '10.0.0.2'},
                    'slice': {'uid': 'sl', 'resourceVersion': '7'}, 'system_identifier': '12345'},
                'secret': {'uid': 'secret-uid', 'resourceVersion': '12', 'data_sha256': '1' * 64},
                'active_marker_sha256': '2' * 64}
        bootstrap.validate_plan(plan, session)
        parented={**plan,'schema':'voice-staging-realtime-bootstrap-v2','prerequisites_sha256':'3'*64}
        bootstrap.validate_plan(parented,session)
        import resource_claim
        authority=resource_claim.InstallationAuthority('owned-isolated-v1','11111111-1111-4111-8111-111111111111',
            'r20260930a4','a'*64,'b'*64,'c'*64)
        scoped_session=SimpleNamespace(operation=session.operation,context=session.context,helper=session.helper,installation=authority)
        scoped={**plan,'schema':'voice-staging-realtime-bootstrap-v3','installation_sha256':authority.sha256}
        scoped_parented={**scoped,'schema':'voice-staging-realtime-bootstrap-v4','prerequisites_sha256':'3'*64}
        bootstrap.validate_plan(scoped,scoped_session)
        bootstrap.validate_plan(scoped_parented,scoped_session)
        for invalid in (plan,{**scoped,'installation_sha256':'0'*64},{**scoped,'extra':True}):
            with self.assertRaises(bootstrap.Blocked):bootstrap.validate_plan(invalid,scoped_session)
        with self.assertRaises(bootstrap.Blocked):bootstrap.validate_plan(scoped,session)
        for invalid in ({**parented,'prerequisites_sha256':'bad'},
                        {**plan,'prerequisites_sha256':'3'*64},
                        {k:v for k,v in parented.items() if k!='prerequisites_sha256'}):
            with self.subTest(parented=invalid),self.assertRaises(bootstrap.Blocked):bootstrap.validate_plan(invalid,session)
        for changed in ({**plan, 'operation': '111111111111'}, {**plan, 'extra': True},
                        {**plan, 'credential_sha256': 'bad'}, {**plan, 'source_sha': 'c' * 40},
                        {**plan, 'actual_images': {'voice-realtime/realtime': 'latest'}}):
            with self.subTest(plan=changed), self.assertRaises(bootstrap.Blocked):
                bootstrap.validate_plan(changed, session)

    def test_secret_addition_is_full_content_cas_and_never_overwrites_key(self):
        secret = {'metadata': {'name': 'voice-app-secrets', 'namespace': 'voice-staging',
                               'uid': 'uid', 'resourceVersion': '17'},
                  'type': 'Opaque', 'data': {'POSTGRES_PASSWORD': 'cHJpdmF0ZQ=='}}
        patch = bootstrap.secret_patch(secret, b'x' * 32)
        self.assertEqual(patch[:3], [
            {'op': 'test', 'path': '/metadata/uid', 'value': 'uid'},
            {'op': 'test', 'path': '/metadata/resourceVersion', 'value': '17'},
            {'op': 'test', 'path': '/data', 'value': secret['data']}])
        self.assertEqual(patch[3]['path'], '/data/REALTIME_DATABASE_URL')
        terminating = copy.deepcopy(secret); terminating['metadata']['deletionTimestamp'] = 'now'
        with self.assertRaises(bootstrap.Blocked): bootstrap.secret_patch(terminating, b'x' * 32)
        with self.assertRaises(bootstrap.Blocked): bootstrap._secret_identity(terminating)
        secret['data']['REALTIME_DATABASE_URL'] = 'already-present'
        with self.assertRaises(bootstrap.Blocked):
            bootstrap.secret_patch(secret, b'x' * 32)

    def test_secret_readback_preserves_other_fields_and_requires_new_revision(self):
        before = {'metadata': {'name': 'voice-app-secrets', 'namespace': 'voice-staging', 'uid': 'uid',
                               'resourceVersion': '17', 'labels': {'owner': 'fixture'}},
                  'type': 'Opaque', 'data': {'keep': 'cHJpdmF0ZQ=='}}
        after = copy.deepcopy(before)
        after['data']['REALTIME_DATABASE_URL'] = bootstrap.secret_patch(before, b'x' * 32)[-1]['value']
        with self.assertRaises(bootstrap.Blocked): bootstrap._verify_secret_after(before, after, b'x' * 32)
        after['metadata']['resourceVersion'] = '18'
        bootstrap._verify_secret_after(before, after, b'x' * 32)
        after['data']['keep'] = 'Y2hhbmdlZA=='
        with self.assertRaises(bootstrap.Blocked): bootstrap._verify_secret_after(before, after, b'x' * 32)

    def test_engine_snapshot_rejects_rebound_volume(self):
        metadata = lambda name, uid: {'name': name, 'namespace': 'voice-staging',
                                       'uid': uid, 'resourceVersion': '1'}
        rows = {
            ('namespace', 'voice-staging'): {'metadata': metadata('voice-staging', 'ns')},
            ('statefulset', 'voice-postgres'): {'metadata': metadata('voice-postgres', 'sts')},
            ('pod', 'voice-postgres-0'): {'metadata': {**metadata('voice-postgres-0', 'pod'), 'labels': {'app': 'voice-postgres'},
                    'ownerReferences': [{'controller': True, 'kind': 'StatefulSet', 'uid': 'sts'}]},
                'spec': {'containers': [{'name': 'postgres', 'volumeMounts': [{'name': 'pgdata',
                    'mountPath': '/var/lib/postgresql/data'}]}], 'volumes': [{'name': 'pgdata',
                    'persistentVolumeClaim': {'claimName': 'pgdata-voice-postgres-0'}}]},
                'status': {'podIP': '10.0.0.2', 'containerStatuses': [{'name': 'postgres', 'state': {'running': {}},
                                                  'containerID': 'containerd://abc', 'restartCount': 0}]}},
            ('persistentvolumeclaim', 'pgdata-voice-postgres-0'): {'metadata': metadata('pgdata-voice-postgres-0', 'pvc'),
                'spec': {'volumeName': 'pv'}, 'status': {'phase': 'Bound'}},
            ('persistentvolume', 'pv'): {'metadata': metadata('pv', 'pvuid'),
                'spec': {'claimRef': {'name': 'pgdata-voice-postgres-0', 'namespace': 'voice-staging', 'uid': 'pvc'}},
                'status': {'phase': 'Bound'}},
            ('service', 'voice-postgres'): {'metadata': metadata('voice-postgres', 'svc'), 'spec': {
                'selector': {'app': 'voice-postgres'}, 'clusterIP': '10.0.0.1', 'ports': [{'port': 5432}]}},
            ('endpoints', 'voice-postgres'): {'metadata': metadata('voice-postgres', 'ep'), 'subsets': [{
                'addresses': [{'ip': '10.0.0.2', 'targetRef': {'kind': 'Pod', 'uid': 'pod'}}],
                'ports': [{'port': 5432}]}]}}
        slices = {'items': [{'metadata': metadata('slice', 'sl'), 'ports': [{'port': 5432}],
                             'endpoints': [{'addresses': ['10.0.0.2'], 'conditions': {'ready': True},
                                            'targetRef': {'kind': 'Pod', 'uid': 'pod'}}]}]}
        kube = SimpleNamespace(get=lambda kind, name: rows[(kind, name)], run=lambda args: slices)
        self.assertEqual(bootstrap.engine_snapshot(kube)[0], 'ns')
        slices['items'][0]['endpoints'][0]['targetRef']['uid'] = 'other-pod'
        with self.assertRaises(bootstrap.Blocked): bootstrap.engine_snapshot(kube)
        slices['items'][0]['endpoints'][0]['targetRef']['uid'] = 'pod'
        rows[('service', 'voice-postgres')]['metadata']['deletionTimestamp'] = 'now'
        with self.assertRaises(bootstrap.Blocked): bootstrap.engine_snapshot(kube)
        del rows[('service', 'voice-postgres')]['metadata']['deletionTimestamp']
        rows[('persistentvolume', 'pv')]['spec']['claimRef']['uid'] = 'different'
        with self.assertRaises(bootstrap.Blocked):
            bootstrap.engine_snapshot(kube)

    def test_new_secret_key_must_not_start_a_writer(self):
        deployment = {'metadata': {'name': 'voice-realtime', 'namespace': 'voice-staging'},
                      'spec': {'replicas': 1, 'template': {'spec': {'containers': [{'name': 'realtime'}]}}}}
        bootstrap.require_no_writer(deployment, [])
        container = deployment['spec']['template']['spec']['containers'][0]
        container['env'] = [{'name': 'DATABASE_URL', 'valueFrom': {'secretKeyRef': {
            'name': 'voice-app-secrets', 'key': 'REALTIME_DATABASE_URL'}}}]
        with self.assertRaises(bootstrap.Blocked): bootstrap.require_no_writer(deployment, [])
        deployment['spec']['replicas'] = 0
        with self.assertRaises(bootstrap.Blocked): bootstrap.require_no_writer(deployment, [{'metadata': {'name': 'pod'}}])
        bootstrap.require_no_writer(deployment, [])
        old_spec = {'containers': [container.copy()]}
        container.pop('env')
        deployment['spec']['replicas'] = 1
        with self.assertRaises(bootstrap.Blocked): bootstrap.require_no_writer(deployment, [{'spec': old_spec}])

    def test_password_stays_in_private_stdin_not_argv_or_environment(self):
        password = b'x' * 32
        with patch('commands.capture', return_value=b'') as capture:
            bootstrap._create_role(password, '12345')
        args, kwargs = capture.call_args
        self.assertNotIn(password.decode(), repr(args))
        self.assertNotIn(password.decode(), repr(kwargs.get('env')))
        self.assertIn(password, kwargs['body'])
        self.assertIn(b'pg_advisory_lock', kwargs['body'])
        self.assertIn(b'pg_control_system', kwargs['body'])
        self.assertEqual(args[0][0:4], ['/usr/local/bin/k3s', 'kubectl', '--namespace', 'voice-staging'])

    def test_lost_effect_response_stays_pending_and_is_not_replayed(self):
        session = SimpleNamespace(check=lambda: None)
        state = {'status': 'RUNNING', 'steps': {},'plan':{'schema':'voice-staging-realtime-bootstrap-v1'}}
        calls = []
        def effect():
            calls.append('submitted')
            raise OSError('lost reply')
        with tempfile.TemporaryDirectory() as directory:
            path = Path(directory) / 'checkpoint.json'
            with self.assertRaises(OSError):
                bootstrap._effect(session, path, state, 'role', {'role': None}, 'a' * 64, effect,
                                  lambda: {'role': None}, lambda _: True)
            saved = __import__('json').loads(path.read_text())
            self.assertEqual(saved['steps']['role']['status'], 'PENDING')
            with self.assertRaises(bootstrap.Blocked):
                bootstrap._effect(session, path, copy.deepcopy(saved), 'role', {'role': None}, 'a' * 64,
                                  effect, lambda: {'role': None}, lambda _: True)
            self.assertEqual(calls, ['submitted'])

    def test_admin_secret_must_match_the_pinned_postgres_container(self):
        secret = {'data': {'POSTGRES_PASSWORD': 'YWRtaW4='}}
        good = __import__('hashlib').sha256(b'admin').hexdigest().encode()
        with patch('commands.capture', return_value=b'voice\n' + good + b'\n'):
            bootstrap._verify_admin(secret)
        with patch('commands.capture', return_value=b'voice\n' + b'0' * 64 + b'\n'):
            with self.assertRaises(bootstrap.Blocked): bootstrap._verify_admin(secret)

    def test_system_identifier_rejects_malformed_or_missing_identity(self):
        with patch('commands.capture', return_value=b'1234567890123456789\n'):
            self.assertEqual(bootstrap._system_identifier(), '1234567890123456789')
        with patch('commands.capture', return_value=b'not-an-identity\n'):
            with self.assertRaises(bootstrap.Blocked): bootstrap._system_identifier()

    def test_database_effect_never_precedes_a_confirmed_role(self):
        state = {'status': 'RUNNING', 'steps': {}}
        submit = lambda: self.fail('out-of-order database effect sent')
        with tempfile.TemporaryDirectory() as directory:
            with self.assertRaises(bootstrap.Blocked):
                bootstrap._effect(SimpleNamespace(check=lambda: None), Path(directory) / 'checkpoint.json',
                                  state, 'database', {}, 'a' * 64, submit, lambda: {}, lambda _: True)

    def test_prepare_without_actual_root_session_cannot_submit_effects(self):
        with patch.object(bootstrap, '_create_role') as submit:
            with self.assertRaises(bootstrap.Blocked):
                bootstrap.run_prepare(SimpleNamespace(tt_session=None), {}, {}, {}, None, None)
            submit.assert_not_called()

    def test_source_authority_metadata_is_required_before_source_recheck(self):
        import resource_claim_test as fixtures
        session, _, _ = fixtures.ResourceTests().fixture();session.admit(start=True)
        actions=SimpleNamespace(tt_session=session,installation=session.installation)
        request={'action':'prepare','mode':'full','nonce':session.operation+'b'*20}
        execution={'head_sha':session.context['commit']}
        approved={'source_sha':session.context['commit']}
        with patch('commands.authority_guard',session.check), \
             patch.object(bootstrap,'_verify_source_capture',side_effect=bootstrap.Blocked('source_checkpoint_reached')) as verify:
            with self.assertRaisesRegex(bootstrap.Blocked,'realtime_bootstrap_authority_invalid'):
                bootstrap.run_prepare(actions,request,approved,execution,None,None)
            verify.assert_not_called()
            approved.update(schema='voice-source-authority-v1',verified=True)
            with self.assertRaisesRegex(bootstrap.Blocked,'source_checkpoint_reached'):
                bootstrap.run_prepare(actions,request,approved,execution,None,None)
            verify.assert_called_once_with(session,approved,execution)

    def test_wrong_installed_helper_map_and_wrong_transport_session_fail_before_sql(self):
        import guard
        import resource_claim_test as fixtures
        session, _, _ = fixtures.ResourceTests().fixture()
        session.admit(start=True)
        helper = __import__('hashlib').sha256(Path(bootstrap.__file__).read_bytes()).hexdigest()
        actions = SimpleNamespace(tt_session=session, binding={'nats-rollout-preservation/realtime_bootstrap.py': helper,
                                                               'other-helper': 'changed'})
        request = {'action': 'prepare', 'mode': 'full', 'nonce': session.operation + 'b' * 20}
        approved = {'schema': 'voice-source-authority-v1', 'verified': True, 'source_sha': session.context['commit'],
                    'source_files': {'scripts/staging/nats-rollout-preservation/realtime_bootstrap.py': helper}}
        source = guard.ROOT / 'installed' / 'sources' / session.operation
        with patch('commands.authority_guard', session.check), patch.object(bootstrap, '_verify_source_capture'), \
             patch.object(bootstrap, 'engine_snapshot') as observe:
            with self.assertRaises(bootstrap.Blocked):
                bootstrap.run_prepare(actions, request, approved, {'head_sha': session.context['commit']}, source, None)
            observe.assert_not_called()
        with patch('commands.authority_guard', lambda *args: None), patch.object(bootstrap, '_catalog') as query:
            with self.assertRaises(bootstrap.Blocked): bootstrap.load_completed(session)
            query.assert_not_called()

    def test_source_capture_requires_exact_protected_root_receipt(self):
        import resource_claim_test as fixtures
        session, _, _ = fixtures.ResourceTests().fixture(); session.admit(start=True)
        approved = {'source': 'captured'}; execution = {'run': 7}
        receipt = {'schema': 'voice-staging-source-capture-v2', 'installation_sha256': session.installation.sha256, 'operation': session.operation,
                   'context': session.context, 'helper_sha256': session.helper,
                   'approved_sha256': bootstrap.digest(approved), 'execution_sha256': bootstrap.digest(execution)}
        with patch('resource_claim.trusted_json', return_value=receipt):
            bootstrap._verify_source_capture(session, approved, execution)
            with self.assertRaises(bootstrap.Blocked):
                bootstrap._verify_source_capture(session, approved, {'run': 8})

    def test_runtime_authentication_uses_service_and_private_stdin(self):
        password = b'fixtureCredential_123456789012345678901234'
        with patch('commands.capture', return_value=b'realtime|realtime_db\n') as capture:
            bootstrap._verify_runtime_auth(password)
        args, kwargs = capture.call_args
        self.assertNotIn(password.decode(), repr(args))
        self.assertNotIn(password.decode(), repr(kwargs.get('env')))
        self.assertIn(password, kwargs['body'])
        self.assertIn('voice-postgres', args[0][-1])

    def test_private_custody_is_exclusive_and_keeps_existing_material(self):
        with tempfile.TemporaryDirectory() as directory:
            installed = Path(directory) / 'installed'; installed.mkdir(mode=0o700)
            password = b'x' * 32
            base = bootstrap._create_custody(installed, '012345abcdef', password, {'data': {'keep': 'value'}})
            self.assertEqual((base / 'credential').read_bytes(), password)
            self.assertEqual((base / 'credential').stat().st_mode & 0o777, 0o600)
            with self.assertRaises((bootstrap.Blocked, FileExistsError)):
                bootstrap._create_custody(installed, '012345abcdef', b'y' * 32, {})
            self.assertEqual((base / 'credential').read_bytes(), password)

    def test_source_contract_selects_deployment_not_same_named_service(self):
        root = Path(__file__).resolve().parents[3]
        names = ('deploy/staging/services.yaml', 'deploy/templates/migrate-realtime-db-job.yaml',
                 'src/backend/migrations/realtime_db/000001_admission.up.sql')
        files = {name: __import__('hashlib').sha256((root / name).read_bytes()).hexdigest() for name in names}
        env = {'name': 'DATABASE_URL', 'valueFrom': {'secretKeyRef': {'name': 'voice-app-secrets', 'key': 'REALTIME_DATABASE_URL'}}}
        def decode(raw):
            text = raw.decode()
            kind = 'Deployment' if 'kind: Deployment' in text else 'Job'
            return {'kind': kind, 'metadata': {'name': 'voice-realtime' if kind == 'Deployment' else 'voice-migrate-realtime-db'},
                    'spec': {'template': {'spec': {'containers': [{'env': [env]}]}}}}
        with patch('compiler.decode_yaml', side_effect=decode), patch('root_main.public_read', side_effect=lambda path, limit: path.read_bytes()):
            bootstrap._source_contract(root, {'source_files': files})
            def wrong_job(raw):
                doc = decode(raw)
                if doc['kind'] == 'Job': doc['metadata']['name'] = 'unrelated-job'
                return doc
            with patch('compiler.decode_yaml', side_effect=wrong_job), self.assertRaises(bootstrap.Blocked):
                bootstrap._source_contract(root, {'source_files': files})

    def test_workload_generation_aba_or_termination_rejected(self):
        deployment = {'metadata': {'uid': 'deployment', 'generation': 4}, 'spec': {'replicas': 1}}
        marker = {'metadata': {'uid': 'marker'}, 'data': {'phase': 'active'}}
        stage = SimpleNamespace(snapshots={'voice-realtime': deployment}, marker=marker)
        view = bootstrap._stage_view(stage)
        kube = SimpleNamespace(get=lambda kind, name: deployment if kind == 'deployment' else marker)
        bootstrap._check_stage(kube, view)
        deployment['metadata']['generation'] = 5
        with self.assertRaises(bootstrap.Blocked): bootstrap._check_stage(kube, view)
        deployment['metadata']['generation'] = 4
        deployment['metadata']['deletionTimestamp'] = 'now'
        with self.assertRaises(bootstrap.Blocked): bootstrap._check_stage(kube, view)


if __name__ == '__main__':
    unittest.main()
