import copy
import pathlib
import unittest
import datetime
from types import SimpleNamespace
from unittest.mock import Mock, patch
from auth_user_adoption import RecoveryRequired, STEPS
from auth_user_adoption_runtime import guard_snapshot, source_config, job_plan, database_guard, Runtime, digest, validate_admission, validate_defaulted_job, validate_consumers, validate_auth_projection, QUALIFIED_TOOLS
import base64
import tempfile
import shutil
import hashlib
import json
from auth_user_adoption_runtime import operator_fingerprint,verify_package,prepare
from auth_user_adoption_runtime import validate_user_projection


class GuardTest(unittest.TestCase):
    def setUp(self):
        self.original = {'namespace': {'uid': 'ns', 'labels': {'voice.environment': 'dev'}},
            'storage': {'pvc': {'uid': 'pvc1', 'volume': 'pv1'}},
            'bindings': {'Secret/app': {'uid': 'secret', 'rv': '1', 'hash': 'unchanged-private-material'}},
            'peer': {'space_pod': 'space-original', 'ready': True, 'endpoints': ['space-original']},
            'workloads': {'voice-auth': {'uid': 'auth', 'spec': {'image': 'old'}},
                          'voice-user': {'uid': 'user', 'spec': {'image': 'old'}},
                          'voice-space': {'uid': 'space', 'spec': {'image': 'unchanged'}}}}
        self.expected = {'resource': {'claim': 'claim', 'generation': 3, 'window': 'window',
                         'candidate': {'images': {'auth': 'new'}}}, 'workloads': copy.deepcopy(self.original['workloads'])}
        self.resource = dict(self.expected['resource'], owner='Dimas-16', state='held', phase='deploy',
                             tasks=['A1-DEV-AUTH-USER-CREDENTIAL-ADOPTION'])

    def test_original_namespace_storage_keys_and_peer_drift_refuse(self):
        for category in ['namespace', 'storage', 'bindings', 'peer']:
            now = copy.deepcopy(self.original)
            now[category] = {'foreign': 'changed'}
            with self.subTest(category=category), self.assertRaises(RecoveryRequired):
                guard_snapshot(self.original, now, self.expected, self.resource)

    def test_same_claim_generation_window_candidate_and_phase_required(self):
        for key in ['claim', 'generation', 'window', 'candidate', 'owner', 'state', 'phase', 'tasks']:
            resource = copy.deepcopy(self.resource)
            resource[key] = 'changed'
            with self.subTest(key=key), self.assertRaises(RecoveryRequired):
                guard_snapshot(self.original, self.original, self.expected, resource)

    def test_only_original_or_exact_auth_user_poststate_allowed(self):
        expected = copy.deepcopy(self.expected)
        expected['workloads']['voice-auth']['spec'] = {'image': 'new'}
        now = copy.deepcopy(self.original)
        now['workloads']['voice-auth']['spec'] = {'image': 'new'}
        guard_snapshot(self.original, now, expected, self.resource)
        now['workloads']['voice-auth']['spec']['extra_init_container'] = 'foreign'
        with self.assertRaises(RecoveryRequired):
            guard_snapshot(self.original, now, expected, self.resource)

    def test_unchanged_snapshot_is_admitted(self):
        guard_snapshot(self.original, self.original, self.expected, self.resource)

    def test_finish_can_validate_actual_verify_phase_after_deployment_record(self):
        expected=copy.deepcopy(self.expected);expected['allowed_phases']=('deploy','verify')
        resource=dict(self.resource,phase='verify')
        guard_snapshot(self.original,self.original,expected,resource)


class JobPlanTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.cached_sources=source_config(pathlib.Path(__file__).resolve().parents[2],'owned')

    def setUp(self):
        self.images = {name: 'repo/' + name + '@sha256:' + 'a' * 64 for name in ['postgres', 'flyway', 'migrate']}
        self.sources = copy.deepcopy(self.cached_sources)

    def test_sources_immutable_and_grants_are_canonical(self):
        sources = source_config(pathlib.Path(__file__).resolve().parents[2], 'owned')
        self.assertTrue(sources['immutable'])
        self.assertIn('V31__auth_identity_ddl_barrier.sql', '\n'.join(sources['data']))
        self.assertIn('roles.sql', sources['data'])
        self.assertNotIn('ALTER ROLE', sources['data']['roles.sql'])

    def test_jobs_are_single_owned_bounded_and_server_deadlined(self):
        for step in ['migrate_auth', 'migrate_user', 'roles', 'grants_auth', 'grants_user']:
            job = job_plan(step, 'owned', self.images, 'voice-auth-user-owned', self.sources)
            with self.subTest(step=step):
                self.assertEqual(0, job['spec']['backoffLimit'])
                self.assertEqual(120, job['spec']['activeDeadlineSeconds'])
                spec = job['spec']['template']['spec']
                self.assertFalse(spec['automountServiceAccountToken'])
                self.assertEqual('Never', spec['restartPolicy'])
                container = spec['containers'][0]
                self.assertIn('@sha256:', container['image'])
                self.assertTrue(container['securityContext']['readOnlyRootFilesystem'])
                self.assertNotIn('envFrom', container)
                env = {e['name']: e for e in container['env']}
                self.assertIn('statement_timeout=60000', env['PGOPTIONS']['value'])
                self.assertIn('lock_timeout=5000', env['PGOPTIONS']['value'])
                self.assertEqual('voice-app-secrets', env['PGPASSWORD']['valueFrom']['secretKeyRef']['name'])
                if step in ['roles', 'grants_auth', 'grants_user']:
                    self.assertEqual('voice-auth-user-owned', env['AUTH_DB_PASSWORD']['valueFrom']['secretKeyRef']['name'])
                self.assertIn('>/dev/null 2>&1', container['args'][0])

    def test_source_volume_resolves_real_migration_directories(self):
        job = job_plan('migrate_auth', 'owned', self.images, 'voice-auth-user-owned', self.sources)
        paths = {i['path'] for i in job['spec']['template']['spec']['volumes'][0]['configMap']['items']}
        self.assertIn('auth/V31__auth_identity_ddl_barrier.sql', paths)
        self.assertTrue(any(p.startswith('user/') and p.endswith('.up.sql') for p in paths))
        self.assertIn('roles.sql', paths)

    def test_mutable_tools_and_unrecognized_step_are_refused(self):
        for mutation in ['mutable', 'step']:
            images = dict(self.images)
            step = 'roles'
            if mutation == 'mutable': images['postgres'] = 'postgres:16-alpine'
            if mutation == 'step': step = 'reset'
            with self.subTest(mutation=mutation), self.assertRaises(RecoveryRequired):
                job_plan(step, 'owned', images, 'owned-secret', self.sources)

    def test_dry_run_defaults_cannot_inject_sidecars_or_privileges(self):
        original=job_plan('roles','owned',self.images,'voice-auth-user-owned',self.sources)
        for fault in ['sidecar','volume','hostNetwork','env','security']:
            defaulted=copy.deepcopy(original); spec=defaulted['spec']['template']['spec']
            if fault=='sidecar': spec['containers'].append({'name':'foreign','image':'foreign'})
            if fault=='volume': spec['volumes'].append({'name':'host','hostPath':{'path':'/'}})
            if fault=='hostNetwork': spec['hostNetwork']=True
            if fault=='env': spec['containers'][0]['env'].append({'name':'PGPASSWORD','value':'foreign'})
            if fault=='security': spec['containers'][0]['securityContext']['allowPrivilegeEscalation']=True
            with self.subTest(fault=fault),self.assertRaises(RecoveryRequired):
                validate_defaulted_job(original,defaulted)

    def test_known_server_defaults_preserve_job_intent(self):
        original=job_plan('roles','owned',self.images,'voice-auth-user-owned',self.sources)
        defaulted=copy.deepcopy(original)
        defaulted['spec'].update(parallelism=1,completions=1,completionMode='NonIndexed',suspend=False)
        spec=defaulted['spec']['template']['spec'];spec.update(dnsPolicy='ClusterFirst',schedulerName='default-scheduler',serviceAccountName='default',securityContext={})
        spec['containers'][0].update(imagePullPolicy='IfNotPresent',terminationMessagePath='/dev/termination-log',terminationMessagePolicy='File')
        spec['volumes'][0]['configMap']['defaultMode']=420
        validate_defaulted_job(original,defaulted)


