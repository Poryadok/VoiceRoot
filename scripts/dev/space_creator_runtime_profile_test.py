import copy
import unittest

import space_creator_runtime_profile as profile


def deployment(service):
    pod={'containers':[{'name':service,'image':'image','env':[], 'volumeMounts':[]}], 'volumes':[]}
    if service == 'space':
        pod['containers'][0]['env']=[{'name':'SPACE_PRINCIPAL_SIGNING_KEYS_DIR','value':'/etc/voice/media/signing'},
                                    {'name':'SPACE_PRINCIPAL_ACTIVE_KID','valueFrom':{'secretKeyRef':{'name':'voice-space-principal-signing','key':'active-kid'}}}]
        pod['containers'][0]['volumeMounts']=[{'name':'media-server','mountPath':'/etc/voice/media/server','readOnly':True},
                                            {'name':'media-signing','mountPath':'/etc/voice/media/signing','readOnly':True}]
        pod['volumes']=[{'name':'media-server','secret':{'secretName':'voice-space-media-server-tls','defaultMode':288,
                        'items':[{'key':k,'path':k} for k in ('tls.crt','tls.key')]}},
                        {'name':'media-signing','secret':{'secretName':'voice-space-principal-signing','defaultMode':288,
                        'items':[{'key':k,'path':k} for k in ('current.pem','next.pem')]}}]
    return {'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-'+service,'namespace':'voice-dev','uid':'uid','resourceVersion':'1'},
            'spec':{'replicas':0,'template':{'spec':pod}}}


class RendererTest(unittest.TestCase):
    def test_space_gateway_targets_are_symmetric_and_preserve_source(self):
        space=deployment('space'); gateway=deployment('gateway'); original=copy.deepcopy(space)
        _, s=profile.render_space(space,[]); _, g=profile.render_gateway(gateway,[])
        se={e['name']:e for e in s['template']['spec']['containers'][0]['env']}
        ge={e['name']:e for e in g['template']['spec']['containers'][0]['env']}
        self.assertEqual(se['SPACE_GATEWAY_LIFECYCLE_GRPC_LISTEN']['value'],':9096')
        self.assertEqual(ge['GATEWAY_SPACE_LIFECYCLE_GRPC_ADDR']['value'],'voice-space:9096')
        self.assertEqual(se['SPACE_CREATOR_AUTH_GRPC_ADDR']['value'],'voice-auth:9092')
        self.assertEqual(se['ROLE_PRINCIPAL_GRPC_ADDR']['value'],'voice-role:9091')
        self.assertEqual(ge['GATEWAY_PRINCIPAL_SIGNING_KEYS_DIR']['value'],'/etc/voice/gateway-principal/keys')
        self.assertEqual(ge['GATEWAY_SESSION_EPOCH_STRICT']['value'],'true')
        self.assertNotIn('SPACE_LIFECYCLE_ENABLED',se)
        self.assertEqual(space,original)

    def test_conflicting_env_mount_or_port_fails_closed(self):
        for fault in ('env','mount','port','shared_config'):
            source=deployment('space'); pod=source['spec']['template']['spec']; container=pod['containers'][0]; keys=[]
            if fault=='env': container['env'].append({'name':'SPACE_CREATOR_AUTH_GRPC_ADDR','value':'other'})
            if fault=='mount': container['volumeMounts'].append({'name':'foreign','mountPath':'/etc/voice/creator'})
            if fault=='port': container['ports']=[{'containerPort':9096}]
            if fault=='shared_config': keys=['SPACE_GATEWAY_LIFECYCLE_GRPC_LISTEN']
            with self.subTest(fault=fault), self.assertRaises(ValueError): profile.render_space(source,keys)

    def test_gateway_partial_signer_is_rejected(self):
        source=deployment('gateway')
        source['spec']['template']['spec']['containers'][0]['env'].append({'name':'GATEWAY_PRINCIPAL_SIGNING_KEYS_DIR','value':'/somewhere'})
        with self.assertRaises(ValueError): profile.render_gateway(source,[])

    def test_already_exact_targets_are_idempotent(self):
        for name,render in (('space',profile.render_space),('gateway',profile.render_gateway)):
            source=deployment(name); _,target=render(source,[]); source['spec']=target
            _,again=render(source,[])
            self.assertEqual(again,target)

    def test_partial_existing_profile_is_rejected(self):
        for name,render in (('space',profile.render_space),('gateway',profile.render_gateway)):
            for fault in ('mount','environment','port'):
                if name=='gateway' and fault=='port': continue
                source=deployment(name); _,source['spec']=render(source,[])
                pod=source['spec']['template']['spec']; container=pod['containers'][0]
                if fault=='mount': container['volumeMounts']=[m for m in container['volumeMounts'] if m['name']!='creator-client']
                if fault=='environment': container['env']=[e for e in container['env'] if not e['name'].startswith('SPACE_CREATOR_AUTH_' if name=='space' else 'GATEWAY_SPACE_LIFECYCLE_')]
                if fault=='port': container['ports']=[]
                with self.subTest(name=name,fault=fault), self.assertRaises(ValueError): render(source,[])

    def test_existing_signer_mount_rejects_subpath(self):
        source=deployment('gateway'); _,source['spec']=profile.render_gateway(source,[])
        mounts=source['spec']['template']['spec']['containers'][0]['volumeMounts']
        next(m for m in mounts if m['name']=='gateway-signing')['subPath']='current.pem'
        with self.assertRaises(ValueError): profile.render_gateway(source,[])

    def test_complete_profile_requires_all_dependencies(self):
        for name,render,fault in (('space',profile.render_space,'role'),('space',profile.render_space,'fsgroup'),
                                  ('gateway',profile.render_gateway,'signer'),('gateway',profile.render_gateway,'strict'),
                                  ('gateway',profile.render_gateway,'fsgroup')):
            source=deployment(name); _,source['spec']=render(source,[])
            pod=source['spec']['template']['spec']; c=pod['containers'][0]
            if fault=='role': c['env']=[e for e in c['env'] if not e['name'].startswith(('ROLE_PRINCIPAL_','SPACE_ROLE_CLIENT_'))]
            if fault=='signer': c['env']=[e for e in c['env'] if not e['name'].startswith('GATEWAY_PRINCIPAL_')]
            if fault=='strict': c['env']=[e for e in c['env'] if e['name']!='GATEWAY_SESSION_EPOCH_STRICT']
            if fault=='fsgroup': del pod['securityContext']['fsGroup']
            with self.subTest(name=name,fault=fault), self.assertRaises(ValueError): render(source,[])

    def test_existing_material_mount_cannot_be_shadowed(self):
        for path in ('/','/etc/voice/media/server/tls.key','//etc/voice/media/server/tls.key'):
            source=deployment('space')
            source['spec']['template']['spec']['containers'][0]['volumeMounts'].append({'name':'foreign','mountPath':path})
            with self.subTest(path=path), self.assertRaises(ValueError): profile.render_space(source,[])


if __name__=='__main__': unittest.main()
