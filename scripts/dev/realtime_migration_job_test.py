import copy
import hashlib
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch, Mock
import realtime_migration_job as job
import realtime_secret_phase_test as secret_fixture
import realtime_db_provision_journal_test as pg_fixture
import realtime_secret_provision as bridge
import media_operation_journal as journal


class RendererTest(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name)
        folder = self.root/'src/backend/migrations/realtime_db'; folder.mkdir(parents=True)
        self.files = {}
        for suffix in ('up','down'):
            p = folder/('000001_admission.'+suffix+'.sql')
            p.write_bytes(('fixture '+suffix).encode())
            self.files[p.relative_to(self.root).as_posix()] = hashlib.sha256(p.read_bytes()).hexdigest()
        self.source = 'a'*40
        self.operation = 'media-c1-'+'b'*32
        self.secret = {'name':'voice-realtime-db-'+'b'*32,'uid':'secret','resourceVersion':'3'}
        self.migrate = job.MIGRATE_IMAGE

    def test_exact_source_immutable_plan(self):
        with patch.object(job.publish_candidate,'source_snapshot',return_value={'source_sha':self.source,'files':self.files}):
            config, planned = job.render(self.root,self.source,self.operation,self.secret,self.migrate)
        self.assertTrue(config['immutable'])
        self.assertEqual(set(config['data']),{'000001_admission.up.sql','000001_admission.down.sql'})
        spec = planned['spec']
        self.assertEqual(spec['backoffLimit'],0)
        self.assertEqual(spec['template']['spec']['restartPolicy'],'Never')
        container = spec['template']['spec']['containers'][0]
        self.assertEqual(container['image'],self.migrate)
        self.assertEqual(container['env'][0]['valueFrom']['secretKeyRef'],{'name':self.secret['name'],'key':'DATABASE_URL'})

    def test_reject_changed_bytes_and_wrong_binding(self):
        for mutation in ('source','secret','image','repository','extra'):
            secret = copy.deepcopy(self.secret); image = self.migrate; files = dict(self.files)
            if mutation=='source': files[next(iter(files))] = 'f'*64
            if mutation=='secret': secret['name']='voice-realtime-db-'+'d'*32
            if mutation=='image': image='example/migrate:latest'
            if mutation=='repository': image='example/migrate@sha256:'+'c'*64
            if mutation=='extra': files['src/backend/migrations/realtime_db/000002_extra.up.sql']='f'*64
            with patch.object(job.publish_candidate,'source_snapshot',return_value={'source_sha':self.source,'files':files}):
                with self.assertRaises(job.RecoveryRequired):
                    job.render(self.root,self.source,self.operation,secret,image)

    def test_only_uid_bound_generated_metadata_labels_are_allowed(self):
        with patch.object(job.publish_candidate,'source_snapshot',return_value={'source_sha':self.source,'files':self.files}):
            _, intended = job.render(self.root,self.source,self.operation,self.secret,self.migrate)
        actual = copy.deepcopy(intended); meta = actual['metadata']; meta.update(uid='uid',resourceVersion='1')
        name = meta['name']; meta['labels']={'app':name,'batch.kubernetes.io/controller-uid':'uid',
            'controller-uid':'uid','batch.kubernetes.io/job-name':name,'job-name':name}
        actual['spec']['selector']={'matchLabels':{'batch.kubernetes.io/controller-uid':'uid'}}
        actual['spec']['template']['metadata']['labels']['batch.kubernetes.io/controller-uid']='uid'
        job.admitted_job(intended,actual)
        for mutation in ('foreign-uid','extra'):
            bad = copy.deepcopy(actual)
            if mutation=='foreign-uid': bad['metadata']['labels']['controller-uid']='foreign'
            else: bad['metadata']['labels']['extra']='untrusted'
            with self.assertRaises(job.RecoveryRequired): job.admitted_job(intended,bad)


