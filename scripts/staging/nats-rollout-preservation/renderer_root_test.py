import copy
import unittest
from unittest.mock import Mock,patch
import renderer_root
import renderer_transition
from controller import Blocked
from stage_runtime import HUB

class RendererLiveTests(unittest.TestCase):
    def test_poststart_reads_only_canonical_hub_revisions_and_keeps_owner_template_checks(self):
        for fault in (None,'owner','template'):
            with self.subTest(fault=fault):
                stage,pod,authority=self.fixture()
                original=stage.kube.run.side_effect
                def bounded_read(args):
                    if args[1]=='replicasets':
                        if args!=['get','replicasets','-l','app='+HUB,'-o','json']:
                            raise Blocked('command_output_limit')
                        result=copy.deepcopy(original(args))
                        if fault=='owner':result['items'][0]['metadata']['ownerReferences'][0]['uid']='foreign'
                        if fault=='template':result['items'][0]['spec']['template']['spec']['containers'][0]['image']='foreign'
                        return result
                    return original(args)
                stage.kube.run.side_effect=bounded_read
                with patch.object(renderer_root,'revalidate'),patch.object(renderer_root,'capture',return_value=('d'*64+'  /etc/nats/nats.conf\n').encode()):
                    if fault:
                        with self.assertRaisesRegex(Blocked,'renderer_live_pod_ambiguous'):
                            renderer_root.verify_live(stage,authority)
                    else:self.assertTrue(renderer_root.verify_live(stage,authority)['verified'])
                stage.scale.assert_not_called()

    def test_actual_paused_cas_and_poststart_proof_accept_status_revision(self):
        for phase in ('paused','running'):
            stage,pod,authority=self.fixture()
            expected=copy.deepcopy(stage.snapshots[HUB])
            expected['spec']['replicas']=0 if phase=='paused' else 1
            if phase=='paused':expected['spec']['template']=copy.deepcopy(authority['descriptor']['original_template'])
            stage.snapshots[HUB]=expected
            current=copy.deepcopy(expected);current['metadata']['resourceVersion']='9'
            current['status']={'readyReplicas':0 if phase=='paused' else 1}
            stage.kube.get.side_effect=lambda kind,name: pod if kind=='pod' else current if name==HUB else stage.snapshots[name]
            target=copy.deepcopy(current);target['spec']['template']=copy.deepcopy(authority['descriptor']['target_template']);target['metadata']['resourceVersion']='10'
            stage.kube.cas.return_value=target
            with self.subTest(phase=phase),patch.object(renderer_root,'revalidate'),patch.object(renderer_root,'capture',return_value=('d'*64+'  /etc/nats/nats.conf\n').encode()):
                if phase=='paused':
                    renderer_root.apply_paused(stage,authority)
                    self.assertEqual(stage.kube.cas.call_args.args[1]['metadata']['resourceVersion'],'9')
                    self.assertEqual(stage.kube.cas.call_args.args[2][0],{'op':'test','path':'/spec/replicas','value':0})
                    self.assertEqual(stage.snapshots[HUB],target)
                else:self.assertTrue(renderer_root.verify_live(stage,authority)['verified'])

    def test_real_stage_active_and_hub_running_identity_check_keeps_closed_gate(self):
        import runtime_stage,stat,types,hashlib,uuid,base64
        from resource_claim import InstallationAuthority
        from pathlib import Path
        fake,pod,authority=self.fixture();kube=fake.kube
        expected=dict(fake.expected,namespace_uid='namespace',marker_uid='marker')
        stage=runtime_stage.RolloutStage(kube,'a'*12,expected,Mock())
        stage.snapshots=copy.deepcopy(fake.snapshots);stage.final_path=Path('/selected')
        stage.final_claim={'metadata':{'name':expected['source_claim'],'uid':'pvc'},'spec':{'volumeName':'pv'}}
        stage.final_pv={'metadata':{'uid':'pv'},'spec':{'local':{'path':'/selected'}}}
        marker={'metadata':{'uid':'marker','resourceVersion':'1'},'data':{'phase':'active','generation':expected['generation'],'dataPVC':expected['source_claim']}}
        original_get=kube.get.side_effect
        def get(kind,name):
            if kind=='namespace':return {'metadata':{'uid':'namespace'}}
            if kind=='configmap':return marker
            if kind=='pvc':return stage.final_claim
            if kind=='pv':return stage.final_pv
            return original_get(kind,name)
        kube.get.side_effect=get;authority['input_binding']={'fixed':'private'}
        stage.marker=copy.deepcopy(marker)
        account=b'synthetic-account';installation=InstallationAuthority('owned-isolated-v1',str(uuid.uuid4()),expected['generation'],hashlib.sha256(account).hexdigest(),'b'*64,'c'*64)
        rows={'operator':{'data':{'account.public':base64.b64encode(account).decode()}}}
        with patch.object(runtime_stage,'pv_storage_path',return_value='/selected'),patch('stage_runtime.pv_storage_path',return_value='/selected'),patch.object(runtime_stage.os,'open',return_value=7),patch.object(runtime_stage.os,'close'),patch.object(runtime_stage.os,'fstat',return_value=types.SimpleNamespace(st_uid=65532,st_gid=10000,st_mode=stat.S_IFDIR|0o2770)),patch.object(renderer_root,'inputs',return_value=(rows,authority['input_binding'])),patch.object(renderer_root,'capture',return_value=('d'*64+'  /etc/nats/nats.conf\n').encode()):
            renderer_root.revalidate(kube,stage,authority,installation=installation)
            with self.assertRaises(Blocked):stage.verify_final_storage()
            marker['data']['phase']='rollout-verified';marker['data']['knownRolloutOperation']='a'*12
            self.assertTrue(renderer_root.verify_live(stage,authority,installation=installation)['verified'])
            for gid,mode in ((65532,0o2770),(10000,0o2777)):
                with self.subTest(gid=gid,mode=mode),patch.object(runtime_stage.os,'fstat',return_value=types.SimpleNamespace(st_uid=65532,st_gid=gid,st_mode=stat.S_IFDIR|mode)):
                    with self.assertRaises(Blocked):renderer_root.revalidate(kube,stage,authority,installation=installation)
            stage.final_claim['metadata']['uid']='changed'
            with self.assertRaises(Blocked):renderer_root.revalidate(kube,stage,authority,installation=installation)
            stage.final_claim['metadata']['uid']='pvc'
            marker['data']['generation']='changed'
            with self.assertRaises(Blocked):renderer_root.revalidate(kube,stage,authority,installation=installation)

    def fixture(self):
        repo='ghcr.io/poryadok/voiceroot/nats-hub-config-renderer'
        old=repo+'@sha256:'+'a'*64;target=repo+'@sha256:'+'b'*64;broker='nats@sha256:'+'c'*64
        storage={'pvc_name':'voice-nats-jsdata-d202610040049430b','pvc_uid':'pvc','pv_uid':'pv','generation':'r20260930a4'}
        hub={'apiVersion':'apps/v1','kind':'Deployment','metadata':{'name':HUB,'namespace':'voice-staging','uid':'hub','resourceVersion':'1'},
            'spec':{'replicas':1,'template':{'metadata':{'labels':{'app':HUB}},'spec':{
                'initContainers':[{'name':'nats-config-renderer','image':repo+':old'}],
                'securityContext':{'fsGroup':10000,'fsGroupChangePolicy':'OnRootMismatch'},
                'containers':[{'name':'nats','image':broker,'securityContext':{'runAsUser':10000,'runAsGroup':10000,'runAsNonRoot':True},'volumeMounts':[{'name':'jsdata','mountPath':'/data'}]}],
                'volumes':[{'name':'jsdata','persistentVolumeClaim':{'claimName':storage['pvc_name']}}]}}}}
        descriptor=renderer_transition.plan(hub,old,target,'e'*40,storage)
        hub['spec']['template']=descriptor['target_template'];hub['metadata']['resourceVersion']='3'
        stage=Mock();stage.expected={'source_claim':storage['pvc_name'],'source_claim_uid':'pvc','source_pv_uid':'pv','generation':'r20260930a4'}
        stage.snapshots={HUB:hub,'voice-user':{'spec':{'replicas':0}}}
        rs={'metadata':{'uid':'rs','ownerReferences':[{'kind':'Deployment','uid':'hub'}]},'spec':{'template':copy.deepcopy(hub['spec']['template'])}}
        rs['spec']['template']['metadata']['labels']['pod-template-hash']='new'
        pod={'metadata':{'name':HUB+'-abc-def','uid':'pod','ownerReferences':[{'kind':'ReplicaSet','uid':'rs'}]},
            'status':{'phase':'Running','initContainerStatuses':[{'name':'nats-config-renderer','imageID':target,'state':{'terminated':{'exitCode':0}}}],
                'containerStatuses':[{'name':'nats','imageID':broker,'ready':True}]}}
        stage.kube.get.side_effect=lambda kind,name: pod if kind=='pod' else stage.snapshots[name]
        stage.kube.run.side_effect=lambda args:{'items':[rs] if args[1]=='replicasets' else [pod]}
        authority={'descriptor':descriptor,'broker_image':broker,'output':{'sha256':'d'*64,'bytes':10,'verified':True},
            'image_identities':{'old':{'requested_image':old,'manifest_image':old,'config_sha256':'a'*64},
                'target':{'requested_image':target,'manifest_image':target,'config_sha256':'e'*64}}}
        return stage,pod,authority

    def test_actual_completed_init_and_broker_and_generated_hash_are_required(self):
        stage,pod,authority=self.fixture()
        with patch.object(renderer_root,'revalidate'),patch.object(renderer_root,'capture',return_value=('d'*64+'  /etc/nats/nats.conf\n').encode()) as read:
            result=renderer_root.verify_live(stage,authority)
        self.assertTrue(result['verified']);self.assertEqual(result['pod_uid'],'pod')
        self.assertEqual(read.call_args.args[0][-3:],['/bin/busybox','sha256sum','/etc/nats/nats.conf'])
        self.assertEqual(stage.scale.call_count,0)

    def test_actual_init_failure_wrong_digest_broker_or_config_cannot_release_apps(self):
        for fault in ('exit','init-image','broker-image','config','apps'):
            with self.subTest(fault=fault):
                stage,pod,authority=self.fixture()
                if fault=='exit':pod['status']['initContainerStatuses'][0]['state']['terminated']['exitCode']=1
                if fault=='init-image':pod['status']['initContainerStatuses'][0]['imageID']='other'
                if fault=='broker-image':pod['status']['containerStatuses'][0]['imageID']='other'
                if fault=='apps':stage.snapshots['voice-user']['spec']['replicas']=1
                raw=('f'*64 if fault=='config' else 'd'*64)+'  /etc/nats/nats.conf\n'
                with patch.object(renderer_root,'revalidate'),patch.object(renderer_root,'capture',return_value=raw.encode()):
                    with self.assertRaises(Blocked):renderer_root.verify_live(stage,authority)
                stage.scale.assert_not_called()

    def test_cross_namespace_descriptor_is_not_admitted(self):
        stage,_,authority=self.fixture();hub=copy.deepcopy(stage.snapshots[HUB]);hub['metadata']['namespace']='voice'
        with self.assertRaises(renderer_transition.TransitionError):
            renderer_transition.plan(hub,authority['descriptor']['images']['old'],authority['descriptor']['images']['target'],'e'*40,authority['descriptor']['storage_binding'])

    def test_live_old_index_alias_requires_exact_shared_child_and_config_chain(self):
        stage,pod,authority=self.fixture()
        old=authority['descriptor']['images']['old'];target=authority['descriptor']['images']['target']
        authority['image_identities']['old']['manifest_image']=target
        authority['image_identities']['old']['config_sha256']='e'*64
        pod['status']['initContainerStatuses'][0]['imageID']=old
        with patch.object(renderer_root,'revalidate'),patch.object(renderer_root,'capture',return_value=('d'*64+'  /etc/nats/nats.conf\n').encode()):
            self.assertTrue(renderer_root.verify_live(stage,authority)['verified'])
        self.assertEqual(renderer_root.image_aliases(authority),{old,target})
        for fault in ('config','other-child','config-as-uri','missing-chain','wrong-repo'):
            with self.subTest(fault=fault):
                bad=copy.deepcopy(authority);badpod=copy.deepcopy(pod)
                if fault=='config':bad['image_identities']['old']['config_sha256']='f'*64
                if fault=='other-child':bad['image_identities']['old']['manifest_image']=old
                if fault=='config-as-uri':badpod['status']['initContainerStatuses'][0]['imageID']=target.rsplit('@',1)[0]+'@sha256:'+'e'*64
                if fault=='missing-chain':bad.pop('image_identities')
                if fault=='wrong-repo':bad['image_identities']['old']['manifest_image']='other@sha256:'+'b'*64
                stage.kube.run.side_effect=lambda args:{'items':[{'metadata':{'uid':'rs','ownerReferences':[{'kind':'Deployment','uid':'hub'}]},'spec':{'template':stage.snapshots[HUB]['spec']['template']}}] if args[1]=='replicasets' else [badpod]}
                with patch.object(renderer_root,'revalidate'),patch.object(renderer_root,'capture',return_value=('d'*64+'  /etc/nats/nats.conf\n').encode()):
                    with self.assertRaises(Blocked):renderer_root.verify_live(stage,bad)

    def test_recovery_pins_original_actual_image_and_preserves_every_other_field(self):
        stage,_,authority=self.fixture();stage.snapshots[HUB]['spec']['replicas']=0
        original=copy.deepcopy(stage.snapshots[HUB])
        original['spec']['template']=copy.deepcopy(authority['descriptor']['original_template'])
        stage.original_snapshots={HUB:original}
        stage.old_images={HUB+'/nats-config-renderer':authority['descriptor']['images']['old']}
        with patch.object(renderer_root,'revalidate'):
            reverse=renderer_root.recovery_authority(stage,authority)
        expected=copy.deepcopy(original['spec']['template'])
        expected['spec']['initContainers'][0]['image']=authority['descriptor']['images']['old']
        self.assertEqual(reverse['descriptor']['target_template'],expected)
        self.assertEqual(reverse['output'],authority['output'])
        self.assertEqual(reverse['broker_image'],authority['broker_image'])
        self.assertEqual(reverse['descriptor']['hub']['resourceVersion'],'3')

    def test_recovery_rejects_unbound_original_or_actual_image_or_current_metadata(self):
        for fault in ('original','old-image','current-metadata'):
            with self.subTest(fault=fault):
                stage,_,authority=self.fixture();stage.snapshots[HUB]['spec']['replicas']=0
                original=copy.deepcopy(stage.snapshots[HUB])
                original['spec']['template']=copy.deepcopy(authority['descriptor']['original_template'])
                stage.original_snapshots={HUB:original}
                stage.old_images={HUB+'/nats-config-renderer':authority['descriptor']['images']['old']}
                if fault=='original':original['spec']['template']['spec']['containers'][0]['image']='changed'
                if fault=='old-image':stage.old_images[HUB+'/nats-config-renderer']='other'
                if fault=='current-metadata':
                    current=copy.deepcopy(stage.snapshots[HUB]);current['metadata']['uid']='foreign-hub-uid'
                    stage.kube.get.side_effect=lambda *args:current
                with patch.object(renderer_root,'revalidate'):
                    with self.assertRaises((Blocked,renderer_transition.TransitionError)):
                        renderer_root.recovery_authority(stage,authority)


