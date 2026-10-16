"""Source-bound producer and protected prerequisite collector contract tests."""
import copy
import hashlib
import json
import tempfile
import unittest
from unittest.mock import patch
from pathlib import Path
import yaml

import gis_prerequisite as prerequisite
import gis_first_install as gis
import gis_first_install_test as gis_fi_test
import controller


ROOT = Path(__file__).resolve().parents[3]


def capture(root):
    producer = prerequisite._load_producer(root)
    inputs = producer.gameintegration_prerequisite_inputs(root)
    paths = {
        'scripts/dev/migrations.py',
        'docker/postgres/provision-gameintegration-runtime-role.sql',
        'docker/postgres/verify-gameintegration-runtime-role.sql',
        'deploy/templates/provision-gameintegration-runtime-role-job.yaml',
        'deploy/templates/migrate-game-integration-db-job.yaml',
        'deploy/templates/verify-game-integration-db-job.yaml',
    }
    paths.update('src/backend/migrations/game_integration_db/' + row['name']
                 for row in inputs['migrations'])
    return {name: hashlib.sha256((root / name).read_bytes()).hexdigest() for name in paths}


class FakeKube:
    def __init__(self):
        self.objects = {}
        self.events = []
        self.next_uid = 10
        self.namespace = {'metadata': {'uid': '00000000-0000-0000-0000-000000000001'}}
        self.service = {'metadata': {'uid': '00000000-0000-0000-0000-000000000002',
            'namespace': 'voice-staging'}, 'spec': {'type': 'ClusterIP',
            'ports': [{'port': 5432}], 'selector': {'app': 'postgres'}}}
        self.postgres = {'metadata': {'name': 'postgres-0', 'namespace': 'voice-staging',
            'uid': '00000000-0000-0000-0000-000000000003', 'labels': {'app': 'postgres'}},
            'status': {'phase': 'Running', 'conditions': [{'type': 'Ready', 'status': 'True'}]}}
        self.config = {'metadata': {'uid': '00000000-0000-0000-0000-000000000004',
            'resourceVersion': '4'}, 'data': {'POSTGRES_USER': 'postgres'}}
        self.secret = {'uid': '00000000-0000-0000-0000-000000000005', 'resourceVersion': '5'}

    def get(self, kind, name):
        if kind == 'namespace': return copy.deepcopy(self.namespace)
        if kind == 'service': return copy.deepcopy(self.service)
        if kind == 'pod' and name == 'postgres-0': return copy.deepcopy(self.postgres)
        if kind == 'configmap' and name == 'voice-app-config': return copy.deepcopy(self.config)
        if kind == 'job': return copy.deepcopy(self.objects[('job', name)])
        raise AssertionError((kind, name))

    def secret_meta(self, name):
        if name != 'voice-app-secrets': raise AssertionError(name)
        return copy.deepcopy(self.secret)

    def run(self, args, body=None, timeout=30):
        self.events.append(tuple(args))
        if args[:2] == ['get', 'endpointslices']:
            return {'items': [{'metadata': {'uid': '00000000-0000-0000-0000-000000000006'},
                'endpoints': [{'conditions': {'ready': True}, 'targetRef': {
                    'kind': 'Pod', 'name': 'postgres-0', 'uid': self.postgres['metadata']['uid']}}]}]}
        if args[:2] == ['get', 'secret']:
            return True
        if args[:2] == ['get', 'configmaps']:
            return {'items': [row for (kind, _), row in self.objects.items() if kind == 'configmap']}
        if args[:2] == ['get', 'jobs']:
            return {'items': [row for (kind, _), row in self.objects.items() if kind == 'job']}
        if args[:2] == ['get', 'pods']:
            job_name = args[args.index('-l') + 1].split('=', 1)[1]
            return {'items': [row for (kind, name), row in self.objects.items()
                if kind == 'pod' and name == job_name + '-pod']}
        if args[0] == 'create':
            obj = copy.deepcopy(body)
            kind = obj['kind'].lower()
            name = obj['metadata']['name']
            self.next_uid += 1
            uid = f'00000000-0000-0000-0000-{self.next_uid:012d}'
            obj['metadata'].update(uid=uid, resourceVersion='1')
            if kind == 'job':
                generated = {'batch.kubernetes.io/controller-uid': uid,
                    'batch.kubernetes.io/job-name': name}
                obj['spec']['selector'] = {'matchLabels': generated}
                obj['spec']['manualSelector'] = False
                obj['spec']['template']['metadata'].setdefault('labels', {}).update(generated)
                obj['status'] = {'succeeded': 1, 'failed': 0,
                    'completionTime': '2026-10-17T00:00:00Z',
                    'conditions': [{'type': 'Complete', 'status': 'True'}]}
                self.objects[(kind, name)] = obj
                spec = obj['spec']['template']['spec']
                init_statuses = []
                container_statuses = []
                for group, status_key in (('initContainers', 'initContainerStatuses'),
                                          ('containers', 'containerStatuses')):
                    for container in spec.get(group, []):
                        status = {'name': container['name'], 'imageID': container['image'],
                            'state': {'terminated': {'exitCode': 0}}}
                        (init_statuses if group == 'initContainers' else container_statuses).append(status)
                pod = {'metadata': {'name': name + '-pod', 'namespace': 'voice-staging',
                    'uid': f'00000000-0000-0000-0000-{self.next_uid + 100:012d}',
                    'ownerReferences': [{'kind': 'Job', 'uid': uid, 'controller': True}]},
                    'status': {'phase': 'Succeeded', 'initContainerStatuses': init_statuses,
                        'containerStatuses': container_statuses}}
                self.objects[('pod', name + '-pod')] = pod
                self.events.append(('created-job', name))
                return [name, uid]
            self.objects[(kind, name)] = obj
            return [name, uid]
        if args[0] == 'wait':
            return copy.deepcopy(self.objects[('job', args[2].split('/', 1)[1])])
        if args[0] == 'logs':
            return json.dumps(prerequisite.OUTPUT, separators=(',', ':')) + '\n'
        raise AssertionError(args)


