import copy
import json
import unittest

import space_creator_peer_role_target as peer
import space_creator_runtime_profile_test as fixtures


def role():
    source=fixtures.deployment('role')
    source['spec']['template']['spec']['containers'][0]['env'].append({'name':'S2S_JWKS_URLS_JSON',
        'value':'{"space":"https://voice-space:8443/.well-known/jwks.json",'
                '"gameintegration":"https://voice-gameintegration:8443/internal/v1/principal/jwks.json",'
                '"voice":"https://voice-voice:8443/internal/v1/principal/jwks.json"}'})
    return source


class PeerRoleTargetTest(unittest.TestCase):
    def test_target_replaces_or_adds_space_issuer_preserving_existing(self):
        for old in (None,'https://old.example/keys'):
            source=role(); entry=source['spec']['template']['spec']['containers'][0]['env'][0]
            original=json.loads(entry['value'])
            if old is None: original.pop('space')
            else: original['space']=old
            entry['value']=json.dumps(original); before=copy.deepcopy(source)
            _,target=peer.render_role(source,[])
            result=json.loads(next(e['value'] for e in target['template']['spec']['containers'][0]['env'] if e['name']=='S2S_JWKS_URLS_JSON'))
            self.assertEqual(result['space'],'https://voice-space:8443/.well-known/jwks.json')
            self.assertEqual(result['gameintegration'],original['gameintegration'])
            self.assertEqual(result['voice'],original['voice']); self.assertEqual(source,before)

    def test_role_target_binds_protected_member_route_without_source_mutation(self):
        source=role();before=copy.deepcopy(source);patch,target=peer.render_role(source,[])
        self.assertEqual(patch[3]['value'],target);self.assertEqual(source,before)
        pod=target['template']['spec'];container=pod['containers'][0]
        values={entry['name']:entry for entry in container['env']}
        self.assertEqual(values['ROLE_SPACE_MEMBER_AUTHORITY_GRPC_ADDR']['value'],'voice-space:9099')
        self.assertEqual(values['ROLE_PRINCIPAL_GRPC_LISTEN']['value'],':9091')
        self.assertEqual(values['ROLE_PRINCIPAL_JWKS_HTTPS_LISTEN']['value'],':8443')
        self.assertIn('voice-role-principal-signing',{volume['secret']['secretName'] for volume in pod['volumes']})
        source['spec']=target;self.assertEqual(peer.render_role(source,[])[1],target)

    def test_role_target_rejects_incomplete_or_unsafe_peer_config(self):
        for fault in ('missing-issuer','extra-issuer','flag-missing-issuer','unknown-flag','duplicate-issuer','http-issuer','bad-port','escape','space','partial-client','password','mount','port','jwks-port'):
            source=role();container=source['spec']['template']['spec']['containers'][0]
            jwks=container['env'][0]
            if fault=='missing-issuer':jwks['value']='{"space":"https://voice-space:8443/.well-known/jwks.json"}'
            if fault=='extra-issuer':jwks['value']=jwks['value'][:-1]+',"other":"https://foreign/keys"}'
            if fault=='flag-missing-issuer':container['env'].append({'name':'ROLE_BOT_LIFECYCLE_ENABLED','value':'true'})
            if fault=='unknown-flag':container['env'].append({'name':'ROLE_AUTHORITY_SOURCE_ENABLED','valueFrom':{'configMapKeyRef':{'name':'foreign','key':'enabled'}}})
            if fault=='duplicate-issuer':jwks['value']='{"space":"https://voice-space:8443/.well-known/jwks.json","space":"https://foreign"}'
            if fault=='http-issuer':jwks['value']=jwks['value'].replace('https://voice-voice','http://voice-voice')
            if fault=='bad-port':jwks['value']=jwks['value'].replace('voice-voice:8443','voice-voice:bad')
            if fault=='escape':jwks['value']=jwks['value'].replace('voice-voice:8443/internal','voice-voice:8443/%zz')
            if fault=='space':jwks['value']=jwks['value'].replace('voice-voice:8443',' voice-voice:8443')
            if fault=='partial-client':container['env'].append({'name':'ROLE_SPACE_MEMBER_AUTHORITY_GRPC_ADDR','value':'voice-space:9099'})
            if fault=='password':container['env'].append({'name':'ROLE_PRINCIPAL_REPLAY_REDIS_PASSWORD','value':'x'})
            if fault=='mount':container['volumeMounts'].append({'name':'other','mountPath':'/etc/voice/media'})
            if fault=='port':source['spec']['template']['spec']['containers'].append({'name':'other','ports':[{'containerPort':9091}]})
            if fault=='jwks-port':source['spec']['template']['spec']['containers'].append({'name':'other','ports':[{'containerPort':8443}]})
            with self.subTest(fault=fault),self.assertRaises(ValueError):peer.render_role(source,[])

    def test_peer_services_require_exact_identity_selector_and_numeric_route(self):
        for name,port in (('role',9091),('role',8443),('user',9090),('social',9090),('auth',9092)):
            deployment=fixtures.deployment(name);deployment['spec']['template']['metadata']={'labels':{'app':'voice-'+name}}
            service={'apiVersion':'v1','kind':'Service','metadata':{'name':'voice-'+name,'namespace':'voice-dev',
                     'uid':'uid','resourceVersion':'4'},'spec':{'clusterIP':'10.0.0.1',
                     'selector':{'app':'voice-'+name},'ports':[{'name':'peer','port':port}]}}
            self.assertEqual(peer.verify_service(service,deployment,port)['port'],port)
            for fault in ('selector','target','namespace','duplicate'):
                broken=copy.deepcopy(service)
                if fault=='selector':broken['spec']['selector']['app']='foreign'
                if fault=='target':broken['spec']['ports'][0]['targetPort']=port+1
                if fault=='namespace':broken['metadata']['namespace']='voice-staging'
                if fault=='duplicate':broken['spec']['ports'].append({'name':'other','port':port})
                with self.subTest(name=name,fault=fault),self.assertRaises(ValueError):peer.verify_service(broken,deployment,port)

    def test_enabled_optional_issuers_require_matching_explicit_flags(self):
        source=role();entries=source['spec']['template']['spec']['containers'][0]['env']
        entries[0]['value']=entries[0]['value'][:-1]+',"bot":"https://bot/keys","gateway":"https://gateway/keys","federation":"https://federation/keys","chat":"https://chat/keys"}'
        for flag in ('ROLE_BOT_LIFECYCLE_ENABLED','ROLE_DELEGATED_USER_ENABLED','ROLE_AUTHORITY_SOURCE_ENABLED'):
            entries.append({'name':flag,'value':'true'})
        self.assertIsInstance(peer.render_role(source,[])[1],dict)

    def test_role_service_adds_jwks_once_and_refuses_port_aliases(self):
        deployment=role();deployment['spec']['template']['metadata']={'labels':{'app':'voice-role'}}
        service={'apiVersion':'v1','kind':'Service','metadata':{'name':'voice-role','namespace':'voice-dev',
                 'uid':'uid','resourceVersion':'4'},'spec':{'clusterIP':'10.0.0.1',
                 'selector':{'app':'voice-role'},'ports':[{'name':'principal-grpc','port':9091}]}}
        before=copy.deepcopy(service);patch,target=peer.render_role_service(service,deployment)
        self.assertEqual(service,before);self.assertEqual(patch[3]['value'],target)
        self.assertEqual({port['port'] for port in target['ports']},{8443,9091})
        for collision in ({'name':'principal-jwks','port':9999},{'name':'other','port':8443},
                          {'name':'other','port':9999,'targetPort':8443},
                          {'name':'other','port':9999,'targetPort':'principal-jwks'}):
            broken=copy.deepcopy(service);broken['spec']['ports'].append(collision)
            with self.subTest(collision=collision),self.assertRaises(ValueError):peer.render_role_service(broken,deployment)
        service['spec']=target;self.assertEqual(peer.render_role_service(service,deployment)[1],target)


if __name__=='__main__':unittest.main()
