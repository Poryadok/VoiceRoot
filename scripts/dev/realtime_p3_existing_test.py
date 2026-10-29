import base64
import copy
import sys
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import realtime_p3_existing as existing
import realtime_p3_create_only_packet_test as fixtures


class Client: pass


class ExistingTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        fixtures.PacketTest.setUpClass()
        cls.addClassCleanup(fixtures.PacketTest.doClassCleanups)

    def setUp(self):
        self.context={'operation_id':'media-c2-'+'a'*32,'commit':'b'*40}
        self.client=Client(); self.row={'status':'active'}
        self.fence=SimpleNamespace(context=self.context,ledger=SimpleNamespace(client=self.client,read=lambda:(copy.deepcopy(self.row),'1')))
        self.roots={'voice-principal-ca':fixtures.PacketTest.principal,'voice-media-ca':fixtures.PacketTest.media,
                    'voice-space-media-server-tls':fixtures.PacketTest.space}
        self.items={}
        for obj in fixtures.PacketTest.envelope['objects']['items']:
            item=copy.deepcopy(obj); item['metadata'].update(uid=item['metadata']['name']+'-uid',resourceVersion='7')
            self.items[item['kind'],item['metadata']['name']]=item
        self.patches=[patch.object(existing.frontend_runtime_executor,'KubectlTransport',Client),
                      patch.object(existing.media_operation_guard,'verify'),patch.object(existing.source,'verify'),
                      patch.object(existing.transport,'get',side_effect=lambda k,n:copy.deepcopy(self.items[k,n])),
                      patch.object(existing.transport,'public_certificate',side_effect=lambda n,k:('uid-'+n+'\n1\n').encode()+base64.b64encode(self.roots[n]))]
        for item in self.patches: item.start(); self.addCleanup(item.stop)

    def test_real_crypto_returns_public_identities_only(self):
        result=existing.observe(self.fence,self.client,{},'ns','/source')
        self.assertEqual(len(result['objects']),7)
        self.assertEqual(result['public_sources']['voice-media-ca']['uid'],'uid-voice-media-ca')
        self.assertEqual(result['scope'],'current-content-only-predecessor-unverified')
        self.assertNotIn('PRIVATE KEY',str(result)); self.assertNotIn('data',str(result))

    def test_c1_wrong_keys_and_changed_content_hold(self):
        self.context['operation_id']='media-c1-'+'a'*32
        with self.assertRaises(existing.ExistingError): existing.observe(self.fence,self.client,{},'ns','/source')
        self.context['operation_id']='media-c2-'+'a'*32
        secret=self.items['Secret','voice-p3-principal-replay']; secret['data']['password']='c2VjcmV0'
        with self.assertRaises(existing.ExistingError): existing.observe(self.fence,self.client,{},'ns','/source')

    def test_lost_guard_during_crypto_does_not_return_receipt(self):
        actual_der = existing.crypto.der; lost = False
        def derive(value):
            nonlocal lost
            result = actual_der(value); lost = True; return result
        def guard(*args):
            if lost: raise RuntimeError('lost')
        with patch.object(existing.crypto,'der',side_effect=derive), \
             patch.object(existing.media_operation_guard,'verify',side_effect=guard):
            with self.assertRaises(existing.ExistingError): existing.observe(self.fence,self.client,{},'ns','/source')

    def test_success_has_authority_check_after_last_crypto(self):
        actual_der = existing.crypto.der; events = []
        def derive(value): events.append('der'); return actual_der(value)
        def guard(*args): events.append('guard')
        with patch.object(existing.crypto,'der',side_effect=derive), \
             patch.object(existing.media_operation_guard,'verify',side_effect=guard):
            existing.observe(self.fence,self.client,{},'ns','/source')
        self.assertIn('der',events); self.assertEqual(events[-1],'guard')

    def test_reuse_requires_authenticated_exact_prior_and_current(self):
        current=existing.observe(self.fence,self.client,{},'ns','/source')
        objects={v['kind'].lower()+'-'+name:{'name':name,**v} for name,v in current['objects'].items()}
        prior={'previous_operation_id':'media-c1-'+'b'*32,'p3_receipt':{
            'operation_id':'media-c1-'+'b'*32,'issuer_source_sha':existing.source.REVIEWED_ISSUER_SHA,
            'namespace_uid':'ns','objects':objects,'public_sources':copy.deepcopy(current['public_sources'])}}
        provider=SimpleNamespace(load=lambda *args:copy.deepcopy(prior))
        predecessor_journal=SimpleNamespace(state={'steps':{'full-cohort-predecessor':{'status':'captured'}}})
        with patch.dict(sys.modules,{'full_cohort_predecessor_phase':provider}), \
             patch.object(existing,'observe',return_value=current):
            result=existing.verify_reuse(self.fence,self.client,{},'ns',predecessor_journal,'/source')
            self.assertEqual(result['scope'],'c2-p3-exact-predecessor-reuse')
            prior['p3_receipt']['objects']['secret-voice-p3-principal-replay']['uid']='foreign'
            with self.assertRaises(existing.ExistingError):
                existing.verify_reuse(self.fence,self.client,{},'ns',predecessor_journal,'/source')

    def test_change_during_second_predecessor_load_holds(self):
        current=existing.observe(self.fence,self.client,{},'ns','/source')
        objects={v['kind'].lower()+'-'+name:{'name':name,**v} for name,v in current['objects'].items()}
        prior={'previous_operation_id':'media-c1-'+'b'*32,'p3_receipt':{
            'operation_id':'media-c1-'+'b'*32,'issuer_source_sha':existing.source.REVIEWED_ISSUER_SHA,
            'namespace_uid':'ns','objects':objects,'public_sources':copy.deepcopy(current['public_sources'])}}
        calls=0
        def load(*args):
            nonlocal calls
            calls+=1
            if calls==2: self.items['Secret','voice-p3-principal-replay']['metadata']['resourceVersion']='8'
            return copy.deepcopy(prior)
        journal=SimpleNamespace(state={'steps':{'full-cohort-predecessor':{'status':'captured'}}})
        with patch.dict(sys.modules,{'full_cohort_predecessor_phase':SimpleNamespace(load=load)}), \
             patch.object(existing,'observe',return_value=current):
            with self.assertRaises(existing.ExistingError):
                existing.verify_reuse(self.fence,self.client,{},'ns',journal,'/source')
        self.assertEqual(calls,2)

    def test_last_guard_cannot_hide_local_or_ledger_drift(self):
        current=existing.observe(self.fence,self.client,{},'ns','/source')
        objects={v['kind'].lower()+'-'+name:{'name':name,**v} for name,v in current['objects'].items()}
        prior={'previous_operation_id':'media-c1-'+'b'*32,'p3_receipt':{
            'operation_id':'media-c1-'+'b'*32,'issuer_source_sha':existing.source.REVIEWED_ISSUER_SHA,
            'namespace_uid':'ns','objects':objects,'public_sources':copy.deepcopy(current['public_sources'])}}
        journal=SimpleNamespace(state={'steps':{'full-cohort-predecessor':{'status':'captured'}}})
        provider=SimpleNamespace(load=lambda *args:copy.deepcopy(prior)); calls=[]
        with patch.dict(sys.modules,{'full_cohort_predecessor_phase':provider}), \
             patch.object(existing,'observe',return_value=current), \
             patch.object(existing.media_operation_guard,'verify',side_effect=lambda *args:calls.append(1)):
            existing.verify_reuse(self.fence,self.client,{},'ns',journal,'/source')
        total=len(calls)
        for fault in ('local','ledger'):
            with self.subTest(fault=fault):
                calls=[]
                def guard(*args):
                    calls.append(1)
                    if len(calls)==total:
                        if fault=='local': journal.state['steps']['full-cohort-predecessor']['status']='foreign'
                        else: self.row['foreign']='changed'
                with patch.dict(sys.modules,{'full_cohort_predecessor_phase':provider}), \
                     patch.object(existing,'observe',return_value=current), \
                     patch.object(existing.media_operation_guard,'verify',side_effect=guard):
                    with self.assertRaises(existing.ExistingError):
                        existing.verify_reuse(self.fence,self.client,{},'ns',journal,'/source')
                journal.state['steps']['full-cohort-predecessor']['status']='captured'; self.row.pop('foreign',None)


if __name__ == '__main__': unittest.main()