class ReadyKube:
    def __init__(self):
        self.objects = {}
        self.next_uid = 10
        self.pod = None
        self.rs = None
        self.slice = None

    def run(self, args, body=None):
        if args[:2] == ['get', 'services,deployments']:
            return {'apiVersion': 'v1', 'kind': 'List', 'metadata': {},
                'items': [row for (kind, _), row in self.objects.items() if kind in ('Service', 'Deployment')]}
        if args[:2] == ['get', 'endpointslices']:
            return {'items': [copy.deepcopy(self.slice)]}
        if args[:2] == ['get', 'replicasets']:
            return {'items': [copy.deepcopy(self.rs)]}
        if args[0] == 'create':
            obj = copy.deepcopy(body)
            kind, name = obj['kind'], obj['metadata']['name']
            self.next_uid += 1
            uid = f'00000000-0000-0000-0000-{self.next_uid:012d}'
            obj['metadata'].update(uid=uid, resourceVersion='1')
            self.objects[(kind, name)] = obj
            return copy.deepcopy(obj)
        if args[0] == 'patch':
            row = self.objects[('Deployment', args[2])]
            patch = json.loads(args[args.index('--patch') + 1])
            if row['metadata']['uid'] != patch[0]['value'] or row['metadata']['resourceVersion'] != patch[1]['value']:
                raise Conflict()
            row['spec']['replicas'] = 1
            row['metadata']['resourceVersion'] = '2'
            row['metadata']['generation'] = 1
            row['status'] = {'observedGeneration': 1, 'readyReplicas': 1,
                'updatedReplicas': 1, 'availableReplicas': 1}
            self._ready_objects()
            return copy.deepcopy(row)
        raise AssertionError(args)

    def get(self, kind, name):
        if kind == 'namespace' and name == gis.NS:
            return copy.deepcopy(self.objects.get(('Namespace', name),
                {'metadata': {'uid': '00000000-0000-0000-0000-000000000001'}}))
        if kind == 'pod' and self.pod and name == self.pod['metadata']['name']:
            return copy.deepcopy(self.pod)
        row = self.objects.get((kind.capitalize(), name))
        if row is None: raise NotFound()
        return copy.deepcopy(row)

    def _ready_objects(self):
        deployment = self.objects[('Deployment', 'voice-gameintegration')]
        dep_uid = deployment['metadata']['uid']
        rs_uid = '00000000-0000-0000-0000-000000000101'
        pod_uid = '00000000-0000-0000-0000-000000000102'
        selector = deployment['spec']['selector']['matchLabels']
        self.rs = {'metadata': {'uid': rs_uid, 'ownerReferences': [
            {'kind': 'Deployment', 'uid': dep_uid, 'controller': True}]}}
        self.pod = {'metadata': {'name': 'voice-gameintegration-rs-pod', 'namespace': 'voice-staging',
            'uid': pod_uid, 'labels': selector, 'ownerReferences': [
                {'kind': 'ReplicaSet', 'uid': rs_uid, 'controller': True}]},
            'status': {'conditions': [{'type': 'Ready', 'status': 'True'}]}}
        self.slice = {'metadata': {'uid': '00000000-0000-0000-0000-000000000103'},
            'endpoints': [{'conditions': {'ready': True}, 'targetRef': {'kind': 'Pod',
                'name': self.pod['metadata']['name'], 'uid': pod_uid}}]}