class PhaseTest(unittest.TestCase):
    def setup_operation(self):
        setup = secret_fixture.SecretPhaseTest()
        base, client, secret = setup.setup_phase(); self.addCleanup(base.doCleanups)
        password = b'a'*32; receipt = secret.run(password)
        fixture = pg_fixture.JournalIntegration(); fixture.setUp(); self.addCleanup(fixture.doCleanups)
        provision = journal.Journal(Path(base.temp.name)/'provision', {'resource_context':base.context,
            'namespace_uid':'ns','postgres':fixture.pg.snapshot(),'realtime_credential':receipt})
        self.addCleanup(provision.close)
        with patch.object(bridge.provision,'KubePostgres',return_value=fixture.pg): bridge.run(secret,provision,password)
        renderer = RendererTest(); renderer.setUp(); self.addCleanup(renderer.doCleanups)
        source = base.context['commit']; operation = base.context['operation_id']
        public = {k:receipt[k] for k in ('name','uid','resourceVersion')}
        spec = {'source_sha':source,'image':renderer.migrate,'platform_digest':'sha256:'+'c'*64,
                'engine':fixture.pg.snapshot(),'database_oid':13,'credential':public}
        operation_journal = journal.Journal(Path(base.temp.name)/'migration',
            {'resource_context':base.context,'namespace_uid':'ns','realtime_migration':spec})
        self.addCleanup(operation_journal.close)
        files = renderer.files | {'src/backend/realtime/internal/lifecycle/store.go':
                  '1e34ba05a4831031409bb90e37b541b8adfbe233cb307618cbb6ba471fb5fdc9'}
        return base,client,secret,provision,fixture.pg,renderer,operation_journal,files,password

    def test_lost_config_create_marks_uncertain_and_never_replays(self):
        base,client,secret,provision,pg,renderer,j,files,password = self.setup_operation()
        original = client.call
        def lost(*args,body=None):
            if args[:2]==('create','-f'): raise TimeoutError('lost')
            return original(*args,body=body)
        client.call = lost
        worker = job.Migration(secret,provision,j,renderer.root,password)
        with patch.object(job.publish_candidate,'source_snapshot',return_value={'source_sha':base.context['commit'],'files':files}), \
             patch.object(job.realtime_db_provision,'KubePostgres',return_value=pg):
            with self.assertRaises(job.RecoveryRequired): worker.run()
            with self.assertRaises(job.RecoveryRequired): worker.run()
        self.assertEqual(base.ledger.row['status'],'uncertain')
        self.assertEqual(j.state['steps']['config']['status'],'pending')

    def test_terminal_job_and_schema_bind_to_owned_config_and_pod(self):
        self.lifecycle(None)

    def test_step_drift_never_submits_or_returns_success(self):
        for fault in ('config','job','terminal','config-drift','local-role','lost-job','pod-image','job-label','dryrun','invocation-ack'):
            with self.subTest(fault=fault): self.lifecycle(fault)

    def lifecycle(self, fault):
        base,client,secret,provision,pg,renderer,j,files,password = self.setup_operation()
        original = client.call
        objects = {}
        def api(*args,body=None):
            if args[:2]==('create','-f') or args[:2]==('create','--dry-run=server'):
                obj = copy.deepcopy(body)
                meta = obj['metadata']; meta.update(uid='uid-'+obj['kind'],resourceVersion='5')
                if obj['kind']=='Job':
                    meta['labels']={'app':meta['name'],'batch.kubernetes.io/controller-uid':meta['uid'],
                                    'controller-uid':meta['uid'],'batch.kubernetes.io/job-name':meta['name'],
                                    'job-name':meta['name']}
                    spec = obj['spec']; spec['selector']={'matchLabels':{'batch.kubernetes.io/controller-uid':meta['uid']}}
                    spec['template']['metadata']['labels']['batch.kubernetes.io/controller-uid']=meta['uid']
                    obj['status']={'conditions':[{'type':'Complete','status':'True'}]}
                    if fault=='dryrun' and args[:2]==('create','--dry-run=server'):
                        spec['backoffLimit']=1
                if args[:2]!=('create','--dry-run=server'):
                    objects[obj['kind']] = copy.deepcopy(obj)
                    if fault=='lost-job' and obj['kind']=='Job': raise TimeoutError('lost response')
                return obj
            if args[:2]==('get','configmap') and args[2].startswith('voice-realtime-migrations-'):
                result = copy.deepcopy(objects['ConfigMap'])
                if fault=='config-drift': result['metadata']['uid']='foreign'
                return result
            if args[:2]==('get','job'):
                result = copy.deepcopy(objects['Job'])
                if fault=='job-label': result['metadata']['labels']['controller-uid']='foreign'
                return result
            if args[:2]==('get','pods'):
                owned = objects['Job']; pod_spec = copy.deepcopy(owned['spec']['template']['spec'])
                pod_spec['nodeName']='fixture-node'
                pod = {'metadata':{'uid':'pod','namespace':'voice-dev','ownerReferences':[{'controller':True,'kind':'Job',
                        'uid':owned['metadata']['uid']}]},'spec':pod_spec,'status':{'phase':'Succeeded',
                        'containerStatuses':[{'name':'migrate','containerID':'containerd://fixture',
                        'restartCount':0,
                        'imageID':'containerd://sha256:'+'c'*64,'state':{'terminated':{'exitCode':0}}}]}}
                if fault=='pod-image': pod['status']['containerStatuses'][0]['imageID']='containerd://sha256:'+'f'*64
                return {'items':[pod]}
            return original(*args,body=body)
        client.call = api
        worker = job.Migration(secret,provision,j,renderer.root,password)
        original_reserve, original_finish = worker._reserve, worker._finish
        original_write = base.ledger.write
        lost_ack = []
        def write(revision, row):
            original_write(revision,row)
            if fault=='invocation-ack' and row.get('realtime_migration',{}).get('status')=='done' and not lost_ack:
                lost_ack.append(True)
                raise TimeoutError('acknowledgement lost')
        def reserve(name, details):
            record = original_reserve(name,details)
            if fault==name: base.ledger.row['realtime_migration']['steps'][name]['nonce']='foreign'
            return record
        def finish(name, record, receipt):
            original_finish(name,record,receipt)
            if fault=='local-role' and name=='config':
                provision.state['steps']['realtime-db-role']['nonce']='foreign'
            if fault=='terminal' and name=='job':
                base.ledger.row['realtime_migration']['steps'][name]['receipt_sha256']='f'*64
        schema = {'database_oid':13,'version':1,'dirty':False,'scope':'realtime-login-schema-readiness'}
        with patch.object(job.publish_candidate,'source_snapshot',return_value={'source_sha':base.context['commit'],'files':files}), \
             patch.object(job.realtime_db_provision,'KubePostgres',return_value=pg), \
             patch.object(job.realtime_migration_verify,'check',return_value=schema), \
             patch.object(worker,'_reserve',side_effect=reserve), patch.object(worker,'_finish',side_effect=finish), \
             patch.object(base.ledger,'write',side_effect=write):
            if fault:
                with self.assertRaises(job.RecoveryRequired): worker.run()
                self.assertEqual(base.ledger.row['status'],'uncertain')
                if fault=='config': self.assertNotIn('ConfigMap',objects)
                if fault=='job': self.assertNotIn('Job',objects)
                if fault in ('config-drift','local-role','dryrun'): self.assertNotIn('Job',objects)
                return
            receipt = worker.run()
        self.assertEqual(receipt['schema'],schema)
        self.assertEqual(j.state['steps']['job']['status'],'done')
        self.assertEqual(j.state['steps']['realtime-migration']['status'],'done')
        self.assertEqual(base.ledger.row['realtime_migration']['status'],'done')
        self.assertEqual(base.ledger.row['realtime_migration']['steps']['job']['status'],'done')


if __name__ == '__main__': unittest.main()