class DatabaseGuardTest(unittest.TestCase):
    def setUp(self):
        self.db = {'owners': {'auth_db':'voice','user_db':'voice'}, 'schemas':{'auth_db':'pg_database_owner','user_db':'pg_database_owner'},
                   'objects': {'auth_db/accounts':'voice','user_db/profiles':'voice'},
                   'roles': {}, 'memberships': [], 'auth_history':[{'version':str(i),'success':True,'checksum':i} for i in range(1,27)],
                   'user_history':{'version':19,'dirty':False}, 'logins':['voice','gameintegration_runtime']}

    def canonical_gate(self):
        return {'barrier_owner':'voice','relation_ok':True,'acl_ok':True,'function_owner':'voice',
          'function_ok':True,'function_source':'\nBEGIN\n  LOCK TABLE ONLY public.auth_identity_ddl_barrier IN ACCESS EXCLUSIVE MODE NOWAIT;\n  RETURN;\nEND ',
          'event_owner':'voice','event_enabled':'O','binding_ok':True,'major_ok':True,'runtime_ok':False,
          'event_tags':['CREATE FUNCTION','ALTER FUNCTION','DROP FUNCTION','CREATE PROCEDURE','ALTER PROCEDURE','DROP PROCEDURE',
                        'ALTER ROUTINE','DROP ROUTINE','CREATE EXTENSION','ALTER EXTENSION','DROP EXTENSION','CREATE LANGUAGE',
                        'ALTER LANGUAGE','DROP LANGUAGE','ALTER SCHEMA','DROP SCHEMA','DROP OWNED','GRANT','REVOKE','ALTER DEFAULT PRIVILEGES']}

    def post_v31(self):
        current=copy.deepcopy(self.db)
        current['auth_history'] += [{'version':str(i),'success':True,'checksum':i} for i in [28,31,32]]
        current['event_triggers']={'auth_db':['auth_identity_ddl_gate'],'user_db':[]}
        current['auth_ddl_gate']=self.canonical_gate()
        return current

    def test_real_guard_transition_after_v31_advances_engine_to_next_effect(self):
        from auth_user_adoption import Journal,Operator
        from auth_user_adoption_test import FakeBackend
        baseline=self.db;current=self.post_v31()
        class Backend(FakeBackend):
            def __init__(self):
                super().__init__();self.db=copy.deepcopy(baseline)
            def guard(self,state): database_guard(baseline,self.db,state['steps'])
            def execute(self,step,state):
                if step=='migrate_auth': self.db=copy.deepcopy(current)
                super().execute(step,state)
        import tempfile
        with tempfile.TemporaryDirectory() as tmp:
            journal=Journal(pathlib.Path(tmp)/'operation',{'operation':'fixture'})
            try:
                backend=Backend()
                with patch('auth_user_adoption.STEPS',('migrate_auth','secret')):
                    Operator(journal,backend).run()
                self.assertEqual(['migrate_auth','secret'],backend.effects)
                self.assertEqual('done',journal.state['steps']['migrate_auth']['status'])
            finally: journal.close()

    def test_only_exact_postmigration_trigger_function_contract_is_admitted(self):
        for fault in ['unknown','before_migration','user_trigger','owner','enabled','body','tags','function']:
            current=self.post_v31();steps={'migrate_auth':{'status':'pending'}}
            if fault=='unknown': current['event_triggers']['auth_db'].append('foreign')
            if fault=='before_migration': steps={}
            if fault=='user_trigger': current['event_triggers']['user_db']=['foreign']
            if fault=='owner': current['auth_ddl_gate']['event_owner']='foreign'
            if fault=='enabled': current['auth_ddl_gate']['event_enabled']='D'
            if fault=='body': current['auth_ddl_gate']['function_source']+=' altered'
            if fault=='tags': current['auth_ddl_gate']['event_tags'].pop()
            if fault=='function': current['auth_ddl_gate']['function_ok']=False
            with self.subTest(fault=fault),self.assertRaises(RecoveryRequired): database_guard(self.db,current,steps)

    def test_original_owners_and_clean_historical_ledgers_required(self):
        for fault in ['owner','object','auth_checksum','auth_missing','user_dirty','unexpected_login','foreign_runtime']:
            current = copy.deepcopy(self.db)
            if fault == 'owner': current['owners']['auth_db'] = 'foreign'
            if fault == 'object': current['objects']['auth_db/accounts'] = 'auth_runtime'
            if fault == 'auth_checksum': current['auth_history'][0]['checksum'] = 999
            if fault == 'auth_missing': current['auth_history'].pop()
            if fault == 'user_dirty': current['user_history']['dirty'] = True
            if fault == 'unexpected_login': current['logins'].append('foreign')
            if fault == 'foreign_runtime': current['roles']['auth_runtime'] = {'superuser':True}
            with self.subTest(fault=fault), self.assertRaises(RecoveryRequired):
                database_guard(self.db, current, {})

    def test_pending_migration_accepts_only_attributable_expected_versions(self):
        current = self.post_v31()
        database_guard(self.db, current, {'migrate_auth':{'status':'pending'}})
        with self.assertRaises(RecoveryRequired):
            database_guard(self.db, current, {})
        current['auth_history'][-1]['version'] = '33'
        with self.assertRaises(RecoveryRequired):
            database_guard(self.db, current, {'migrate_auth':{'status':'pending'}})

    def test_new_objects_must_stay_admin_owned_and_no_runtime_memberships(self):
        current = copy.deepcopy(self.db)
        current['objects']['auth_db/new_table'] = 'foreign'
        with self.assertRaises(RecoveryRequired):
            database_guard(self.db, current, {'migrate_auth':{'status':'pending'}})
        current = copy.deepcopy(self.db)
        current['memberships'] = [{'role':'voice','member':'auth_runtime'}]
        with self.assertRaises(RecoveryRequired):
            database_guard(self.db, current, {'roles':{'status':'pending'}})


class RuntimeControlTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.cached_sources=source_config(pathlib.Path(__file__).resolve().parents[2],'owned')

    def runtime(self):
        root = pathlib.Path(__file__).resolve().parents[2]
        baseline={'namespace':{},'storage':{},'bindings':{},'peer':{},'workloads':{},'ready':{}}
        db={'owners':{},'schemas':{},'objects':{},'roles':{},'memberships':[],'auth_history':[],
            'user_history':{'version':19,'dirty':False},'logins':['voice'],'sessions':[]}
        resource={'claim':'claim','generation':1,'window':'window','candidate':{'images':{'auth':'ref'}}}
        intent={'operation':'owned','resource':resource,'baseline':baseline,'database':db,
                'expires_at':(datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(minutes=5)).isoformat(),
                'sources_sha256':digest(self.cached_sources)}
        state={'intent':intent,'credentials':{'auth':'a'*40,'user':'b'*40},
               'steps':{'lock':{'status':'done','receipt':{'uid':'owned-lock'}}}}
        kube=Mock()
        kube.call.return_value={}
        card=dict(resource,owner='Dimas-16',state='held',phase='deploy',tasks=['A1-DEV-AUTH-USER-CREDENTIAL-ADOPTION'])
        with patch('auth_user_adoption_runtime.source_config',return_value=copy.deepcopy(self.cached_sources)):
            runtime=Runtime(SimpleNamespace(state=state),kube,root,resource=lambda:card)
        return runtime,baseline,db

    def test_missing_owned_lock_prevents_progress(self):
        runtime,baseline,db=self.runtime()
        with patch('auth_user_adoption_runtime.capture',return_value=(baseline,{},[])), \
             patch('auth_user_adoption_runtime.database_state',return_value=db), self.assertRaises(RecoveryRequired):
            runtime.guard(runtime.journal.state)

    def test_window_expiry_stops_even_readied_candidate(self):
        runtime,_,_=self.runtime()
        runtime.intent['expires_at']='2000-01-01T00:00:00+00:00'
        with self.assertRaises(RecoveryRequired):
            runtime.guard(runtime.journal.state)
        runtime.kube.call.assert_not_called()

    def test_lost_finish_response_reconciles_absence_without_second_delete(self):
        runtime,_,_=self.runtime()
        state=runtime.journal.state
        state['steps']={step:{'status':'done','receipt':{'uid':'owned-lock'}} for step in STEPS}
        state['finish']={'status':'pending','lock_uid':'owned-lock'}
        runtime.guard=Mock();runtime.verify_isolation=Mock();runtime.journal.save=Mock()
        runtime.finish()
        self.assertEqual('done',state['finish']['status'])
        self.assertTrue(all(call.args[0]=='get' for call in runtime.kube.call.call_args_list))

    def test_lingering_old_session_prevents_finish_and_delete(self):
        runtime,_,_=self.runtime()
        runtime.journal.state['steps']={step:{'status':'done','receipt':{'uid':'owned-lock'}} for step in STEPS}
        runtime.guard=Mock();runtime.verify_isolation=Mock(side_effect=RecoveryRequired('old admin session'))
        with self.assertRaises(RecoveryRequired): runtime.finish()
        runtime.kube.call.assert_not_called()

    def test_late_grant_catalog_or_default_change_blocks_next_effect(self):
        from auth_user_adoption import Operator
        from auth_user_adoption_test import FakeBackend
        for step,db_name in [('grants_auth','auth_db'),('grants_user','user_db')]:
            for changed in ('relations','defaults'):
                runtime,baseline,db=self.runtime()
                db['acls']={'auth_db':{'relations':{'accounts':'original'},'defaults':[]},
                            'user_db':{'relations':{'profiles':'original'},'defaults':[]}}
                state=runtime.journal.state;runtime.journal.save=Mock()
                state['steps'][step]={'status':'done','receipt':{'uid':'grant-job','catalog_sha256':digest(db['acls'][db_name])}}
                state['plans']={step:{'metadata':{'name':'owned-grant'},'spec':{'exact':True}}}
                def call(*args,**kwargs):
                    if args[:2]==('get','configmap'): return {'metadata':{'uid':'owned-lock'},'data':runtime.lock['data']}
                    if args[:2]==('get','job'): return {'metadata':{'uid':'grant-job'},'spec':{'exact':True}}
                    raise AssertionError('unexpected API route')
                runtime.kube.call.side_effect=call
                backend=FakeBackend();backend.guard=runtime.guard
                backend.actual={name:{'status':'complete','receipt':copy.deepcopy(record['receipt'])} for name,record in state['steps'].items()}
                db['acls'][db_name][changed]='late-foreign-change'
                with self.subTest(step=step,changed=changed), \
                     patch('auth_user_adoption_runtime.capture',return_value=(baseline,{},[])), \
                     patch('auth_user_adoption_runtime.database_state',return_value=db), \
                     patch('auth_user_adoption.STEPS',('lock',step,'next_effect')):
                    with self.assertRaises(RecoveryRequired): Operator(runtime.journal,backend).run()
                    self.assertEqual([],backend.effects)


