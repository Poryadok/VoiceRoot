"""Actual source/compiler FULL23 contract; only external Kubernetes is synthetic."""
import copy
import hashlib
import json
import os
from pathlib import Path
import shutil
import tempfile
import unittest
from unittest.mock import patch

import compiler
import source_plan
import source_plan_test as fixtures
from controller import Blocked

APPLICATIONS = tuple('analytics auth bot chat file gameintegration gateway matchmaking messaging moderation notification realtime role search social space story subscription user voice web admin developer-portal'.split())
REQUEST = '.github/ci/full-application-build-request.json'


class FullApplicationCompilerTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory(); self.addCleanup(self.tmp.cleanup)
        self.root = Path(self.tmp.name) / 'source'; self.root.mkdir()
        # Real source bytes, with a test-only canonical request. No hand-built
        # target, source-plan or normalized desired objects.
        for relative in ('deploy', 'scripts/staging', 'scripts/storage',
                         'src/backend/migrations', 'src/backend/auth/src/main/resources/db/migration',
                         'docker/clickhouse/init'):
            shutil.copytree(fixtures.SOURCE / relative, self.root / relative,
                            ignore=shutil.ignore_patterns('__pycache__'))
        path = self.root / REQUEST; path.parent.mkdir(parents=True)
        path.write_text(json.dumps({'schema':'voice-full-application-build-request-v1',
                                   'cycle':1,'request_id':'1'*32}))

    def inputs(self, mode='full', cold=False):
        kube, p = fixtures.SourceTests().canonical_fixture(mode)
        p.update(generation='r20260930a4', dataPVC='voice-nats-jsdata-d202610040049430b',
                 changed_services=[], contract={'scripts':[]})
        for part in ('realtime','notification','search','analytics-chat'):
            raw = (self.root / ('deploy/templates/nats-'+part+'-bootstrap.yaml')).read_bytes().split(b'---')[0]
            p['contract']['scripts'].append({'part':part,'sha256':hashlib.sha256(fixtures.decoder(raw)['data']['bootstrap.sh'].encode()).hexdigest()})
        names = {'voice-'+s for s in APPLICATIONS} | {source_plan.PRINCIPAL_PUBLISHER}
        p['images'] = {}
        for (kind,name), row in kube.objects.items():
            if kind != 'Deployment' or name not in names: continue
            spec = row['spec']['template']['spec']
            for c in spec.get('initContainers',[]) + spec['containers']:
                p['images'][name+'/'+c['name']] = 'registry/'+c['name']+'@sha256:'+'a'*64
        p['images']['voice-p3-principal-jwks/jwks'] = 'docker.io/library/nginx@sha256:'+'e'*64
        p['enrolled'] = sorted(names)
        for name in source_plan.FRONTENDS:
            row = kube.objects['Deployment','voice-'+name]
            row['spec']['template'].setdefault('metadata',{}).setdefault('annotations',{})['fixture/config'] = 'keep'
            row['spec']['template']['spec']['containers'][0].setdefault('env',[]).append({'name':'EXISTING_CONFIG','value':'keep'})
            for (kind,key),other in kube.objects.items():
                if key=='voice-'+name and kind=='Ingress':other['spec']['rules'][0]['host']='existing.example.invalid'
        if cold:
            p['gis_first_install'] = True
            for kind in ('Service','Deployment'): del kube.objects[kind,'voice-gameintegration']
            original = kube.run
            def run(args, body=None, timeout=30):
                if args == ['get','services,deployments','-o','json']:
                    return {'apiVersion':'v1','kind':'List','metadata':{'resourceVersion':'2'},
                            'items':[copy.deepcopy(row) for (kind,name),row in kube.objects.items() if kind in ('Service','Deployment')]}
                if args[:2] == ['get','--raw'] and ('/deployments?' in args[2] or '/services?' in args[2]):
                    return {'apiVersion':'v1','kind':('DeploymentList' if '/deployments?' in args[2] else 'ServiceList'),
                            'metadata':{'resourceVersion':'2'},'items':[]}
                return original(args,body,timeout)
            kube.run = run
        return kube,p

    def compile(self,kube,p):
        folder=Path(self.tmp.name)/('out'+str(len(list(Path(self.tmp.name).glob('out*')))));folder.mkdir()
        params=folder/'parameters.json';params.write_text(json.dumps(p))
        output=folder/('target-'+p['tag']+'.json')
        with patch.object(compiler,'decode_yaml',side_effect=fixtures.decoder):
            compiler.main([str(self.root),str(params),str(output)],producer=kube)
        return json.loads(output.read_bytes())

    def test_request_and_scope_are_closed_and_legacy_selection_stays_legacy(self):
        kube,p=self.inputs()
        self.assertTrue(compiler.full_application_scope(self.root,p))
        for change in ({'mode':'images-only'}, {'changed_services':['web']},
                       {'images':{k:v for k,v in p['images'].items() if k!='voice-admin/admin'}}):
            self.assertFalse(compiler.full_application_scope(self.root,{**p,**change}))
        path=self.root/REQUEST;raw=path.read_bytes()
        for invalid in (b'{"schema":"voice-full-application-build-request-v1","cycle":true,"request_id":"11111111111111111111111111111111"}',
                        raw[:-1]+b',"cycle":1}',raw[:-1]+b',"extra":true}'):
            path.write_bytes(invalid)
            with self.assertRaisesRegex(source_plan.PlanError,'request_invalid'):self.compile(kube,p)
        path.write_bytes(raw)
        p['images']['voice-unselected/unselected']='registry/unselected@sha256:'+'f'*64
        with self.assertRaisesRegex(Blocked,'image_inventory_changed'):self.compile(kube,p)

    def test_full23_frontend_updates_keep_live_configuration_and_complete_target(self):
        for mode in ('full','app-only'):
            with self.subTest(mode=mode):
                kube,p=self.inputs(mode);before=copy.deepcopy(kube.objects)
                pack=self.compile(kube,p)
                self.assertEqual(pack['target']['images'],p['images'])
                self.assertTrue({'voice-'+s for s in APPLICATIONS} <= set(pack['target']['template_hashes']))
                for name in source_plan.FRONTENDS:
                    row=next(x for x in pack['manifests'] if x['kind']=='Deployment' and x['metadata']['name']=='voice-'+name)
                    expected=copy.deepcopy(before['Deployment','voice-'+name]['spec']['template'])
                    for c in expected['spec']['containers']:c['image']=p['images']['voice-'+name+'/'+c['name']]
                    self.assertEqual(row['spec']['template'],expected)
                    kube.objects['Deployment','voice-'+name]['spec']=copy.deepcopy(row['spec'])
                self.assertEqual(pack['target']['source_hashes'][REQUEST],hashlib.sha256((self.root/REQUEST).read_bytes()).hexdigest())
                # A no-op frontend still belongs to the same complete target.
                repeated=self.compile(kube,p)
                self.assertEqual(repeated['target']['images'],p['images'])
                for check in repeated['nonnats']['checks']:
                    if check['id']!='frontends':continue
                    for descriptor in check['objects']:
                        if descriptor['kind']=='Deployment':continue
                        self.assertEqual(descriptor['desired'],source_plan.guard.semantic(before[descriptor['kind'],descriptor['name']]))
                        self.assertNotEqual(descriptor.get('disposition',check['disposition']),'action')

    def test_full23_cold_gis_defers_only_create_pair(self):
        kube,p=self.inputs(cold=True);pack=self.compile(kube,p)
        self.assertEqual(pack['target']['images'],p['images'])
        self.assertTrue({'voice-'+s for s in APPLICATIONS} <= set(pack['target']['template_hashes']))
        identities={(x['kind'],x['metadata']['name']) for x in pack['manifests']}
        self.assertNotIn(('Deployment','voice-gameintegration'),identities)
        self.assertNotIn(('Service','voice-gameintegration'),identities)
        self.assertIn(('Deployment','voice-realtime'),identities)
        self.assertEqual({(x['kind'],x['metadata']['name']) for x in pack['gis_first_install']['objects']},
                         {('Service','voice-gameintegration'),('Deployment','voice-gameintegration')})
        checked={x['name'] for row in pack['nonnats']['checks'] for x in row['objects'] if x['kind']=='Deployment'}
        self.assertTrue({'voice-'+s for s in APPLICATIONS if s!='gameintegration'} <= checked)
        with patch.object(compiler,'decode_yaml',side_effect=fixtures.decoder):
            self.assertEqual(compiler.prebootstrap_image_inventory(self.root,p,kube),p['images'])

    @unittest.skipUnless(os.name=='posix' and os.geteuid()==0,'real root journal requires Linux root fixture')
    def test_compiled_full_target_binds_real_root_session_without_losing_frontends(self):
        import resource_claim as claim
        import resource_claim_test as fixture
        kube,p=self.inputs(cold=True);pack=self.compile(kube,p);target=pack['target']
        context=fixture.context();context.update(commit=p['tag'],images=copy.deepcopy(p['images']))
        card=fixture.card(context)
        card['observed']['images']={name+'/'+c['name']:c['image'] for (kind,name),row in kube.objects.items()
                                  if kind=='Deployment' for c in row['spec']['template']['spec'].get('initContainers',[])+row['spec']['template']['spec']['containers']}
        private=tempfile.TemporaryDirectory(dir='/root');self.addCleanup(private.cleanup)
        root=Path(private.name);(root/'installed').mkdir(mode=0o700)
        journal=claim.RootJournal(root)
        helper={'nats-rollout-preservation/resource_claim.py':hashlib.sha256(Path(claim.__file__).read_bytes()).hexdigest()}
        session=claim.Session(context,'a'*12,claim.digest(helper),lambda:copy.deepcopy(card),journal)
        session.admit(start=True)
        partial={k:v for k,v in target['images'].items() if k!='voice-admin/admin'}
        with self.assertRaisesRegex(RuntimeError,'candidate_changed'):
            session.bind_target(target['tag'],partial,card['observed']['images'],claim.digest(target))
        session.bind_target(target['tag'],target['images'],card['observed']['images'],claim.digest(target))
        state={'operation':session.operation,'target':target,'resource_fence':session.binding(),'code_capture':helper}
        self.assertEqual(claim.historical_binding(state),session.binding())
        changed=copy.deepcopy(state);changed['target']['images']=partial
        changed['resource_fence']['target_sha256']=claim.digest(changed['target'])
        with self.assertRaisesRegex(RuntimeError,'historical_candidate_changed'):claim.historical_binding(changed)
        card['claim']='3'*36
        with self.assertRaises(Exception):session.check()


if __name__ == '__main__': unittest.main()
