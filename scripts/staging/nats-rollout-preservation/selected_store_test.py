import copy
import stat
import types
import unittest
from pathlib import Path
from unittest.mock import patch,Mock
import runtime_stage
from docker_runtime import DockerRuntime
from stage_runtime import selected_store_descriptor, verify_selected_store_leaf, Blocked, HUB


class SelectedStoreCustodyTests(unittest.TestCase):
    def test_private_broker_and_clients_never_select_hub_identity_or_copy_inputs(self):
        runtime=object.__new__(DockerRuntime);runtime.base=Path('/private-proof');runtime.selected_stores={}
        runtime.create=Mock(return_value='owned-private');runtime.run=Mock();runtime.inspect=Mock()
        runtime.selected_broker_config=Mock(side_effect=AssertionError('private copy must not select hub inputs'))
        runtime.start_broker('private',runtime.base/'store')
        self.assertEqual(runtime.create.call_args.kwargs,{})
        self.assertEqual(runtime.create.call_args.args[2],[(runtime.base/'store','/data',True),(runtime.base/'server.conf','/server.conf',False)])
        runtime.selected_broker_config.assert_not_called()

    def test_selected_config_reader_rejects_wrong_owner_group_mode_link_or_changed_inode(self):
        def metadata(uid=0,gid=10000,mode=0o440,nlink=1,ino=2):
            return types.SimpleNamespace(st_mode=stat.S_IFREG|mode,st_uid=uid,st_gid=gid,st_nlink=nlink,st_size=1,
                st_dev=1,st_ino=ino,st_mtime_ns=3,st_ctime_ns=4)
        for row in (metadata(uid=1000),metadata(gid=65532),metadata(mode=0o444),metadata(nlink=2)):
            with patch('docker_runtime.os.open',return_value=8),patch('docker_runtime.os.fstat',return_value=row),patch('docker_runtime.os.close'),self.assertRaises(Blocked):
                DockerRuntime.selected_config_bytes('/fixed',10000)
        with patch('docker_runtime.os.open',return_value=8),patch('docker_runtime.os.fstat',side_effect=[metadata(),metadata(ino=3)]),\
             patch('docker_runtime.os.read',return_value=b'x'),patch('docker_runtime.os.close'),self.assertRaises(Blocked):
            DockerRuntime.selected_config_bytes('/fixed',10000)

    def binding(self):
        claim={'metadata':{'uid':'11111111-2222-3333-4444-555555555555','name':'selected'}}
        pv={'metadata':{'uid':'66666666-7777-8888-9999-aaaaaaaaaaaa'},'spec':{
            'claimRef':{'uid':claim['metadata']['uid']},'local':{'path':'/var/lib/rancher/k3s/storage/pvc-'+claim['metadata']['uid']+'_voice-staging_selected'},
            'nodeAffinity':{'required':{'nodeSelectorTerms':[{'matchExpressions':[{'key':'kubernetes.io/hostname','operator':'In','values':['pmdebook']}]}]}}}}
        hub={'metadata':{'name':HUB,'namespace':'voice-staging','uid':'captured-hub'},'spec':{'template':{'spec':{
            'securityContext':{'fsGroup':10000,'fsGroupChangePolicy':'OnRootMismatch'},
            'containers':[{'name':'nats','securityContext':{'runAsUser':10000,'runAsGroup':10000,'runAsNonRoot':True},
                'volumeMounts':[{'name':'jsdata','mountPath':'/data'}]}],
            'volumes':[{'name':'jsdata','persistentVolumeClaim':{'claimName':'selected'}}]}}}}
        return hub,claim,pv

    def test_captured_kubernetes_fsgroup_leaf_and_exact_descriptor(self):
        descriptor=selected_store_descriptor(*self.binding())
        verify_selected_store_leaf(types.SimpleNamespace(st_uid=65532,st_gid=10000,st_mode=stat.S_IFDIR|0o2770),descriptor)
        for uid,gid,mode in ((0,10000,0o2770),(65532,65532,0o2770),(65532,10000,0o2777),
                             (65532,10000,0o2775),(65532,10000,0o770),(65532,10000,0o6770),(65532,10000,0o700)):
            with self.subTest(uid=uid,gid=gid,mode=mode),self.assertRaises(Blocked):
                verify_selected_store_leaf(types.SimpleNamespace(st_uid=uid,st_gid=gid,st_mode=stat.S_IFDIR|mode),descriptor)
        altered=copy.deepcopy(descriptor);altered['mode']=0o2777
        with self.assertRaises(Blocked):verify_selected_store_leaf(types.SimpleNamespace(st_uid=65532,st_gid=10000,st_mode=stat.S_IFDIR|0o2777),altered)

    def test_wrong_security_context_or_storage_authority_rejects(self):
        for mutation in ('group','policy','broker','mount','claim','namespace','pv'):
            hub,claim,pv=self.binding();pod=hub['spec']['template']['spec']
            if mutation=='group':pod['securityContext']['fsGroup']=65532
            if mutation=='policy':pod['securityContext']['fsGroupChangePolicy']='Always'
            if mutation=='broker':pod['containers'][0]['securityContext']['runAsUser']=0
            if mutation=='mount':pod['containers'][0]['volumeMounts'][0]['readOnly']=True
            if mutation=='claim':pod['volumes'][0]['persistentVolumeClaim']['claimName']='other'
            if mutation=='namespace':hub['metadata']['namespace']='other'
            if mutation=='pv':pv['spec']['local']['path']+='/other'
            with self.subTest(mutation=mutation),self.assertRaises(Blocked):selected_store_descriptor(hub,claim,pv)

    def stage(self):
        hub,claim,pv=self.binding();claim['spec']={'volumeName':'selected-pv'}
        marker={'metadata':{'uid':'marker'},'data':{'generation':'generation','dataPVC':'selected'}}
        namespace={'metadata':{'uid':'namespace'}}
        kube=Mock();kube.get.side_effect=lambda kind,name:copy.deepcopy({'namespace':namespace,'configmap':marker,'pvc':claim,'pv':pv,'deployment':hub}[kind])
        stage=runtime_stage.RolloutStage(kube,'123456abcdef',{},lambda event:None)
        stage.expected={'namespace_uid':'namespace','marker_uid':'marker','generation':'generation',
                        'source_claim':'selected','source_claim_uid':claim['metadata']['uid'],'source_pv_uid':pv['metadata']['uid']}
        stage.snapshots={HUB:hub};stage.final_claim=claim;stage.final_pv=pv
        stage.final_path=Path(pv['spec']['local']['path'])
        return stage

    def test_real_stage_leaf_traversal_and_docker_registration_share_authority(self):
        stage=self.stage();leaf=types.SimpleNamespace(st_uid=65532,st_gid=10000,st_mode=stat.S_IFDIR|0o2770)
        descriptor=stage.selected_store_descriptor();depth=len(stage.final_path.parts)-1
        opened=[]
        def open_fd(name,*args,**kwargs):opened.append(name);return len(opened)
        def fstat(fd):return leaf if fd==depth+1 else types.SimpleNamespace(st_uid=0,st_gid=0,st_mode=stat.S_IFDIR|0o755)
        with patch.object(runtime_stage.os,'open',side_effect=open_fd),patch.object(runtime_stage.os,'close'),patch.object(runtime_stage.os,'fstat',side_effect=fstat):
            stage.verify_selected_storage_identity()
        runtime=object.__new__(DockerRuntime);runtime.base=Path('/private-proof');runtime.new_stores=set()
        with patch.object(Path,'resolve',lambda p,**kwargs:p),patch.object(Path,'lstat',return_value=leaf):
            runtime.allow_bound_store(stage.final_path,stage.final_claim,stage.final_pv,Mock(),descriptor=descriptor)
        self.assertEqual(runtime.new_stores,{str(stage.final_path)})
        for bad in (types.SimpleNamespace(st_uid=65532,st_gid=10001,st_mode=stat.S_IFDIR|0o2770),
                    types.SimpleNamespace(st_uid=65532,st_gid=10000,st_mode=stat.S_IFDIR|0o2777)):
            opened.clear();leaf=bad
            with patch.object(runtime_stage.os,'open',side_effect=open_fd),patch.object(runtime_stage.os,'close'),patch.object(runtime_stage.os,'fstat',side_effect=fstat),self.assertRaises(Blocked):
                stage.verify_selected_storage_identity()
            with patch.object(Path,'resolve',lambda p,**kwargs:p),patch.object(Path,'lstat',return_value=bad),self.assertRaises(Blocked):
                runtime.allow_bound_store(stage.final_path,stage.final_claim,stage.final_pv,Mock(),descriptor=descriptor)

    def test_current_hub_template_drift_rejects_before_any_store_open(self):
        stage=self.stage();original=stage.kube.get.side_effect
        def changed(kind,name):
            value=original(kind,name)
            if kind=='deployment':value['spec']['template']['spec']['securityContext']['fsGroup']=65532
            return value
        stage.kube.get.side_effect=changed
        with patch.object(runtime_stage.os,'open') as opened,self.assertRaisesRegex(Blocked,'hub_authority_changed'):
            stage.verify_selected_storage_identity()
        opened.assert_not_called()


if __name__=='__main__':unittest.main()