class RendererImageAcquisitionTests(unittest.TestCase):
    def runtime(self,base,image,*,cached=False,pull_fails=False,wrong_digest=False):
        import json
        class Runtime:
            operation='renderer123456'
            owned={}
            def __init__(self):
                self.base=base;self.calls=[];self.cached=cached;self.owned={}
            def run(self,args,**kwargs):
                self.calls.append((list(args),kwargs))
                if args[:2]==['image','inspect']:
                    if not self.cached:raise Blocked('command_failed')
                    return json.dumps([{'Id':'sha256:'+'c'*64,
                        'RepoDigests':[image if not wrong_digest else image.replace('a'*64,'b'*64)]}])
                if args[0]=='pull':
                    if pull_fails:raise Blocked('command_failed')
                    self.cached=True;return image
                if args[0]=='create':return 'd'*64
                raise AssertionError(args)
            def inspect(self,name):
                entry=self.owned[name]
                assert entry['network']=='none' and entry['image']==image
                return {}
        return Runtime()

    def mounts(self,base):
        targets=('input','template','jwt','tls','out')
        paths=[]
        for name in targets:
            path=base/name;path.mkdir();paths.append(path)
        return [(paths[0],'/run/nats/input',False),
            (paths[1],'/run/nats/template',False),(paths[2],'/etc/nats/jwt',False),
            (paths[3],'/etc/nats/tls',False),(paths[4],'/run/nats-rendered',True)]

    def test_actual_create_fetches_missing_exact_renderer_then_reinspects_before_network_none_container(self):
        import tempfile
        from pathlib import Path
        import renderer_execution
        image='ghcr.io/poryadok/voiceroot/nats-hub-config-renderer@sha256:'+'a'*64
        for cached in (False,True):
            with self.subTest(cached=cached),tempfile.TemporaryDirectory() as td:
                base=Path(td).resolve();runtime=self.runtime(base,image,cached=cached)
                name=renderer_execution.create(runtime,image,self.mounts(base),'target')
                self.assertIn(name,runtime.owned)
                commands=[args for args,_ in runtime.calls]
                if cached:
                    self.assertFalse(any(args[0]=='pull' for args in commands))
                else:
                    self.assertEqual(commands[:3],[['image','inspect',image],['pull',image],['image','inspect',image]])
                    pull_options=runtime.calls[1][1]
                    self.assertGreater(pull_options['timeout'],0)
                    self.assertLessEqual(pull_options['timeout'],120)
                create=commands[-1]
                self.assertEqual(create[create.index('--network')+1],'none')
                self.assertEqual(create[create.index('--entrypoint')+1],'/nats-hub-config-renderer')
                self.assertIn(image,create)
                self.assertEqual(runtime.owned[name]['image_id'],'sha256:'+'c'*64)

    def test_fetch_failure_or_wrong_fetched_digest_never_creates_and_unscoped_image_never_pulls(self):
        import tempfile
        from pathlib import Path
        import renderer_execution
        image='ghcr.io/poryadok/voiceroot/nats-hub-config-renderer@sha256:'+'a'*64
        for fault in ('pull-fails','wrong-digest','wrong-repository','tag'):
            with self.subTest(fault=fault),tempfile.TemporaryDirectory() as td:
                base=Path(td).resolve()
                runtime=self.runtime(base,image,pull_fails=fault=='pull-fails',wrong_digest=fault=='wrong-digest')
                attempted=image
                if fault=='wrong-repository':attempted=image.replace('/nats-hub-config-renderer','/story')
                if fault=='tag':attempted=image.split('@')[0]+':latest'
                with self.assertRaises(Blocked):
                    renderer_execution.create(runtime,attempted,self.mounts(base),'target')
                self.assertFalse(any(args[0]=='create' for args,_ in runtime.calls))
                if fault in ('pull-fails','wrong-digest'):
                    self.assertEqual([args for args,_ in runtime.calls if args[0]=='pull'],[['pull',image]])
                else:self.assertEqual(runtime.calls,[])


