import copy
import unittest
from pathlib import Path

import staging_p3_prerequisites as p3


class StagingP3Contract(unittest.TestCase):
    def test_publisher_endpoint_requires_owned_single_ready_pod_and_exact_image(self):
        d={'metadata':{'uid':'dep','generation':2},'status':{'observedGeneration':2,'readyReplicas':1,'availableReplicas':1}}
        rs={'metadata':{'uid':'rs','ownerReferences':[{'kind':'Deployment','uid':'dep','controller':True}]},'spec':{'replicas':1}}
        pod={'metadata':{'uid':'pod','labels':{'app':'voice-p3-principal-jwks'},
                         'ownerReferences':[{'kind':'ReplicaSet','uid':'rs','controller':True}]},
             'spec':{'nodeName':'node'},'status':{'phase':'Running','podIP':'10.2.3.4',
             'conditions':[{'type':'Ready','status':'True'}],
             'containerStatuses':[{'name':'jwks','ready':True,'state':{'running':{}},'imageID':'docker-pullable://nginx@sha256:'+'b'*64}]}}
        sl={'metadata':{'labels':{'kubernetes.io/service-name':'voice-p3-principal-jwks'}},
            'ports':[{'name':'https','port':8443,'protocol':'TCP'}],
            'endpoints':[{'addresses':['10.2.3.4'],'conditions':{'ready':True},'targetRef':{'kind':'Pod','uid':'pod','namespace':'voice-staging'}}]}
        node={'metadata':{'name':'node','uid':'node-uid'},'status':{'nodeInfo':{'operatingSystem':'linux','architecture':'amd64'},
                'conditions':[{'type':'Ready','status':'True'}]}}
        args=(d,[rs],[pod],[sl],node,'node-uid','sha256:'+'b'*64)
        self.assertEqual(p3.publisher_endpoint(*args),'10.2.3.4')
        for index in (1,2,3):
            bad=list(copy.deepcopy(args)); bad[index].append(copy.deepcopy(bad[index][0]))
            with self.subTest(index=index),self.assertRaises(p3.PrerequisiteError): p3.publisher_endpoint(*bad)
        bad=list(copy.deepcopy(args)); bad[2][0]['status']['containerStatuses'][0]['imageID']='sha256:'+'c'*64
        with self.assertRaises(p3.PrerequisiteError): p3.publisher_endpoint(*bad)

    def test_service_server_allocation_has_only_exact_bounded_difference(self):
        intended={'apiVersion':'v1','kind':'Service','metadata':{'name':'voice-p3-principal-jwks','namespace':'voice-staging'},
                  'spec':{'type':'ClusterIP','selector':{'app':'voice-p3-principal-jwks'},'ports':[{'port':8443}],
                          'ipFamilies':['IPv4'],'ipFamilyPolicy':'SingleStack'}}
        actual=copy.deepcopy(intended); actual['metadata'].update(uid='owned',resourceVersion='17')
        actual['spec'].update(clusterIP='10.20.0.7',clusterIPs=['10.20.0.7'])
        self.assertEqual(p3.service_allocation(intended,actual),['10.20.0.7'])
        for mutation in ('different_primary','duplicate','wrong_family','extra_spec','wrong_uid','owner'):
            bad=copy.deepcopy(actual)
            if mutation=='different_primary': bad['spec']['clusterIP']='10.20.0.8'
            if mutation=='duplicate': bad['spec']['clusterIPs'].append('10.20.0.7')
            if mutation=='wrong_family': bad['spec']['ipFamilies']=['IPv6']
            if mutation=='extra_spec': bad['spec']['selector']['other']='unexpected'
            if mutation=='wrong_uid': del bad['metadata']['uid']
            if mutation=='owner': bad['metadata']['ownerReferences']=[{'uid':'foreign'}]
            with self.subTest(mutation=mutation),self.assertRaises(p3.PrerequisiteError): p3.service_allocation(intended,bad)

    def test_private_plan_requires_closed_exact_inventory_and_provenance_shapes(self):
        ids=p3.inventory_intent(); digest='a'*64
        objects=[{'id':i,'kind':i.split('/')[0],'name':i.split('/')[1],
                  'namespace':'voice-staging','desired_sha256':digest,'payload_sha256':digest} for i in ids]
        plan={'schema':'voice-staging-p3-prerequisites-plan-v1','operation':'operation',
              'namespace_uid':'namespace-uid','source_capture_sha256':digest,'source_contract_sha256':digest,
              'issuer_authority_sha256':digest,'objects':objects,'preserved':{},
              'effects':[{**x,'before':{'absent':True,'list_rv':'9'}} for x in objects],
              'auxiliary_image':{'workload':'voice-p3-principal-jwks','container':'jwks',
               'source_ref':'nginx:stable-alpine','image_ref':'docker.io/library/nginx@sha256:'+digest,
               'platform':{'os':'linux','architecture':'amd64','variant':None},
               'manifest_sha256':digest,'config_sha256':digest,'index_sha256':None,'source_authority_sha256':digest}}
        self.assertEqual(p3.validate_plan_shape(plan),plan)
        preserved=copy.deepcopy(plan); preserved['effects']=[]
        for obj in objects:
            observed={'kind':obj['kind'],'name':obj['name'],'namespace':obj['namespace'],
                      'uid':'owned','resourceVersion':'10','semantic_sha256':digest}
            if obj['kind']=='Deployment': observed['generation']=1
            preserved['preserved'][obj['id']]={'authority':{'operation':'prior','receipt_sha256':digest},'observed':observed}
        self.assertEqual(p3.validate_plan_shape(preserved),preserved)
        broken=copy.deepcopy(preserved); del broken['preserved'][ids[0]]
        with self.assertRaises(p3.PrerequisiteError): p3.validate_plan_shape(broken)
        for mutate in ('unknown','duplicate','overlap','extra_effect','bad_image','bad_before'):
            bad=copy.deepcopy(plan)
            if mutate=='unknown': bad['extra']=True
            if mutate=='duplicate': bad['objects'][1]=bad['objects'][0]
            if mutate=='overlap': bad['preserved'][ids[0]]={}
            if mutate=='extra_effect': bad['effects'].append(bad['effects'][0])
            if mutate=='bad_image': bad['auxiliary_image']['image_ref']='nginx:stable-alpine'
            if mutate=='bad_before': bad['effects'][0]['before']['absent']=False
            with self.subTest(mutate=mutate),self.assertRaises(p3.PrerequisiteError): p3.validate_plan_shape(bad)

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
        wrong=copy.deepcopy(rows); wrong[1]['spec']['replicas']=2
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
        self.assertLess(intent.index('Service/voice-p3-principal-jwks'),intent.index('Deployment/voice-p3-principal-jwks'))
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
