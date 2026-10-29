import base64
import copy
import sys
from types import SimpleNamespace
import unittest
from unittest.mock import patch

import realtime_config_transition_authority as authority
import realtime_p3_adoption as adoption
import space_p3_issuer_create as issuer


class AuthorityTest(unittest.TestCase):
    def setUp(self):
        self.cut=SimpleNamespace(fence=SimpleNamespace(context={'operation_id':'media-c1-'+'a'*32,'commit':'b'*40},
            ledger=SimpleNamespace(read=lambda:({'status':'active'},'1'))),lock={},namespace_uid='ns',_guard=lambda:None)
        self.values={}
        self.p3={'objects':{},'public_sources':{'voice-space-media-server-tls':{'uid':'space-cert','resourceVersion':'7'}},
                 'migration':{'job':{'state':'done'}}}
        for name,keys in adoption.ASSETS.items():
            if name in ('voice-app-secrets','voice-app-config'): continue
            kind='ConfigMap' if name=='voice-p3-principal-public' else 'Secret'
            data={k:('public' if kind=='ConfigMap' else base64.b64encode(('public-'+k).encode()).decode()) for k in keys}
            self.values[kind,name]={'kind':kind,'apiVersion':'v1','metadata':{'name':name,'namespace':'voice-dev',
                'uid':'uid-'+name,'resourceVersion':'1'},'data':data,**({'immutable':True} if kind=='ConfigMap' else {'type':'Opaque'})}
            self.p3['objects'][name]={'kind':kind,'uid':'uid-'+name,'resourceVersion':'1'}
        credential={'name':'voice-realtime-db-'+'a'*32,'uid':'db','resourceVersion':'2'}
        self.values['Secret',credential['name']]={'apiVersion':'v1','kind':'Secret','metadata':{'name':credential['name'],'namespace':'voice-dev','uid':'db','resourceVersion':'2'},'type':'Opaque',
            'immutable':True,'data':{'DATABASE_URL':'private-sentinel'}}
        self.migration=SimpleNamespace(intent={'realtime_migration':{'credential':credential}})
        self.config={'kind':'ConfigMap','apiVersion':'v1','metadata':{'name':'voice-app-config','namespace':'voice-dev','uid':'cm','resourceVersion':'3'},
                     'data':{'REDIS_ADDR':'voice-redis:6379'}}
        self.cut.client=SimpleNamespace(call=lambda verb,kind,name,*args:copy.deepcopy(self.config if kind=='configmap' else self.values['Secret',name]))
        self.session=object(); self.phase=object()
        self.publisher=SimpleNamespace(fence=SimpleNamespace(context=self.cut.fence.context,ledger=self.cut.fence.ledger),lock={},namespace_uid='ns')
        self.published={'operation_id':self.cut.fence.context['operation_id'],'scope':'create-only'}
        self.space={'observation':{'jwks_bytes':base64.b64encode(b'public-space').decode()}}
        self.service={'name':'voice-realtime','namespace':'voice-dev','uid':'svc','resourceVersion':'4','type':'ClusterIP',
                      'clusterIP':'10.0.0.1','selector':{'app':'voice-realtime'},'ports':[{'name':'realtime-lifecycle','port':9091}]}
        pubmod=SimpleNamespace(verify_publisher=lambda p:copy.deepcopy(self.published))
        chatmod=SimpleNamespace(verify_chat_service=lambda *a:{'receipt':{'uid':'chat'}})
        self.patches=[patch.dict(sys.modules,{'realtime_jwks_publisher':pubmod,'realtime_config_transition_service':chatmod}),
            patch.object(authority.inputs,'observe',side_effect=lambda *a:copy.deepcopy(self.p3)),
            patch.object(authority,'_space',side_effect=lambda *a:(copy.deepcopy(self.space),b'public-space')),
            patch.object(authority.redis,'observe',return_value={'scope':'sampled'}),
            patch.object(authority,'_realtime_service',side_effect=lambda *a:copy.deepcopy(self.service)),
            patch.object(authority.transport,'get',side_effect=lambda k,n:copy.deepcopy(self.values[k,n]) if n!=credential['name'] else (_ for _ in ()).throw(AssertionError('credential must not use P3 transport'))),
            patch.object(authority.transport,'public_certificate',return_value=b'space-cert\n7\ncHVibGlj')]
        for p in self.patches: p.start(); self.addCleanup(p.stop)

    def test_derives_renderer_input_without_private_values(self):
        result=authority.collect(self.cut,object(),self.session,self.migration,self.phase,self.publisher,'/source','token')
        self.assertEqual(result['schema']['database'],'realtime_db')
        self.assertEqual(set(result['public_hashes']),adoption.PUBLIC_HASHES)
        self.assertNotIn('private-sentinel',str(result))

    def test_config_change_during_second_read_holds(self):
        calls=0
        def read(verb,kind,name,*args):
            nonlocal calls
            if kind=='secret': return copy.deepcopy(self.values['Secret',name])
            calls+=1; value=copy.deepcopy(self.config)
            if calls>1: value['metadata']['resourceVersion']='4'
            return value
        self.cut.client.call=read
        with self.assertRaises(authority.AuthorityError):
            authority.collect(self.cut,object(),self.session,self.migration,self.phase,self.publisher,'/source','token')

    def test_issuer_proof_must_match_p3_certificate_and_ca(self):
        ca={'uid':'ca','resourceVersion':'8','der_sha256':'a'*64}
        self.p3['public_sources']['voice-media-ca']=copy.deepcopy(ca)
        self.p3['public_sources']['voice-space-media-server-tls']['der_sha256']='b'*64
        proof={'schema':'voice-dev-space-p3-issuer-v1',
               'tls':{'uid':'space-cert','resourceVersion':'7','certificate_sha256':'b'*64},
               'media_ca':copy.deepcopy(ca)}
        with patch.object(authority,'_space',return_value=(proof,b'public-space')):
            authority.collect(self.cut,object(),self.session,self.migration,self.phase,self.publisher,'/source','token')
            proof['tls']['certificate_sha256']='c'*64
            with self.assertRaises(authority.AuthorityError):
                authority.collect(self.cut,object(),self.session,self.migration,self.phase,self.publisher,'/source','token')

    def test_issuer_dispatch_checks_fence_and_detached_jwks(self):
        phase=issuer.Session.__new__(issuer.Session)
        phase.fence=self.cut.fence; phase.lock=self.cut.lock; phase.namespace_uid=self.cut.namespace_uid
        proof={'schema':'voice-dev-space-p3-issuer-v1','operation_id':self.cut.fence.context['operation_id'],
               'source_sha':self.cut.fence.context['commit'],'namespace_uid':'ns',
               'scope':'space-two-secret-create-only','public_jwks':{'keys':[]}}
        self.patches[2].stop()
        with patch.object(issuer,'verify_completed',return_value=proof):
            self.assertEqual(authority._space(self.cut,phase,'token'),(proof,b'{"keys":[]}'))
            phase.namespace_uid='foreign'
            with self.assertRaises(ValueError): authority._space(self.cut,phase,'token')

    def test_fixed9_space_requires_current_tls_and_verified_binding(self):
        import realtime_p3_create_only_crypto as crypto
        import space_creator_runtime_profile_target_trust as target
        phase=SimpleNamespace(proof={'objects':{'voice-space-media-server-tls':{'uid':'space-cert','resourceVersion':'7'}},
            'public_jwks':{'voice-space-principal-signing':{'keys':[]}},'ca':{'uid':'ca','resourceVersion':'8','der_sha256':'a'*64}})
        binding=authority.BoundFixed9SpaceAuthority(self.cut,object(),object(),phase,object(),'token')
        self.assertNotIn('token',repr(binding))
        self.patches[2].stop()
        with patch.object(target,'verify_fixed9_successor') as verify, patch.object(crypto,'der',return_value=b'der'):
            proof,jwks=authority._space(self.cut,binding,'token')
            self.assertEqual(proof['schema'],'voice-dev-space-delegated-v1'); self.assertEqual(jwks,b'{"keys":[]}')
            self.assertEqual(verify.call_count,2)
            with self.assertRaises(ValueError):authority._space(self.cut,binding,'wrong')
            with patch.object(authority.transport,'public_certificate',return_value=b'foreign\n7\ncHVibGlj'):
                with self.assertRaises(ValueError):authority._space(self.cut,binding,'token')

    def test_fixed9_binding_rejects_untyped_inputs(self):
        with self.assertRaises(authority.AuthorityError):
            authority.bind_delegated_space(self.cut,object(),object(),object(),object(),'token')

    def test_delegated_proof_must_match_p3_certificate_and_ca(self):
        ca={'uid':'ca','resourceVersion':'8','der_sha256':'a'*64}; self.p3['public_sources']['voice-media-ca']=ca
        self.p3['public_sources']['voice-space-media-server-tls']['der_sha256']='b'*64
        proof={'schema':'voice-dev-space-delegated-v1','tls':{'uid':'space-cert','resourceVersion':'7','certificate_sha256':'b'*64},'media_ca':ca}
        with patch.object(authority,'_space',return_value=(proof,b'public-space')):
            authority.collect(self.cut,object(),self.session,self.migration,self.phase,self.publisher,'/source','token')
            proof['tls']['certificate_sha256']='c'*64
            with self.assertRaises(authority.AuthorityError):authority.collect(self.cut,object(),self.session,self.migration,self.phase,self.publisher,'/source','token')


if __name__=='__main__': unittest.main()
