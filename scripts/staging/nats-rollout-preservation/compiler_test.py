import copy
import tempfile
from pathlib import Path
import unittest
import hashlib
import json
import runpy
import sys
from unittest.mock import patch
from types import SimpleNamespace
import compiler
from controller import Blocked

IMAGE='example.invalid/gateway@sha256:'+'a'*64
def deployment():
    return {'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':'voice-gateway','namespace':'voice-staging'},
            'spec':{'replicas':1,'template':{'spec':{'containers':[{'name':'gateway','image':'IMAGE_PLACEHOLDER'}]}}}}

class CompilerTest(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.addCleanup(self.temp.cleanup)
        self.source=Path(self.temp.name);p=self.source/'deploy/nats/acl-intent.yaml';p.parent.mkdir(parents=True);p.write_text('intent')
    def compile(self,docs=None,mode='images-only'):
        return compiler.compile_target(self.source,docs or [deployment()],'example.invalid','b'*40,mode,['gateway'],
          {'voice-gateway/gateway':IMAGE},{'streams':[],'consumer_pairs':[]},{'voice-gateway'},['deploy/nats/acl-intent.yaml'])
    def test_images_only_frozen_immutable_target_and_explicit_empty_migrations(self):
        original=deployment();pack=self.compile([original])
        self.assertEqual(pack['migrations'],[])
        self.assertEqual(pack['manifests'][0]['spec']['template']['spec']['containers'][0]['image'],IMAGE)
        self.assertEqual(pack['target']['manifest_sha256'],compiler.digest(pack['manifests']))
        self.assertEqual(original['spec']['template']['spec']['containers'][0]['image'],'IMAGE_PLACEHOLDER')
    def test_reserved_nats_resources_and_unenrolled_target_rejected(self):
        for row in ({'kind':'ConfigMap','metadata':{'name':'voice-nats-generation','namespace':'voice-staging'}},
                    dict(deployment(),metadata={'name':'voice-game','namespace':'voice-staging'})):
            with self.subTest(kind=row['kind']):
                with self.assertRaises(Blocked):self.compile([row])
    def test_extra_image_mapping_is_rejected(self):
        with self.assertRaises(Blocked):
            compiler.compile_target(self.source,[deployment()],'example.invalid','b'*40,'images-only',[],
              {'voice-gateway/gateway':IMAGE,'voice-game/game':IMAGE},{},{'voice-gateway'},['deploy/nats/acl-intent.yaml'])
    def test_actual_cli_canonical_full_and_app_only_bind_pinned_plan_and_ingress(self):
        import source_plan_test as fixtures
        for mode in ('full','app-only'):
            with self.subTest(mode=mode):
                kube,p=fixtures.SourceTests().canonical_fixture(mode)
                gateway_ingress=next(obj for (kind,name),obj in kube.objects.items() if kind=='Ingress' and 'gateway' in name)
                gateway_ingress['spec']['rules'][0]['host']='previous.example.invalid'
                p.update(generation='r20260930a4',dataPVC='voice-nats-jsdata-d202610040049430b',changed_services=[])
                p['images']={}
                import runtime_stage
                targets={'voice-gateway',*('voice-'+n for n in runtime_stage.LEAVES),'voice-web','voice-admin','voice-developer-portal','voice-gameintegration','voice-p3-principal-jwks'}
                for (kind,name),obj in kube.objects.items():
                    if kind!='Deployment' or name not in targets:continue
                    for c in obj['spec']['template']['spec'].get('initContainers',[])+obj['spec']['template']['spec']['containers']:
                        p['images'][name+'/'+c['name']]='example.invalid/'+c['name']+'@sha256:'+'a'*64
                p['enrolled']=sorted({key.split('/')[0] for key in p['images']})
                p['images']['voice-p3-principal-jwks/jwks']='docker.io/library/nginx@sha256:'+'e'*64
                p['contract']={'scripts':[]}
                for part in ('realtime','notification','search','analytics-chat'):
                    path=fixtures.SOURCE/('deploy/templates/nats-'+part+'-bootstrap.yaml')
                    row=fixtures.decoder(path.read_bytes().split(b'---')[0])
                    p['contract']['scripts'].append({'part':part,'sha256':hashlib.sha256(row['data']['bootstrap.sh'].encode()).hexdigest()})
                folder=self.source/mode;folder.mkdir();params=folder/'parameters.json';params.write_text(json.dumps(p));output=folder/('target-'+p['tag']+'.json')
                def capture(args,body=b'',**kwargs):
                    if args[1]=='create':return json.dumps(fixtures.decoder(body)).encode()
                    if args[3:5]==['get','--raw']:return json.dumps(kube.run(args[3:])).encode()
                    return json.dumps(kube.run(args[3:],None if not body else json.loads(body))).encode() if args[3] in ('apply','exec') else json.dumps(kube.get(args[4],args[5])).encode()
                with patch('commands.capture',side_effect=capture),patch.object(sys,'argv',[compiler.__file__,str(fixtures.SOURCE),str(params),str(output)]):
                    runpy.run_path(compiler.__file__,run_name='__main__')
                pack=json.loads(output.read_bytes());self.assertEqual(len(pack['nonnats']['checks']),16)
                import guard,guard_test,datetime as dt
                authorization=guard_test.receipt()
                authorization.update(schema='nats-rollout-preservation-v1',target=copy.deepcopy(pack['target']),
                    backup={'archive_sha256':'1'*64,'manifest_sha256':'2'*64,'census_sha256':'3'*64,'off_node_verified':True},
                    expires_at=(dt.datetime.now(dt.timezone.utc)+dt.timedelta(hours=1)).isoformat())
                authorization['target']['input_template_hashes']=dict(pack['target']['template_hashes'])
                guard.validate(authorization,p['registry'],p['tag'],mode,p['changed_services'],fixtures.SOURCE)
                import resource_claim,resource_claim_test
                installation=resource_claim.production_installation()
                scoped=copy.deepcopy(authorization);scoped['schema']='nats-rollout-preservation-v2'
                scoped['installation_sha256']=installation.sha256;scoped['target']['installation_sha256']=installation.sha256
                c=resource_claim_test.context();c.update(commit=p['tag'],images=copy.deepcopy(scoped['target']['images']))
                scoped['resource_fence']={'schema':'voice-staging-resource-fence-v2','installation_sha256':installation.sha256,
                    'context':c,'operation':scoped['operation'],'helper_sha256':'7'*64,
                    'target_sha256':resource_claim.digest(scoped['target'])}
                guard.validate(scoped,p['registry'],p['tag'],mode,p['changed_services'],fixtures.SOURCE,installation=installation)
                owned=resource_claim.InstallationAuthority('owned-isolated-v1','11111111-1111-4111-8111-111111111111',
                    'r20260930a4','1'*64,'2'*64,'3'*64)
                owned_receipt=copy.deepcopy(scoped);owned_receipt['installation_sha256']=owned.sha256
                owned_receipt['target']['installation_sha256']=owned.sha256
                owned_receipt['resource_fence']['installation_sha256']=owned.sha256
                owned_receipt['resource_fence']['target_sha256']=resource_claim.digest(owned_receipt['target'])
                guard.validate(owned_receipt,p['registry'],p['tag'],mode,p['changed_services'],fixtures.SOURCE,installation=owned)
                with self.assertRaises(guard.Blocked):
                    guard.validate(owned_receipt,p['registry'],p['tag'],mode,p['changed_services'],fixtures.SOURCE)
                with self.assertRaises(guard.Blocked):
                    guard.validate(authorization,p['registry'],p['tag'],mode,p['changed_services'],fixtures.SOURCE,installation=owned)
                for mutation in ('foreign','legacy-fence','target'):
                    changed=copy.deepcopy(scoped)
                    if mutation=='foreign':changed['installation_sha256']='f'*64
                    elif mutation=='legacy-fence':changed['resource_fence']['schema']='voice-staging-resource-fence-v1'
                    else:changed['target']['installation_sha256']='f'*64
                    with self.subTest(mode=mode,mutation=mutation),self.assertRaises(guard.Blocked):
                        guard.validate(changed,p['registry'],p['tag'],mode,p['changed_services'],fixtures.SOURCE,installation=installation)
                if mode=='full':self.assertIn('docker/clickhouse/init/001_events.sql',pack['target']['source_hashes'])
                self.assertEqual(pack['target']['images'],p['images'])
                self.assertTrue(any(r['kind']=='Ingress' for r in pack['manifests']))
                byname={r['metadata']['name']:r for r in pack['manifests'] if r['kind']=='Deployment'}
                for action in pack['nonnats']['actions']:
                    row=action['manifest']
                    if row['kind']=='Deployment':self.assertEqual(row['spec']['template'],byname[row['metadata']['name']]['spec']['template'])
                # Next release can preserve already-pinned frontends. Their
                # approved pins remain producer inputs, not deploy actions.
                fronts={'voice-web','voice-admin','voice-developer-portal'}
                for name in fronts:
                    row=byname[name];live=kube.objects[('Deployment',name)]
                    live['spec']=copy.deepcopy(row['spec'])
                    for key in ('labels','annotations'):
                        if key in row['metadata']:live['metadata'][key]=copy.deepcopy(row['metadata'][key])
                replay=folder/'replay';replay.mkdir();second=replay/output.name
                with patch('commands.capture',side_effect=capture),patch.object(sys,'argv',[compiler.__file__,str(fixtures.SOURCE),str(params),str(second)]):runpy.run_path(compiler.__file__,run_name='__main__')
                repeated=json.loads(second.read_bytes())
                self.assertFalse(any(a['manifest']['kind']=='Deployment' and a['manifest']['metadata']['name'] in fronts for a in repeated['nonnats']['actions']))
                self.assertEqual(repeated['target']['images'],{key:value for key,value in p['images'].items() if key.split('/')[0] not in fronts})

    def test_actual_cli_produces_bound_images_only_target(self):
        # Run the actual __main__ path; only the external YAML decoder is a seam.
        for index,relative in enumerate(compiler.SOURCE_MANIFESTS):
            p=self.source/relative;p.parent.mkdir(parents=True,exist_ok=True)
            row=deployment() if index==0 else {'kind':'Service','metadata':{'name':'voice-test-'+str(index),'namespace':'voice-staging'}}
            p.write_text(json.dumps(row))
        script='echo compatible\n';contract={'scripts':[]}
        for part in ('realtime','notification','search','analytics-chat'):
            p=self.source/('deploy/templates/nats-'+part+'-bootstrap.yaml')
            p.write_text(json.dumps({'kind':'ConfigMap','metadata':{'name':'voice-nats-'+part+'-bootstrap'},'data':{'bootstrap.sh':script}}))
            contract['scripts'].append({'part':part,'sha256':hashlib.sha256(script.encode()).hexdigest()})
        parameters={'registry':'example.invalid','tag':'b'*40,'mode':'images-only','changed_services':['gateway'],
            'images':{'voice-gateway/gateway':IMAGE},'contract':contract,'enrolled':['voice-gateway'],
            'generation':'r20260930a4','dataPVC':'voice-nats-jsdata-d202610040049430b','s3_signing_endpoint':'https://storage.example.invalid'}
        params=self.source/'parameters.json';params.write_text(json.dumps(parameters))
        output=self.source/('target-'+'b'*40+'.json')
        with patch('commands.capture',side_effect=lambda args,body,**kwargs:body),\
             patch('source_plan.compile_plan',return_value={'checks':[],'actions':[]}),\
             patch.object(sys,'argv',[compiler.__file__,str(self.source),str(params),str(output)]):
            runpy.run_path(compiler.__file__,run_name='__main__')
        pack=json.loads(output.read_bytes())
        self.assertEqual(pack['target']['images'],parameters['images'])
        self.assertEqual(pack['migrations'],[])

    def mixed_images_fixture(self):
        import source_plan_test as fixtures
        kube,p=fixtures.SourceTests().canonical_fixture('images-only')
        selected=['gateway','social','user','voice','web']
        p.update(generation='r20260930a4',dataPVC='voice-nats-jsdata-d202610040049430b',changed_services=selected)
        kube.objects[('Deployment','voice-gateway')]['spec']['template'].setdefault('metadata',{}).setdefault('annotations',{})['kubectl.kubernetes.io/restartedAt']='2026-10-16T00:00:00Z'
        web=kube.objects[('Deployment','voice-web')]
        web['metadata'].setdefault('annotations',{})['fixture/operator-note']='preserve'
        web['spec']['template'].setdefault('metadata',{}).setdefault('annotations',{})['fixture/runtime']='preserve'
        web['spec']['template']['spec']['containers'].append({'name':'retained-sidecar','image':'registry/sidecar@sha256:'+'c'*64,'env':[{'name':'FIXTURE','value':'unchanged'}]})
        web['spec']['template']['spec'].setdefault('initContainers',[]).append({'name':'retained-init','image':'registry/init@sha256:'+'d'*64})
        p['images']={}
        for service in selected:
            name='voice-'+service;spec=kube.objects[('Deployment',name)]['spec']['template']['spec']
            for container in spec.get('initContainers',[])+spec['containers']:
                p['images'][name+'/'+container['name']]=(container['image'] if container['name'].startswith('retained-')
                    else 'registry/'+container['name']+'@sha256:'+'a'*64)
        p['enrolled']=['voice-'+name for name in selected]
        p['contract']={'scripts':[]}
        for part in ('realtime','notification','search','analytics-chat'):
            path=fixtures.SOURCE/('deploy/templates/nats-'+part+'-bootstrap.yaml')
            row=fixtures.decoder(path.read_bytes().split(b'---')[0])
            p['contract']['scripts'].append({'part':part,'sha256':hashlib.sha256(row['data']['bootstrap.sh'].encode()).hexdigest()})
        return fixtures,kube,p

    def test_actual_main_mixed_images_scope_keeps_selected_live_frontend_and_exact_pins(self):
        fixtures,kube,p=self.mixed_images_fixture()
        before=copy.deepcopy(kube.objects[('Deployment','voice-web')])
        params=self.source/'mixed-parameters.json';params.write_text(json.dumps(p))
        output=self.source/('target-'+p['tag']+'.json')
        with patch.object(compiler,'decode_yaml',side_effect=fixtures.decoder):
            compiler.main([str(fixtures.SOURCE),str(params),str(output)],producer=kube)
        pack=json.loads(output.read_bytes())
        self.assertEqual(set(pack['target']['template_hashes']),set(p['enrolled']))
        self.assertEqual(pack['target']['images'],p['images'])
        self.assertEqual(pack['migrations'],[]);self.assertEqual(pack['nonnats']['actions'],[])
        self.assertEqual({row['kind'] for row in pack['manifests']},{'Deployment'})
        self.assertEqual(len(pack['manifests']),5)
        expected=copy.deepcopy(before)
        for container in expected['spec']['template']['spec'].get('initContainers',[])+expected['spec']['template']['spec']['containers']:
            container['image']=p['images']['voice-web/'+container['name']]
        self.assertEqual(next(row for row in pack['manifests'] if row['metadata']['name']=='voice-web'),expected)
        self.assertEqual(kube.objects[('Deployment','voice-web')],before)
        self.assertEqual(pack['target']['source_hashes']['deploy/staging/flutter-web.yaml'],
            hashlib.sha256((fixtures.SOURCE/'deploy/staging/flutter-web.yaml').read_bytes()).hexdigest())
        self.assertNotIn('deploy/staging/admin.yaml',pack['target']['source_hashes'])
        from normalize import image_only_documents
        snapshots={name:copy.deepcopy(kube.objects[('Deployment',name)]) for name in p['enrolled']}
        stage=SimpleNamespace(snapshots=snapshots,expected={'source_claim':p['dataPVC']},old_images={})
        normalized,target=image_only_documents(stage,pack['manifests'],pack['target'])
        for row in normalized:
            name=row['metadata']['name'];expected_spec=copy.deepcopy(snapshots[name]['spec'])
            for container in expected_spec['template']['spec'].get('initContainers',[])+expected_spec['template']['spec']['containers']:
                container['image']=p['images'][name+'/'+container['name']]
            self.assertEqual(row['spec'],expected_spec)
        gateway=next(row for row in normalized if row['metadata']['name']=='voice-gateway')
        self.assertEqual(gateway['spec']['template']['metadata']['annotations']['kubectl.kubernetes.io/restartedAt'],'2026-10-16T00:00:00Z')
        self.assertEqual(target['images'],p['images'])

    def test_actual_main_mixed_images_rejects_unused_pins_and_duplicate_frontend(self):
        for fault in ('unknown-container','unselected-frontend','duplicate-frontend'):
            with self.subTest(fault=fault):
                fixtures,kube,p=self.mixed_images_fixture()
                if fault=='unknown-container':p['images']['voice-gateway/foreign']=IMAGE
                elif fault=='unselected-frontend':p['images']['voice-admin/admin']=IMAGE
                params=self.source/('parameters-'+fault+'.json');params.write_text(json.dumps(p))
                output=self.source/('target-'+p['tag']+'.json')
                original=compiler.rendered_source
                def render(*args):
                    rows=original(*args)
                    if fault=='duplicate-frontend':rows.append(copy.deepcopy(kube.objects[('Deployment','voice-web')]))
                    return rows
                with patch.object(compiler,'decode_yaml',side_effect=fixtures.decoder),\
                     patch.object(compiler,'rendered_source',side_effect=render),self.assertRaises(Blocked):
                    compiler.main([str(fixtures.SOURCE),str(params),str(output)],producer=kube)
                self.assertFalse(output.exists())

if __name__=='__main__':unittest.main()
