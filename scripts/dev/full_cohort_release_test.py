"""Release journal boundaries; external phase and Kubernetes observations are synthetic."""
import copy
from pathlib import Path
import unittest
from unittest.mock import Mock, patch

import frontend_runtime_executor as transport
import full_cohort_activation_plan as activation
import full_cohort_image_apply as images
import full_cohort_package_phase as package
import full_cohort_release as subject
import media_backup_phase_test as fixture
import media_operation_journal as journals
import resource_fencing
import writer_cut


class ReleaseTest(unittest.TestCase):
    def setUp(self):
        self.base=fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.base.setUp(); self.addCleanup(self.base.doCleanups)
        self.root=Path(self.base.temp.name); self.context=self.base.context; self.lock=self.base.lock
        endpoint_patch=patch.object(subject.media_backup_upload_runner,'TT','http://fixture')
        endpoint_patch.start(); self.addCleanup(endpoint_patch.stop)
        self.check=resource_fencing.ResourceCheck('http://fixture',self.context)
        check_patch=patch.object(resource_fencing.ResourceCheck,'__call__',return_value=copy.deepcopy(self.base.card))
        check_patch.start(); self.addCleanup(check_patch.stop)
        def attach(obj,name):
            obj.journal=journals.Journal(self.root/name,{'resource_context':copy.deepcopy(self.context),'namespace_uid':'ns'})
            self.addCleanup(obj.journal.close); obj.client=transport.KubectlTransport(obj.journal)
            obj.client.call=Mock(side_effect=self.call); ledger=resource_fencing.KubeLedger(obj.client)
            ledger.read=self.base.ledger.read; ledger.write=self.base.ledger.write
            obj.fence=resource_fencing.Fence(self.context,self.check,ledger,self.check.mark_uncertain)
            obj.lock=copy.deepcopy(self.lock); obj.namespace_uid='ns'; return obj
        self.package=attach(package.PackagePhase.__new__(package.PackagePhase),'package')
        self.image=attach(images.FullCohortImageApply.__new__(images.FullCohortImageApply),'image')
        self.activation=attach(activation.ActivationPhase.__new__(activation.ActivationPhase),'activation')
        self.cut=attach(writer_cut.WriterCut.__new__(writer_cut.WriterCut),'cut')
        self.cut.started=True; self.cut.drained=False
        self.cut.admission={'controllers':[{'kind':'Deployment','metadata':{'name':'voice-space'}}]}
        self.health={}
        for name,cls in (('space_health',subject.space_health.SpaceHealthPhase),
                         ('publisher_health',subject.publisher_health.PublisherHealthPhase),
                         ('chat_health',subject.chat_health.ChatHealthPhase)):
            value=attach(cls.__new__(cls),name); value.record={'status':'done'}
            self.health[name]=value
        self.cut.activation={'phase':self.activation,'token':'token','stage':'done','plan_sha256':'a'*64,'health':{
            name:{'phase':value,'token':'token','marker':{'status':'done'}} for name,value in self.health.items()}}
        self.cut.verify_activation_stage=Mock(return_value={'stage':'done'})
        self.cut._activation_verify_sources=Mock()
        self.cut.current_resumed_proof=Mock(return_value={'controllers':[{'kind':'Deployment','name':'voice-space'}]})
        self.package.verify_current=Mock(return_value={'package_sha256':'a'*64})
        self.image.admission=type('Admission',(),{'cut':self.cut})()
        self.image.verify_after_resume=Mock(return_value={name:{} for name in images.CATALOG})
        self.activation.package=self.package; self.activation.image=self.image; self.activation.cut=self.cut
        own=attach(type('Own',(),{})(),'release')
        self.phase=subject.ReleasePhase(own.journal,own.fence,own.client,self.lock,'ns',
                                        self.package,self.image,self.activation,self.cut)

    def call(self,*args):
        if args[1]=='namespace':return {'metadata':{'name':'voice-dev','uid':'ns','labels':self.lock['metadata']['labels']}}
        if args[1]=='configmap':return copy.deepcopy(self.lock)
        raise AssertionError('unexpected synthetic Kubernetes request')

    def test_active_fence_and_owned_smoke_marker_revalidate(self):
        receipt=self.phase.run('token')
        self.assertEqual(set(receipt),{'operation_id','source_sha','namespace_uid','package_sha256'})
        self.base.ledger.row['core_smoke']={'status':'pending'}; self.base.ledger.version+=1
        self.assertEqual(subject.verify_completed(self.phase,'token'),receipt)
        self.base.ledger.row['status']='uncertain'
        with self.assertRaises(subject.ReleaseError):subject.verify_completed(self.phase,'token')

    def test_final_callback_changes_dependency_journal_refuses(self):
        self.phase.run('token')
        def drift():
            self.activation.journal.state['steps']['foreign']={'status':'done'}
            return {name:{} for name in images.CATALOG}
        self.image.verify_after_resume.side_effect=drift
        with self.assertRaises(subject.ReleaseError):subject.verify_completed(self.phase,'token')

    def test_late_health_journal_close_refuses(self):
        self.phase.run('token')
        def close_health():
            self.health['space_health'].journal.close()
            return {name:{} for name in images.CATALOG}
        self.image.verify_after_resume.side_effect=close_health
        with self.assertRaises(subject.ReleaseError):subject.verify_completed(self.phase,'token')

    def test_done_write_cannot_mutate_health_evidence(self):
        original=self.phase.fence.ledger.write
        def mutate(revision,row):
            result=original(revision,row)
            if row.get(subject.LEDGER,{}).get('status')=='done':
                self.health['chat_health'].journal.state['steps']['foreign']={'status':'done'}
            return result
        self.phase.fence.ledger.write=mutate
        with self.assertRaises(subject.ReleaseError):self.phase.run('token')

    def test_done_write_cannot_mutate_remote_health_marker(self):
        self.base.ledger.row['realtime_space_health']={'status':'done','nonce':'original'}
        original=self.phase.fence.ledger.write
        def mutate(revision,row):
            result=original(revision,row)
            if row.get(subject.LEDGER,{}).get('status')=='done':
                self.base.ledger.row['realtime_space_health']={'status':'done','nonce':'changed'}
                self.base.ledger.version+=1
            return result
        self.phase.fence.ledger.write=mutate
        with self.assertRaises(subject.ReleaseError):self.phase.run('token')

    def test_done_write_cannot_mutate_activation_binding(self):
        original=self.phase.fence.ledger.write
        def mutate(revision,row):
            result=original(revision,row)
            if row.get(subject.LEDGER,{}).get('status')=='done':self.cut.activation['stage']='changed'
            return result
        self.phase.fence.ledger.write=mutate
        with self.assertRaises(subject.ReleaseError):self.phase.run('token')

    def test_replaced_dependency_check_refuses_before_effect(self):
        self.image.fence.check=resource_fencing.ResourceCheck('http://fixture',self.context)
        with self.assertRaises(subject.ReleaseError):self.phase.run('token')
        self.assertNotIn(subject.LEDGER,self.base.ledger.row)

    def test_incomplete_health_holds_before_release_marker(self):
        del self.cut.activation['health']['chat_health']
        with self.assertRaises(subject.ReleaseError):self.phase.run('token')
        self.assertNotIn(subject.LEDGER,self.base.ledger.row)


if __name__=='__main__':unittest.main()