class RendererScratchCustodyTests(unittest.TestCase):
    def test_actual_prove_keeps_fixed_group_read_custody_under_service_umask(self):
        import base64,os,stat,tempfile,hashlib,uuid
        from resource_claim import InstallationAuthority
        from pathlib import Path
        stage,_,authority=RendererLiveTests().fixture()
        hub=stage.snapshots[HUB]
        stage.old_images={HUB+'/nats-config-renderer':authority['descriptor']['images']['old'],
            HUB+'/nats':authority['broker_image']}
        tokens={key:('synthetic-'+key).encode() for key in
            ('operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public')}
        installation=InstallationAuthority('owned-isolated-v1',str(uuid.uuid4()),stage.expected['generation'],hashlib.sha256(tokens['account.public']).hexdigest(),'b'*64,'c'*64)
        tls={key:('synthetic-'+key).encode() for key in ('tls.crt','tls.key','ca.crt')}
        encode=lambda values:{'data':{key:base64.b64encode(value).decode() for key,value in values.items()}}
        rows={'operator':encode(tokens),'tls':encode(tls),
            'template':{'data':{'nats.conf':'synthetic renderer template\n'}}}
        observed=[]
        def isolated(base,descriptor,dirs,unchanged):
            unchanged()
            self.assertEqual(set(dirs),{'input','jwt','tls','template'})
            expected={'input':tokens,'jwt':{'operator.jwt':tokens['operator.jwt']},
                'tls':tls,'template':{'nats.conf':b'synthetic renderer template\n'}}
            for name,path in dirs.items():
                info=path.stat()
                self.assertEqual((info.st_uid,info.st_gid,stat.S_IMODE(info.st_mode)),(0,65532,0o750))
                self.assertEqual({p.name for p in path.iterdir()},set(expected[name]))
                for filename,wanted in expected[name].items():
                    item=path/filename;info=item.stat()
                    self.assertEqual((info.st_uid,info.st_gid,stat.S_IMODE(info.st_mode),info.st_nlink),
                        (0,65532,0o440,1))
                    self.assertEqual(item.read_bytes(),wanted)
            observed.append(True)
            return {'verified':True,'sha256':'d'*64,'bytes':10}
        with tempfile.TemporaryDirectory() as td:
            prior=os.umask(0o077)
            try:
                with patch.object(renderer_root.guard,'ROOT',Path(td)), \
                     patch.object(renderer_root,'storage',return_value=authority['descriptor']['storage_binding']), \
                     patch.object(renderer_root,'inputs',return_value=(rows,{})), \
                     patch.object(renderer_root,'image_identity',return_value={}), \
                     patch.object(renderer_root,'revalidate'), \
                     patch.object(renderer_root.renderer_execution,'prove',side_effect=isolated):
                    result=renderer_root.prove(stage.kube,stage,'e'*40,authority['descriptor']['images']['target'],installation=installation)
                self.assertTrue(result['output']['verified'])
                self.assertEqual(observed,[True])
                self.assertEqual(list(Path(td).iterdir()),[])
            finally:os.umask(prior)


if __name__=='__main__':unittest.main()
