import copy
import hashlib
import json
import re
import tempfile
import unittest
from unittest.mock import patch
from pathlib import Path

import staging_p3_prerequisites as p3


class StagingP3Contract(unittest.TestCase):
    def test_source_image_reloads_real_session_capture_and_refuses_record_drift(self):
        import compiler, guard, resource_claim, resource_claim_test, staging_prerequisites as parent
        rows=json.loads(Path(__file__).with_name('staging_p3_prerequisites_source_fixture.json').read_text())
        raw=(Path(__file__).resolve().parents[3]/'deploy/staging/services.yaml').read_bytes()
        context=resource_claim_test.context(); card=resource_claim_test.card(context)
        session=resource_claim.Session(context,'a'*12,'b'*64,lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal()); session.admit(start=True)
        with tempfile.TemporaryDirectory(dir='/root') as tmp:
            root=Path(tmp); source=root/'installed'/'sources'/session.operation
            path=source/'deploy'/'staging'/'services.yaml'; path.parent.mkdir(parents=True,mode=0o700)
            path.write_bytes(raw); path.chmod(0o600)
            approved={'source_sha':context['commit'],'source_files':{'deploy/staging/services.yaml':hashlib.sha256(raw).hexdigest(),
                       'scripts/deploy/staging_p3_auxiliary_image.py':'f'*64}}
            authority={'approved':approved,'execution':{},'source':str(source)}
            record={'schema':'voice-staging-p3-auxiliary-image-v1','operation':session.operation,'nonce':'a'*32,'context':context,
                    'helper_sha256':session.helper,'source_capture_sha256':'c'*64,'source_module_sha256':'f'*64,
                    'source_ref':'nginx:stable-alpine','repository':'docker.io/library/nginx','index_sha256':'d'*64,
                    'manifest_sha256':'e'*64,'config_sha256':'9'*64,'platform':{'os':'linux','architecture':'amd64','variant':None},
                    'node':{'name':'node','uid':'uid','os':'linux','architecture':'amd64'},'image_ref':'docker.io/library/nginx@sha256:'+'e'*64}
            aux={k:record[k] for k in ('source_ref','index_sha256','manifest_sha256','config_sha256','platform','image_ref')}
            aux.update(workload='voice-p3-principal-jwks',container='jwks',source_authority_sha256=resource_claim.digest(record))
            plan={'source_capture_sha256':'c'*64,'source_contract_sha256':resource_claim.digest(p3.source_contract(approved,raw)),
                  'auxiliary_image':aux}
            def decode(piece):
                text=piece.decode(); name=re.search(r'^  name: ([^\s]+)',text,re.M).group(1); kind=re.search(r'^kind: ([^\s]+)',text,re.M).group(1)
                return copy.deepcopy(next(row for row in rows if row['kind']==kind and row['metadata']['name']==name))
            with patch.object(guard,'ROOT',root),patch.object(parent,'load_authority',return_value=authority), \
                 patch.object(parent,'load_auxiliary_image',return_value=record,create=True),patch.object(compiler,'decode_yaml',side_effect=decode):
                built,verified=p3._source_and_image(session,plan)
                self.assertEqual(verified,record); self.assertEqual(len(built),3)
                altered=copy.deepcopy(record); altered['manifest_sha256']='0'*64
                with patch.object(parent,'load_auxiliary_image',return_value=altered),self.assertRaises(ValueError):
                    p3._source_and_image(session,plan)
                path.write_bytes(raw+b' ')
                with self.assertRaises(p3.PrerequisiteError): p3._source_and_image(session,plan)

    def test_source_consumers_check_actual_p3_paths_and_ports(self):
        rows=json.loads((Path(__file__).with_name('staging_p3_prerequisites_source_fixture.json')).read_text())
        rows=[r for r in rows if r['metadata']['name'] in ('voice-realtime','voice-chat','voice-space')]
        self.assertTrue(p3.source_consumers(rows))
        for kind,name in [('Deployment','voice-realtime'),('Deployment','voice-chat'),('Deployment','voice-space'),('Service','voice-realtime')]:
            bad=copy.deepcopy(rows); row=next(x for x in bad if x['kind']==kind and x['metadata']['name']==name)
            if kind=='Service': row['spec']['ports'][0]['port']=9999
            else:
                container=next(c for c in row['spec']['template']['spec']['containers'] if c['name']==name.removeprefix('voice-'))
                target={'voice-realtime':'REALTIME_PRINCIPAL_JWKS_FILE','voice-chat':'CHAT_REALTIME_PRINCIPAL_JWKS_URL',
                        'voice-space':'SPACE_PRINCIPAL_JWKS_TLS_CERT_FILE'}[name]
                next(x for x in container['env'] if x['name']==target)['value']='wrong'
            with self.subTest(name=name),self.assertRaises(p3.PrerequisiteError): p3.source_consumers(bad)
        bad=copy.deepcopy(rows); realtime=next(x for x in bad if x['kind']=='Deployment' and x['metadata']['name']=='voice-realtime')
        trust=next(v for v in realtime['spec']['template']['spec']['volumes'] if v['name']=='p3-trust')
        trust['secret']['items'][0]['key']='wrong'
        with self.assertRaises(p3.PrerequisiteError): p3.source_consumers(bad)

    def test_private_semantic_retains_payload_and_ownership_controls(self):
        obj={'apiVersion':'v1','kind':'Secret','metadata':{'name':'voice-realtime-principal-signing',
              'namespace':'voice-staging','uid':'first','resourceVersion':'1','creationTimestamp':'now',
              'annotations':{'voice.poryadok.io/prerequisites-operation':'operation'}},'type':'Opaque','data':{'current.pem':'YWJj'}}
        first=p3.private_semantic(obj)
        changed=copy.deepcopy(obj); changed['metadata']['uid']='second'; changed['metadata']['resourceVersion']='2'
        self.assertEqual(first,p3.private_semantic(changed))
        changed['data']['current.pem']='eHl6'; self.assertNotEqual(first,p3.private_semantic(changed))
        changed=copy.deepcopy(obj); changed['metadata']['finalizers']=['foreign']
        self.assertNotEqual(first,p3.private_semantic(changed))

    def test_source_contract_binds_exact_approved_source_and_file(self):
        root=Path(__file__).resolve().parents[3]; raw=(root/'deploy/staging/services.yaml').read_bytes()
        approved={'source_sha':'a'*40,'source_files':{'deploy/staging/services.yaml':hashlib.sha256(raw).hexdigest()}}
        contract=p3.source_contract(approved,raw)
        self.assertEqual(contract['source_sha'],'a'*40)
        with self.assertRaises(p3.PrerequisiteError): p3.source_contract(approved,raw+b' ')
        bad=copy.deepcopy(approved); bad['source_sha']='bad'
        with self.assertRaises(p3.PrerequisiteError): p3.source_contract(bad,raw)

    def test_private_material_payload_roundtrip_is_exact(self):
        values={name:{key:b'value' for key in keys} for name,keys in p3.ASSET_KEYS.items()}
        rows=p3.material_objects(values)
        for row in rows: row['metadata']['annotations']={'voice.poryadok.io/prerequisites-operation':'op'}
        encode=lambda row:json.dumps(row,sort_keys=True,separators=(',',':')).encode()
        payloads={row['kind']+'/'+row['metadata']['name']:encode(row) for row in rows}
        operations={key:'op' for key in payloads}
        self.assertEqual(p3.decode_material_payloads(payloads,operations),values)
        mixed=copy.deepcopy(rows); mixed[0]['metadata']['annotations']['voice.poryadok.io/prerequisites-operation']='prior'
        mixed_payloads={row['kind']+'/'+row['metadata']['name']:encode(row) for row in mixed}
        mixed_ops={**operations,mixed[0]['kind']+'/'+mixed[0]['metadata']['name']:'prior'}
        self.assertEqual(p3.decode_material_payloads(mixed_payloads,mixed_ops),values)
        with self.assertRaises(p3.PrerequisiteError): p3.decode_material_payloads(mixed_payloads,operations)
        bad=copy.deepcopy(payloads); key=next(iter(bad)); bad[key]+=b' '
        with self.assertRaises(p3.PrerequisiteError): p3.decode_material_payloads(bad,operations)
        changed=copy.deepcopy(rows[0]); changed['metadata']['annotations']['foreign']='yes'; bad=copy.deepcopy(payloads); bad[changed['kind']+'/'+changed['metadata']['name']]=encode(changed)
        with self.assertRaises(p3.PrerequisiteError): p3.decode_material_payloads(bad,operations)

    def test_operation_marker_only_on_exact_fixed_object_set(self):
        rows=[{'kind':key.split('/')[0],'metadata':{'name':key.split('/')[1],'namespace':'voice-staging'}}
              for key in reversed(p3.inventory_intent())]
        out=p3.mark_operation(rows,'bound-operation')
        self.assertEqual([row['kind']+'/'+row['metadata']['name'] for row in out],p3.inventory_intent())
        self.assertTrue(all(row['metadata']['annotations']=={'voice.poryadok.io/prerequisites-operation':'bound-operation'} for row in out))
        self.assertNotIn('annotations',rows[0]['metadata'])
        wrong=copy.deepcopy(rows); wrong[0]['metadata']['annotations']={'voice.poryadok.io/prerequisites-operation':'foreign'}
        with self.assertRaises(p3.PrerequisiteError): p3.mark_operation(wrong,'bound-operation')
        with self.assertRaises(p3.PrerequisiteError): p3.mark_operation(rows[:-1],'bound-operation')

    def test_publisher_endpoint_requires_owned_single_ready_pod_and_exact_image(self):
        d={'metadata':{'name':'voice-p3-principal-jwks','namespace':'voice-staging','uid':'dep','generation':2},'status':{'observedGeneration':2,'readyReplicas':1,'availableReplicas':1}}
        rs={'metadata':{'namespace':'voice-staging','uid':'rs','ownerReferences':[{'kind':'Deployment','uid':'dep','controller':True}]},'spec':{'replicas':1}}
        pod={'metadata':{'namespace':'voice-staging','uid':'pod','labels':{'app':'voice-p3-principal-jwks'},
                         'ownerReferences':[{'kind':'ReplicaSet','uid':'rs','controller':True}]},
             'spec':{'nodeName':'node'},'status':{'phase':'Running','podIP':'10.2.3.4',
             'conditions':[{'type':'Ready','status':'True'}],
             'containerStatuses':[{'name':'jwks','ready':True,'state':{'running':{}},'imageID':'docker-pullable://nginx@sha256:'+'b'*64}]}}
        sl={'metadata':{'namespace':'voice-staging','ownerReferences':[{'kind':'Service','uid':'svc','controller':True}],
                        'labels':{'kubernetes.io/service-name':'voice-p3-principal-jwks'}},
            'ports':[{'name':'https','port':8443,'protocol':'TCP'}],
            'endpoints':[{'addresses':['10.2.3.4'],'conditions':{'ready':True},'targetRef':{'kind':'Pod','uid':'pod','namespace':'voice-staging'}}]}
        node={'metadata':{'name':'node','uid':'node-uid'},'status':{'nodeInfo':{'operatingSystem':'linux','architecture':'amd64'},
                'conditions':[{'type':'Ready','status':'True'}]}}
        service={'metadata':{'name':'voice-p3-principal-jwks','namespace':'voice-staging','uid':'svc'},
                 'spec':{'selector':{'app':'voice-p3-principal-jwks'},'ports':[{'name':'https','port':8443,'protocol':'TCP'}]}}
        args=(d,[rs],[pod],[sl],node,'node-uid','sha256:'+'b'*64,service)
        self.assertEqual(p3.publisher_endpoint(*args),'10.2.3.4')
        for index in (1,2,3):
            bad=list(copy.deepcopy(args)); bad[index].append(copy.deepcopy(bad[index][0]))
            with self.subTest(index=index),self.assertRaises(p3.PrerequisiteError): p3.publisher_endpoint(*bad)
        bad=list(copy.deepcopy(args)); bad[2][0]['status']['containerStatuses'][0]['imageID']='sha256:'+'c'*64
        with self.assertRaises(p3.PrerequisiteError): p3.publisher_endpoint(*bad)
        bad=list(copy.deepcopy(args)); bad[2][0]['metadata']['namespace']='voice-dev'
        with self.assertRaises(p3.PrerequisiteError): p3.publisher_endpoint(*bad)
        bad=list(copy.deepcopy(args)); del bad[2][0]['spec']['nodeName']; del bad[4]['metadata']['name']
        with self.assertRaises(p3.PrerequisiteError): p3.publisher_endpoint(*bad)
        for index in (3,4):
            bad=list(copy.deepcopy(args)); row=bad[index][0] if index==3 else bad[index]; row['metadata']['deletionTimestamp']='now'
            with self.subTest(index=index),self.assertRaises(p3.PrerequisiteError): p3.publisher_endpoint(*bad)
        bad=list(copy.deepcopy(args)); bad[3][0]['endpoints'][0]['conditions']['terminating']=True
        with self.assertRaises(p3.PrerequisiteError): p3.publisher_endpoint(*bad)
        bad=list(copy.deepcopy(args)); bad[3][0]['metadata']['ownerReferences'][0]['uid']='foreign'
        with self.assertRaises(p3.PrerequisiteError): p3.publisher_endpoint(*bad)
        bad=list(copy.deepcopy(args)); bad[7]['spec']['ports'][0]['targetPort']=9999
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
              'issuer_authority_sha256':digest,'issuer_input_sha256':digest,'objects':objects,'preserved':{},
              'effects':[{**x,'before':{'absent':True,'list_rv':'9'}} for x in objects],
              'auxiliary_image':{'workload':'voice-p3-principal-jwks','container':'jwks',
               'source_ref':'nginx:stable-alpine','image_ref':'docker.io/library/nginx@sha256:'+digest,
               'platform':{'os':'linux','architecture':'amd64','variant':None},
               'manifest_sha256':digest,'config_sha256':digest,'index_sha256':None,'source_authority_sha256':digest}}
        self.assertEqual(p3.validate_plan_shape(plan),plan)
        indexed=copy.deepcopy(plan); indexed['auxiliary_image']['index_sha256']='b'*64
        self.assertEqual(p3.validate_plan_shape(indexed),indexed)
        import staging_prerequisites as parent
        import resource_claim
        import resource_claim_test
        context=resource_claim_test.context(); card=resource_claim_test.card(context)
        session=resource_claim.Session(context,'a'*12,'b'*64,lambda:copy.deepcopy(card),resource_claim_test.MemoryJournal())
        session.admit(start=True); plan['operation']=session.operation
        events=[]
        def pending(s,slot,ident): events.append(('pending',slot,ident))
        def submit(s,slot,ident): events.append(('submit',slot,ident)); return {'fixture':ident}
        def done(s,slot,ident,observation): events.append(('done',slot,ident)); self.assertEqual(observation,{'fixture':ident})
        with patch.object(parent,'require_child',return_value=plan),patch.object(parent,'record_pending',side_effect=pending), \
             patch.object(parent,'submit_effect',side_effect=submit,create=True),patch.object(parent,'record_done',side_effect=done):
            p3.execute(session)
        self.assertEqual(events,[(action,'p3',ident) for ident in ids for action in ('pending','submit','done')])
        import stage_runtime
        payload={'apiVersion':'v1','kind':'Secret','metadata':{'name':ids[0].split('/')[1],'namespace':'voice-staging',
                 'annotations':{'voice.poryadok.io/prerequisites-operation':session.operation}},'type':'Opaque','data':{'current.pem':'eA==','next.pem':'eQ=='}}
        raw=json.dumps(payload,sort_keys=True,separators=(',',':')).encode()
        plan['objects'][0]['payload_sha256']=hashlib.sha256(raw).hexdigest()
        plan['objects'][0]['desired_sha256']=resource_claim.digest(p3.private_semantic(payload))
        plan['effects'][0].update(plan['objects'][0]); current=copy.deepcopy(payload)
        current['metadata'].update(uid='owned',resourceVersion='17')
        observation={'kind':'Secret','name':payload['metadata']['name'],'namespace':'voice-staging','uid':'owned',
                     'resourceVersion':'17','semantic_sha256':resource_claim.digest(p3.private_semantic(current))}
        with patch.object(parent,'require_child',return_value=plan),patch.object(parent,'load_payload',return_value=raw), \
             patch.object(stage_runtime.Kube,'get',return_value=current):
            self.assertEqual(p3.verify_effect(session,ids[0],observation),observation)
            wrong=copy.deepcopy(current); wrong['data']['current.pem']='eg=='
            with patch.object(stage_runtime.Kube,'get',return_value=wrong),self.assertRaises(p3.PrerequisiteError):
                p3.verify_effect(session,ids[0],observation)
            wrong_payload=copy.deepcopy(payload); wrong_payload['metadata']['annotations']['voice.poryadok.io/prerequisites-operation']='foreign'
            wrong_raw=json.dumps(wrong_payload,sort_keys=True,separators=(',',':')).encode()
            wrong_plan=copy.deepcopy(plan); wrong_plan['objects'][0]['payload_sha256']=hashlib.sha256(wrong_raw).hexdigest()
            wrong_plan['objects'][0]['desired_sha256']=resource_claim.digest(p3.private_semantic(wrong_payload))
            wrong_plan['effects'][0].update(wrong_plan['objects'][0])
            with patch.object(parent,'require_child',return_value=wrong_plan),patch.object(parent,'load_payload',return_value=wrong_raw),self.assertRaises(p3.PrerequisiteError):
                p3.verify_effect(session,ids[0],observation)
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
