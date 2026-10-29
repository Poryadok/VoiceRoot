"""Bridge preservation admission negatives with a real private root journal.

The TT card is synthetic; no Kubernetes, remote service or live host is used.
These tests establish a hold boundary, not a successful preservation or rollout.
"""
import copy
import os
import shutil
import subprocess
import sys
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch

import bridge_root
import commands
import guard
import migrations
import resource_claim
from resource_claim_test import card, context
import staging_prerequisites
import root_composition_fixture
import runtime_stage
import stage_runtime


def stage_cluster_fixture(session,store_kube=None):
    namespace='00000000-0000-0000-0000-000000000001'
    claim='00000000-0000-0000-0000-000000000002'
    pv='00000000-0000-0000-0000-000000000003'
    names=[stage_runtime.HUB,'voice-gateway',*('voice-'+name for name in stage_runtime.LEAVES),
           'voice-web','voice-admin','voice-developer-portal']
    rows={}; images=[]; replicasets={}
    for index,name in enumerate(names,1):
        uid='deployment-'+str(index);rsuid='rs-'+str(index);container=name.removeprefix('voice-')
        image=(stage_runtime.NATS_IMAGE if name==stage_runtime.HUB else 'fixture.invalid/'+container+':old')
        spec={'containers':[{'name':container,'image':image}]}
        if name==stage_runtime.HUB:spec['volumes']=[{'name':'jsdata','persistentVolumeClaim':{'claimName':'fixture'}}]
        template={'metadata':{'labels':{'app':name}},'spec':spec}
        if store_kube is not None and ('Deployment',name) in store_kube.objects:
            template=copy.deepcopy(store_kube.get('Deployment',name)['spec']['template'])
            for volume in template['spec'].get('volumes',[]):
                if volume.get('name')=='nats-service-creds' and 'secret' in volume:
                    volume['secret']['secretName']='voice-nats-service-credentials-r20260930a4'
        rows[name]={'metadata':{'name':name,'namespace':'voice-staging','uid':uid,'resourceVersion':'1','generation':1},
                    'spec':{'replicas':1,'template':template}}
        rs={'kind':'ReplicaSet','metadata':{'name':'replicaset-'+str(index),'uid':rsuid,
            'ownerReferences':[{'kind':'Deployment','uid':uid}]},'spec':{'template':copy.deepcopy(template)}}
        replicasets[rs['metadata']['name']]=rs;images.append(copy.deepcopy(rs))
        def statuses(containers):return [{'name':c['name'],'ready':True,
            'imageID':('ghcr.io/poryadok/voiceroot/story@sha256:8b142297965b3121fc19e0d970b137ad4ff8c8afa2c79eb49e490d59324f3c5f'
                if name=='voice-story' and c['name']=='story' and os.environ.get('VOICE_TEST_STORY_OCI')
                else 'fixture.invalid/'+c['name']+'@sha256:'+'a'*64)} for c in containers]
        images.append({'kind':'Pod','metadata':{'uid':'pod-'+str(index),'ownerReferences':[{'kind':'ReplicaSet','uid':rsuid}]},
            'status':{'phase':'Running','containerStatuses':statuses(template['spec']['containers']),
                      'initContainerStatuses':statuses(template['spec'].get('initContainers',[]))}})
    fixed={('namespace','voice-staging'):{'metadata':{'uid':namespace,'resourceVersion':'1'}},
           ('configmap','voice-nats-generation'):{'metadata':{'uid':'cbafb90c-6974-4642-86d6-09b2eef1c843','resourceVersion':'1'},
                 'data':{'phase':'active','generation':'r20260930a4','dataPVC':'fixture'}},
           ('pvc','fixture'):{'metadata':{'name':'fixture','uid':claim},'spec':{'volumeName':'pv'}},
           ('pv','pv'):{'metadata':{'uid':pv},'spec':{'claimRef':{'uid':claim},
                'local':{'path':'/var/lib/rancher/k3s/storage/pvc-'+claim+'_voice-staging_fixture'},
                'nodeAffinity':{'required':{'nodeSelectorTerms':[{'matchExpressions':[{'key':'kubernetes.io/hostname',
                    'operator':'In','values':['pmdebook']}]}]}}}},
           ('service','voice-nats'):{'metadata':{'uid':'service','resourceVersion':'1'}}}
    class Cluster:
        def get(self,kind,name):
            session.check()
            if kind=='deployment':return copy.deepcopy(rows[name])
            if kind=='replicaset':return copy.deepcopy(replicasets[name])
            return copy.deepcopy(fixed[(kind,name)])
        def run(self,args,**kwargs):
            session.check()
            if args[:2]==['get','hpa,jobs,deployments']:return {'items':[]}
            if args[:2]==['get','pods,replicasets']:return {'items':copy.deepcopy(images)}
            if args==['get','pods','-o','json']:
                return {'items':[{'metadata':{'name':'fixture-hub','uid':'fixture-hub-uid'},
                    'spec':{'volumes':[{'name':'jsdata','persistentVolumeClaim':{'claimName':'fixture'}}]}}]}
            if args==['create','--dry-run=server','-f','/dev/stdin','-o','json']:
                result=copy.deepcopy(kwargs['body'])
                if result['kind']=='Service':result['spec'].update(type='ClusterIP',clusterIP='10.2.0.1',
                    clusterIPs=['10.2.0.1'],ipFamilies=['IPv4'])
                return result
            raise AssertionError('unexpected external cluster request')
    return Cluster, names, claim