class Tests(unittest.TestCase):
    def test_producer_metadata_and_all_file_hashes_are_bound_to_captured_source(self):
        bound = capture(ROOT)
        inputs = prerequisite.source_inputs(ROOT, bound)
        self.assertEqual(inputs['schema'], 'voice-gis-prerequisite-v1')
        self.assertEqual(inputs['jobs']['verify'], 'voice-verify-gameintegration-db')
        self.assertEqual(inputs['configmaps']['verify'], 'voice-gameintegration-db-verifier-sql')
        self.assertEqual(inputs['migration_inputs_sha256'], prerequisite._sha(
            prerequisite._canonical(inputs['migrations']).encode()))
        changed = copy.deepcopy(bound)
        changed['docker/postgres/verify-gameintegration-runtime-role.sql'] = '0' * 64
        with self.assertRaisesRegex(gis.Blocked, 'capture_changed'):
            prerequisite.source_inputs(ROOT, changed)

    def test_collector_builds_only_source_pinned_immutable_inputs_and_jobs(self):
        bound = capture(ROOT)
        inputs = prerequisite.source_inputs(ROOT, bound)
        plan = {'inputs': inputs, 'inputs_sha256': gis.digest(inputs), 'operation': 'a'*12}
        for role, relative in {
            'provision': 'deploy/templates/provision-gameintegration-runtime-role-job.yaml',
            'migrate': 'deploy/templates/migrate-game-integration-db-job.yaml',
            'verify': 'deploy/templates/verify-game-integration-db-job.yaml',
        }.items():
            raw = (ROOT / relative).read_text(encoding='utf-8')
            obj = yaml.safe_load(raw.replace('__K_NAMESPACE__', prerequisite.NS))
            self.assertEqual((obj.get('kind'), obj.get('metadata', {}).get('name'),
                obj.get('metadata', {}).get('namespace')),
                ('Job', inputs['jobs'][role], prerequisite.NS), role)
        configmaps, jobs = prerequisite._documents(ROOT, plan, bound)
        self.assertEqual(set(configmaps), {'provision', 'migrate', 'verify'})
        self.assertEqual(set(jobs), {'provision', 'migrate', 'verify'})
        for row in configmaps.values():
            self.assertIs(row['immutable'], True)
            self.assertEqual(row['metadata']['labels'][prerequisite.OP_LABEL], 'a'*12)
        expected_backoff = {'provision': 2, 'migrate': 2, 'verify': 0}
        for role, job in jobs.items():
            self.assertEqual(job['metadata']['name'], inputs['jobs'][role])
            self.assertEqual(job['metadata']['labels'][prerequisite.OP_LABEL], 'a'*12)
            self.assertEqual(job['spec']['template']['metadata']['labels'][prerequisite.OP_LABEL], 'a'*12)
            self.assertEqual(job['spec']['backoffLimit'], expected_backoff[role])

    def test_verifier_output_contract_requires_ledger_write_denial(self):
        self.assertEqual(prerequisite.OUTPUT['schema_ledger_runtime_mutation_denied'], True)
        altered = dict(prerequisite.OUTPUT, schema_ledger_runtime_mutation_denied=False)
        self.assertNotEqual(altered, prerequisite.OUTPUT)

    def test_actual_collector_runs_source_owned_jobs_in_order_and_returns_bound_receipt(self):
        bound = capture(ROOT)
        inputs = prerequisite.source_inputs(ROOT, bound)
        plan = {'schema': gis.SCHEMA, 'operation': 'a'*12, 'source_sha': 'b'*40,
            'source_plan_sha256': 'c'*64, 'source_ci': {'run_id': 1, 'run_attempt': 1,
                'workflow_id': 1, 'tree_sha': 'd'*40, 'lock_artifact_id': 1, 'lock_sha256': 'e'*64},
            'inputs': inputs, 'inputs_sha256': gis.digest(inputs), 'source_capture': bound,
            'custody_binding_sha256': 'f'*64, 'phase': 'PLANNED'}
        kube = FakeKube()
        saved = []
        receipt = prerequisite.execute(kube, plan, ROOT, bound, saved.append)
        created_jobs = [event[1] for event in kube.events
                        if len(event) == 2 and event[0] == 'created-job']
        self.assertEqual(created_jobs, [inputs['jobs'][role] for role in prerequisite.JOB_ROLES])
        self.assertEqual(receipt['verifier'], prerequisite.OUTPUT)
        self.assertEqual(set(receipt['jobs']), {'provision', 'migrate', 'verify'})
        self.assertTrue(gis.validate_prerequisite_receipt(receipt, plan))
        self.assertEqual(plan['prerequisite_phase'], 'VERIFIED')

    def test_authorize_gis_stage_orders_real_producer_jobs_before_only_pair_creates(self):
        import transaction
        class FullKube(FakeKube):
            def run(self, args, body=None, timeout=30):
                if args[:2] == ['get', 'services,deployments']:
                    rows=[row for (kind, _), row in self.objects.items()
                          if kind in ('Service','Deployment')]
                    return {'apiVersion':'v1','kind':'List','metadata':{},'items':rows}
                return super().run(args,body,timeout)
            def create(self, obj):
                key=(obj['kind'],obj['metadata']['name'])
                self.next_uid+=1
                created=copy.deepcopy(obj)
                created['metadata'].update(uid=f'00000000-0000-0000-0000-{self.next_uid:012d}',resourceVersion='1')
                self.objects[key]=created
                self.events.append(('gis-create',key[0],key[1]))
                return copy.deepcopy(created)
            def start(self, live, *, replicas, expected_uid, expected_resource_version):
                row=self.objects[(live['kind'],live['metadata']['name'])]
                if row['metadata']['uid']!=expected_uid or row['metadata']['resourceVersion']!=expected_resource_version:
                    raise AssertionError('start precondition changed')
                row['spec']['replicas']=replicas
                row['metadata']['resourceVersion']=str(int(expected_resource_version)+1)
                self.events.append(('gis-start',row['metadata']['name']))
                return copy.deepcopy(row)
            def delete(self,kind,name,*,uid,resource_version):
                row=self.objects[(kind,name)]
                if row['metadata']['uid']!=uid or row['metadata']['resourceVersion']!=resource_version:
                    raise AssertionError('delete precondition changed')
                del self.objects[(kind,name)]
                self.events.append(('gis-delete',kind,name))
        source=capture(ROOT)
        inputs=prerequisite.source_inputs(ROOT,source)
        gis_plan=gis_fi_test.plan()
        gis_plan['inputs']=inputs
        gis_plan['inputs_sha256']=gis.digest(inputs)
        gis_plan['source_capture']=source
        gis_plan['source_root']=str(ROOT)
        gis_plan.pop('prerequisite_receipt')
        gis_plan['created']={};gis_plan['jobs']={};gis_plan['phase']='PLANNED'
        for key in ('database_identity','secret_metadata','config_metadata'):
            gis_plan.pop(key,None)
        kube=FullKube()
        prerequisite.preflight(kube,gis_plan)
        state={'operation':gis_plan['operation'],'target':{'tag':gis_plan['source_sha'],'mode':'app-only'},
            'gis_first_install':gis_plan,'custody':{'verified':True},
            'cut':{'manifest':{'archive_sha256':'6'*64},'manifest_sha256':'7'*64,'census_sha256':'8'*64},
            'context':{'snapshots':{'voice-'+str(i):{} for i in range(22)}}}
        class Stage:
            expected={'namespace_uid':'00000000-0000-0000-0000-000000000001',
                'marker_uid':'00000000-0000-0000-0000-000000000011',
                'source_claim_uid':'00000000-0000-0000-0000-000000000012',
                'source_pv_uid':'00000000-0000-0000-0000-000000000013'}
            def verify_final_storage(self): pass
        journal=[]
        with tempfile.TemporaryDirectory() as folder, patch.object(transaction,'save'):
            receipt=transaction.execute_gis_first_install(kube,folder,state,Stage(),journal.append)
        events=[event for event in kube.events if event[0] in ('created-job','gis-create','gis-start')]
        self.assertEqual([event[0] for event in events],
            ['created-job']*3+['gis-create','gis-create'])
        self.assertEqual([event[1] for event in events[:3]],
            [inputs['jobs'][role] for role in prerequisite.JOB_ROLES])
        self.assertEqual({(event[1],event[2]) for event in events if event[0]=='gis-create'},
            {('Service','voice-gameintegration'),('Deployment','voice-gameintegration')})
        self.assertEqual(receipt['schema'],gis.RESOURCE_RECEIPT_SCHEMA)
        self.assertEqual(kube.objects[gis.DEPLOYMENT]['spec']['replicas'],0)
        self.assertTrue(state['gis_first_install']['prerequisite_receipt']['verifier']['schema_ledger_runtime_mutation_denied'])

        import apply
        docs=apply.paused_documents([], 
            {'operation':state['operation'],'target':{'gis_first_install_sha256':'c'*64}})
        self.assertEqual(docs,[])
        with self.assertRaisesRegex(controller.Blocked,'gis_first_install_manifest_forbidden'):
            apply.paused_documents(state['gis_first_install']['objects'],
                {'operation':state['operation'],'target':{'gis_first_install_sha256':'c'*64}})
        self.assertEqual(kube.objects[gis.DEPLOYMENT]['spec']['replicas'],0)
        # Generic apply cannot create, adopt, or rewrite either identity.
        started=gis.start_owned(kube,state['gis_first_install'],lambda _row:None)
        self.assertEqual(started['schema'],gis.RESOURCE_RECEIPT_SCHEMA)
        self.assertEqual(kube.objects[gis.DEPLOYMENT]['spec']['replicas'],1)
        self.assertEqual([event[0] for event in kube.events if event[0]=='gis-start'],['gis-start'])
        with patch.object(transaction,'save'):
            self.assertTrue(transaction.rollback_gis_first_install(kube,folder,state,journal.append))
        self.assertFalse(any(identity in kube.objects for identity in gis.IDENTITIES))
        self.assertIn(('job',inputs['jobs']['verify']),kube.objects)
        self.assertIn(('configmap',inputs['configmaps']['verify']),kube.objects)
        self.assertEqual(state['gis_first_install_rollback']['phase'],'ROLLED_BACK')
        self.assertTrue(state['gis_first_install_rollback']['remaining_database_prerequisite'])

    def test_root_authorize_apply_finish_and_failure_rollback_callbacks(self):
        import transaction
        import apply

        class FullKube(FakeKube):
            def run(self, args, body=None, timeout=30):
                if args[:2] == ['get', 'services,deployments']:
                    rows=[row for (kind, _), row in self.objects.items()
                          if kind in ('Service','Deployment')]
                    return {'apiVersion':'v1','kind':'List','metadata':{},'items':rows}
                return super().run(args,body,timeout)
            def create(self, obj):
                key=(obj['kind'],obj['metadata']['name'])
                self.next_uid+=1
                created=copy.deepcopy(obj)
                created['metadata'].update(uid=f'00000000-0000-0000-0000-{self.next_uid:012d}',resourceVersion='1')
                self.objects[key]=created
                self.events.append(('gis-create',key[0],key[1]))
                return copy.deepcopy(created)
            def start(self, live, *, replicas, expected_uid, expected_resource_version):
                row=self.objects[(live['kind'],live['metadata']['name'])]
                if row['metadata']['uid']!=expected_uid or row['metadata']['resourceVersion']!=expected_resource_version:
                    raise AssertionError('start precondition changed')
                row['spec']['replicas']=replicas
                row['metadata']['resourceVersion']=str(int(expected_resource_version)+1)
                self.events.append(('gis-start',row['metadata']['name']))
                return copy.deepcopy(row)
            def delete(self,kind,name,*,uid,resource_version):
                row=self.objects[(kind,name)]
                if row['metadata']['uid']!=uid or row['metadata']['resourceVersion']!=resource_version:
                    raise AssertionError('delete precondition changed')
                del self.objects[(kind,name)]
                self.events.append(('gis-delete',kind,name))

        source=capture(ROOT)
        inputs=prerequisite.source_inputs(ROOT,source)
        for failure in ('none','authorize-post-create','finish-post-start'):
            with self.subTest(failure=failure), tempfile.TemporaryDirectory() as folder:
                plan=gis_fi_test.plan()
                plan.update(inputs=inputs,inputs_sha256=gis.digest(inputs),
                    source_capture=source,source_root=str(ROOT),phase='PLANNED',created={},jobs={})
                for key in ('prerequisite_receipt','resource_receipt','receipt','database_identity',
                            'secret_metadata','config_metadata'):
                    plan.pop(key,None)
                kube=FullKube()
                protected={name:{'apiVersion':'apps/v1','kind':'Deployment',
                    'metadata':{'name':name,'namespace':gis.NS,'uid':name+'-uid','resourceVersion':'17'},
                    'spec':{'replicas':1,'template':{'spec':{'containers':[{'name':name,'image':'pinned'}]}}}}
                    for name in ('voice-notification','voice-realtime')}
                for name,row in protected.items():
                    kube.objects[('Deployment',name)]=row
                protected_before=copy.deepcopy(protected)
                prerequisite.preflight(kube,plan)
                snapshots={'voice-'+str(i):{} for i in range(22)}
                target={'tag':plan['source_sha'],'mode':'app-only',
                    'migration_sha256':transaction.canonical([]),
                    'gis_first_install_sha256':plan['source_plan_sha256']}
                state={'operation':plan['operation'],'phase':'AWAITING_OFF_NODE','status':'WAITING',
                    'events':[],'context':{'snapshots':snapshots},'provenance':{},
                    'target':target,'migrations':[],'migration_secret_metadata':{},
                    'gis_first_install':plan,'custody':{'verified':True},
                    'cut':{'manifest':{'archive_sha256':'6'*64},'manifest_sha256':'7'*64,
                        'census_sha256':'8'*64}}
                class Stage:
                    expected={'namespace_uid':'00000000-0000-0000-0000-000000000001',
                        'marker_uid':'00000000-0000-0000-0000-000000000011',
                        'source_claim_uid':'00000000-0000-0000-0000-000000000012',
                        'source_pv_uid':'00000000-0000-0000-0000-000000000013'}
                    final_path=Path('/selected-store')
                    marker={'metadata':{'uid':'marker','resourceVersion':'7'},
                        'data':{'generation':'g','previousGeneration':'p','dataPVC':'claim'}}
                    final_claim={'metadata':{'name':'claim','uid':'claim-uid'},'spec':{'volumeName':'pv'}}
                    final_pv={'metadata':{'uid':'pv-uid'},'spec':{'local':{}}}
                    def verify_final_storage(self): pass
                    def prepared(self): pass
                    def adopt_applied_target(self,*_args): pass
                    def verified(self): pass
                    def restart(self): return {'standard_apply_restart':True}
                    def refence(self): return {'verified':True}
                stage=Stage()
                journal=[]
                after_migrate=(lambda *_args: (_ for _ in ()).throw(RuntimeError('after_create_failure'))
                    if failure=='authorize-post-create' else None)
                patches=[
                    patch.object(transaction,'reconstruct',return_value=stage),
                    patch.object(transaction,'context',return_value={'snapshots':snapshots}),
                    patch.object(transaction,'save'),
                    patch.object(transaction,'file_sha',return_value='7'*64),
                    patch.object(transaction,'verify_archive',return_value=True),
                    patch.object(transaction,'revalidate_inputs'),
                    patch.object(transaction.space_safeguards,'before_migrate'),
                    patch.object(transaction.space_safeguards,'after_migrate',side_effect=after_migrate),
                    patch.object(transaction.migrations,'execute',return_value=[]),
                    patch.object(transaction,'DockerRuntime'),
                    patch.object(transaction,'verify_post_apply',return_value={'verified':True}),
                    patch.object(transaction,'attach_closed_preservation'),
                    patch.object(gis,'verify_ready',side_effect=lambda _k,p:{
                        'schema':'voice-gis-first-install-ready-v1',
                        'namespace':gis.NS,
                        'service_uid':p['receipt']['objects']['Service/voice-gameintegration']['uid'],
                        'service_spec_sha256':'a'*64,
                        'deployment_uid':p['receipt']['objects']['Deployment/voice-gameintegration']['uid'],
                        'deployment_spec_sha256':'b'*64,
                        'endpoints':[{'pod_uid':'00000000-0000-0000-0000-000000000014'}]}),
                ]
                with patches[0], patches[1], patches[2], patches[3], patches[4], patches[5], \
                     patches[6], patches[7], patches[8], patches[9], patches[10], patches[11], patches[12]:
                    if failure=='authorize-post-create':
                        with self.assertRaisesRegex(RuntimeError,'after_create_failure'):
                            transaction.authorize(kube,Path(folder),state,'6'*64,'7'*64,{})
                        self.assertEqual(state['phase'],'GIS_FIRST_INSTALL_ROLLED_BACK')
                        self.assertFalse(any(identity in kube.objects for identity in gis.IDENTITIES))
                        self.assertEqual({name:kube.objects[('Deployment',name)] for name in protected},protected_before)
                        self.assertTrue(any(key[0]=='job' for key in kube.objects))
                        continue

                    authorization=transaction.authorize(kube,Path(folder),state,'6'*64,'7'*64,{})
                    self.assertEqual(state['phase'],'PAUSED_APPLY')
                    self.assertEqual([e[0] for e in kube.events if e[0] in ('created-job','gis-create')],
                        ['created-job']*3+['gis-create','gis-create'])
                    self.assertEqual(apply.paused_documents([],{'operation':state['operation'],
                        'target':{'gis_first_install_sha256':plan['source_plan_sha256']}}),[])
                    self.assertEqual(kube.objects[gis.DEPLOYMENT]['spec']['replicas'],0)

                    publication=None
                    if failure=='finish-post-start':
                        def publication(_state,*,authority_guard):
                            authority_guard()
                            raise RuntimeError('post_start_failure')
                    if failure=='finish-post-start':
                        with self.assertRaisesRegex(RuntimeError,'post_start_failure'):
                            transaction.finish(kube,Path(folder),state,authorization,'claim-rv',{},
                                publication=publication)
                        self.assertEqual(state['phase'],'GIS_FIRST_INSTALL_ROLLED_BACK')
                        self.assertFalse(any(identity in kube.objects for identity in gis.IDENTITIES))
                        self.assertEqual({name:kube.objects[('Deployment',name)] for name in protected},protected_before)
                        self.assertTrue(any(event[0]=='gis-start' for event in kube.events))
                    else:
                        finished=transaction.finish(kube,Path(folder),state,authorization,'claim-rv',{})
                        self.assertEqual(finished['status'],'PASS')
                        self.assertEqual(kube.objects[gis.DEPLOYMENT]['spec']['replicas'],1)
                        self.assertEqual({name:kube.objects[('Deployment',name)] for name in protected},protected_before)
                        self.assertIn('gis_first_install_transition_receipt',finished)

    def test_transition_receipt_requires_ready_owned_endpoint_and_binds_22_to_23(self):
        kube = ReadyKube()
        current = gis_fi_test.plan()
        gis.execute(kube, current, lambda _row: None)
        ready = gis.verify_ready(kube, current, timeout=1, sleep=lambda _delay: None)
        receipt = gis.transition_receipt(current, ready, source_ci=current['source_ci'],
            custody=current['custody'], target_mode='app-only')
        state = {'operation': current['operation'], 'target': {'mode': 'app-only'},
            'gis_first_install': current}
        self.assertTrue(gis.validate_transition_receipt(receipt, state))
        self.assertEqual(receipt['workload_transition'], {'from': 22, 'to': 23})
        bad = copy.deepcopy(receipt)
        bad['workload_transition'] = {'from': 21, 'to': 22}
        with self.assertRaisesRegex(gis.Blocked, 'transition_receipt_binding_invalid'):
            gis.validate_transition_receipt(bad, state)

    def test_next_ordinary_operation_consumes_only_live_first_install_receipt(self):
        kube = ReadyKube()
        current = gis_fi_test.plan()
        gis.execute(kube, current, lambda _row: None)
        ready = gis.verify_ready(kube, current, timeout=1, sleep=lambda _delay: None)
        receipt = gis.transition_receipt(current, ready, source_ci=current['source_ci'],
            custody=current['custody'], target_mode='app-only')
        source_state = {'operation': current['operation'], 'status': 'PASS',
            'target': {'mode': 'app-only'}, 'gis_first_install': current,
            'gis_first_install_transition_receipt': receipt}
        binding = gis.validate_next_operation(receipt, source_state, kube)
        binding['receipt'] = copy.deepcopy(receipt)
        self.assertTrue(gis.validate_transition_binding(binding, kube))
        kube.objects[gis.SERVICE]['metadata']['uid'] = 'replacement'
        with self.assertRaisesRegex(gis.Blocked, 'receipt_object_changed'):
            gis.validate_transition_binding(binding, kube)

    def test_next_operation_rejects_unfinished_or_wrong_namespace_receipt(self):
        kube = ReadyKube()
        current = gis_fi_test.plan()
        gis.execute(kube, current, lambda _row: None)
        ready = gis.verify_ready(kube, current, timeout=1, sleep=lambda _delay: None)
        receipt = gis.transition_receipt(current, ready, source_ci=current['source_ci'],
            custody=current['custody'], target_mode='app-only')
        source_state = {'operation': current['operation'], 'status': 'BLOCKED',
            'target': {'mode': 'app-only'}, 'gis_first_install': current,
            'gis_first_install_transition_receipt': receipt}
        with self.assertRaisesRegex(gis.Blocked, 'transition_source_invalid'):
            gis.validate_next_operation(receipt, source_state, kube)
        source_state['status'] = 'PASS'
        kube.objects[('Namespace', gis.NS)] = {'metadata': {'uid': 'replacement'}}
        with self.assertRaisesRegex(gis.Blocked, 'transition_namespace_changed'):
            gis.validate_next_operation(receipt, source_state, kube)

    def test_ready_service_endpoint_must_belong_to_owned_deployment(self):
        kube = ReadyKube()
        current = gis_fi_test.plan()
        gis.execute(kube, current, lambda _row: None)
        kube._ready_objects()
        kube.rs['metadata']['ownerReferences'][0]['uid'] = '00000000-0000-0000-0000-000000000999'
        with self.assertRaisesRegex(gis.Blocked, 'endpoint_pod_invalid'):
            gis.verify_ready(kube, current, timeout=1, sleep=lambda _delay: None)


if __name__ == '__main__':
    unittest.main()