class AdmissionTest(unittest.TestCase):
    def setUp(self):
        self.manifest={'source_sha':'f'*40,'operator_sha256':'1'*64,'products':{
            'auth':'repo/auth@sha256:'+'a'*64,'user':'repo/user@sha256:'+'b'*64},
            'runtime_ids':{'auth':['containerd://sha256:'+'c'*64],'user':['containerd://sha256:'+'d'*64]},
            'recovery_mode':'reviewed_forward_only'}
        self.manifest['tools']=dict(QUALIFIED_TOOLS)
        self.evidence={kind:{'kind':kind,'status':'PASS','source_sha':'f'*40,
             'operator_sha256':'1'*64,'products':copy.deepcopy(self.manifest['products']),
             'reference':'TT retained immutable attachment', 'recovery_mode':'reviewed_forward_only'}
             for kind in ['image_provenance','runtime_roles','recovery','operator_review']}
        self.evidence['operator_review']['reviewer']='Dimas-8'
        self.evidence['recovery']['reviewer']='Dimas-8'
        self.evidence['image_provenance'].update(runtime_ids=copy.deepcopy(self.manifest['runtime_ids']),
            node_accessibility=True,artifact_scope='qualified_node_import')

    def test_complete_bound_independent_forward_recovery_record_is_admitted(self):
        validate_admission(self.manifest,self.evidence)

    def test_missing_stale_or_wrong_scope_records_are_not_live_eligibility(self):
        for fault in ['missing','status','products','source','operator','reviewer','recovery','mutable','runtime_ids']:
            manifest=copy.deepcopy(self.manifest); evidence=copy.deepcopy(self.evidence)
            if fault=='missing': evidence.pop('recovery')
            if fault=='status': evidence['runtime_roles']['status']='INPUT_CLEAR'
            if fault=='products': evidence['image_provenance']['products']['user']='unqualified'
            if fault=='source': evidence['runtime_roles']['source_sha']='old'
            if fault=='operator': evidence['operator_review']['operator_sha256']='old'
            if fault=='reviewer': evidence['operator_review']['reviewer']='Dimas-16'
            if fault=='recovery': evidence['recovery']['recovery_mode']='recovered_executable_recipe'
            if fault=='mutable': manifest['products']['user']='repo/user:tag'
            if fault=='runtime_ids': manifest['runtime_ids']['user']=[]
            with self.subTest(fault=fault), self.assertRaises(RecoveryRequired):
                validate_admission(manifest,evidence)


