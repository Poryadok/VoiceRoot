import copy
import unittest

import realtime_config_transition as transition
import realtime_p3_adoption_test as fixtures


class RenderTest(unittest.TestCase):
    def setUp(self):
        fixture = fixtures.PlanTest('test_preserves_original_and_fences_full_spec'); fixture.setUp()
        self.realtime = fixture.original; self.realtime['spec']['replicas'] = 0
        self.receipts = fixture.receipts
        self.op = 'media-c1-' + 'a'*32
        self.receipts['assets'].pop('voice-app-secrets')
        self.receipts['assets']['voice-realtime-db-'+'a'*32] = {
            'uid':'db-uid','resourceVersion':'1','namespace':'voice-dev','kind':'Secret',
            'keys':['DATABASE_URL'],'immutable':True}
        self.chat = copy.deepcopy(self.realtime)
        self.chat['metadata']['name'] = 'voice-chat'
        self.chat['spec']['template']['metadata']['labels']['app'] = 'voice-chat'
        self.chat['spec']['template']['spec']['containers'][0]['name'] = 'chat'

    def test_realtime_uses_existing_planner_and_preserves_cut_and_images(self):
        source = copy.deepcopy(self.realtime)
        patch,target = transition.render_realtime(source,self.receipts,self.op)
        self.assertEqual(source,self.realtime); self.assertEqual(target['replicas'],0)
        self.assertEqual(target['template']['spec']['containers'][0]['image'],'old')
        self.assertEqual(target['template']['spec']['containers'][1]['image'],'other')
        self.assertEqual(patch[2]['value'],source['spec']); self.assertEqual(patch[3]['value'],target)
        env = target['template']['spec']['containers'][0]['env']
        self.assertEqual(next(x for x in env if x['name']=='DATABASE_URL')['valueFrom']['secretKeyRef']['name'],
                         'voice-realtime-db-'+'a'*32)

    def test_chat_fixed_contract_preserves_existing_and_rejects_collision(self):
        source = copy.deepcopy(self.chat)
        patch,target = transition.render_chat(source,[])
        self.assertEqual(source,self.chat); self.assertEqual(target['replicas'],0)
        self.assertEqual(target['template']['spec']['containers'][0]['image'],'old')
        env = target['template']['spec']['containers'][0]['env']
        self.assertEqual(next(x for x in env if x['name']=='CHAT_REALTIME_PRINCIPAL_JWKS_URL')['value'],
                         'https://voice-p3-principal-jwks.voice-dev.svc:8443/realtime.json')
        self.assertEqual(patch[2]['value'],source['spec'])
        with self.assertRaises(ValueError): transition.render_chat(source,['CHAT_REALTIME_LIFECYCLE_GRPC_LISTEN'])
        collision = copy.deepcopy(source)
        collision['spec']['template']['spec']['containers'][0]['env'].append({'name':'CHAT_REALTIME_LIFECYCLE_GRPC_LISTEN','value':':1'})
        with self.assertRaises(ValueError): transition.render_chat(collision,[])

    def test_unknown_templates_and_nondrained_rejected(self):
        for name in ('realtime','chat'):
            source = copy.deepcopy(getattr(self,name)); source['spec']['replicas'] = 1
            with self.assertRaises(ValueError):
                (transition.render_realtime(source,self.receipts,self.op) if name == 'realtime'
                 else transition.render_chat(source,[]))

    def test_c2_exact_existing_configuration_and_image_change(self):
        for service in ('chat','realtime'):
            with self.subTest(service=service):
                previous=copy.deepcopy(getattr(self,service))
                _,target=(transition.render_realtime(previous,self.receipts,self.op) if service=='realtime'
                          else transition.render_chat(previous,[]))
                current=copy.deepcopy(previous); current['spec']=target
                current['spec']['template']['spec']['containers'][0]['image']='new-admitted-image'
                self.assertEqual(transition.verify_existing(current,previous,self.receipts,self.op,service,[]),current['spec'])
                current['spec']['template']['spec']['containers'][0]['env'].append({'name':'UNPLANNED','value':'x'})
                with self.assertRaises(ValueError):
                    transition.verify_existing(current,previous,self.receipts,self.op,service,[])

    def test_chat_service_adds_fixed_port_and_rejects_collision(self):
        source={'apiVersion':'v1','kind':'Service','metadata':{'name':'voice-chat','namespace':'voice-dev',
            'uid':'svc','resourceVersion':'4'},'spec':{'clusterIP':'10.0.0.1','selector':{'app':'voice-chat'},
            'ports':[{'name':'http','port':8080}]}}
        before=copy.deepcopy(source); patch,target=transition.render_chat_service(source,{'app':'voice-chat'})
        self.assertEqual(source,before); self.assertEqual(patch[2]['value'],before['spec'])
        self.assertEqual(target['ports'][-1],{'name':'realtime-lifecycle','port':9099,'targetPort':9099,'protocol':'TCP'})
        source['spec']['ports'].append({'name':'foreign','port':9099})
        with self.assertRaises(ValueError): transition.render_chat_service(source,{'app':'voice-chat'})

    def test_space_only_adds_signer_and_https_configuration(self):
        source=copy.deepcopy(self.chat); source['metadata']['name']='voice-space'
        source['spec']['template']['spec']['containers'][0]['name']='space'
        before=copy.deepcopy(source); patch,target=transition.render_space(source,[])
        self.assertEqual(source,before); self.assertEqual(target['replicas'],0)
        env=target['template']['spec']['containers'][0]['env']
        self.assertEqual({e['name'] for e in env if e['name'].startswith('SPACE_')},
            {'SPACE_PRINCIPAL_SIGNING_KEYS_DIR','SPACE_PRINCIPAL_ACTIVE_KID','SPACE_PRINCIPAL_JWKS_TLS_KEY_FILE',
             'SPACE_PRINCIPAL_JWKS_TLS_CERT_FILE','SPACE_PRINCIPAL_JWKS_HTTPS_LISTEN'})
        self.assertEqual(target['template']['spec']['containers'][0]['image'],'old')
        self.assertEqual(patch[2]['value'],before['spec'])
        with self.assertRaises(ValueError): transition.render_space(source,['SPACE_PRINCIPAL_ACTIVE_KID'])


if __name__ == '__main__': unittest.main()
