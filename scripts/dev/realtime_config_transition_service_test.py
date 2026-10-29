import copy
from pathlib import Path
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import media_backup_phase_test as fixtures
import media_operation_journal as journal
import realtime_config_transition_service as service


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
        self.source={'apiVersion':'v1','kind':'Service','metadata':{'name':'voice-chat','namespace':'voice-dev','uid':'svc','resourceVersion':'4'},
            'spec':{'clusterIP':'10.0.0.1','selector':{'app':'voice-chat'},'ports':[{'name':'http','port':8080}]}}
        self.cut=Cut(); self.cut.fence=self.base.fence; self.cut.lock=self.base.lock; self.cut.namespace_uid='ns'
        self.cut.client=Client(self.source); self.cut.invocation='a'*32; self.cut.journal=self.base.journal
        self.cut.admission={'controllers':[{'kind':'Deployment','metadata':{'name':'voice-chat'},
            'spec':{'template':{'metadata':{'labels':{'app':'voice-chat'}}}}}]}
        intent={'resource_context':copy.deepcopy(self.base.context),'namespace_uid':'ns','realtime_chat_service':{
            'source':copy.deepcopy(self.source),'source_sha':self.base.context['commit'],'cut_invocation':self.cut.invocation}}
        self.journal=journal.Journal(Path(self.base.temp.name)/'chat-service',intent); self.addCleanup(self.journal.close)
        self.patcher=patch.object(service.writer_cut,'WriterCut',Cut); self.patcher.start(); self.addCleanup(self.patcher.stop)

    def cleanup(self,base):
        if not base.doCleanups(): raise RuntimeError('cleanup failed')

    def test_fenced_cas_and_post_resume_historical_verifier(self):
        receipt=service.apply_chat_service(self.cut,self.journal)
        self.assertEqual(receipt['resourceVersion'],'5')
        self.cut.verify_drained=lambda:(_ for _ in ()).throw(AssertionError('must not call'))
        proof=service.verify_chat_service(self.cut,self.journal)
        self.assertEqual(proof['receipt'],receipt)
        self.cut.client.source['metadata']['resourceVersion']='6'
        with self.assertRaises(service.ChatServiceError): service.verify_chat_service(self.cut,self.journal)

    def test_lost_patch_response_is_uncertain_and_no_retry(self):
        def lost(*args):
            self.cut.client.source['metadata']['resourceVersion']='5'; raise OSError('private')
        self.cut._patch=lost
        with self.assertRaises(service.ChatServiceError) as error: service.apply_chat_service(self.cut,self.journal)
        self.assertNotIn('private',str(error.exception)); self.assertEqual(self.base.ledger.row['status'],'uncertain')
        with self.assertRaises(service.ChatServiceError): service.apply_chat_service(self.cut,self.journal)


if __name__=='__main__': unittest.main()
