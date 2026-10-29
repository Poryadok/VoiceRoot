"""Fixed STAGING cut adapter; no caller-supplied transport or authority."""
import copy
import json
import re
import time
import guard
import staging_cohort_preservation as owner
from controller import Blocked
from apply import digest
from stage_runtime import Kube, HUB
from runtime_stage import RolloutStage, terminated_hub


class CohortKube(Kube):
    def __init__(self,session):
        self.session=session
        super().__init__()

    def _run(self,args,body=None,timeout=30):
        if args==['version','-o','json'] and body is None:return owner.version_read(self.session)
        return Kube._run(args,body=body,timeout=timeout)

    def cas(self,kind,row,changes):
        return owner.cut_cas(self.session,kind,row,changes)


class CohortStage(RolloutStage):
    @classmethod
    def from_captured(cls,session,stage,journal):
        if type(stage) is not RolloutStage:raise Blocked('cohort_actual_stage_required')
        obj=cls(CohortKube(session),stage.operation,copy.deepcopy(stage.expected),journal)
        obj.session=session
        for name in ('snapshots','original_snapshots','old_images','marker','service','final_claim','final_pv','final_path'):
            setattr(obj,name,copy.deepcopy(getattr(stage,name)))
        return obj

    def shutdown_original_hub(self):
        """Same retained-Pod orderly-exit proof as RolloutStage, fixed transport."""
        from docker_runtime import DockerRuntime,NATS_IMAGE
        from rollout_census import census
        version=self.kube.run(['version','-o','json'])['serverVersion']
        if int(version['major'])!=1 or int(version['minor'].rstrip('+'))<27:
            raise Blocked('rollout_retained_pod_version_unsupported')
        hub=self.kube.get('deployment',HUB);expected=self.snapshots[HUB]
        if hub['metadata']['uid']!=expected['metadata']['uid'] or hub['spec']['template']!=expected['spec']['template']:
            raise Blocked('rollout_original_hub_changed')
        pods=self.kube.run(['get','pods','-o','json'])['items']
        selected=[p for p in pods if any(v.get('persistentVolumeClaim',{}).get('claimName')==self.expected['source_claim']
            for v in p['spec'].get('volumes',[]))]
        if len(selected)!=1:raise Blocked('rollout_original_hub_inventory_invalid')
        pod=selected[0];meta=pod['metadata'];spec=pod['spec'];statuses=pod['status'].get('containerStatuses',[])
        refs=meta.get('ownerReferences',[])
        if (meta.get('deletionTimestamp') or pod['status'].get('phase')!='Running' or len(refs)!=1
            or refs[0].get('kind')!='ReplicaSet' or refs[0].get('controller') is not True
            or len(spec['containers'])!=1 or spec['containers'][0]['name']!='nats'
            or spec['containers'][0]['image']!=NATS_IMAGE or len(statuses)!=1):
            raise Blocked('rollout_original_hub_inventory_invalid')
        rs=self.kube.get('replicaset',refs[0]['name'])
        if (rs['metadata']['uid']!=refs[0]['uid'] or not any(o.get('kind')=='Deployment'
            and o.get('uid')==hub['metadata']['uid'] and o.get('controller') is True for o in rs['metadata'].get('ownerReferences',[]))):
            raise Blocked('rollout_original_hub_owner_changed')
        current=statuses[0];runtime_image=current.get('imageID','').removeprefix('docker-pullable://')
        fixed_images={NATS_IMAGE,'docker.io/library/nats@'+NATS_IMAGE.split('@',1)[1]}
        if (current.get('name')!='nats' or current.get('ready') is not True
            or not re.fullmatch('containerd://[a-f0-9]{64}',current.get('containerID','')) or runtime_image not in fixed_images
            or type(current.get('restartCount')) is not int or set(current.get('state',{}))!={'running'}):
            raise Blocked('rollout_original_hub_container_changed')
        node=self.kube.get('node',spec['nodeName'])
        if node['metadata'].get('deletionTimestamp') or not any(c.get('type')=='Ready' and c.get('status')=='True' for c in node.get('status',{}).get('conditions',[])):
            raise Blocked('rollout_original_hub_node_not_ready')
        def identity():
            self.owned_marker();live=self.kube.get('pod',meta['name'])
            matches=[s for s in live['status'].get('containerStatuses',[]) if s['name']=='nats']
            if live['metadata']['uid']!=meta['uid'] or live['metadata'].get('deletionTimestamp') or matches!=statuses:
                raise Blocked('rollout_original_hub_changed')
        identity();tree=json.loads(owner.monitor_capture(self.session,meta['name']));identity()
        class Snapshot:
            def monitor_jsz(self,name):return tree
        self.original_live_census=census(Snapshot(),'original',DockerRuntime(self.runtime_base,self.operation).account_id())
        finalizer='voice.io/nats-preservation-'+self.operation;existing=meta.get('finalizers',[])
        if finalizer in existing:raise Blocked('rollout_original_hub_finalizer_already_present')
        self.save({'kind':'original_hub_shutdown_intent','pod_uid':meta['uid'],'container_id':current['containerID'],
            'finalizer':finalizer,'live_census_sha256':digest(self.original_live_census)})
        self.kube.cas('pod',pod,[{'op':'add','path':'/metadata/finalizers','value':existing+[finalizer]}])
        self.scale(HUB,0);deadline=time.monotonic()+90
        while time.monotonic()<deadline:
            self.owned_marker();retained=self.kube.get('pod',meta['name'])
            if retained['metadata']['uid']!=meta['uid'] or finalizer not in retained['metadata'].get('finalizers',[]):
                raise Blocked('rollout_original_hub_retained_pod_changed')
            status=[s for s in retained['status'].get('containerStatuses',[]) if s['name']=='nats']
            if len(status)!=1 or status[0].get('containerID')!=current['containerID'] or status[0].get('restartCount')!=current['restartCount']:
                raise Blocked('rollout_original_hub_container_changed')
            if 'terminated' in status[0].get('state',{}):
                proof=terminated_hub(pod,retained)
                self.save({'kind':'original_hub_orderly_exit_verified','proof':proof})
                finals=retained['metadata']['finalizers'];index=finals.index(finalizer)
                self.kube.cas('pod',retained,[{'op':'test','path':'/metadata/finalizers','value':finals},
                    {'op':'remove','path':'/metadata/finalizers/'+str(index)}])
                self.save({'kind':'original_hub_owned_finalizer_removed','pod_uid':meta['uid']});return proof
            time.sleep(.2)
        raise Blocked('rollout_original_hub_shutdown_timeout')
