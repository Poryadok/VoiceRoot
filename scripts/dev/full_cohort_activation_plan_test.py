import copy
from pathlib import Path
import unittest
from unittest.mock import patch

import full_cohort_activation_plan as activation
import media_backup_phase_test as fixture
import media_operation_journal as journal


class ActivationTest(unittest.TestCase):
    def setUp(self):
        self.base=fixture.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.base.setUp(); self.addCleanup(self.base.doCleanups)
        self.journal=journal.Journal(Path(self.base.temp.name)/'activation',
            {'resource_context':copy.deepcopy(self.base.context),'namespace_uid':'ns'})
        self.addCleanup(self.journal.close)
        class Client(fixture.Client): pass
        self.client=Client(self.base.lock); self.client.journal=self.journal; self.base.ledger.client=self.client
        class Common:
            def __init__(other): other.fence=self.base.fence; other.lock=copy.deepcopy(self.base.lock); other.namespace_uid='ns'
        class Package(Common):
            def verify_current(other,token): return {'package_sha256':'a'*64}
        class Cut(Common):
            def __init__(other):
                super().__init__(); other.invocation='f'*32
                other.admission={'controllers':[{'kind':'Deployment','metadata':{'name':x}} for x in
                    ('voice-chat','voice-user','voice-realtime','voice-space')]}
        self.cut=Cut(); self.package=Package()
        class Image(Common):
            def __init__(other):
                super().__init__(); other.admission=type('Admission',(),{'cut':self.cut})()
                other.journal=type('ImageJournal',(),{'state':{'steps':{}}})()
                for name in activation.images.CATALOG:
                    other.journal.state['steps']['full-cohort-image-'+name]={'status':'done','receipt':{
                        'uid':name,'generation':1,'spec':{'replicas':0}}}
            def _guard(other): pass
            def _records(other,row): pass
            def verify_activation_lineage(other):
                other._records(self.base.ledger.row)
                return {'operation_id':self.base.context['operation_id'],'source_sha':self.base.context['commit'],
                        'images':{name:copy.deepcopy(other.journal.state['steps']['full-cohort-image-'+name]['receipt'])
                                  for name in activation.images.CATALOG}}
            def _drained(other,row): pass
            def _get(other,kind,name): return {'metadata':{'uid':name,'generation':1},'spec':{'replicas':0}}
            def _match(other,name,obj,uid,spec,generation=None): return obj['metadata']['uid']==uid and obj['spec']==spec
        self.image=Image()
        self.patches=[patch.object(activation.frontend_runtime_executor,'KubectlTransport',Client),
            patch.object(activation.package,'PackagePhase',Package),patch.object(activation.images,'FullCohortImageApply',Image),
            patch.object(activation.writer_cut,'WriterCut',Cut)]
        for p in self.patches: p.start(); self.addCleanup(p.stop)
        self.phase=activation.ActivationPhase(self.journal,self.base.fence,self.client,self.base.lock,'ns',self.package,self.image,self.cut)

    def test_plan_binds_order_and_holds_receipt_drift(self):
        result=self.phase.run('token')
        self.assertEqual([x['name'] for x in result['ordered']],['voice-user','voice-space','voice-chat','voice-realtime'])
        self.assertEqual(activation.verify_completed(self.phase,self.cut,'token'),result)
        self.journal.state['steps'][activation.STEP]['receipt']['ordered'].pop()
        with self.assertRaises(activation.ActivationError): activation.verify_completed(self.phase,self.cut,'token')

    def test_ledger_marker_drift_during_verify_is_rejected(self):
        self.phase.run('token')
        def mutate(row): self.base.ledger.row['full_cohort_activation_plan']['nonce']='changed'
        with patch.object(self.image,'_records',side_effect=mutate):
            with self.assertRaises(activation.ActivationError): activation.verify_completed(self.phase,self.cut,'token')

    def test_target_drift_after_reservation_marks_uncertain(self):
        original=self.image._get; calls=[]
        def observe(kind,name):
            calls.append(name); result=original(kind,name)
            if len(calls)>4: result['spec']['replicas']=1
            return result
        with patch.object(self.image,'_get',side_effect=observe):
            with self.assertRaises(activation.ActivationError): self.phase.run('token')
        self.assertEqual(self.base.ledger.row['status'],'uncertain')

    def test_local_pending_drift_marks_uncertain(self):
        original=self.image.verify_activation_lineage; calls=[]
        def lineage():
            calls.append(True); result=original()
            if len(calls)==2: self.journal.state['steps'][activation.STEP]['nonce']='changed'
            return result
        with patch.object(self.image,'verify_activation_lineage',side_effect=lineage):
            with self.assertRaises(activation.ActivationError): self.phase.run('token')
        self.assertEqual(self.base.ledger.row['status'],'uncertain')


if __name__=='__main__': unittest.main()
