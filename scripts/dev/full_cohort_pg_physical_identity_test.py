"""Fenced physical PostgreSQL capture; Kubernetes and SQL transport are fixtures."""
import copy
import importlib
import json
import subprocess
from pathlib import Path
import tempfile
from types import SimpleNamespace
import unittest
from unittest import mock

import frontend_runtime_executor as runtime
import full_cohort_affected_plan as affected
import media_backup_phase_test as fixture
import media_operation_journal
import resource_fencing
import writer_cut
import writer_cut_test


class PhysicalPhaseTest(unittest.TestCase):
    def setUp(self):
        self.context={'project':'voice','resource':'DEV','actor':'scheduled_devops',
            'claim':'11111111-1111-4111-8111-111111111111','window':'22222222-2222-4222-8222-222222222222',
            'revision':1,'generation':1,'operation_id':'media-c1-'+'a'*32,'commit':'a'*40,
            'images':{'voice-file/file':'ghcr.io/x@sha256:'+'b'*64}}
        self.ledger=fixture.Ledger(self.context)
        self.fence=resource_fencing.Fence(self.context,lambda:{},self.ledger)
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup)
        self.journal=media_operation_journal.Journal(Path(self.tmp.name)/'journal',
            {'resource_context':self.context,'namespace_uid':'ns'})
        self.addCleanup(self.journal.close)
        self.client=object.__new__(runtime.KubectlTransport);self.client.journal=self.journal;self.ledger.client=self.client
        actual_ledger=resource_fencing.KubeLedger(self.client)
        actual_ledger.read=lambda:self.ledger.read()
        actual_ledger.write=lambda revision,row:self.ledger.write(revision,row)
        self.fence.ledger=actual_ledger
        self.lock={};self.catalog=object.__new__(affected.FullCatalogPhase)
        self.catalog.fence=self.fence;self.catalog.lock=self.lock;self.catalog.namespace_uid='ns'
        self.catalog.journal=self.journal;self.catalog.client=self.client
        self.catalog.inventory=SimpleNamespace(_admin_role=lambda:'postgres')
        self.cut=object.__new__(writer_cut.WriterCut)
        self.cut.fence=self.fence;self.cut.lock=self.lock;self.cut.namespace_uid='ns';self.cut.verify_drained=mock.Mock()
        self.cut.journal=self.journal;self.cut.client=self.client
        self.engine={'namespace':'voice-dev','namespace_uid':'ns','pod_name':'voice-postgres-0','pod_uid':'pod',
            'container_name':'postgres','container_id':'containerd://fixture','restart_count':0,
            'pvc_uid':'pvc','pv_uid':'pv'}
        self.receipt={'operation_id':self.context['operation_id'],'source_sha':self.context['commit'],
            'namespace_uid':'ns','engine':self.engine,'server_start':'start',
            'databases':{'postgres':42},'all_databases':{'postgres':42},'excluded':{},'properties':{}}
        self.catalog.verify_current=mock.Mock(return_value=self.receipt)
        self.catalog.verify_receipt=mock.Mock(return_value=self.receipt)
        self.objects={
            ('statefulset','voice-postgres'):{'metadata':{'name':'voice-postgres','namespace':'voice-dev','uid':'sts'}},
            ('pod','voice-postgres-0'):{'metadata':{'name':'voice-postgres-0','namespace':'voice-dev','uid':'pod',
                'ownerReferences':[{'kind':'StatefulSet','uid':'sts','controller':True}]},
                'spec':{'nodeName':'node','containers':[{'name':'postgres','volumeMounts':[{'name':'data','mountPath':'/var/lib/postgresql/data'}]}],
                    'volumes':[{'name':'data','persistentVolumeClaim':{'claimName':'pgdata-voice-postgres-0'}}]},
                'status':{'containerStatuses':[{'name':'postgres','containerID':'containerd://fixture','restartCount':0,'ready':True,'state':{'running':{}}}]}},
            ('persistentvolumeclaim','pgdata-voice-postgres-0'):{'metadata':{'name':'pgdata-voice-postgres-0','namespace':'voice-dev','uid':'pvc'},'spec':{'volumeName':'pvc-volume'},'status':{'phase':'Bound'}},
            ('persistentvolume','pvc-volume'):{'metadata':{'name':'pvc-volume','uid':'pv'},'spec':{'claimRef':{'uid':'pvc','name':'pgdata-voice-postgres-0','namespace':'voice-dev'}},'status':{'phase':'Bound'}},
            ('node','node'):{'metadata':{'name':'node','uid':'node-uid'},'status':{'nodeInfo':{'operatingSystem':'linux'},'conditions':[{'type':'Ready','status':'True'}]}}}
        self.client.call=mock.Mock(side_effect=lambda _get,kind,name,*_args:copy.deepcopy(self.objects[(kind,name)]))
        self.pg={'database':'postgres','database_oid':42,'role':'postgres','server_start':'start','read_only':'on',
            'data_directory':'/var/lib/postgresql/data','pg_major':16,'system_identifier':'7350000000000000001'}

    def test_real_phase_binds_observed_physical_identity_and_refuses_drift(self):
        try: module=importlib.import_module('full_cohort_pg_physical_identity')
        except ModuleNotFoundError as error:
            if error.name!='full_cohort_pg_physical_identity':raise
            self.fail('source-owned physical identity phase is required')
        marker={'status':'done','receipt_sha256':module._digest(self.receipt)}
        self.ledger.row['full_cohort_db_catalog']=marker
        with mock.patch.object(module.media_operation_guard,'verify'), \
             mock.patch.object(module.schema,'kube_engine_snapshot',return_value=lambda:copy.deepcopy(self.engine)), \
             mock.patch.object(module.schema,'linux_psql',side_effect=lambda _args:json.dumps(self.pg).encode()), \
             mock.patch.object(module.PhysicalPhase,'_mount_proof',return_value=None):
            phase=module.PhysicalPhase(self.journal,self.fence,self.client,self.lock,'ns',self.catalog,self.cut)
            expected=phase.run('token')
            self.assertEqual(expected,{'namespace_uid':'ns','pod_uid':'pod','statefulset_uid':'sts','pvc_uid':'pvc',
                'pv_name':'pvc-volume','pv_uid':'pv','container':'postgres','container_id':'containerd://fixture',
                'restart_count':0,'mount_path':'/var/lib/postgresql/data','data_directory':'/var/lib/postgresql/data','pg_major':16})
            self.assertEqual(module.verify_current(phase,'token'),expected)
            self.assertEqual(module.verify_receipt(phase,'token')['physical']['system_identifier'],self.pg['system_identifier'])
            self.pg['system_identifier']='7350000000000000002'
            self.assertEqual(module.verify_receipt(phase,'token')['physical']['system_identifier'],'7350000000000000001')
            with self.assertRaises(module.PhysicalError):module.verify_current(phase,'token')
            self.pg['system_identifier']='7350000000000000001'
            saved=copy.deepcopy(self.journal.state['steps'][module.STEP])
            self.journal.state['steps'][module.STEP]['receipt']['physical']['system_identifier']='7'
            with self.assertRaises(module.PhysicalError):module.verify_receipt(phase,'token')
            self.journal.state['steps'][module.STEP]=saved
            calls=[0]
            def catalog_drift(_token):
                calls[0]+=1
                if calls[0]==2:self.journal.state['steps'].pop(module.STEP)
                return copy.deepcopy(self.receipt)
            with mock.patch.object(self.catalog,'verify_receipt',side_effect=catalog_drift):
                with self.assertRaises(module.PhysicalError):module.verify_receipt(phase,'token')
            self.journal.state['steps'][module.STEP]=copy.deepcopy(saved)
            real_ledger=phase._ledger;calls=[0]
            def ledger_drift():
                calls[0]+=1
                result=real_ledger()
                if calls[0]==4:self.journal.state['steps'].pop(module.STEP)
                return result
            with mock.patch.object(phase,'_ledger',side_effect=ledger_drift):
                with self.assertRaises(module.PhysicalError):module.verify_receipt(phase,'token')
            self.journal.state['steps'][module.STEP]=saved
            for kind, fault in [('pod','uid'),('pv','uid'),('claim','uid'),('mount','path'),('node','uid'),('database','oid'),('server','start'),('system','identifier'),('fence','claim'),('ledger','status')]:
                with self.subTest(fault=kind):
                    before=copy.deepcopy(self.objects);pg=copy.deepcopy(self.pg);context=copy.deepcopy(self.fence.context)
                    ledger=copy.deepcopy(self.ledger.row)
                    if kind=='pod':self.objects[('pod','voice-postgres-0')]['metadata']['uid']='other'
                    elif kind=='pv':self.objects[('persistentvolume','pvc-volume')]['metadata']['uid']='other'
                    elif kind=='claim':self.objects[('persistentvolume','pvc-volume')]['spec']['claimRef']['uid']='other'
                    elif kind=='mount':self.objects[('pod','voice-postgres-0')]['spec']['containers'][0]['volumeMounts'][0]['mountPath']='/foreign'
                    elif kind=='node':self.objects[('node','node')]['metadata']['uid']='other'
                    elif kind=='database':self.pg['database_oid']=99
                    elif kind=='server':self.pg['server_start']='changed'
                    elif kind=='system':self.pg['system_identifier']='7350000000000000002'
                    elif kind=='fence':self.fence.context['claim']='33333333-3333-4333-8333-333333333333'
                    else:self.ledger.row['status']='uncertain'
                    with self.assertRaises(module.PhysicalError):module.verify_current(phase,'token')
                    self.objects=before;self.pg=pg;self.fence.context.clear();self.fence.context.update(context);self.ledger.row=ledger

    def test_lost_ledger_write_keeps_pending_and_prevents_observation(self):
        module=importlib.import_module('full_cohort_pg_physical_identity')
        self.ledger.row['full_cohort_db_catalog']={'status':'done','receipt_sha256':module._digest(self.receipt)}
        original=self.ledger.write
        def lost(revision,row):
            original(revision,row)
            if row.get(module.LEDGER,{}).get('status')=='pending':raise RuntimeError('lost reply')
        with mock.patch.object(module.media_operation_guard,'verify'),mock.patch.object(self.ledger,'write',side_effect=lost):
            phase=module.PhysicalPhase(self.journal,self.fence,self.client,self.lock,'ns',self.catalog,self.cut)
            with mock.patch.object(phase,'_observe') as observe:
                with self.assertRaisesRegex(module.PhysicalError,'reconcile'):phase.run('token')
                observe.assert_not_called()
        self.assertTrue(phase.poisoned)
        self.assertEqual(self.journal.state['steps'][module.STEP]['status'],'pending')
        self.assertEqual(self.ledger.row[module.LEDGER]['status'],'pending')

    def test_distinct_facades_bind_same_ledger_and_reject_drift(self):
        module=importlib.import_module('full_cohort_pg_physical_identity')
        self.ledger.row['full_cohort_db_catalog']={'status':'done','receipt_sha256':module._digest(self.receipt)}
        def facade(name):
            journal=media_operation_journal.Journal(Path(self.tmp.name)/name,
                {'resource_context':copy.deepcopy(self.context),'namespace_uid':'ns'})
            self.addCleanup(journal.close)
            client=object.__new__(runtime.KubectlTransport);client.journal=journal
            ledger=resource_fencing.KubeLedger(client)
            ledger.read=lambda:self.ledger.read()
            ledger.write=lambda revision,row:self.ledger.write(revision,row)
            fence=resource_fencing.Fence(copy.deepcopy(self.context),lambda:{},ledger)
            return journal,client,fence
        cj,cc,cf=facade('catalog-facade');wj,wc,wf=facade('cut-facade')
        self.catalog.journal,self.catalog.client,self.catalog.fence=cj,cc,cf
        self.cut.journal,self.cut.client,self.cut.fence=wj,wc,wf
        observed={'expected':{'fixture':'physical'},'engine':self.engine,'server_start':'start',
                  'database_oid':42,'system_identifier':'7','node':{'name':'node','uid':'node-uid'}}
        with mock.patch.object(module.media_operation_guard,'verify'), \
             mock.patch.object(module.PhysicalPhase,'_observe',return_value=observed):
            phase=module.PhysicalPhase(self.journal,self.fence,self.client,self.lock,'ns',self.catalog,self.cut)
            self.assertEqual(phase.run('token'),{'fixture':'physical'})
            original=wf.ledger.read;wf.ledger.read=lambda:(copy.deepcopy(self.ledger.row),('other-uid','1'))
            with self.assertRaises(module.PhysicalError):module.verify_current(phase,'token')
            wf.ledger.read=original
            self.cut.client=cc
            with self.assertRaises(module.PhysicalError):module.verify_current(phase,'token')

    def test_closed_physical_journal_invalidates_historical_receipt(self):
        module=importlib.import_module('full_cohort_pg_physical_identity')
        self.ledger.row['full_cohort_db_catalog']={'status':'done','receipt_sha256':module._digest(self.receipt)}
        value={'expected':{'fixture':'physical'},'engine':self.engine,'server_start':'start',
               'database_oid':42,'system_identifier':'7','node':{'name':'node','uid':'node-uid'}}
        with mock.patch.object(module.media_operation_guard,'verify'), \
             mock.patch.object(module.PhysicalPhase,'_observe',return_value=value):
            phase=module.PhysicalPhase(self.journal,self.fence,self.client,self.lock,'ns',self.catalog,self.cut)
            phase.run('token')
            self.journal.close()
            with self.assertRaises(module.PhysicalError):module.verify_receipt(phase,'token')

    def test_physical_journal_closed_during_catalog_recheck_holds(self):
        module=importlib.import_module('full_cohort_pg_physical_identity')
        self.ledger.row['full_cohort_db_catalog']={'status':'done','receipt_sha256':module._digest(self.receipt)}
        value={'expected':{'fixture':'physical'},'engine':self.engine,'server_start':'start',
               'database_oid':42,'system_identifier':'7','node':{'name':'node','uid':'node-uid'}}
        with mock.patch.object(module.media_operation_guard,'verify'), \
             mock.patch.object(module.PhysicalPhase,'_observe',return_value=value):
            phase=module.PhysicalPhase(self.journal,self.fence,self.client,self.lock,'ns',self.catalog,self.cut)
            phase.run('token'); calls=[0]
            def close_on_recheck(_token):
                calls[0]+=1
                if calls[0]==2:self.journal.close()
                return copy.deepcopy(self.receipt)
            with mock.patch.object(self.catalog,'verify_receipt',side_effect=close_on_recheck):
                with self.assertRaises(module.PhysicalError):module.verify_receipt(phase,'token')

    def test_catalog_cut_and_physical_share_authority_through_distinct_facades(self):
        module=importlib.import_module('full_cohort_pg_physical_identity')
        base=writer_cut_test.WriterCutTest('test_drain_and_resume_use_current_template_not_old')
        self.addCleanup(lambda:self.assertTrue(base.doCleanups()))
        base.setUp()
        backing=base.ledger
        class LedgerView:
            def __init__(inner,client): inner.client=client
            def read(inner): return backing.read()
            def write(inner,revision,row): return backing.write(revision,row)
        def facade(name,extra):
            j=media_operation_journal.Journal(Path(base.temp.name)/name,
                {'resource_context':copy.deepcopy(base.context),'namespace_uid':'ns',**extra})
            self.addCleanup(j.close)
            c=writer_cut_test.Client(base.lock,j); c.workloads=base.client.workloads
            f=resource_fencing.Fence(copy.deepcopy(base.context),lambda:copy.deepcopy(base.card),LedgerView(c))
            return j,c,f
        observed={'database_observations':{'workloads':{'user':[{'status':'observed','result':{'metadata':{
            'engine':{'namespace_uid':'ns'},'session':{'server_start':'start'}}}}]}}}
        class Inventory:
            def __init__(inner,fence): inner.fence=fence;inner.lock=base.lock;inner.namespace_uid='ns'
            def verify_current(inner,token): return copy.deepcopy(observed)
            def verify_receipt(inner,token): return copy.deepcopy(observed)
        cj,cc,cf=facade('physical-catalog',{})
        full={'databases':{'postgres':42},'all_databases':{'postgres':42},'excluded':{},'properties':{}}
        with mock.patch.object(resource_fencing,'KubeLedger',LedgerView), \
             mock.patch.object(affected.inventory,'InventoryPhase',Inventory), \
             mock.patch.object(affected,'_full_catalog',return_value=full), \
             mock.patch.object(module.media_operation_guard,'verify'):
            catalog=affected.FullCatalogPhase(cj,cf,cc,base.lock,'ns',Inventory(cf))
            receipt=catalog.run('token')
            admission=copy.deepcopy(base.admission)
            admission['session_scope']={'engine':receipt['engine'],'server_start':'start',
                                        'databases':['postgres'],'database_oids':{'postgres':42}}
            wj,wc,wf=facade('physical-cut',{'writer_cut':admission})
            cut=writer_cut.WriterCut(wj,wf,wc,base.lock,'ns',admission,
                lambda scope:{'scope':scope,'active_sessions':0,'active_transactions':0})
            with mock.patch.object(cut,'_patch',side_effect=wc.patch):cut.drain()
            pj,pc,pf=facade('physical-phase',{})
            value={'expected':{'fixture':'physical'},'engine':receipt['engine'],'server_start':'start',
                   'database_oid':42,'system_identifier':'7','node':{'name':'node','uid':'node-uid'}}
            with mock.patch.object(module.PhysicalPhase,'_observe',return_value=value):
                phase=module.PhysicalPhase(pj,pf,pc,base.lock,'ns',catalog,cut)
                self.assertEqual(phase.run('token'),{'fixture':'physical'})
                self.assertEqual(module.verify_current(phase,'token'),{'fixture':'physical'})

    def test_mount_proof_uses_existing_producer_and_rejects_nested_mount(self):
        module=importlib.import_module('full_cohort_pg_physical_identity')
        phase=object.__new__(module.PhysicalPhase)
        phase._guard=mock.Mock();phase.cut=SimpleNamespace(verify_drained=mock.Mock())
        expected={'mount_path':'/var/lib/postgresql/data','data_directory':'/var/lib/postgresql/data','pg_major':16}
        mount='10 1 0:1 / /var/lib/postgresql/data rw - ext4 /dev/fixture rw\n'
        physical={'data_directory':expected['data_directory'],'version':'160003','mountinfo':mount}
        def output(args,limit):
            if 'test ! -L' in args[-3]:return b'ok'
            if 'cat /proc/self/mountinfo' in args[-4]:
                return (expected['data_directory']+'\n'+expected['mount_path']+'\n'+mount).encode()
            return json.dumps(physical).encode()
        with mock.patch.object(module.schema,'_bounded_process',side_effect=output):
            phase._mount_proof(expected)
            self.assertGreaterEqual(phase._guard.call_count,4)
            mount+='11 10 0:2 / /var/lib/postgresql/data/base rw - ext4 /dev/other rw\n'
            physical['mountinfo']=mount
            with self.assertRaises(module.capture.CaptureError):phase._mount_proof(expected)
            mount='10 1 0:1 / /var/lib/postgresql/data rw - ext4 /dev/fixture rw\n'
            physical['mountinfo']=mount
            def external(args,limit):
                if 'test ! -L' in args[-3]:return b'foreign-tablespace'
                return output(args,limit)
            with mock.patch.object(module.schema,'_bounded_process',side_effect=external):
                with self.assertRaises(module.capture.CaptureError):phase._mount_proof(expected)

    def test_fixed_storage_probe_executes_and_rejects_external_links(self):
        module=importlib.import_module('full_cohort_pg_physical_identity')
        script=getattr(module,'TABLESPACE_SCRIPT',None)
        self.assertIsInstance(script,str,'fixed storage probe is required')
        with tempfile.TemporaryDirectory() as tmp:
            root=Path(tmp);(root/'pg_wal').mkdir();(root/'pg_tblspc').mkdir()
            def check():
                return subprocess.run(['sh','-ec',script,'--',str(root)],stdout=subprocess.PIPE,stderr=subprocess.DEVNULL)
            self.assertEqual(check().stdout,b'ok');self.assertEqual(check().returncode,0)
            (root/'pg_tblspc'/'external').symlink_to('/tmp')
            self.assertNotEqual(check().returncode,0)
            (root/'pg_tblspc'/'external').unlink();(root/'pg_wal').rmdir();(root/'pg_wal').symlink_to('/tmp')
            self.assertNotEqual(check().returncode,0)


if __name__=='__main__':unittest.main()
