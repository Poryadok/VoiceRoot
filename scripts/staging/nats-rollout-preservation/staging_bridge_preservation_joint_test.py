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
        if store_kube is not None and name in ('voice-user','voice-file'):
            template=copy.deepcopy(store_kube.get('Deployment',name)['spec']['template'])
        rows[name]={'metadata':{'name':name,'namespace':'voice-staging','uid':uid,'resourceVersion':'1','generation':1},
                    'spec':{'replicas':1,'template':template}}
        rs={'kind':'ReplicaSet','metadata':{'name':'replicaset-'+str(index),'uid':rsuid,
            'ownerReferences':[{'kind':'Deployment','uid':uid}]},'spec':{'template':copy.deepcopy(template)}}
        replicasets[rs['metadata']['name']]=rs;images.append(copy.deepcopy(rs))
        def statuses(containers):return [{'name':c['name'],'ready':True,
            'imageID':'fixture.invalid/'+c['name']+'@sha256:'+'a'*64} for c in containers]
        images.append({'kind':'Pod','metadata':{'uid':'pod-'+str(index),'ownerReferences':[{'kind':'ReplicaSet','uid':rsuid}]},
            'status':{'phase':'Running','containerStatuses':statuses(template['spec']['containers']),
                      'initContainerStatuses':statuses(template['spec'].get('initContainers',[]))}})
    fixed={('namespace','voice-staging'):{'metadata':{'uid':namespace}},
           ('configmap','voice-nats-generation'):{'metadata':{'uid':'marker','resourceVersion':'1'},
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
        source = Path(__file__).resolve().parents[3]
        with tempfile.TemporaryDirectory(dir='/root') as directory:
            root = Path(directory)
            installed = root / 'installed'
            installed.mkdir(mode=0o700)
            code = installed / 'code'
            pins = (staging_prerequisites.BINARY_PINS if os.environ.get('VOICE_TEST_PINNED_ARCHIVE')
                    else root_composition_fixture.SYNTHETIC_PINS)
            with patch.object(staging_prerequisites, 'BINARY_PINS', pins):
                binding = root_composition_fixture.helper_binding(source, code)
            workspace = installed / 'sources' / ('a' * 12)
            workspace.mkdir(parents=True, mode=0o700)
            # Copy the entire immutable Git archive: source plans and helper
            # custody must see the same tree, including NATS known-baseline.
            shutil.copytree(source, workspace, dirs_exist_ok=True)
            for path in workspace.rglob('*'):
                path.chmod(0o700 if path.is_dir() else 0o600)
            script = r'''
import base64, copy, hashlib, http.client, json, os, subprocess, sys
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
 window='22222222-2222-4222-8222-222222222222',revision=12,operation_id='stg-run-17',commit='c'*40,
 images={'voice-gameintegration/gameintegration':source_authority.REGISTRY+'/gameintegration@sha256:'+'c'*64})
card=dict(code='STAGING',environment='stg',state='held',phase='deploy',owner=c['actor'],claim=c['claim'],
 generation=c['generation'],window=c['window'],revision=c['revision'],operation=2,
 candidate=dict(state='known',commit=c['commit'],images=copy.deepcopy(c['images'])),
 observed=dict(state='known',claim=c['claim'],window=c['window'],operation=1,
 images={'voice-user/user':'registry/user@sha256:'+'b'*64}))
approved={'source_sha':c['commit'],'images':{'gameintegration':c['images']['voice-gameintegration/gameintegration']},
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
  def get(kind,name):
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
   if args[:2]==['apply','--dry-run=server']:return store_kube.run(args,**kwargs)
   return cluster.run(args,**kwargs)
  def capture(argv,**kwargs):
   commands.authority_guard(argv)
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
  class Https:
   sock=None
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
  with patch.object(stage_runtime.Kube,'get',side_effect=get), \
       patch.object(stage_runtime.Kube,'_run',side_effect=run),patch.object(commands,'capture',side_effect=capture), \
       patch.object(compiler,'capture',side_effect=capture), \
       patch.object(http.client,'HTTPSConnection',Https):
   try: producer.prepare(actions,{'mode':'app-only','changed_services':['gameintegration'],
      'gis_first_install':True},approved,execution,workspace)
   except KeyError as exc:
    assert 'VOICE_TEST_OCI' not in os.environ and exc.args==(('node','pmdebook'),), str(exc)
   except Exception as exc:
    raise AssertionError('producer after OCI transport: '+type(exc).__name__+': '+str(exc)) from exc
  assert not (root/'installed'/'cohort-preservation').exists()
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