@unittest.skipUnless(os.name == 'posix' and getattr(os, 'geteuid', lambda: -1)() == 0, 'root Linux fixture required')
class BridgePreservationAdmission(unittest.TestCase):
    def test_actual_stage_capture_with_owned_synthetic_cluster(self):
        """Build the same read-only stage seam without replacing capture/preflight."""
        root=Path(self.enterContext(tempfile.TemporaryDirectory(dir='/root')))
        (root/'installed').mkdir(mode=0o700)
        candidate=context();tt_card=card(candidate)
        session=resource_claim.Session(candidate,'a'*12,'b'*64,lambda:copy.deepcopy(tt_card),
                                       resource_claim.RootJournal(root))
        self.enterContext(patch('grp.getgrnam',return_value=SimpleNamespace(gr_gid=0)))
        session.admit(start=True)
        Cluster, names, claim=stage_cluster_fixture(session)
        events=[]
        result=runtime_stage.RolloutStage.capture(Cluster(),'a'*12,events.append,
            ['voice-web','voice-admin','voice-developer-portal'])
        self.assertEqual(set(result.snapshots),set(names))
        self.assertEqual(len(result.old_images),len(names)-1)
        self.assertEqual(result.expected['source_claim_uid'],claim)
        self.assertTrue(any(row['kind']=='rollout_original_capture' for row in events))
        self.assertEqual(session.check()['phase'],'prepare')
        class NamespaceDrift(Cluster):
            def __init__(self):self.reads=0
            def get(self,kind,name):
                result=super().get(kind,name)
                if kind=='namespace':
                    self.reads+=1
                    if self.reads>1:result['metadata']['uid']='foreign'
                return result
        with self.assertRaisesRegex(stage_runtime.Blocked,'staging_identity_changed'):
            runtime_stage.RolloutStage.capture(NamespaceDrift(),'a'*12,lambda _:None,
                ['voice-web','voice-admin','voice-developer-portal'])
        self.assertEqual(session.check()['phase'],'prepare')

    def test_producer_imports_from_captured_helper_directory(self):
        source = Path(__file__).resolve().parents[3]
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            code = Path(directory) / 'code'
            # The optional pinned archive exercises production binary identities;
            # otherwise this test uses synthetic binary bytes only.
            pins = (staging_prerequisites.BINARY_PINS if os.environ.get('VOICE_TEST_PINNED_ARCHIVE')
                    else root_composition_fixture.SYNTHETIC_PINS)
            with patch.object(staging_prerequisites, 'BINARY_PINS', pins):
                binding = root_composition_fixture.helper_binding(source, code)
            modules = ('staging_cohort_preservation', 'staging_cohort_preservation_runtime',
                       'staging_cohort_preservation_pg', 'staging_cohort_preservation_auxiliary',
                       'staging_cohort_preservation_auth')
            for name in modules:
                self.assertIn('nats-rollout-preservation/' + name + '.py', binding)
            script = ('import importlib,json; names=' + repr(modules) + '; '
                      'print(json.dumps([importlib.import_module(n).__file__ for n in names]))')
            env = dict(os.environ, PYTHONPATH=os.pathsep.join((str(code / 'nats-rollout-preservation'),
                                                               str(code / 'nats-known-baseline'))))
            result = subprocess.run([sys.executable, '-c', script], cwd=directory, env=env,
                                    check=True, capture_output=True, text=True)
            import json
            self.assertEqual(json.loads(result.stdout),
                             [str(code / 'nats-rollout-preservation' / (name + '.py')) for name in modules])

    def test_captured_helpers_check_protected_source_receipt(self):
        # A separately frozen test-source archive carries the request marker;
        # this mutable test harness is never copied into that source capture.
        source = Path(os.environ.get('VOICE_TEST_SOURCE_ROOT', Path(__file__).resolve().parents[3]))
        with tempfile.TemporaryDirectory(dir=os.environ.get('VOICE_COHORT_NATIVE_ROOT','/root')) as directory:
            root = Path(directory)
            installed = root / 'installed'
            installed.mkdir(mode=0o700)
            code = installed / 'code'
            pins = (staging_prerequisites.BINARY_PINS if os.environ.get('VOICE_TEST_PINNED_ARCHIVE')
                    else root_composition_fixture.SYNTHETIC_PINS)
            with patch.object(staging_prerequisites, 'BINARY_PINS', pins):
                binding = root_composition_fixture.helper_binding(source, code)
            for name in pins:
                (code/name).chmod(0o550)
            workspace = installed / 'sources' / ('a' * 12)
            workspace.mkdir(parents=True, mode=0o700)
            # Copy the entire immutable Git archive: source plans and helper
            # custody must see the same tree, including NATS known-baseline.
            shutil.copytree(source, workspace, dirs_exist_ok=True)
            for path in workspace.rglob('*'):
                path.chmod(0o700 if path.is_dir() else 0o600)
            script = r'''
import base64, contextlib, copy, hashlib, http.client, io, json, os, re, subprocess, sys, urllib.request, zipfile
from pathlib import Path
from types import SimpleNamespace
from unittest.mock import patch
import bridge_root, commands, guard, resource_claim, stage_runtime, staging_prerequisites as parent
import staging_cohort_preservation as producer
import compiler, source_authority, yaml
sys.path.append(sys.argv[3])
from staging_bridge_preservation_joint_test import stage_cluster_fixture
import source_plan_test
root=Path(sys.argv[1]); code=root/'installed'/'code'; workspace=root/'installed'/'sources'/('a'*12)
binding=json.loads((code/'capture-manifest.json').read_bytes()); pins=json.loads(sys.argv[2])
c=dict(project='voice',resource='STAGING',actor='operator',claim='11111111-1111-4111-8111-111111111111',generation=7,
 window='22222222-2222-4222-8222-222222222222',revision=12,operation_id='stg-run-17',commit=os.environ.get('VOICE_TEST_SOURCE_SHA','c'*40),
 images={'voice-gameintegration/gameintegration':source_authority.REGISTRY+'/gameintegration@sha256:'+'c'*64})
card=dict(code='STAGING',environment='stg',state='held',phase='deploy',owner=c['actor'],claim=c['claim'],
 generation=c['generation'],window=c['window'],revision=c['revision'],operation=2,
 candidate=dict(state='known',commit=c['commit'],images=copy.deepcopy(c['images'])),
 observed=dict(state='known',claim=c['claim'],window=c['window'],operation=1,
 images={'voice-user/user':'registry/user@sha256:'+'b'*64}))
approved={'schema':'voice-source-authority-v1','repository':'Poryadok/VoiceRoot','verified':True,
 'run_id':17,'run_attempt':1,'source_sha':c['commit'],'images':{name:source_authority.REGISTRY+'/'+name+'@sha256:'+'c'*64
 for name in source_authority.IMAGE_NAMES},
 'source_files':{p.relative_to(workspace).as_posix():hashlib.sha256(p.read_bytes()).hexdigest()
 for p in workspace.rglob('*') if p.is_file()}}
execution={'head_sha':c['commit']}; session=resource_claim.Session(c,'a'*12,resource_claim.digest(binding),
 lambda:copy.deepcopy(card),resource_claim.RootJournal(root))
actions=object.__new__(bridge_root.Actions);actions.code=code;actions.binding=binding;actions.tt_session=session
with patch.object(guard,'ROOT',root),patch.object(bridge_root,'INSTALLED',root/'installed'), \
 patch.object(parent,'BINARY_PINS',pins),patch('grp.getgrnam',return_value=SimpleNamespace(gr_gid=0)):
 session.admit(start=True)
 with resource_claim.command_scope(session.check):
  bridge_root.record_source_authority(session,approved,execution)
  wrong=copy.deepcopy(approved); wrong['source_sha']='d'*40
  try: parent._capture_authority(actions,session,wrong,execution)
  except parent.Blocked as exc:
   assert str(exc)=='staging_prerequisites_source_changed', str(exc)
  else: raise AssertionError('counterfeit source receipt accepted')
  assert not (root/'installed'/'prerequisites').exists()
  from root_main import save
  store_kube,store_parameters=source_plan_test.SourceTests().canonical_fixture('app-only')
  for (kind,name),row in list(store_kube.objects.items()):
   if kind!='PersistentVolumeClaim':continue
   pv_name=row['spec']['volumeName']
   store_kube.objects[('PersistentVolume',pv_name)]={'apiVersion':'v1','kind':'PersistentVolume',
    'metadata':{'name':pv_name,'uid':'uid-'+pv_name,'resourceVersion':'1'},
    'spec':{'claimRef':{'uid':row['metadata']['uid'],'name':name,'namespace':'voice-staging'}}}
  policy_fields=('s3_signing_endpoint','gateway_host','storage_host','livekit_host','web_host','admin_host',
   'developer_portal_host','gateway_tls_secret','storage_tls_secret','minio_image','minio_mc_image',
   'minio_storage_class','minio_storage_size','web_origin')
  save(root/'installed'/'policy.json',{key:store_parameters[key] for key in policy_fields if key in store_parameters})
  inputs=root/'installed'/'prerequisite-inputs';inputs.mkdir(mode=0o700)
  def openssl(*args):
   subprocess.run(['/usr/bin/openssl',*map(str,args)],check=True,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL)
  key=root/'ca.key';cert=root/'ca.crt';peerkey=root/'peer.key';csr=root/'peer.csr';peer=root/'peer.crt';ext=root/'ext'
  openssl('req','-x509','-newkey','rsa:2048','-nodes','-keyout',key,'-out',cert,'-days','180',
   '-subj','/CN=synthetic-root','-addext','basicConstraints=critical,CA:TRUE',
   '-addext','keyUsage=critical,keyCertSign,cRLSign')
  openssl('req','-newkey','rsa:2048','-nodes','-keyout',peerkey,'-out',csr,'-subj','/CN=voice-space')
  ext.write_text('basicConstraints=critical,CA:FALSE\nsubjectAltName=DNS:voice-space.voice-staging.svc\nextendedKeyUsage=serverAuth,clientAuth\n')
  openssl('x509','-req','-in',csr,'-CA',cert,'-CAkey',key,'-set_serial','1234','-days','150','-extfile',ext,'-out',peer)
  ca=cert.read_text();leaf=peer.read_text()
  import staging_p3_prerequisites_crypto as crypto
  save(inputs/'gis-issuer.json',{'schema':'voice-staging-gis-issuer-input-v1','receipt':{},'ca_certificate_pem':ca,
   'ca_private_key_pem':key.read_text(),'jwks_ca_bundle_pem':ca,'ca_secret':{}})
  save(inputs/'p3-issuer.json',{'schema':'voice-staging-p3-issuer-input-v1','receipt':{},'ca_certificate_pem':ca,
   'ca_private_key_pem':key.read_text(),'trust':{name:ca for name in crypto.TRUST_KEYS},
   'space_client_certificate_pem':leaf,'space_jwks_certificate_pem':leaf,'space_server_ca_pem':ca,
   'replay_password_b64':base64.b64encode(b'synthetic-replay-password').decode()})
  Cluster, names, claim=stage_cluster_fixture(session,store_kube); cluster=Cluster()
  original_get=cluster.get
  pg_controller='00000000-0000-4000-8000-000000000011'
  pg_pod='00000000-0000-4000-8000-000000000012'
  pg_claim='00000000-0000-4000-8000-000000000013'
  pg_volume='00000000-0000-4000-8000-000000000014'
  pg_objects={('statefulset','voice-postgres'):{'metadata':{'uid':pg_controller,'resourceVersion':'1'},'spec':{'replicas':1}},
   ('pod','voice-postgres-0'):{'metadata':{'uid':pg_pod,'resourceVersion':'1','ownerReferences':[{'kind':'StatefulSet',
    'name':'voice-postgres','uid':pg_controller,'controller':True}]},'spec':{'volumes':[{'persistentVolumeClaim':
    {'claimName':'pgdata-voice-postgres-0'}}]},'status':{'phase':'Running','containerStatuses':[{'name':'postgres',
    'ready':True,'imageID':'docker.io/library/postgres@sha256:'+'a'*64}]}},
   ('persistentvolumeclaim','pgdata-voice-postgres-0'):{'metadata':{'uid':pg_claim,'resourceVersion':'1'},
    'spec':{'volumeName':'fixture-postgres-pv'},'status':{'phase':'Bound'}},
   ('persistentvolume','fixture-postgres-pv'):{'metadata':{'uid':pg_volume,'resourceVersion':'1'},
    'spec':{'claimRef':{'uid':pg_claim,'name':'pgdata-voice-postgres-0','namespace':'voice-staging'}}}}
  nats_secrets={};bootstrap_configmaps={}
  if 'VOICE_TEST_NATS_INPUTS' in os.environ:
   import actor_root, bootstrap_auth, bootstrap_selection
   nats=Path(os.environ['VOICE_TEST_NATS_INPUTS']);config=(nats/'server.conf').read_text()
   value=lambda name:json.loads(re.search(r'^'+name+r': (.+)$',config,re.M)[1])
   operator=value('operator');system=value('system_account');resolver=value('resolver_preload')
   account=next(name for name in resolver if name!=system)
   def secret(name,data):
    return {'apiVersion':'v1','kind':'Secret','metadata':{'name':name,'namespace':'voice-staging',
      'uid':name,'resourceVersion':'1'},'type':'Opaque','data':{k:base64.b64encode(v).decode() for k,v in data.items()}}
   name='voice-nats-operator-r20260930a4'
   nats_secrets[name]=secret(name,{k:v.encode() for k,v in {'operator.jwt':operator,'account.jwt':resolver[account],
    'system-account.jwt':resolver[system],'account.public':account,'system-account.public':system}.items()})
   name='voice-nats-service-credentials-r20260930a4'
   nats_secrets[name]=secret(name,{role+'.creds':(nats/'inputs'/(role+'.creds')).read_bytes()
    for role in ('social','realtime','space','messaging','story')})
   name='voice-nats-hub-tls-r20260930a4'
   nats_secrets[name]=secret(name,{})
   deployed=json.loads((code/'nats-known-baseline'/'deployed-contract.json').read_bytes())
   for item in deployed['scripts']:
    raw=(workspace/'scripts/staging/nats-known-baseline/testdata/deployed-bootstrap-20261004'/
     (item['part']+'-bootstrap.sh')).read_bytes()
    if len(raw)!=item['bytes'] or hashlib.sha256(raw).hexdigest()!=item['sha256']:
     raise AssertionError('deployed bootstrap fixture source changed')
    name='voice-nats-'+item['part']+'-bootstrap'
    bootstrap_configmaps[name]={'apiVersion':'v1','kind':'ConfigMap',
     'metadata':{'name':name,'namespace':'voice-staging','uid':item['configMapUID'],
      'resourceVersion':item['configMapResourceVersion']},'data':{'bootstrap.sh':raw.decode()}}
   synthetic_account_sha=hashlib.sha256(account.encode()).hexdigest()
   # Explicit external synthetic identity seam; actual JWT/ACL/auth checks run.
   actor_root.ACCOUNT_SHA=synthetic_account_sha
   bootstrap_auth.ACCOUNT_SHA=synthetic_account_sha
   bootstrap_selection.ACCOUNT_SHA=synthetic_account_sha
  def get(kind,name):
   session.check()
   if kind.lower()=='secret' and name in nats_secrets:return copy.deepcopy(nats_secrets[name])
   if kind.lower()=='configmap' and name in bootstrap_configmaps:return copy.deepcopy(bootstrap_configmaps[name])
   if (kind.lower(),name) in pg_objects:return copy.deepcopy(pg_objects[(kind.lower(),name)])
   if (kind,name)==('node','pmdebook'):
    if 'VOICE_TEST_OCI' not in os.environ:raise KeyError((kind,name))
    return {'metadata':{'name':name,'uid':'synthetic-node'},'status':{'nodeInfo':{'operatingSystem':'linux',
      'architecture':'amd64'},'conditions':[{'type':'Ready','status':'True'}]}}
   if kind.lower()=='secret' and name=='voice-app-secrets':
    row=store_kube.get('Secret',name)
    row['metadata'].update(uid='00000000-0000-4000-8000-000000000004',resourceVersion='1')
    row['data'].pop('GAME_INTEGRATION_DATABASE_URL',None);row['data'].pop('GAME_INTEGRATION_DB_PASSWORD',None)
    return row
   if kind=='secret' and name in ('voice-media-ca','voice-space-media-client-tls','voice-space-media-server-tls'):
    data=({'ca.crt':ca,'jwks-ca.crt':ca} if name=='voice-media-ca' else {'tls.crt':leaf,'tls.key':peerkey.read_text()})
    return {'apiVersion':'v1','kind':'Secret','metadata':{'name':name,'namespace':'voice-staging',
      'uid':'00000000-0000-4000-8000-000000000005','resourceVersion':'1'},'type':'Opaque',
      'data':{k:base64.b64encode(v.encode()).decode() for k,v in data.items()}}
   try:return original_get(kind.lower(),name)
   except KeyError:
    spelling={'secret':'Secret','configmap':'ConfigMap','statefulset':'StatefulSet',
      'deployment':'Deployment','service':'Service','persistentvolumeclaim':'PersistentVolumeClaim',
      'pvc':'PersistentVolumeClaim','pv':'PersistentVolume'}
    return store_kube.get(spelling.get(kind,kind),name)
  def run(args,**kwargs):
   session.check()
   import actor_root
   if args==['get','pods','-l','app='+stage_runtime.HUB,'-o','json']:
    return {'items':[{'metadata':{'name':stage_runtime.HUB+'-synthetic',
      'ownerReferences':[{'kind':'ReplicaSet','name':'replicaset-1','uid':'rs-1'}]},
      'status':{'phase':'Running','conditions':[{'type':'Ready','status':'True'}]}}]}
   if args[:2]==['get','--raw'] and ':8222/proxy/jsz?' in args[-1]:
    from nats_contract_plan_test import live
    import nats_contract_plan as contract
    from rollout_census_test import CREATED
    streams=live();details=[]
    for name,subjects in (('voice_events',contract.VOICE_SUBJECTS_12),('user_events',contract.USER_SUBJECTS_14)):
     streams[name]=copy.deepcopy(streams['file_events'])
     streams[name]['config']['name']=name;streams[name]['config']['subjects']=sorted(subjects)
    for name,info in streams.items():
     state={**info['state'],'consumer_count':0,'bytes':100}
     details.append({'name':name,'created':CREATED,'config':info['config'],'state':state,'consumer_detail':[]})
    return {'streams':len(details),'consumers':0,'messages':sum(row['state']['messages'] for row in details),
     'bytes':sum(row['state']['bytes'] for row in details),'total':1,
     'account_details':[{'id':account,'stream_detail':details}]}
   if args==list(actor_root.STORY_SCHEMA_ARGS):
    return {'database':'story_db','username':'voice','version':4,'dirty':False}
   if args[:2]==['exec','voice-postgres-0'] and 'sha256sum' in ' '.join(args):
    password=base64.b64decode(get('secret','voice-app-secrets')['data']['POSTGRES_PASSWORD'])
    return {'password_sha256':hashlib.sha256(password).hexdigest()}
   if args[:2]==['apply','--dry-run=server']:return store_kube.run(args,**kwargs)
   return cluster.run(args,**kwargs)
  def capture(argv,**kwargs):
   commands.authority_guard(argv)
   if 'VOICE_TEST_SPACE_OCI' in os.environ:
    fixture=json.loads((Path(os.environ['VOICE_TEST_SPACE_OCI'])/'identity.json').read_bytes())['identity']
    image=fixture['immutable_image']
    if argv[0]=='/usr/bin/docker' and argv[-4:]==['pull','--platform','linux/amd64',image]:
     return b'' # Registry pull is modeled; the exact locally built image exists.
    if argv==['/usr/bin/docker','--host=unix:///var/run/docker.sock','image','inspect',image]:
     rows=json.loads(subprocess.check_output(['docker','image','inspect','voice-wt30-native-tool:46b6e99']))
     assert rows[0]['Id']==fixture['config_digest']
     rows[0]['RepoDigests']=[image] # Synthetic registry digest binding.
     return json.dumps(rows).encode()
   if argv==['kubectl','create','--dry-run=client','--validate=false','-f','/dev/stdin','-o','json']:
    return json.dumps(yaml.safe_load(kwargs['body'])).encode()
   tail=argv[4:]
   if tail[:2]==['get','--raw']:
    plural=tail[2].split('/')[5].split('?')[0]
    kind={'secrets':'Secret','configmaps':'ConfigMap','services':'Service'}[plural]
    return json.dumps({'apiVersion':'v1','kind':kind+'List','metadata':{'resourceVersion':'1'},'items':[]}).encode()
   if tail[:2]==['get','deployments']:
    return json.dumps({'apiVersion':'apps/v1','kind':'DeploymentList','metadata':{'resourceVersion':'1'},'items':[]}).encode()
   raise AssertionError('unexpected external command '+repr(tail))
  class Response:
   status=200
   def __init__(self,body):self.body=body
   def getheader(self,name):return None
   def read(self,size):chunk=self.body[:size];self.body=self.body[size:];return chunk
  class Https(http.client.HTTPSConnection):
   sock=None;debuglevel=0
   def __init__(self,host,**kwargs):self.host=host
   def request(self,method,path,headers):self.path=path
   def getresponse(self):
    if self.host=='auth.docker.io':return Response(b'{"token":"synthetic-token"}')
    if self.host!='registry-1.docker.io':raise AssertionError('unexpected registry host')
    name=('index' if self.path.endswith('0e52bd054126cdc040b1276c0af347aeb02a41b9cfe305d6094767ac30e97949') else
          'manifest' if self.path.endswith('84ed8c0dd296c188bfc24ed506f46589e9285fb0b11cf3a04d91d524d3067e11') else
          'config' if self.path.endswith('88f52f3d58a505b7e1ad96ca6969478517bcccb58418e75a54223271a64da7df') else None)
    if name is None:raise AssertionError('unexpected registry digest')
    return Response((Path(os.environ['VOICE_TEST_OCI'])/(name+'.json')).read_bytes())
   def close(self):pass
  class RegistryResponse:
   status=200
   def __init__(self,raw,headers):self.raw=raw;self.headers=headers
   def read(self,size):chunk=self.raw[:size];self.raw=self.raw[size:];return chunk
   def close(self):pass
   def __enter__(self):return self
   def __exit__(self,*args):self.close()
  github_calls=[]
  # Synthetic GitHub/registry transport, with real locally built image bytes
  # and real content, closure, source and compiled-gate proofs.
  if 'VOICE_TEST_SPACE_OCI' in os.environ:
   data=json.loads((Path(os.environ['VOICE_TEST_SPACE_OCI'])/'identity.json').read_bytes())
   identity=data['identity']; inventory=data['inventory']; proof=data['proof']
   request=json.loads((workspace/'.github/ci/full-application-build-request.json').read_bytes())
   tag='ghcr.io/poryadok/voiceroot/space-native-installer:source-'+c['commit']+'-17-1'
   pub={'schema':'voice-space-native-tool-publication-v1','source_sha':c['commit'],'run_id':17,'run_attempt':1,
    'tag':tag,**identity,'source_inventory':inventory,'request':request,'content':proof,
    'runtime_qualified':False,'deployment_performed':False}
   archive=io.BytesIO()
   with zipfile.ZipFile(archive,'w') as z:z.writestr('space-native-tool.json',json.dumps(pub))
   archive=archive.getvalue()
   repo={'id':9,'full_name':'Poryadok/VoiceRoot','private':False,'default_branch':'master'}
   run_record={'id':17,'run_attempt':1,'head_sha':c['commit'],'head_branch':'master','event':'push',
    'path':'.github/workflows/ci.yml','status':'completed','conclusion':'success',
    'repository':repo,'head_repository':repo}
   jobs=[]
   names='analytics bot chat file gameintegration gateway matchmaking messaging moderation notification realtime role search social space story subscription user voice'.split()
   required={'staging-images-push ('+name+')':'Build and push '+name+' image' for name in names}
   required.update({'backend-auth':'Push auth image to GHCR','web':'Build and push DEV web image',
    'admin':'Build and push admin image','developer-portal':'Build and push developer-portal image',
    'nats-hub-config-renderer-image':'Build and push renderer image','staging-stack-lock':'Render stack lock',
    'ci-gate':'Verify required jobs ran'})
   for number,(name,step) in enumerate(required.items(),1):
    jobs.append({'id':number,'name':name,'run_id':17,'run_attempt':1,'head_sha':c['commit'],
     'status':'completed','conclusion':'success','steps':[{'name':step,'conclusion':'success'}]})
   jobs.append({'id':100,'name':'Space native tool image','run_id':17,'run_attempt':1,'head_sha':c['commit'],
    'status':'completed','conclusion':'success','started_at':'2025-01-01T00:00:00Z',
    'completed_at':'2025-01-01T00:01:00Z','steps':[{'name':s,'conclusion':'success'} for s in
     ('Verify Space native tool source','Capture Space native tool identity')]})
   for number,name in enumerate(('staging-images-promote','deploy-staging'),101):
    jobs.append({'id':number,'name':name,'run_id':17,'run_attempt':1,'head_sha':c['commit'],
     'status':'completed','conclusion':'skipped'})
   artifact={'id':109,'name':'space-native-tool-17-1','expired':False,'size_in_bytes':len(archive),
    'created_at':'2025-01-01T00:00:30Z','expires_at':'2099-01-01T00:00:00Z',
    'workflow_run':{'id':17,'head_sha':c['commit'],'repository_id':9,'head_repository_id':9}}
   def gh_raw(url):
    suffix=url[len(source_authority.API):]
    if suffix=='':value=repo
    elif suffix=='/git/ref/heads/master':value={'ref':'refs/heads/master','object':{'type':'commit','sha':c['commit']}}
    elif suffix=='/actions/runs/17':value=run_record
    elif suffix.startswith('/actions/runs/17/attempts/1/jobs?'):
     page=int(urllib.parse.parse_qs(urllib.parse.urlsplit(url).query)['page'][0])
     value={'total_count':len(jobs),'jobs':jobs[(page-1)*20:page*20]}
    elif suffix.startswith('/actions/runs/17/artifacts?'):value={'total_count':1,'artifacts':[artifact]}
    elif suffix=='/actions/artifacts/109':value=artifact
    elif suffix=='/actions/artifacts/109/zip':return archive
    else:raise AssertionError('unexpected synthetic Github endpoint '+suffix)
    return json.dumps(value).encode()
   def registry_open(self,request,timeout=None):
    url=request.full_url
    if url.startswith('https://ghcr.io/token?'):return RegistryResponse(b'{"token":"synthetic-token"}',{})
    tail=url.rsplit('/',1)[-1]
    name=('index' if tail in (identity['index_digest'],tag.rsplit(':',1)[1]) else
          'manifest' if tail==identity['platform_digest'] else 'config' if tail==identity['config_digest'] else None)
    if name is None:raise AssertionError('unexpected synthetic registry path')
    return RegistryResponse((Path(os.environ['VOICE_TEST_SPACE_OCI'])/(name+'.json')).read_bytes(),{})
  def story_request(url,headers,limit,deadline):
   if url.startswith('https://api.github.com/'):
    github_calls.append(url)
    if 'VOICE_TEST_SPACE_OCI' in os.environ:return RegistryResponse(gh_raw(url),{})
    raise OSError('synthetic CI unavailable')
   if url.startswith('https://ghcr.io/token?'):return RegistryResponse(b'{"token":"synthetic-token"}',{})
   records=json.loads((Path(os.environ['VOICE_TEST_STORY_OCI'])/'records.json').read_bytes())
   record=records[url];raw=(Path(os.environ['VOICE_TEST_STORY_OCI'])/(record['sha256']+'.json')).read_bytes()
   return RegistryResponse(raw,record['headers'])
  if 'VOICE_TEST_NATS_INPUTS' in os.environ:
   import bootstrap_enrollment, bootstrap_root
   bootstrap=(nats/'inputs'/'bootstrap.creds').read_bytes()
   actor_sha=hashlib.sha256(actor_root.public_user(bootstrap,'bootstrap').encode()).hexdigest()
   bootstrap_root.ACCOUNT_SHA=synthetic_account_sha;bootstrap_root.ACTOR_SHA=actor_sha
   bootstrap_selection.ACTOR_SHA=actor_sha
   old_name='voice-nats-bootstrap-credentials-r20260930a4'
   old=secret(old_name,{'bootstrap.creds':bootstrap});old['immutable']=True
   old['metadata'].update(uid='cd1f24fe-cbf1-40f2-9043-f8affd24127d',resourceVersion='2864382')
   nats_secrets[old_name]=old
   marker=original_get('configmap','voice-nats-generation')
   operator_row=nats_secrets['voice-nats-operator-r20260930a4']
   ident=lambda row:{k:row['metadata'][k] for k in ('name','uid','resourceVersion')}
   binding={'generation':'r20260930a4','marker_uid':marker['metadata']['uid'],
    'marker_rv':marker['metadata']['resourceVersion'],'account_sha256':synthetic_account_sha,'actor_sha256':actor_sha,
    'old_secret':ident(old)|{'credential_sha256':bootstrap_enrollment.credential_hash(old)},'operator_secret':ident(operator_row)}
   class EnrollmentCluster:
    def get(self,kind,name):
     session.check()
     return copy.deepcopy(marker if kind=='configmap' else nats_secrets[name])
    def run(self,args,body=None):
     session.check()
     if args[:2]==['get','secrets']:
      name=args[2].split('metadata.name=',1)[1];return {'items':[copy.deepcopy(nats_secrets[name])] if name in nats_secrets else []}
     if args==['create','-f','/dev/stdin','-o','json']:
      value=copy.deepcopy(body);name=value['metadata']['name'];assert name not in nats_secrets
      value['metadata'].update(uid='synthetic-'+name,resourceVersion='1');nats_secrets[name]=value;return copy.deepcopy(value)
     raise AssertionError('unexpected enrollment cluster call')
   enrollment_kube=EnrollmentCluster()
   def unchanged():
    session.check();assert enrollment_kube.get('configmap','voice-nats-generation')==marker
    assert enrollment_kube.get('secret',old_name)==old and enrollment_kube.get('secret',operator_row['metadata']['name'])==operator_row
   prior_proof={}
   def v1_auth(credentials):
    prior_proof.update(bootstrap_auth.prove(credentials,operator_row,unchanged));return prior_proof['verified'] is True
   def v1_commit(receipt):
    # Exact legacy projection of a real scratch proof, not an invented PASS.
    fields=('verified','server_image','account_sha256','reply_prefix','normalized_consumer','normalized_consumer_sha256')
    save(root/'bootstrap-enrollment.json',{**receipt,'auth_proof':{k:prior_proof[k] for k in fields}})
   bootstrap_enrollment.enroll(bootstrap_enrollment.EnrollmentKube(enrollment_kube),binding,bootstrap,
     unchanged,v1_auth,v1_commit)
   seed_dir=bootstrap_root.SEED.parent;seed_dir.mkdir(mode=0o700,parents=True,exist_ok=True)
   bootstrap_root.SEED.write_bytes((nats/'signer.seed').read_bytes()+b'\n');bootstrap_root.SEED.chmod(0o600)
   def private_read(path):
    from root_main import private_json
    return private_json(root/Path(path).name)
   def private_save(path,value):save(root/Path(path).name,value)
   bootstrap_root.enroll_current(enrollment_kube,code,actions.binding,unchanged,
    private_read,private_save,bootstrap_auth.prove)
  with patch.object(stage_runtime.Kube,'get',side_effect=get), \
       patch.object(stage_runtime.Kube,'_run',side_effect=run),patch.object(commands,'capture',side_effect=capture), \
       patch.object(stage_runtime.Kube,'secret_meta',side_effect=lambda name:{k:get('secret',name)['metadata'][k] for k in ('uid','resourceVersion')}), \
       patch.object(compiler,'capture',side_effect=capture), \
       patch.object(source_authority.transport,'_request',side_effect=story_request), \
       patch.object(http.client,'HTTPSConnection',Https), \
       patch.object(urllib.request.OpenerDirector,'open',registry_open) if 'VOICE_TEST_SPACE_OCI' in os.environ else contextlib.nullcontext():
   if 'VOICE_TEST_SPACE_OCI' in os.environ:
    import runtime_stage
    preview=runtime_stage.RolloutStage.capture(stage_runtime.Kube(),session.operation,lambda _:None,
     ['voice-web','voice-admin','voice-developer-portal'])
    # Synthetic TT baseline comes from the modeled cluster's real capture.
    card['observed']['images']=copy.deepcopy(preview.old_images)
   trace_errors=[]
   def trace(frame,event,arg):
    if event=='exception' and frame.f_code.co_filename.endswith(('space_native_tool_identity_staging.py','source_authority.py','github_custody.py')):
     trace_errors.append((frame.f_code.co_name,frame.f_lineno,arg[0].__name__,str(arg[1])))
    return trace
   if 'VOICE_TEST_SPACE_OCI' in os.environ:sys.settrace(trace)
   try: producer.prepare(actions,{'mode':'app-only','token':'synthetic-ci-token','changed_services':['gameintegration'],
      'gis_first_install':True},approved,execution,workspace)
   except KeyError as exc:
    assert 'VOICE_TEST_OCI' not in os.environ and exc.args==(('node','pmdebook'),), str(exc)
   except parent.Blocked as exc:
    raise AssertionError('producer blocked: '+str(exc)) from exc
   except Exception as exc:
    if 'VOICE_TEST_SPACE_OCI' in os.environ:
     raise AssertionError('positive producer stopped: '+type(exc).__name__+': '+str(exc)+' reader='+repr(trace_errors[-8:])+' requests='+repr(github_calls)) from exc
    if type(exc).__name__!='StagingToolError' or str(exc)!='STAGING native publication unavailable' or not github_calls:
     raise AssertionError('producer after OCI transport: '+type(exc).__name__+': '+str(exc)) from exc
  checkpoint=root/'installed'/'cohort-preservation'/session.operation/'checkpoint.json'
  if github_calls:
   assert json.loads(checkpoint.read_bytes())['status']=='PREPARED'
  else:assert not checkpoint.exists()
  p=workspace/'scripts/staging/nats-rollout-preservation/staging_cohort_preservation.py'
  p.write_bytes(p.read_bytes()+b'\n# fixture drift\n')
  try: parent._verify_helper_source(session,approved,workspace)
  except parent.Blocked as exc:
   assert str(exc)=='staging_prerequisites_source_changed', str(exc)
  else: raise AssertionError('changed source accepted')
  assert session.check()['phase']=='prepare'
'''
            import json
            env = dict(os.environ, PYTHONPATH=os.pathsep.join((str(code / 'nats-rollout-preservation'),
                                                               str(code / 'nats-known-baseline'))))
            result=subprocess.run([sys.executable, '-c', script, str(root), json.dumps(pins),str(Path(__file__).parent)], cwd=directory,
                                  env=env, capture_output=True, text=True)
            self.assertEqual(result.returncode,0,result.stderr)

    def test_missing_preservation_holds_before_parent_or_cluster(self):
        for mode in ('full', 'app-only'):
            with self.subTest(mode=mode), tempfile.TemporaryDirectory(dir='/root') as directory:
                root = Path(directory)
                (root / 'installed').mkdir(mode=0o700)
                candidate = context()
                tt_card = card(candidate)
                journal = resource_claim.RootJournal(root)
                session = resource_claim.Session(candidate, 'a' * 12, 'b' * 64,
                    lambda: copy.deepcopy(tt_card), journal)
                actions = object.__new__(bridge_root.Actions)
                actions.tt_session = session
                with patch.object(guard, 'ROOT', root), \
                     patch('grp.getgrnam', return_value=SimpleNamespace(gr_gid=0)):
                    session.admit(start=True)
                    with resource_claim.command_scope(session.check), \
                         patch.object(commands, 'capture', side_effect=AssertionError('unexpected cluster command')), \
                         patch.object(staging_prerequisites, 'run_prepare', wraps=staging_prerequisites.run_prepare) as parent:
                        with self.assertRaisesRegex(migrations.Blocked, 'staging_migration_preservation_authority_missing'):
                            actions._prepare({'mode': mode})
                        parent.assert_not_called()
                    self.assertEqual(session.check()['phase'], 'prepare')
                    self.assertFalse((root / 'installed' / 'cohort-preservation').exists())
                    self.assertFalse((root / 'installed' / 'prerequisites').exists())


if __name__ == '__main__':
    unittest.main()