class ConsumerTest(unittest.TestCase):
    def test_recognized_gis_default_connect_does_not_imply_data_access(self):
        validate_consumers([{'name':'gameintegration_runtime','read_tables':0,'write_tables':0,'schema_create':False}])

    def test_unknown_login_or_existing_data_grant_blocks_public_revocation(self):
        for fault in ['name','read_tables','write_tables','schema_create']:
            record={'name':'gameintegration_runtime','read_tables':0,'write_tables':0,'schema_create':False}
            record[fault]='foreign' if fault=='name' else 1
            with self.subTest(fault=fault),self.assertRaises(RecoveryRequired):
                validate_consumers([record])


class ProjectionTest(unittest.TestCase):
    def test_user_alias_literal_or_full_projection_of_admin_or_auth_password_refused(self):
        own={'metadata':{'name':'own'},'data':{'USER_DATABASE_URL':base64.b64encode(b'postgres://user_runtime:'+b'b'*40+b'@voice-postgres:5432/user_db?sslmode=disable').decode()}}
        for fault in ['volume','env','literal','configmap','auth_password']:
            pod={'containers':[{'name':'user','env':[{'name':'DATABASE_URL','valueFrom':{'secretKeyRef':{'name':'own','key':'USER_DATABASE_URL'}}}]}],'volumes':[]}
            kube=Mock();kube.call.return_value={'data':{'alias':base64.b64encode(b'c'*40).decode()}}
            if fault=='volume': pod['volumes']=[{'name':'unknown','secret':{'secretName':'other'}}]
            if fault=='env': pod['containers'][0]['env'].append({'name':'UNKNOWN','valueFrom':{'secretKeyRef':{'name':'other','key':'alias'}}})
            if fault=='literal': pod['containers'][0]['env'].append({'name':'UNKNOWN','value':'c'*40})
            if fault=='auth_password': pod['containers'][0]['env'].append({'name':'UNKNOWN','value':'a'*40})
            if fault=='configmap':
                pod['containers'][0]['envFrom']=[{'configMapRef':{'name':'other'}}];kube.call.return_value={'data':{'alias':'c'*40}}
            with self.subTest(fault=fault),self.assertRaises(RecoveryRequired):
                validate_user_projection(kube,{'spec':{'template':{'spec':pod}}},own,[b'c'*40,b'a'*40])

    def test_user_own_url_and_legitimate_r2_nats_peer_bindings_survive(self):
        own={'metadata':{'name':'own'},'data':{'USER_DATABASE_URL':base64.b64encode(b'postgres://user_runtime:'+b'b'*40+b'@voice-postgres:5432/user_db?sslmode=disable').decode()}}
        pod={'containers':[{'name':'user','env':[{'name':'DATABASE_URL','valueFrom':{'secretKeyRef':{'name':'own','key':'USER_DATABASE_URL'}}}],
               'envFrom':[{'configMapRef':{'name':'config'}}]}],
             'volumes':[{'name':'nats','secret':{'secretName':'nats'}}]}
        kube=Mock();kube.call.side_effect=lambda *args,**kwargs: {'data':{'POSTGRES_USER':'voice','SPACE_GRPC_ADDR':'voice-space:9090'}} if args[1]=='configmap' else {'data':{'user.creds':base64.b64encode(b'private-fixture-nats').decode()}}
        validate_user_projection(kube,{'spec':{'template':{'spec':pod}}},own,[b'c'*40,b'a'*40])

    def test_aliased_or_projected_user_admin_credentials_cannot_reach_auth(self):
        own={'metadata':{'name':'own'},'data':{'AUTH_DB_PASSWORD':base64.b64encode(b'a'*40).decode()}}
        for fault in ['volume','env','literal','configmap']:
            pod={'containers':[{'name':'auth','env':[{'name':'SPRING_DATASOURCE_PASSWORD','valueFrom':{'secretKeyRef':{'name':'own','key':'AUTH_DB_PASSWORD'}}}]}],'volumes':[]}
            kube=Mock()
            kube.call.return_value={'data':{'alias':base64.b64encode(b'c'*40).decode()}}
            if fault=='volume': pod['volumes']=[{'name':'unknown','secret':{'secretName':'other'}}]
            if fault=='env': pod['containers'][0]['env'].append({'name':'UNKNOWN','valueFrom':{'secretKeyRef':{'name':'other','key':'alias'}}})
            if fault=='literal': pod['containers'][0]['env'].append({'name':'UNKNOWN','value':'c'*40})
            if fault=='configmap':
                pod['containers'][0]['envFrom']=[{'configMapRef':{'name':'other'}}]
                kube.call.return_value={'data':{'alias':'c'*40}}
            with self.subTest(fault=fault),self.assertRaises(RecoveryRequired):
                validate_auth_projection(kube,{'spec':{'template':{'spec':pod}}},own,[b'c'*40,b'b'*40])

    def test_only_own_runtime_env_and_explicit_jwt_projection_is_admitted(self):
        own={'metadata':{'name':'own'},'data':{'AUTH_DB_PASSWORD':base64.b64encode(b'a'*40).decode()}}
        pod={'containers':[{'name':'auth','env':[{'name':'SPRING_DATASOURCE_PASSWORD','valueFrom':{'secretKeyRef':{'name':'own','key':'AUTH_DB_PASSWORD'}}}]}],
             'volumes':[{'name':'jwt','secret':{'secretName':'other','items':[{'key':'jwt.key','path':'jwt.key'}]}}]}
        kube=Mock();kube.call.return_value={'data':{'jwt.key':base64.b64encode(b'private-fixture-jwt').decode(),'USER_DB_PASSWORD':base64.b64encode(b'b'*40).decode()}}
        validate_auth_projection(kube,{'spec':{'template':{'spec':pod}}},own,[b'c'*40,b'b'*40])


