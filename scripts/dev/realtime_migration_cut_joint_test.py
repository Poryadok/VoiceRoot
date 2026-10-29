"""Actual phase composition with synthetic Kubernetes and PostgreSQL transports."""
import copy
import hashlib
import json
from pathlib import Path
import unittest
from unittest import mock

import frontend_runtime_executor
import full_cohort_affected_plan as affected
import media_backup_phase_test as base_fixture
import media_operation_journal as journals
import media_backup_phase
import cohort_backup_phases
import realtime_db_provision as provision
import realtime_migration_job as migration
import realtime_migration_job_test as migration_fixture
import realtime_secret_phase as secret_phase
import realtime_secret_provision as bridge
import resource_fencing
import writer_cut
import writer_cut_migration
import writer_cut_test


class JointTest(unittest.TestCase):
    def exercise(self, fault):
        base = base_fixture.PhaseTest(); base.setUp(); self.addCleanup(base.doCleanups)
        renderer = migration_fixture.RendererTest(); renderer.setUp(); self.addCleanup(renderer.doCleanups)
        password = b'a' * 32
        objects = {}; events = []; observed = []; pod_phases = []; runtime_ip = ['10.1.2.3']
        lifecycle = fault not in (None, 'lost', 'busy')
        combined = fault in ('combined', 'combined-wrong-uid', 'combined-busy', 'combined-ip-drift')
        tick = [0]
        def advance(seconds):
            tick[0] += 700 if fault == 'timeout' else seconds
        def current_phase():
            if not lifecycle: return 'Succeeded'
            if fault == 'timeout': return 'Scheduling'
            if (fault == 'emergence' or combined) and tick[0] == 0: return 'Absent'
            return ('Scheduling', 'Pulling', 'Running', 'Succeeded')[min(tick[0] // 2, 3)]
        self.enterContext(mock.patch.object(migration.time, 'sleep', side_effect=advance))
        self.enterContext(mock.patch.object(migration.time, 'monotonic', side_effect=lambda: tick[0]))
        class Client(writer_cut_test.Client):
            def call(inner, *args, body=None):
                if args[:2] == ('get', 'nodes'):
                    return {'items': [migration_fixture.PhaseTest.node_object(renderer.node)]}
                if args[0] == 'create':
                    obj = copy.deepcopy(body); kind = obj['kind']; meta = obj['metadata']
                    dry = '--dry-run=server' in args
                    if kind != 'Pod': meta.update(uid='uid-' + kind, resourceVersion='5')
                    if kind == 'Job':
                        uid = meta['uid']; name = meta['name']
                        generated = {'batch.kubernetes.io/controller-uid':uid,'controller-uid':uid,
                                     'batch.kubernetes.io/job-name':name,'job-name':name}
                        meta['labels'] = {'app':name, **generated}
                        obj['spec']['selector']={'matchLabels':{'batch.kubernetes.io/controller-uid':uid}}
                        obj['spec']['template']['metadata']['labels']['batch.kubernetes.io/controller-uid']=uid
                        obj['status']={}
                    if not dry:
                        objects[kind] = copy.deepcopy(obj)
                        if kind == 'Job':
                            events.append('job-post')
                            if fault == 'lost': raise TimeoutError('lost response')
                    return obj
                if args[0] == 'get' and args[1] in ('secret','configmap','job') and len(args)>2:
                    kind={'secret':'Secret','configmap':'ConfigMap','job':'Job'}[args[1]]
                    value=objects.get(kind)
                    if value and value['metadata']['name']==args[2]:
                        value = copy.deepcopy(value)
                        if kind == 'Job':
                            phase = current_phase()
                            if phase == 'Succeeded': value['status']={'conditions':[{'type':'Complete','status':'True'}]}
                            if fault == 'failed-job': value['status']={'conditions':[{'type':'Failed','status':'True'}]}
                        return value
                    if kind != 'ConfigMap' or args[2] != 'voice-dev-image-update-lock': return {}
                if args[:2] == ('get','jobs'):
                    return {'items':[inner.call('get','job',objects['Job']['metadata']['name'])] if 'Job' in objects else []}
                if args[:2] == ('get','pods') and 'Job' in objects:
                    job=objects['Job']; template=job['spec']['template']; spec=copy.deepcopy(template['spec'])
                    spec['nodeName']=renderer.node['name']
                    pod = {'apiVersion':'v1','kind':'Pod','metadata':{'name':'job-pod','namespace':'voice-dev',
                        'uid':'pod-uid','resourceVersion':'2','labels':copy.deepcopy(template['metadata']['labels']),
                        'ownerReferences':[{'kind':'Job','uid':job['metadata']['uid'],'controller':True}]},
                        'spec':spec,'status':{'phase':'Succeeded','podIP':'10.1.2.3','containerStatuses':[{
                            'name':'migrate','containerID':'containerd://fixture','restartCount':0,
                            'imageID':'containerd://'+migration.PLATFORMS['amd64'],
                            'state':{'terminated':{'exitCode':0}}}]}}
                    phase = current_phase(); pod_phases.append(phase)
                    if phase == 'Absent': return {'items':[]}
                    pod['status']['podIP'] = runtime_ip[0]
                    if phase in ('Scheduling', 'Pulling'):
                        pod['status']={'phase':'Pending'}
                        if phase == 'Scheduling': pod['spec'].pop('nodeName')
                        else:
                            pod['status'].update(podIP='10.1.2.3', containerStatuses=[{
                                'name':'migrate','restartCount':0,'imageID':'','containerID':'',
                                'state':{'waiting':{'reason':'ContainerCreating'}}}])
                    elif phase == 'Running':
                        pod['status']['phase']='Running'
                        pod['status']['containerStatuses'][0]['state']={'running':{'startedAt':'fixture'}}
                    if fault in ('wrong-uid','mid-wrong-uid') and phase != 'Scheduling': pod['metadata']['uid']='foreign'
                    if fault == 'combined-wrong-uid' and phase == 'Running': pod['metadata']['uid']='foreign'
                    if fault == 'wrong-node': pod['spec']['nodeName']='foreign'
                    if fault == 'wrong-image' and phase == 'Pulling':
                        pod['status']['containerStatuses'][0]['imageID']='containerd://sha256:'+'f'*64
                    if fault == 'pending-execution' and phase == 'Pulling':
                        pod['status']['containerStatuses'][0].update(containerID='containerd://unexpected',
                            state={'running':{}})
                    if fault == 'failed-pod': pod['status']['phase']='Failed'
                    return {'items':[pod]}
                return super().call(*args)
        backing=base.ledger
        class LedgerView:
            def __init__(inner, client): inner.client=client
            def read(inner): return backing.read()
            def write(inner, revision, row): return backing.write(revision,row)
        def facade(journal):
            client=Client(base.lock,journal)
            return client,resource_fencing.Fence(base.context,lambda:copy.deepcopy(base.card),LedgerView(client))
        def journal(name, extra):
            value=journals.Journal(Path(base.temp.name)/name,{'resource_context':base.context,'namespace_uid':'ns',**extra})
            self.addCleanup(value.close); return value
        self.enterContext(mock.patch.object(frontend_runtime_executor,'KubectlTransport',Client))
        self.enterContext(mock.patch.object(resource_fencing,'KubeLedger',LedgerView))
        secret_j=journal('secret',{}); secret_client,secret_fence=facade(secret_j)
        secret=secret_phase.SecretPhase(secret_j,secret_fence,secret_client,base.lock,'ns')
        credential=secret.run(password)
        engine={'namespace_uid':'ns','pod_uid':'p','pvc_uid':'pvc','pv_uid':'pv',
                'container_id':'containerd://'+'c'*64,'restart_count':1}
        scope={'engine':engine,'server_start':'start','databases':['auth_db'],'database_oids':{'auth_db':10}}
        baseline={'engine':engine,'server_start':'start','all_databases':{'auth_db':10},'databases':{'auth_db':10},
                  'excluded':{},'properties':{'auth_db':{'owner':1,'allowconn':True}},
                  'operation_id':base.context['operation_id'],'source_sha':base.context['commit'],
                  'namespace_uid':'ns','scope':'all-databases-connectable-backup'}
        class Catalog:
            def verify_receipt(inner,token): return copy.deepcopy(baseline)
            def verify_current(inner,token):
                if pg.state['database'] is not None: raise ValueError('original catalog changed')
                return copy.deepcopy(baseline)
            def _guard(inner): pass
        self.enterContext(mock.patch.object(affected,'FullCatalogPhase',Catalog))
        catalog_phase=Catalog(); catalog_j=journal('catalog',{}); catalog_client,catalog_fence=facade(catalog_j)
        catalog_phase.journal,catalog_phase.client,catalog_phase.fence=catalog_j,catalog_client,catalog_fence
        catalog_phase.lock,catalog_phase.namespace_uid=base.lock,'ns'
        probe=affected.CatalogScopedProbe(catalog_phase,'token')
        source=writer_cut_test.workload('Deployment','voice-a','uid-a',1,'old')
        admission={'controllers':[source],'guards':{'jobs':[],'cronjobs':[],'hpas':[]},
                   'linked':{'jobs':[],'cronjobs':[]},'session_scope':scope}
        cut_j=journal('cut',{'writer_cut':admission}); cut_client,cut_fence=facade(cut_j)
        cut_client.workloads={('Deployment','voice-a'):copy.deepcopy(source)}
        cut=writer_cut.WriterCut(cut_j,cut_fence,cut_client,base.lock,'ns',admission,probe)
        self.enterContext(mock.patch.object(cut,'_patch',side_effect=cut_client.patch))
        class PG:
            instance=None
            def __new__(cls,*args):
                if cls.instance is None: cls.instance=super().__new__(cls)
                return cls.instance
            def __init__(inner,*args):
                if not hasattr(inner,'state'): inner.state={'role':None,'database':None}
            def snapshot(inner): return copy.deepcopy(engine)
            def catalog(inner): return copy.deepcopy(inner.state)
            def execute(inner,sql):
                if sql==provision.database_sql(): inner.state['database']={'oid':13,'owner_oid':12}
                else: inner.state['role']={'oid':12,'login':True,'superuser':False,'createdb':False,
                    'createrole':False,'inherit':False,'replication':False,'bypassrls':False,'memberships':0,
                    'connection_limit':-1,'valid_until':None,'config':None}
        pg=PG()
        self.enterContext(mock.patch.object(provision,'KubePostgres',PG))
        foreign=[False]
        def catalog(*args):
            db={'auth_db':10}
            if pg.state['database']: db['realtime_db']=13
            if foreign[0]: db['foreign_db']=99
            return {'all_databases':db,'databases':copy.deepcopy(db),'excluded':{},
                    'properties':{'auth_db':{'owner':1,'allowconn':True}}}
        self.enterContext(mock.patch.object(affected,'_full_catalog',side_effect=catalog))
        self.enterContext(mock.patch.object(affected.pg,'inspect',side_effect=lambda scope,guard:{
            'scope':copy.deepcopy(scope),'active_sessions':0,'active_transactions':0}))
        cut.drain()
        catalog_hash=hashlib.sha256(json.dumps(baseline,sort_keys=True,separators=(',',':'),ensure_ascii=True,allow_nan=False).encode('ascii')).hexdigest()
        binding={'operation_id':base.context['operation_id'],'source_sha':base.context['commit'],
                 'snapshot_id':'capture-'+'b'*32,'databases':['auth_db'],
                 'sources':{'roles':{'engine':engine,'server_start':'start'},
                            'auth_db':{'engine':engine,'server_start':'start','database_oid':10}},
                 'full_catalog':{'receipt_sha256':catalog_hash,'properties':baseline['properties']}}
        capture={'snapshot_id':binding['snapshot_id'],**{k:engine[k] for k in
                 ('pod_uid','pvc_uid','pv_uid','container_id','restart_count')}}
        backup_j=journal('backup',{'cohort_binding':binding,'backup_capture':capture})
        backup_client,backup_fence=facade(backup_j)
        backup=media_backup_phase.BackupPhases(backup_j,backup_fence,backup_client,base.lock,'ns')
        cohort=cohort_backup_phases.CohortPhases(backup,catalog_phase,'token')
        cohort.bind_catalog_cut(cut)
        provision_j=journal('provision',{'postgres':engine,'realtime_credential':credential})
        provision_client,provision_fence=facade(provision_j)
        worker,receipt=bridge.run_owned(secret,provision_j,provision_client,provision_fence,base.lock,'ns',password)
        self.assertEqual(receipt['database_oid'],13)
        cut.bind_provision_scope(worker)
        cohort._guard()
        public={k:credential[k] for k in ('name','uid','resourceVersion')}
        spec={'source_sha':base.context['commit'],'image':renderer.migrate,'platform_digest':migration.PLATFORMS['amd64'],
              'engine':engine,'database_oid':13,'credential':public,'node':renderer.node}
        migration_j=journal('migration',{'realtime_migration':spec}); migration_client,migration_fence=facade(migration_j)
        phase=migration.Migration(secret,provision_j,migration_j,renderer.root,password,migration_client,migration_fence)
        files=renderer.files | {'src/backend/realtime/internal/lifecycle/store.go':
               '1e34ba05a4831031409bb90e37b541b8adfbe233cb307618cbb6ba471fb5fdc9'}
        self.enterContext(mock.patch.object(migration.publish_candidate,'source_snapshot',return_value={'source_sha':base.context['commit'],'files':files}))
        self.enterContext(mock.patch.object(migration.realtime_migration_verify,'check',return_value={
            'database_oid':13,'version':1,'dirty':False,'scope':'realtime-login-schema-readiness'}))
        in_guards = [False]
        real_guards = cut._guards
        def guards():
            in_guards[0] = True
            try: return real_guards()
            finally: in_guards[0] = False
        self.enterContext(mock.patch.object(cut, '_guards', side_effect=guards))
        paired_identities = []
        def sessions(scope,identity,guard):
            guard()
            value={'scope':copy.deepcopy(scope),'active_sessions':0,'active_transactions':0,'attributed_sessions':[]}
            state=base.ledger.row.get('realtime_migration',{}).get('steps',{}).get('job',{}).get('status')
            observed.append(state)
            if state=='pending': cohort._guard()
            if lifecycle and current_phase() in ('Scheduling','Pulling') and state == 'owned':
                self.assertIsNone(identity['pod_ip'])
                self.assertIsNone(identity['pod_uid'])
            self.assertEqual(scope['database_oids']['realtime_db'],13)
            if fault=='busy' and state=='pending': value['active_sessions']=1
            if combined and not in_guards[0] and state == 'owned' and cut.migration.get('job_uid') and tick[0] < 4:
                tick[0] = 1 if tick[0] == 0 else 4
                paired_identities.append(copy.deepcopy(identity))
            elif fault in ('mid-observation','mid-wrong-uid','mid-busy','mid-ip-drift','emergence') and state == 'owned' and tick[0] == 0 and cut.migration.get('job_uid'):
                tick[0] = 4
            elif fault in ('mid-busy', 'combined-busy') and tick[0] == 4: value['active_sessions'] = 1
            elif fault in ('mid-ip-drift', 'combined-ip-drift') and tick[0] == 4: runtime_ip[0] = '10.1.2.' + str(int(runtime_ip[0].rsplit('.',1)[1]) + 1)
            if combined and not in_guards[0] and tick[0] == 4 and identity['pod_ip']:
                paired_identities.append(copy.deepcopy(identity))
            return value
        self.enterContext(mock.patch.object(writer_cut_migration.writer_cut_pg_probe,'inspect_migration',side_effect=sessions))
        if fault and fault not in ('progression','mid-observation','emergence','combined'):
            with self.assertRaises(migration.RecoveryRequired): phase.run(cut)
            self.assertEqual(base.ledger.row['status'],'uncertain')
            self.assertIn('pending',observed)
            if lifecycle:
                if fault not in ('wrong-node','failed-pod'): self.assertIn('owned', observed)
                if fault == 'timeout': self.assertGreaterEqual(tick[0], 650)
                if fault in ('wrong-uid','wrong-image','pending-execution'): self.assertIn('Pulling', pod_phases)
                self.assertEqual(migration_j.state['steps']['job']['status'], 'owned')
            self.assertEqual(len(events),0 if fault=='busy' else 1)
            with self.assertRaises(migration.RecoveryRequired): phase.run(cut)
            self.assertEqual(len(events),0 if fault=='busy' else 1)
        else:
            phase.run(cut)
            self.assertEqual(events,['job-post'])
            if combined:
                self.assertEqual([i['pod_ip'] for i in paired_identities[:3]], [None, None, '10.1.2.3'])
                self.assertEqual(cut.migration['pod_uid'], 'pod-uid')
            self.assertTrue({'pending','owned','done'}.issubset(observed))
            self.assertEqual(migration_j.state['steps']['job']['status'],'done')
            self.assertEqual(base.ledger.row['realtime_migration']['status'],'done')
            self.assertTrue(cut.migration['finished'])
            if lifecycle:
                expected = {'Absent','Running','Succeeded'} if fault == 'emergence' else {'Absent','Scheduling','Running','Succeeded'} if combined else {'Scheduling','Running','Succeeded'} if fault == 'mid-observation' else {'Scheduling','Pulling','Running','Succeeded'}
                self.assertTrue(expected.issubset(pod_phases))
            step='writer-cut-migration-finish-'+phase.invocation
            self.assertEqual(cut_j.state['steps'][step]['status'],'done')
            self.assertEqual(base.ledger.row['writer_cut_effects'][step]['status'],'done')
            cohort._guard()
            cut.bind_realtime_scope()
            cohort._guard()
            old_scope=copy.deepcopy(cut.provision_successor['scope'])
            cut.provision_successor['scope']['database_oids']['realtime_db']=99
            with self.assertRaises(cohort_backup_phases.CohortPhaseError): cohort._guard()
            cut.provision_successor['scope']=old_scope
            cohort._guard()
            foreign[0]=True
            with self.assertRaises(cohort_backup_phases.CohortPhaseError): cohort._guard()

    def test_combined_startup_edges_during_session_samples(self): self.exercise('combined')
    def test_pod_appears_during_session_sample(self): self.exercise('emergence')
    def test_pending_to_running_during_session_sample(self): self.exercise('mid-observation')
    def test_pending_progression(self): self.exercise('progression')
    def test_pending_identity_failure_and_timeout(self):
        for fault in ('wrong-uid','wrong-node','wrong-image','pending-execution','failed-job','failed-pod','timeout','mid-wrong-uid','mid-busy','mid-ip-drift','combined-wrong-uid','combined-busy','combined-ip-drift'):
            with self.subTest(fault=fault): self.exercise(fault)

    def test_success(self): self.exercise(None)
    def test_lost_reply(self): self.exercise('lost')
    def test_pending_busy(self): self.exercise('busy')


if __name__=='__main__': unittest.main()
