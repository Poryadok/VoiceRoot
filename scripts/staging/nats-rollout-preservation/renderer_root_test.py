import copy
import unittest
from unittest.mock import Mock,patch
import renderer_root
import renderer_transition
from controller import Blocked
from stage_runtime import HUB

class RendererLiveTests(unittest.TestCase):
    def test_real_stage_active_and_hub_running_identity_check_keeps_closed_gate(self):
        import runtime_stage,stat,types
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
        with patch.object(runtime_stage,'pv_storage_path',return_value='/selected'),patch('stage_runtime.pv_storage_path',return_value='/selected'),patch.object(runtime_stage.os,'open',return_value=7),patch.object(runtime_stage.os,'close'),patch.object(runtime_stage.os,'fstat',return_value=types.SimpleNamespace(st_uid=65532,st_gid=10000,st_mode=stat.S_IFDIR|0o2770)),patch.object(renderer_root,'inputs',return_value=({},authority['input_binding'])),patch.object(renderer_root,'capture',return_value=('d'*64+'  /etc/nats/nats.conf\n').encode()):
            renderer_root.revalidate(kube,stage,authority)
            with self.assertRaises(Blocked):stage.verify_final_storage()
            marker['data']['phase']='rollout-verified';marker['data']['knownRolloutOperation']='a'*12
            self.assertTrue(renderer_root.verify_live(stage,authority)['verified'])
            for gid,mode in ((65532,0o2770),(10000,0o2777)):
                with self.subTest(gid=gid,mode=mode),patch.object(runtime_stage.os,'fstat',return_value=types.SimpleNamespace(st_uid=65532,st_gid=gid,st_mode=stat.S_IFDIR|mode)):
                    with self.assertRaises(Blocked):renderer_root.revalidate(kube,stage,authority)
            stage.final_claim['metadata']['uid']='changed'
            with self.assertRaises(Blocked):renderer_root.revalidate(kube,stage,authority)
            stage.final_claim['metadata']['uid']='pvc'
            marker['data']['generation']='changed'
            with self.assertRaises(Blocked):renderer_root.revalidate(kube,stage,authority)

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

    def test_recovery_rejects_unbound_original_or_actual_image_or_current_rv(self):
        for fault in ('original','old-image','current-rv'):
            with self.subTest(fault=fault):
                stage,_,authority=self.fixture();stage.snapshots[HUB]['spec']['replicas']=0
                original=copy.deepcopy(stage.snapshots[HUB])
                original['spec']['template']=copy.deepcopy(authority['descriptor']['original_template'])
                stage.original_snapshots={HUB:original}
                stage.old_images={HUB+'/nats-config-renderer':authority['descriptor']['images']['old']}
                if fault=='original':original['spec']['template']['spec']['containers'][0]['image']='changed'
                if fault=='old-image':stage.old_images[HUB+'/nats-config-renderer']='other'
                if fault=='current-rv':
                    current=copy.deepcopy(stage.snapshots[HUB]);current['metadata']['resourceVersion']='99'
                    stage.kube.get.side_effect=lambda *args:current
                with patch.object(renderer_root,'revalidate'):
                    with self.assertRaises((Blocked,renderer_transition.TransitionError)):
                        renderer_root.recovery_authority(stage,authority)

if __name__=='__main__':unittest.main()