class SchemaBindingTest(unittest.TestCase):
    def test_preprepare_migration_and_catalog_tamper_is_rejected_before_live_access(self):
        actual=pathlib.Path(__file__).resolve().parents[2]
        with tempfile.TemporaryDirectory() as tmp:
            root=pathlib.Path(tmp)/'package'
            for relative in ['scripts/dev','docker/postgres','src/backend/auth/src/main/resources/db/migration',
                             'src/backend/migrations/user_db','src/backend/auth/src/main/resources/roleidentityauthority']:
                (root/relative).mkdir(parents=True,exist_ok=True)
            for suffix in ('.py','_runtime.py','_backup.py','_smoke.py','_restore.py'):
                relative='scripts/dev/auth_user_adoption'+suffix
                shutil.copy2(actual/relative,root/relative)
            for relative in ['docker/postgres/adopt-auth-user-runtime-roles.sql','docker/postgres/provision-auth-user-runtime-roles.sql']:
                shutil.copy2(actual/relative,root/relative)
            for folder in ['src/backend/auth/src/main/resources/db/migration','src/backend/migrations/user_db']:
                for source in (actual/folder).glob('*.sql'): shutil.copy2(source,root/folder/source.name)
            catalog='src/backend/auth/src/main/resources/roleidentityauthority/gate-catalog.sql'
            shutil.copy2(actual/catalog,root/catalog)
            fixture=AdmissionTest();fixture.setUp();manifest=fixture.manifest
            original_hash=operator_fingerprint(root);manifest['operator_sha256']=original_hash
            manifest['operation']='fixture'
            manifest['evidence']={}
            for kind,record in fixture.evidence.items():
                record['operator_sha256']=original_hash
                path=pathlib.Path(tmp)/(kind+'.json');path.write_text(json.dumps(record))
                manifest['evidence'][kind]={'path':str(path),'sha256':hashlib.sha256(path.read_bytes()).hexdigest()}
            verify_package(root,manifest)
            manifest_path=pathlib.Path(tmp)/'candidate.json';manifest_path.write_text(json.dumps(manifest))
            targets=[root/'src/backend/auth/src/main/resources/db/migration/V32__linked_oauth_transport.sql',
                     sorted((root/'src/backend/migrations/user_db').glob('*.up.sql'))[-1],root/catalog]
            for target in targets:
                raw=target.read_bytes();target.write_bytes(raw+b'\n-- unqualified tamper\n')
                resource=Mock(side_effect=AssertionError('live resource accessed before provenance rejection'))
                kube=Mock()
                try:
                    with self.subTest(path=target.name):
                        with self.assertRaises(RecoveryRequired):
                            prepare(root,pathlib.Path(tmp)/'journal',manifest_path,kube,resource=resource)
                        resource.assert_not_called();kube.call.assert_not_called()
                finally: target.write_bytes(raw)


if __name__ == '__main__':
    unittest.main()
