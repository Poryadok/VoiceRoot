"""Real Journal/Fence state machine; Kubernetes, PG, tool and runtime are synthetic."""
import copy
import json
from pathlib import Path
import sys
import tempfile
import types
import unittest
from unittest import mock

import frontend_runtime_executor as transport
import full_cohort_affected_plan as catalog_module
import full_cohort_pg_physical_identity as physical_module
import media_backup_phase_test as fixtures
import media_operation_journal as journals
import resource_fencing
import space_fresh_install_phase as phase_module
import space_fresh_install_phase_events_test as event_fixture
import space_fresh_install_phase_pg as pg_module
import space_fresh_install_phase_plan as plan_module
import space_fresh_install_phase_job as job_module
import space_native_tool_identity as tools
import update_images
import writer_cut


class PhaseTest(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.context={'project':'voice','resource':'DEV','actor':'scheduled_devops',
            'claim':'11111111-1111-4111-8111-111111111111','window':'22222222-2222-4222-8222-222222222222',
            'revision':4,'generation':3,'operation_id':'media-c1-'+'a'*32,'commit':'a'*40,
            'images':{'voice-space/space':'ghcr.io/x@sha256:'+'b'*64}}
        self.card={'code':'DEV','environment':'dev','owner':'scheduled_devops',
            'claim':self.context['claim'],'window':self.context['window'],'revision':4,'generation':3,
            'state':'held','phase':'deploy','candidate':{'state':'known','commit':'a'*40,'images':self.context['images']}}
        self.lock={'metadata':{'name':update_images.LOCK,'namespace':'voice-dev','uid':'lock','resourceVersion':'8','labels':update_images.LABELS},
            'data':{'operation':'token','resource_operation':self.context['operation_id'],
                    'resource_context':json.dumps(self.context,sort_keys=True,separators=(',',':'))}}
        self.intent={'resource_context':self.context,'namespace_uid':'ns',
                     'space_fresh_install':{'mode':'dev-reset-native51','database':'space_db'}}
        self.journal=journals.Journal(Path(self.temp.name)/'journal',self.intent);self.addCleanup(self.journal.close)
        self.backing=fixtures.Ledger(self.context)
        self.client=transport.KubectlTransport(self.journal)
        self.ledger=resource_fencing.KubeLedger(self.client)
        self.ledger.read=self.backing.read;self.ledger.write=self.backing.write
        self.fence=resource_fencing.Fence(self.context,lambda:copy.deepcopy(self.card),self.ledger)
        self.catalog=object.__new__(catalog_module.FullCatalogPhase)
        self.physical=object.__new__(physical_module.PhysicalPhase)
        self.tool=object.__new__(tools.SpaceNativeToolPhase)
        self.cut=object.__new__(writer_cut.WriterCut)
        for obj in (self.catalog,self.physical,self.tool,self.cut):
            obj.journal=self.journal;obj.fence=self.fence;obj.client=self.client
            obj.lock=copy.deepcopy(self.lock);obj.namespace_uid='ns'
        self.engine={'namespace_uid':'ns','pod_uid':'pg','container_id':'containerd://pg'}
        self.original={'operation_id':self.context['operation_id'],'source_sha':'a'*40,'namespace_uid':'ns',
            'engine':self.engine,'server_start':'start','all_databases':{'postgres':1,'space_db':11,'voice_db':12},
            'databases':{'postgres':1,'space_db':11,'voice_db':12},'excluded':{},
            'properties':{'postgres':{'owner':'voice'},'space_db':{'owner':'voice'},'voice_db':{'owner':'voice'}}}
        self.oid=11;self.drops=[];self.calls=[];self.objects={};self.fail_drop=False;self.fail_create=False
        self.physical_receipt={'physical':{'system_identifier':'123','engine':self.engine,'server_start':'start',
            'database_oid':1,'node':{'name':'node','uid':'node-uid'}}}
        self.tool_receipt={'operation_id':self.context['operation_id'],'namespace_uid':'ns','source_sha':'a'*40,
            'immutable_image':'ghcr.io/test/native@sha256:'+'9'*64,'platform_digest':'sha256:'+'9'*64,
            'build_source_sha256':'b'*64,'executable_sha256':'c'*64,'migrator_sha256':'d'*64,'sql_inventory_sha256':'e'*64,
            'nodes':[{'name':'node','uid':'node-uid'}]}
        self.catalog.verify_current=mock.Mock(return_value=copy.deepcopy(self.original))
        self.catalog.verify_receipt=mock.Mock(return_value=copy.deepcopy(self.original))
        self.catalog.inventory=types.SimpleNamespace(_admin_role=lambda:'voice')
        self.cut.verify_drained=mock.Mock(return_value={})
        self.cut.bind_space_fresh=mock.Mock(return_value={})
        self.cut.verify_space_pending=mock.Mock(return_value={})
        self.cut.verify_space_running=mock.Mock(return_value={})
        self.cut.finish_space_fresh=mock.Mock(return_value={})
        self.client.call=self.call
        self.cm={'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':'sql','namespace':'voice-dev','annotations':{}},
                 'immutable':True,'data':{'000045_a.up.sql':'45','000050_a.up.sql':'50','000051_a.up.sql':'51'}}
        self.job={'apiVersion':'batch/v1','kind':'Job','metadata':{'name':'native','namespace':'voice-dev','annotations':{},'labels':{'app':'space-native'}},
            'spec':{'backoffLimit':0,'template':{'metadata':{'labels':{'app':'space-native'}},'spec':{'restartPolicy':'Never','containers':[{'name':'migrate','image':self.tool_receipt['immutable_image'],
                'env':[{'name':'SPACE_NATIVE_INSTALLER_MODE','value':'provision'}]}]}}}}
        event=event_fixture.CreatedEventTest();event.setUp();self.created=event.raw()
        self.complete=copy.deepcopy(event.receipt)
        base=event.receipt['creation']|{'epoch':'94db7e08-e958-44fa-94b2-988410ab0044'}
        for key,version,seal in (('trusted44',44,'1'),('trusted50',50,'3'),('final',51,'4')):
            self.complete[key]=base|{'version':version,'seal_sha256':seal*64}
        import hashlib
        for key,old,new,sql in (('transition45','1','2','45'),('transition50','2','3','50'),('transition51','3','4','51')):
            self.complete[key]=base|{'old_seal_sha256':old*64,'new_seal_sha256':new*64,
                'migration_sha256':hashlib.sha256(sql.encode()).hexdigest(),'changed':True}
        self.complete['transition51'].update(profile='native_catalog_rowjson_51',migration_engine='github.com/golang-migrate/migrate/v4@v4.18.1',
            migration_sha256='9271fe4135719a204a57e6b0c868f81038d4f320b03ada228f4c5b9fca40da47')
        self.logs=self.created+(json.dumps({'event':'complete','receipt':self.complete})+'\n').encode()
        owner=self
        class PG:
            def __init__(self,guard):self.guard=guard
            def snapshot(self):return copy.deepcopy(owner.engine)
            def space_oid(self):self.guard();return owner.oid
            def drop_exact(self,oid):
                self.guard();owner.drops.append(oid)
                if owner.oid!=oid:raise ValueError('wrong target')
                owner.oid=0
                if owner.fail_drop:raise TimeoutError('secret-in-error')
                self.guard()
        class Runtime:
            def __init__(self,journal,fence,client,lock,namespace_uid,catalog,cut,source_root,token):
                self.journal,self.fence,self.client,self.lock,self.namespace_uid=journal,fence,client,lock,namespace_uid
                self.receipt=None
            def prepare(self):
                self.prepared={'scope':'synthetic-runtime-preflight'};return copy.deepcopy(self.prepared)
            def run(self,phase):
                proof=phase_module.verify_native_completed_before_runtime(phase,'token')
                self.receipt={'new_oid':proof['new_oid'],'role':'space_runtime','scope':'synthetic-runtime'}
                return copy.deepcopy(self.receipt)
        self.runtime_module=types.SimpleNamespace(SpaceRuntimeRolePhase=Runtime,
            verify_completed=lambda p,token:copy.deepcopy(p.receipt),verify_receipt=lambda p,token:copy.deepcopy(p.receipt),
            verify_prepared=lambda p,token:copy.deepcopy(p.prepared))
        self.patches=[mock.patch.dict(sys.modules,{'space_fresh_install_phase_runtime':self.runtime_module}),
            mock.patch.object(pg_module,'KubeSpacePostgres',PG),
            mock.patch.object(physical_module,'verify_receipt',return_value=self.physical_receipt,create=True),
            mock.patch.object(tools,'verify_completed',return_value=self.tool_receipt),
            mock.patch.object(plan_module,'render',side_effect=lambda *a:copy.deepcopy((self.cm,self.job))),
            mock.patch.object(catalog_module,'_full_catalog',side_effect=lambda *a:self.current_catalog())]
        for patcher in self.patches:patcher.start();self.addCleanup(patcher.stop)

    def current_catalog(self):
        result={k:copy.deepcopy(self.original[k]) for k in ('all_databases','databases','excluded','properties')}
        for key in result:
            result[key].pop('space_db',None)
        if self.oid:
            result['all_databases']['space_db']=self.oid;result['databases']['space_db']=self.oid
            result['properties']['space_db']={'owner':'voice'}
        return result

    def call(self,*args,body=None):
        if args[:2]==('get','namespace'):return {'metadata':{'name':'voice-dev','uid':'ns','labels':update_images.LABELS}}
        if args[:3]==('get','configmap',update_images.LOCK):return copy.deepcopy(self.lock)
        if args[0]=='create':
            self.calls.append(('create',body['kind'],body['metadata']['name']))
            obj=copy.deepcopy(body);obj['metadata'].update(uid='uid-'+obj['metadata']['name'],resourceVersion='1')
            if obj['kind']=='Job':
                obj['status']={'succeeded':1,'active':0,'conditions':[{'type':'Complete','status':'True'}]};self.oid=42
                obj['spec']['selector']={'matchLabels':{'batch.kubernetes.io/controller-uid':obj['metadata']['uid']}}
                obj['spec']['template']['metadata']['labels']['batch.kubernetes.io/controller-uid']=obj['metadata']['uid']
            self.objects[(obj['kind'].lower(),obj['metadata']['name'])]=obj
            if self.fail_create and obj['kind']=='Job':raise TimeoutError('secret-in-error')
            return copy.deepcopy(obj)
        if args[:2]==('get','pods'):
            name=args[3].split('=',1)[1]
            if ('job',name) not in self.objects:return {'items':[]}
            job=self.objects[('job',name)]
            return {'items':[{'apiVersion':'v1','kind':'Pod','metadata':{'name':name+'-pod','uid':name+'-pod-uid',
                'namespace':'voice-dev','resourceVersion':'1','labels':copy.deepcopy(job['spec']['template']['metadata']['labels']),
                'ownerReferences':[{'kind':'Job','name':name,'uid':job['metadata']['uid'],'controller':True}]},
                'spec':copy.deepcopy(job['spec']['template']['spec']),
                'status':{'phase':'Succeeded','podIP':'10.0.0.9','containerStatuses':[{'name':'migrate',
                    'containerID':'containerd://fixture','imageID':'containerd://'+self.tool_receipt['platform_digest'],
                    'restartCount':0,'state':{'terminated':{'exitCode':0}}}]}}]}
        if args[0]=='get':return copy.deepcopy(self.objects.get((args[1],args[2]),{}))
        if args[0]=='delete':
            path=args[1].removeprefix('--raw=');kind='job' if '/jobs/' in path else 'configmap';name=path.rsplit('/',1)[1]
            obj=self.objects[(kind,name)]
            if body['preconditions']!={'uid':obj['metadata']['uid'],'resourceVersion':obj['metadata']['resourceVersion']}:raise ValueError()
            self.calls.append(('delete',kind,name));del self.objects[(kind,name)];return {}
        raise AssertionError(args)

    def phase(self):
        phase=phase_module.SpaceFreshInstallPhase(self.journal,self.fence,self.client,self.lock,'ns',
            self.catalog,self.physical,self.cut,self.tool,Path(self.temp.name),'token')
        phase._logs=mock.Mock(side_effect=lambda name:(json.dumps({'event':'verified','receipt':self.complete})+'\n').encode()
                              if 'verify' in name else self.logs)
        phase.prepare()
        return phase

    def test_lost_drop_is_uncertain_and_never_replayed(self):
        self.fail_drop=True;p=self.phase()
        with self.assertRaises(phase_module.SpaceFreshError):p.run()
        self.assertEqual(self.drops,[11]);self.assertEqual(self.calls,[])
        self.assertTrue(p.poisoned);self.assertEqual(self.backing.row['status'],'uncertain')
        with self.assertRaises(phase_module.SpaceFreshError):p.run()
        self.assertEqual(self.drops,[11])

    def test_historical_receipt_does_not_claim_current_catalog(self):
        p=self.phase();completed=p.run()
        with mock.patch.object(p,'_catalog',side_effect=AssertionError('no current PG read')), \
             mock.patch.object(p.pg,'snapshot',side_effect=AssertionError('no PG read')), \
             mock.patch.object(self.runtime_module,'verify_completed',side_effect=AssertionError('historical runtime only')):
            receipt=phase_module.verify_historical(p,'token')
        self.assertEqual(receipt['new_oid'],42)
        self.assertEqual(receipt['catalog'],completed['catalog'])
        self.assertEqual(receipt['original_catalog_sha256'],phase_module._digest(self.original))
        receipt['catalog']['all_databases']['space_db']=999
        self.assertEqual(phase_module.verify_historical(p,'token')['catalog']['all_databases']['space_db'],42)

    def test_historical_final_guard_local_drift_is_rejected(self):
        p=self.phase();p.run();original=p._guard
        def mutate():
            original()
            self.journal.state['steps'].pop(phase_module.STEP,None)
        with mock.patch.object(p,'_stable',wraps=p._stable) as stable:
            original_stable=stable._mock_wraps
            def final(before):
                with mock.patch.object(p,'_guard',side_effect=mutate):original_stable(before)
            stable.side_effect=final
            with self.assertRaises(phase_module.SpaceFreshError):phase_module.verify_historical(p,'token')

    def test_missing_reset_intent_has_no_effect(self):
        del self.journal.intent['space_fresh_install']
        self.journal.state['intent']=copy.deepcopy(self.journal.intent)
        with self.assertRaises(phase_module.SpaceFreshError):self.phase()
        self.assertEqual(self.drops,[]);self.assertEqual(self.calls,[])

    def test_one_shot_success_retains_original_and_checked_cleanup(self):
        p=self.phase();receipt=p.run()
        self.assertEqual(receipt['old_oid'],11);self.assertEqual(receipt['new_oid'],42)
        self.assertEqual(receipt['native_receipt'],self.complete)
        self.assertEqual(receipt['runtime']['role'],'space_runtime')
        self.assertEqual(self.drops,[11])
        self.assertEqual(len([c for c in self.calls if c[:2]==('create','Job')]),2)
        self.assertFalse(any(kind=='job' for kind,name in self.objects))
        self.assertEqual(phase_module.verify_completed(p,'token'),receipt)
        with self.assertRaises(phase_module.SpaceFreshError):p.run()
        self.assertEqual(self.drops,[11])

    def test_lost_job_response_never_adopts_or_deletes_by_name(self):
        self.fail_create=True;p=self.phase()
        with self.assertRaises(phase_module.SpaceFreshError):p.run()
        self.assertEqual(len([c for c in self.calls if c[:2]==('create','Job')]),1)
        self.assertFalse(any(c[0]=='delete' for c in self.calls))
        self.assertEqual(self.backing.row['status'],'uncertain')

    def test_terminal_local_drift_during_last_guard_refuses_receipt(self):
        p=self.phase();p.run();guard=p._guard;count=0
        def count_guard():
            nonlocal count
            count+=1;guard()
        with mock.patch.object(p,'_guard',side_effect=count_guard):phase_module.verify_completed(p,'token')
        final=count;count=0
        def mutate_guard():
            nonlocal count
            count+=1;guard()
            if count==final:self.journal.state['steps'].pop(phase_module.STEP)
        with mock.patch.object(p,'_guard',side_effect=mutate_guard):
            with self.assertRaises(phase_module.SpaceFreshError):phase_module.verify_completed(p,'token')

    def test_running_before_first_created_log_is_bounded_not_replayed(self):
        p=self.phase();reads=0;original=p._logs;call=self.client.call
        def running_call(*args,body=None):
            result=call(*args,body=body)
            if args[0]=='create' and body['kind']=='Job' and not body['metadata']['name'].endswith('-verify'):
                self.objects[('job',body['metadata']['name'])]['status']={'active':1}
                result['status']={'active':1}
            return result
        def logs(name):
            nonlocal reads
            reads+=1
            if reads==1:
                for (kind,_),value in self.objects.items():
                    if kind=='job':value['status']={'active':1}
                return b''
            for (kind,_),value in self.objects.items():
                if kind=='job':value['status']={'succeeded':1,'active':0,'conditions':[{'type':'Complete','status':'True'}]}
            return original(name)
        with mock.patch.object(p,'_logs',side_effect=logs),mock.patch.object(self.client,'call',side_effect=running_call),mock.patch.object(phase_module.time,'sleep'):
            self.assertEqual(p.run()['new_oid'],42)
        self.assertEqual(self.drops,[11])

    def test_other_database_drift_refuses_before_drop(self):
        p=self.phase();current=self.current_catalog
        def changed():
            value=current();value['properties']['voice_db']['owner']='foreign';return value
        with mock.patch.object(catalog_module,'_full_catalog',side_effect=lambda *a:changed()):
            with self.assertRaises(phase_module.SpaceFreshError):p.run()
        self.assertEqual(self.drops,[]);self.assertEqual(self.calls,[])

    def test_foreign_created_source_cannot_reach_runtime_grants(self):
        p=self.phase();self.logs=self.logs.replace(b'a'*40,b'f'*40)
        with mock.patch.object(p.runtime,'run',wraps=p.runtime.run) as grants:
            with self.assertRaises(phase_module.SpaceFreshError):p.run()
            grants.assert_not_called()
        self.assertEqual(self.drops,[11]);self.assertEqual(self.backing.row['status'],'uncertain')

    def test_lost_delete_response_is_not_retried_or_marked_removed(self):
        p=self.phase();original=self.client.call
        def lost(*args,body=None):
            value=original(*args,body=body)
            if args[0]=='delete':raise TimeoutError('secret-in-error')
            return value
        with mock.patch.object(self.client,'call',side_effect=lost):
            with self.assertRaises(phase_module.SpaceFreshError):p.run()
        self.assertEqual(len([c for c in self.calls if c[0]=='delete']),1)
        saved=self.journal.state['steps'][phase_module.STEP]['details']
        self.assertEqual(saved['resources']['provision']['state'],'removing')
        self.assertEqual(self.backing.row['status'],'uncertain')

    def test_ledger_commit_lost_reply_prevents_drop(self):
        p=self.phase();original=self.ledger.write
        def lost(revision,row):
            original(revision,row)
            if row.get(phase_module.LEDGER,{}).get('state')=='drop_pending':raise TimeoutError('response lost')
        with mock.patch.object(self.ledger,'write',side_effect=lost):
            with self.assertRaises(phase_module.SpaceFreshError):p.run()
        self.assertEqual(self.drops,[]);self.assertEqual(self.calls,[])

    def test_runtime_failure_stops_readonly_verify_job_and_cleanup(self):
        p=self.phase()
        with mock.patch.object(p.runtime,'run',side_effect=ValueError('secret-in-error')):
            with self.assertRaises(phase_module.SpaceFreshError) as error:p.run()
        self.assertNotIn('secret-in-error',str(error.exception))
        self.assertEqual(len([c for c in self.calls if c[:2]==('create','Job')]),1)
        self.assertFalse(any(c[0]=='delete' for c in self.calls))

    def test_pending_pod_must_keep_exact_uid_until_running(self):
        p=self.phase();call=self.client.call;pending=True
        def delayed(*args,body=None):
            nonlocal pending
            result=call(*args,body=body)
            if args[:2]==('get','pods') and result['items'] and pending:
                pending=False;result['items'][0]['status']['phase']='Pending';self.oid=0
            elif args[:2]==('get','pods') and result['items']:
                self.oid=42
            return result
        with mock.patch.object(self.client,'call',side_effect=delayed),mock.patch.object(phase_module.time,'sleep'):
            self.assertEqual(p.run()['new_oid'],42)

    def test_changed_owned_pod_uid_refuses_before_grants(self):
        p=self.phase();call=self.client.call;reads=0
        def changed(*args,body=None):
            nonlocal reads
            result=call(*args,body=body)
            if args[:2]==('get','pods') and result['items']:
                reads+=1
                if reads>1:result['items'][0]['metadata']['uid']='foreign'
            return result
        with mock.patch.object(self.client,'call',side_effect=changed),mock.patch.object(p.runtime,'run',wraps=p.runtime.run) as grants:
            with self.assertRaises(phase_module.SpaceFreshError):p.run()
            grants.assert_not_called()


if __name__=='__main__':unittest.main()
