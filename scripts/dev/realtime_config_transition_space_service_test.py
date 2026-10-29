import copy
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import media_backup_phase_test as fixtures
import media_operation_journal as journal
import realtime_config_transition_space_service as service
import realtime_config_transition as renderer


class Client:
    def __init__(self,source): self.source=copy.deepcopy(source)
    def call(self,verb,kind,name,*args): return copy.deepcopy(self.source)


class Cut:
    def _guard(self): pass
    def verify_drained(self): pass
    def _owned(self,row): return self.fence.owned(row)
    def _patch(self,kind,name,patch):
        if patch[2]['value']!=self.client.source['spec']: raise ValueError()
        self.client.source['spec']=copy.deepcopy(patch[3]['value'])
        self.client.source['metadata']['resourceVersion']='5'
        return copy.deepcopy(self.client.source)


class ServiceTest(unittest.TestCase):
    def setUp(self):
        self.base=fixtures.PhaseTest('test_exact_terminal_receipt_and_no_replay')
        self.addCleanup(self.cleanup,self.base); self.base.setUp()
        self.source={'apiVersion':'v1','kind':'Service','metadata':{'name':'voice-space','namespace':'voice-dev','uid':'svc','resourceVersion':'4'},
            'spec':{'clusterIP':'10.0.0.1','selector':{'app':'voice-space'},'ports':[{'name':'http','port':8080}]}}
        self.cut=Cut(); self.cut.fence=self.base.fence; self.cut.lock=self.base.lock; self.cut.namespace_uid='ns'
        self.cut.client=Client(self.source); self.cut.invocation='a'*32; self.cut.journal=self.base.journal
        self.cut.admission={'controllers':[{'kind':'Deployment','metadata':{'name':'voice-space'},
            'spec':{'template':{'metadata':{'labels':{'app':'voice-space'}}}}}]}
        intent={'resource_context':copy.deepcopy(self.base.context),'namespace_uid':'ns','realtime_space_service':{
            'source':copy.deepcopy(self.source),'source_sha':self.base.context['commit'],'cut_invocation':self.cut.invocation}}
        self.journal=journal.Journal(Path(self.base.temp.name)/'space-service',intent); self.addCleanup(self.journal.close)
        self.patcher=patch.object(service.writer_cut,'WriterCut',Cut); self.patcher.start(); self.addCleanup(self.patcher.stop)

    def cleanup(self,base):
        if not base.doCleanups(): raise RuntimeError('cleanup failed')

    def test_creator_target_adds_both_ports_in_one_patch_and_is_idempotent(self):
        labels={'app':'voice-space'}
        patch,target=renderer.render_space_creator_service(self.source,labels)
        self.assertEqual(patch[3]['value'],target)
        self.assertIn({'name':'principal-jwks','port':8443,'targetPort':8443,'protocol':'TCP'},target['ports'])
        self.assertIn({'name':'gateway-lc','port':9096,'targetPort':9096,'protocol':'TCP'},target['ports'])
        current=copy.deepcopy(self.source);current['spec']=target
        self.assertEqual(renderer.render_space_creator_service(current,labels)[1],target)

    def test_creator_target_refuses_name_port_and_target_port_conflicts(self):
        for collision in ({'name':'gateway-lc','port':9010},
                          {'name':'foreign','port':9096},{'name':'foreign','port':9010,'targetPort':9096},
                          {'name':'foreign','port':9010,'targetPort':'gateway-lc'}):
            source=copy.deepcopy(self.source);source['spec']['ports'].append(collision)
            with self.assertRaises(ValueError):renderer.render_space_creator_service(source,{'app':'voice-space'})

    def test_fenced_cas_and_post_resume_historical_verifier(self):
        receipt=service.apply_space_service(self.cut,self.journal)
        self.assertEqual(receipt['resourceVersion'],'5')
        self.assertIn({'name':'principal-jwks','port':8443,'targetPort':8443,'protocol':'TCP'},self.cut.client.source['spec']['ports'])
        self.cut.verify_drained=lambda:(_ for _ in ()).throw(AssertionError('must not call'))
        proof=service.verify_space_service(self.cut,self.journal)
        self.assertEqual(proof['receipt'],receipt)
        self.cut.client.source['metadata']['resourceVersion']='6'
        with self.assertRaises(service.SpaceServiceError): service.verify_space_service(self.cut,self.journal)

    def test_lost_patch_response_is_uncertain_and_no_retry(self):
        def lost(*args):
            self.cut.client.source['metadata']['resourceVersion']='5'; raise OSError('private')
        self.cut._patch=lost
        with self.assertRaises(service.SpaceServiceError) as error: service.apply_space_service(self.cut,self.journal)
        self.assertNotIn('private',str(error.exception)); self.assertEqual(self.base.ledger.row['status'],'uncertain')
        with self.assertRaises(service.SpaceServiceError): service.apply_space_service(self.cut,self.journal)

    def test_existing_exact_port_records_observation_without_kubernetes_write(self):
        port={'name':'principal-jwks','port':8443,'targetPort':8443,'protocol':'TCP'}
        self.cut.client.source['spec']['ports'].append(copy.deepcopy(port))
        self.journal.intent['realtime_space_service']['source']['spec']['ports'].append(copy.deepcopy(port))
        self.journal.state['intent']=copy.deepcopy(self.journal.intent)
        self.cut._patch=lambda *a: (_ for _ in ()).throw(AssertionError('unexpected write'))
        receipt=service.apply_space_service(self.cut,self.journal)
        self.assertEqual(receipt['resourceVersion'],'4')
        self.assertEqual(service.verify_space_service(self.cut,self.journal)['receipt'],receipt)


if __name__=='__main__': unittest.main()
