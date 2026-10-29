import copy
import unittest
from pathlib import Path

import staging_p3_prerequisites as p3


class StagingP3Contract(unittest.TestCase):
    def test_publisher_uses_exact_source_shape_and_immutable_nginx(self):
        raw=(Path(__file__).resolve().parents[3]/'deploy/staging/services.yaml').read_text()
        section=raw.split('  default.conf: |\n',1)[1].split('\n---',1)[0]
        config='\n'.join(line[4:] for line in section.splitlines())+'\n'
        rows=[{'apiVersion':'v1','kind':'ConfigMap','metadata':{'name':'voice-p3-principal-publisher-config','namespace':'voice-staging'},
               'data':{'default.conf':config}},
              {'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-p3-principal-jwks','namespace':'voice-staging'},
               'spec':{'replicas':1,'selector':{'matchLabels':{'app':'voice-p3-principal-jwks'}},
                 'template':{'metadata':{'labels':{'app':'voice-p3-principal-jwks'}},'spec':{
                   'containers':[{'name':'jwks','image':'nginx:stable-alpine','ports':[{'containerPort':8443}],
                     'volumeMounts':[{'name':'public','mountPath':'/srv/jwks','readOnly':True},
                       {'name':'tls','mountPath':'/etc/voice/tls','readOnly':True},
                       {'name':'config','mountPath':'/etc/nginx/conf.d','readOnly':True}],
                     'readinessProbe':{'tcpSocket':{'port':8443},'periodSeconds':5}}],
                   'volumes':[{'name':'public','configMap':{'name':'voice-p3-principal-public','items':[{'key':'realtime.json','path':'realtime.json'}]}},
                     {'name':'tls','secret':{'secretName':'voice-p3-principal-jwks-tls','defaultMode':288,'items':[{'key':'tls.crt','path':'tls.crt'},{'key':'tls.key','path':'tls.key'}]}},
                     {'name':'config','configMap':{'name':'voice-p3-principal-publisher-config'}}]}}}},
              {'apiVersion':'v1','kind':'Service','metadata':{'name':'voice-p3-principal-jwks','namespace':'voice-staging'},
               'spec':{'selector':{'app':'voice-p3-principal-jwks'},'ports':[{'name':'https','port':8443}]}}]
        image='docker.io/library/nginx@sha256:'+'a'*64
        out=p3.publisher_objects(rows,image,'sha256:'+'b'*64)
        self.assertEqual(next(x for x in out if x['kind']=='Deployment')['spec']['template']['spec']['containers'][0]['image'],image)
        self.assertEqual(rows[1]['spec']['template']['spec']['containers'][0]['image'],'nginx:stable-alpine')
        for bad in ('nginx:stable-alpine','example.invalid/nginx@sha256:'+'a'*64):
            with self.assertRaises(p3.PrerequisiteError): p3.publisher_objects(rows,bad,'sha256:'+'b'*64)
        wrong=copy.deepcopy(rows); wrong[0]['data']['default.conf']=config.replace('root /srv/jwks;','root /etc/voice/tls;')
        with self.assertRaises(p3.PrerequisiteError): p3.publisher_objects(wrong,image,'sha256:'+'b'*64)
    def test_material_has_exact_staging_objects_and_keys(self):
        values={name:{key:b'value' for key in keys} for name,keys in p3.ASSET_KEYS.items()}
        objects=p3.material_objects(values)
        self.assertEqual(len(objects),7)
        self.assertEqual({x['metadata']['namespace'] for x in objects},{'voice-staging'})
        self.assertTrue(next(x for x in objects if x['kind']=='ConfigMap')['immutable'])
        self.assertEqual({x['metadata']['name'] for x in objects},set(p3.ASSET_KEYS))
        for bad in ({**values,'extra':{}}, {**values,'voice-p3-principal-replay':{}},
                    {**values,'voice-p3-principal-replay':{'password':b'x','other':b'y'}}):
            with self.assertRaises(p3.PrerequisiteError): p3.material_objects(bad)

    def test_c1_is_missing_only_and_c2_is_exact_receipt_revalidation(self):
        intent=p3.inventory_intent()
        self.assertEqual(len(intent),10)
        absent={item:None for item in intent}
        self.assertEqual(p3.classify_inventory('c1',absent,None),'create')
        current={item:{'uid':'uid-'+str(i),'resourceVersion':'1','content_sha256':f'{i+1:064x}'}
                 for i,item in enumerate(intent)}
        self.assertEqual(p3.classify_inventory('c2',current,copy.deepcopy(current)),'revalidate')
        fresh=copy.deepcopy(current); fresh[intent[-1]]['resourceVersion']='2'
        self.assertEqual(p3.classify_inventory('c2',fresh,current),'revalidate')
        changed=copy.deepcopy(current); changed[intent[0]]['uid']='foreign'
        for cycle,actual,receipt in [('c1',current,None),('c2',changed,current),('c2',absent,current)]:
            with self.subTest(cycle=cycle),self.assertRaises(p3.PrerequisiteError):
                p3.classify_inventory(cycle,actual,receipt)


if __name__=='__main__': unittest.main()
