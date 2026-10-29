import copy
import json
import unittest

import space_creator_peer_target as peer
import space_creator_runtime_profile as creator
import space_creator_runtime_profile_test as fixtures


def target():
    source=fixtures.deployment('space')
    _,source['spec']=creator.render_space(source,[])
    source['spec']['template']['spec']['containers'][0]['env'].append(
        {'name':'S2S_JWKS_URLS_JSON','value':'{"role":"https://voice-role:8443/.well-known/jwks.json"}'})
    return source


class PeerTargetTest(unittest.TestCase):
    def test_observer_target_is_additive_idempotent_and_keeps_deletion_disabled(self):
        source=target(); original=copy.deepcopy(source)
        patch,spec=peer.render_space(source,[]); self.assertEqual(patch[3]['value'],spec)
        container=spec['template']['spec']['containers'][0]
        values={e['name']:e for e in container['env']}
        self.assertEqual(values['SPACE_ROLE_OVERRIDE_PRINCIPAL_GRPC_LISTEN']['value'],':9099')
        self.assertNotIn('SPACE_LIFECYCLE_ENABLED',values)
        self.assertIn({'name':'role-authority','containerPort':9099,'protocol':'TCP'},container['ports'])
        self.assertEqual(source,original);source['spec']=spec
        self.assertEqual(peer.render_space(source,[])[1],spec)

    def test_adds_role_and_preserves_other_issuers(self):
        for original in (None, '{"legacy":"https://existing.example/keys"}'):
            source=target(); entries=source['spec']['template']['spec']['containers'][0]['env']
            entry=next(e for e in entries if e['name']=='S2S_JWKS_URLS_JSON')
            if original is None: entries.remove(entry)
            else: entry['value']=original
            before=copy.deepcopy(source); _,spec=peer.render_space(source,[])
            result=json.loads(next(e['value'] for e in spec['template']['spec']['containers'][0]['env'] if e['name']=='S2S_JWKS_URLS_JSON'))
            self.assertEqual(result['role'],'https://voice-role:8443/.well-known/jwks.json')
            if original: self.assertEqual(result['legacy'],'https://existing.example/keys')
            self.assertEqual(source,before)

    def test_observer_rejects_unsafe_jwks_and_conflicts(self):
        for fault in ('duplicate','http','bad-port','control','escape','space','redis','port','env-only','shared','creator'):
            source=target(); container=source['spec']['template']['spec']['containers'][0]; keys=[]
            jwks=next(e for e in container['env'] if e['name']=='S2S_JWKS_URLS_JSON')
            if fault=='duplicate':jwks['value']='{"role":"https://voice-role:8443/.well-known/jwks.json","role":"https://foreign"}'
            if fault=='http':jwks['value']='{"role":"https://voice-role:8443/.well-known/jwks.json","other":"http://foreign"}'
            if fault=='bad-port':jwks['value']='{"role":"https://voice-role:8443/.well-known/jwks.json","other":"https://foreign:bogus/keys"}'
            if fault=='control':jwks['value']='{"role":"https://voice-role:8443/.well-known/jwks.json","other":"https://foreign/\\tkeys"}'
            if fault=='escape':jwks['value']='{"role":"https://voice-role:8443/.well-known/jwks.json","other":"https://foreign/%zz"}'
            if fault=='space':jwks['value']='{"role":"https://voice-role:8443/.well-known/jwks.json","other":"https:// foreign/keys"}'
            if fault=='redis':container['env'].append({'name':'SPACE_ROLE_OVERRIDE_PRINCIPAL_REPLAY_REDIS_PASSWORD','value':'x'})
            if fault=='port':container.setdefault('ports',[]).append({'name':'foreign','containerPort':9099})
            if fault=='env-only':container['env'].append({'name':'SPACE_ROLE_OVERRIDE_PRINCIPAL_GRPC_LISTEN','value':':9099'})
            if fault=='shared':keys=['SPACE_ROLE_OVERRIDE_PRINCIPAL_GRPC_LISTEN']
            if fault=='creator':container['env']=[e for e in container['env'] if not e['name'].startswith('SPACE_CREATOR_AUTH_')]
            with self.subTest(fault=fault),self.assertRaises(ValueError):peer.render_space(source,keys)

    def test_service_one_patch_and_collision_guard(self):
        source={'apiVersion':'v1','kind':'Service','metadata':{'name':'voice-space','namespace':'voice-dev','uid':'svc','resourceVersion':'4'},
                'spec':{'clusterIP':'10.0.0.1','selector':{'app':'voice-space'},'ports':[{'name':'http','port':8080}]}}
        patch,spec=peer.render_service(source,{'app':'voice-space'})
        self.assertEqual(patch[3]['value'],spec)
        self.assertEqual({p['port'] for p in spec['ports']},{8080,8443,9096,9099})
        for entry in ({'name':'role-authority','port':9999},{'name':'foreign','port':9099},
                      {'name':'foreign','port':9999,'targetPort':9099},
                      {'name':'foreign','port':9999,'targetPort':'role-authority'}):
            changed=copy.deepcopy(source);changed['spec']['ports'].append(entry)
            with self.subTest(entry=entry),self.assertRaises(ValueError):peer.render_service(changed,{'app':'voice-space'})
        source['spec']=spec;self.assertEqual(peer.render_service(source,{'app':'voice-space'})[1],spec)


if __name__=='__main__':unittest.main()
