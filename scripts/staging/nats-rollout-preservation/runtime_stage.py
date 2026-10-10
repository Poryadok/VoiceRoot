"""Root-owned rollout adapter reusing the tested physical fence/startup cycle.

This adapter has no PVC allocation, selection, bootstrap or archive replay path.
The same live-selected claim is retained through target application and rollback.
"""
import copy
import os
import re
from pathlib import Path
import stat
import datetime as dt
import json
import time
import guard
from stage_runtime import Staging, HUB, MARKER, LEAVES, pv_storage_path, selected_store_descriptor, verify_selected_store_leaf, fence_objects
from controller import Blocked
from apply import digest


def terminated_hub(before,after):
    """Current kubelet termination of the exact retained original container."""
    try:
        old=[s for s in before['status']['containerStatuses'] if s['name']=='nats']
        new=[s for s in after['status']['containerStatuses'] if s['name']=='nats']
        if (len(old)!=1 or len(new)!=1 or before['metadata']['uid']!=after['metadata']['uid']
            or not after['metadata'].get('deletionTimestamp')
            or old[0]['containerID']!=new[0]['containerID']
            or old[0]['restartCount']!=new[0]['restartCount']
            or set(old[0]['state'])!={'running'} or set(new[0]['state'])!={'terminated'}):
            raise ValueError()
        stopped=new[0]['state']['terminated'];started=old[0]['state']['running']['startedAt']
        opening=dt.datetime.fromisoformat(started.replace('Z','+00:00'))
        closing=dt.datetime.fromisoformat(stopped['finishedAt'].replace('Z','+00:00'))
        if (type(stopped.get('exitCode')) is not int or stopped['exitCode']!=0
            or stopped.get('signal',0)!=0 or stopped.get('reason')=='OOMKilled'
            or stopped.get('startedAt')!=started or opening.tzinfo is None
            or closing.tzinfo is None or closing<opening):raise ValueError()
        return {'schema':'voice-original-hub-orderly-exit-v1','pod_uid':before['metadata']['uid'],
            'container_id':old[0]['containerID'],'restart_count':old[0]['restartCount'],
            'started_at':started,'finished_at':stopped['finishedAt'],'exit_code':0,
            'source':'KUBELET_CURRENT_TERMINATED_RETAINED_POD'}
    except (KeyError,TypeError,ValueError):raise Blocked('rollout_original_hub_orderly_exit_invalid') from None

IMAGE_TEMPLATE=('{"items":[{{range $i,$r := .items}}{{if $i}},{{end}}'
    '{"kind":{{printf "%q" $r.kind}},"metadata":{"name":{{printf "%q" $r.metadata.name}},"uid":{{printf "%q" $r.metadata.uid}},'
    '"ownerReferences":[{{range $j,$o := $r.metadata.ownerReferences}}{{if $j}},{{end}}'
    '{"uid":{{printf "%q" $o.uid}},"kind":{{printf "%q" $o.kind}}}{{end}}]},'
    '"status":{"phase":{{if $r.status.phase}}{{printf "%q" $r.status.phase}}{{else}}""{{end}},'
    '"containerStatuses":[{{range $j,$c := $r.status.containerStatuses}}{{if $j}},{{end}}'
    '{"name":{{printf "%q" $c.name}},"ready":{{$c.ready}},"imageID":{{printf "%q" $c.imageID}}}{{end}}],'
    '"initContainerStatuses":[{{range $j,$c := $r.status.initContainerStatuses}}{{if $j}},{{end}}'
    '{"name":{{printf "%q" $c.name}},"imageID":{{printf "%q" $c.imageID}}}{{end}}]}}{{end}}]}')

def revision_template(template):
    result=copy.deepcopy(template)
    result.get('metadata',{}).get('labels',{}).pop('pod-template-hash',None)
    return result

def running_image_pins(deployments,rows,*,include_hub=False):
    if not isinstance(rows,list) or len(rows)>2048:raise Blocked('rollout_old_image_inventory_invalid')
    pins={}
    for name,deployment in deployments.items():
        if name==HUB and not include_hub:continue
        replicasets={r['metadata']['uid'] for r in rows if r['kind']=='ReplicaSet' and
            any(o.get('kind')=='Deployment' and o['uid']==deployment['metadata']['uid'] for o in r['metadata'].get('ownerReferences',[]))}
        pods=[r for r in rows if r['kind']=='Pod' and r.get('status',{}).get('phase')=='Running' and
            any(o.get('kind')=='ReplicaSet' and o['uid'] in replicasets for o in r['metadata'].get('ownerReferences',[]))]
        ready_rs={o['uid'] for pod in pods for o in pod['metadata'].get('ownerReferences',[]) if o.get('kind')=='ReplicaSet'}
        for rs in rows:
            if rs['kind']=='ReplicaSet' and rs['metadata']['uid'] in ready_rs:
                if revision_template(rs.get('spec',{}).get('template',{}))!=revision_template(deployment['spec']['template']):
                    raise Blocked('rollout_old_revision_unconverged')
        spec=deployment['spec']['template']['spec']
        for container in spec.get('initContainers',[])+spec['containers']:
            candidates=set()
            init=container in spec.get('initContainers',[])
            for pod in pods:
                statuses=pod['status'].get('initContainerStatuses' if init else 'containerStatuses',[])
                for status in statuses:
                    if status['name']!=container['name'] or (not init and status.get('ready') is not True):continue
                    image=status.get('imageID','').removeprefix('docker-pullable://')
                    if not re.fullmatch(r'[a-z0-9][a-zA-Z0-9._/:~-]{1,255}@sha256:[a-f0-9]{64}',image):
                        raise Blocked('rollout_old_image_identity_unusable')
                    candidates.add(image)
            if len(candidates)!=1:raise Blocked('rollout_old_image_ambiguous')
            pins[name+'/'+container['name']]=candidates.pop()
    return pins


class RolloutStage(Staging):
    @classmethod
    def capture(cls,kube,operation,save,extra_deployments=()):
        namespace=kube.get('namespace',guard.NS);marker=kube.get('configmap',MARKER)
        data=marker['data']
        if data.get('phase')!='active' or any(k in data for k in ('knownBaselineOperation','knownRolloutOperation')):
            raise Blocked('rollout_active_marker_required')
        extras=set(extra_deployments)-{HUB,'voice-gateway',*('voice-'+s for s in LEAVES)}
        if not extras<={'voice-web','voice-admin','voice-developer-portal'}:raise Blocked('rollout_extra_workload_unsupported')
        rows={n:kube.get('deployment',n) for n in (HUB,'voice-gateway',*('voice-'+s for s in LEAVES),*sorted(extras))}
        volumes=rows[HUB]['spec']['template']['spec']['volumes']
        claims=[v.get('persistentVolumeClaim',{}).get('claimName') for v in volumes if v['name']=='jsdata']
        if len(claims)!=1 or not claims[0] or data.get('dataPVC')!=claims[0]:
            raise Blocked('rollout_explicit_selected_claim_required')
        claim=kube.get('pvc',claims[0]);pv=kube.get('pv',claim['spec']['volumeName'])
        path=pv_storage_path(pv,claim['metadata']['uid'],claims[0],pv['metadata']['uid'])
        expected={'namespace_uid':namespace['metadata']['uid'],'marker_uid':marker['metadata']['uid'],
                  'generation':data['generation'],'source_claim':claims[0],
                  'source_claim_uid':claim['metadata']['uid'],'source_pv_uid':pv['metadata']['uid'],
                  'deployment_uids':{n:r['metadata']['uid'] for n,r in rows.items()}}
        stage=cls(kube,operation,expected,save)
        for name in extras:
            if rows[name]['spec'].get('replicas',1)!=1:raise Blocked('rollout_extra_replica_baseline_unsupported')
            stage.snapshots[name]=rows[name]
        stage.preflight();stage.original_snapshots=copy.deepcopy(stage.snapshots)
        image_rows=kube.run(['get','pods,replicasets','-o','go-template='+IMAGE_TEMPLATE])['items']
        if not isinstance(image_rows,list) or len(image_rows)>2048:raise Blocked('rollout_old_image_inventory_invalid')
        owners={r['metadata']['uid'] for r in stage.original_snapshots.values()}
        running_rs={o['uid'] for row in image_rows if row['kind']=='Pod' and row['status'].get('phase')=='Running' for o in row['metadata'].get('ownerReferences',[]) if o.get('kind')=='ReplicaSet'}
        for row in image_rows:
            if row['kind']!='ReplicaSet' or row['metadata']['uid'] not in running_rs or not any(o.get('kind')=='Deployment' and o['uid'] in owners for o in row['metadata'].get('ownerReferences',[])):continue
            actual=kube.get('replicaset',row['metadata']['name'])
            if actual['metadata']['uid']!=row['metadata']['uid']:raise Blocked('rollout_old_revision_identity_changed')
            row['spec']={'template':actual['spec']['template']}
        stage.old_images=running_image_pins(stage.original_snapshots,image_rows,include_hub=HUB in extra_deployments)
        stage.final_claim=claim;stage.final_pv=pv;stage.final_path=Path(path)
        save({'kind':'rollout_original_capture','snapshots':stage.original_snapshots,
              'marker':stage.marker,'service':stage.service,'expected':expected,'store':path,'old_images':stage.old_images})
        return stage

    def fence(self):
        self.maintenance()
        clients=tuple(n for n in self.snapshots if n!=HUB)
        for name in clients:self.scale(name,0)
        self.drain_client_pods(clients)
        self.shutdown_original_hub()
        names=clients+(HUB,)
        self.no_pods(names);self.verify_closed()
        return {'verified':True,'workloads':list(names),'hub_zero_pods':True}

    def drain_client_pods(self,names,timeout=90):
        deadline=time.monotonic()+timeout;wanted={self.snapshots[n]['metadata']['uid'] for n in names}
        while time.monotonic()<deadline:
            self.owned_marker()
            for name in names:
                row=self.kube.get('deployment',name);expected=self.snapshots[name]
                if (row['metadata']['uid']!=expected['metadata']['uid'] or row['spec'].get('replicas',1)!=0
                    or row['spec']['template']!=expected['spec']['template']):raise Blocked('rollout_client_fence_changed')
            rows=fence_objects(self.kube)
            rs={r['metadata']['uid'] for r in rows if r['kind']=='ReplicaSet'
                and any(o['uid'] in wanted for o in r['metadata'].get('ownerReferences',[]))}
            if not any(r['kind']=='Pod' and any(o['uid'] in rs for o in r['metadata'].get('ownerReferences',[])) for r in rows):return
            time.sleep(.2)
        raise Blocked('rollout_client_fence_timeout')

    def shutdown_original_hub(self):
        from commands import capture
        from docker_runtime import DockerRuntime,NATS_IMAGE
        from rollout_census import census
        version=self.kube.run(['version','-o','json'])['serverVersion']
        if int(version['major'])!=1 or int(version['minor'].rstrip('+'))<27:
            raise Blocked('rollout_retained_pod_version_unsupported')
        hub=self.kube.get('deployment',HUB);expected=self.snapshots[HUB]
        if hub['metadata']['uid']!=expected['metadata']['uid'] or hub['spec']['template']!=expected['spec']['template']:
            raise Blocked('rollout_original_hub_changed')
        claims=self.expected['source_claim']
        pods=self.kube.run(['get','pods','-o','json'])['items']
        selected=[p for p in pods if any(v.get('persistentVolumeClaim',{}).get('claimName')==claims
            for v in p['spec'].get('volumes',[]))]
        if len(selected)!=1:raise Blocked('rollout_original_hub_inventory_invalid')
        pod=selected[0];metadata=pod['metadata'];spec=pod['spec'];statuses=pod['status'].get('containerStatuses',[])
        owners=metadata.get('ownerReferences',[])
        if (metadata.get('deletionTimestamp') or pod['status'].get('phase')!='Running'
            or len(owners)!=1 or owners[0].get('kind')!='ReplicaSet' or owners[0].get('controller') is not True
            or len(spec['containers'])!=1 or spec['containers'][0]['name']!='nats'
            or spec['containers'][0]['image']!=NATS_IMAGE or len(statuses)!=1):
            raise Blocked('rollout_original_hub_inventory_invalid')
        rs=self.kube.get('replicaset',owners[0]['name'])
        if (rs['metadata']['uid']!=owners[0]['uid'] or not any(o.get('kind')=='Deployment'
            and o.get('uid')==hub['metadata']['uid'] and o.get('controller') is True for o in rs['metadata'].get('ownerReferences',[]))):
            raise Blocked('rollout_original_hub_owner_changed')
        current=statuses[0]
        if (current.get('name')!='nats' or current.get('ready') is not True
            or not re.fullmatch('containerd://[a-f0-9]{64}',current.get('containerID',''))
            or current.get('imageID','').removeprefix('docker-pullable://')!=NATS_IMAGE
            or type(current.get('restartCount')) is not int or set(current.get('state',{}))!={'running'}):
            raise Blocked('rollout_original_hub_container_changed')
        node=self.kube.get('node',spec['nodeName'])
        if not any(c.get('type')=='Ready' and c.get('status')=='True' for c in node.get('status',{}).get('conditions',[])):
            raise Blocked('rollout_original_hub_node_not_ready')
        def identity():
            self.owned_marker();live=self.kube.get('pod',metadata['name'])
            matches=[s for s in live['status'].get('containerStatuses',[]) if s['name']=='nats']
            if (live['metadata']['uid']!=metadata['uid'] or live['metadata'].get('deletionTimestamp')
                or matches!=statuses):raise Blocked('rollout_original_hub_changed')
        identity()
        raw=capture(['/usr/local/bin/k3s','kubectl','--namespace',guard.NS,'exec',metadata['name'],
            '-c','nats','--','/bin/busybox','wget','-q','-O','-',
            'http://127.0.0.1:8222/jsz?accounts=true&streams=true&consumers=true&config=true&limit=2048'],
            timeout=30,limit=64<<20,env={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'})
        identity();tree=json.loads(raw)
        class Snapshot:
            def monitor_jsz(self,name):return tree
        runtime=DockerRuntime(self.runtime_base,self.operation)
        self.original_live_census=census(Snapshot(),'original',runtime.account_id())
        finalizer='voice.io/nats-preservation-'+self.operation
        original=copy.deepcopy(pod);existing=metadata.get('finalizers',[])
        if finalizer in existing:raise Blocked('rollout_original_hub_finalizer_already_present')
        self.save({'kind':'original_hub_shutdown_intent','pod_uid':metadata['uid'],
            'container_id':current['containerID'],'finalizer':finalizer,
            'live_census_sha256':digest(self.original_live_census)})
        retained=self.kube.cas('pod',pod,[{'op':'add','path':'/metadata/finalizers','value':existing+[finalizer]}])
        self.scale(HUB,0);deadline=time.monotonic()+90
        while time.monotonic()<deadline:
            self.owned_marker();retained=self.kube.get('pod',metadata['name'])
            if retained['metadata']['uid']!=metadata['uid'] or finalizer not in retained['metadata'].get('finalizers',[]):
                raise Blocked('rollout_original_hub_retained_pod_changed')
            status=[s for s in retained['status'].get('containerStatuses',[]) if s['name']=='nats']
            if len(status)!=1 or status[0].get('containerID')!=current['containerID'] or status[0].get('restartCount')!=current['restartCount']:
                raise Blocked('rollout_original_hub_container_changed')
            if 'terminated' in status[0].get('state',{}):
                proof=terminated_hub(original,retained)
                self.save({'kind':'original_hub_orderly_exit_verified','proof':proof})
                finals=retained['metadata']['finalizers'];index=finals.index(finalizer)
                self.kube.cas('pod',retained,[{'op':'test','path':'/metadata/finalizers','value':finals},
                    {'op':'remove','path':'/metadata/finalizers/'+str(index)}])
                self.save({'kind':'original_hub_owned_finalizer_removed','pod_uid':metadata['uid']});return proof
            time.sleep(.2)
        raise Blocked('rollout_original_hub_shutdown_timeout')

    def refence(self):
        released=getattr(self,'finish_released_marker',None)
        if released is not None:
            # Safety-only reacquisition of THIS call's exact released CAS.
            # Never take over an arbitrary active marker or renew permission.
            current=self.kube.get('configmap',MARKER)
            if (current!=released or current['data'].get('phase')!='active'
                or current['data'].get('knownRolloutOperation') is not None
                or current['data'].get('dataPVC')!=self.expected['source_claim']):
                raise Blocked('finish_released_marker_changed')
            self.verify_selected_storage_identity()
            self.marker=self.kube.cas('configmap',current,[
                {'op':'test','path':'/data/phase','value':'active'},
                {'op':'test','path':'/data/dataPVC','value':self.expected['source_claim']},
                {'op':'replace','path':'/data/phase','value':'rollout-verified'},
                {'op':'add','path':'/data/knownRolloutOperation','value':self.operation}])
            del self.finish_released_marker
            self.save({'kind':'expired_finish_release_refenced','marker':self.marker})
        self.owned_marker()
        names=tuple(n for n in self.snapshots if n!=HUB)+(HUB,)
        for name in names:self.scale(name,0)
        self.no_pods(names);self.verify_closed()
        return {'verified':True,'hub_zero_pods':True}

    def maintenance(self):
        row=self.kube.get('configmap',MARKER)
        if row['metadata']['uid']!=self.marker['metadata']['uid'] or row['metadata']['resourceVersion']!=self.marker['metadata']['resourceVersion']:
            raise Blocked('rollout_marker_race')
        self.marker=self.kube.cas('configmap',row,[
            {'op':'test','path':'/data/phase','value':'active'},
            {'op':'replace','path':'/data/phase','value':'rollout-capturing'},
            {'op':'add','path':'/data/knownRolloutOperation','value':self.operation}])
        self.save({'kind':'rollout_maintenance_owned','marker':self.marker})

    def owned_marker(self):
        row=self.kube.get('configmap',MARKER)
        if row['metadata']['uid']!=self.marker['metadata']['uid'] or row['metadata']['resourceVersion']!=self.marker['metadata']['resourceVersion'] or row['data'].get('knownRolloutOperation')!=self.operation or row['data'].get('phase') not in ('rollout-capturing','rollout-prepared','rollout-applying','rollout-verified') or row['data'].get('dataPVC')!=self.expected['source_claim']:
            raise Blocked('rollout_ownership_changed')
        return row

    def prepared(self):
        self.verify_final_storage()
        row=self.owned_marker()
        if row['data']['phase']!='rollout-capturing':raise Blocked('rollout_prepare_phase_invalid')
        self.marker=self.kube.cas('configmap',row,[{'op':'replace','path':'/data/phase','value':'rollout-prepared'}])
        self.save({'kind':'rollout_prepared','marker':self.marker})

    def adopt_applied_target(self,receipt,claim_rv):
        # Runner advances RV exactly once. Never accept arbitrary latest RV.
        row=self.kube.get('configmap',MARKER)
        if row['metadata']['uid']!=self.marker['metadata']['uid'] or row['metadata']['resourceVersion']!=claim_rv or row['data'].get('phase')!='rollout-applying' or row['data'].get('knownRolloutOperation')!=self.operation:
            raise Blocked('rollout_apply_owner_changed')
        replacement={}
        for name,old in self.snapshots.items():
            current=self.kube.get('deployment',name)
            wanted=receipt['target']['template_hashes'].get(name,digest(old['spec']['template']))
            if current['metadata']['uid']!=old['metadata']['uid'] or current['spec'].get('replicas',1)!=0 or digest(current['spec']['template'])!=wanted:
                raise Blocked('rollout_applied_target_changed')
            replacement[name]=current
        self.marker=row;self.snapshots=replacement
        self.verify_final_storage()
        self.save({'kind':'rollout_target_observed','snapshots':self.snapshots,'marker':self.marker})

    def verified(self):
        self.verify_final_storage();row=self.owned_marker()
        if row['data']['phase']=='rollout-verified':
            # Observe only THIS operation's exact UID/RV/storage-bound marker.
            # A finish-only proof must not replay the completed applying CAS.
            self.marker=row
            return
        if row['data']['phase']!='rollout-applying':raise Blocked('rollout_verify_phase_invalid')
        self.marker=self.kube.cas('configmap',row,[{'op':'replace','path':'/data/phase','value':'rollout-verified'}])
        self.save({'kind':'rollout_verified','marker':self.marker})

    def adopt_rollback_target(self,receipt,claim_rv):
        row=self.kube.get('configmap',MARKER)
        if row['metadata']['uid']!=self.marker['metadata']['uid'] or row['metadata']['resourceVersion']!=claim_rv or row['data'].get('phase')!='rollout-applying' or row['data'].get('knownRolloutOperation')!=self.operation:
            raise Blocked('rollout_rollback_owner_changed')
        replacement={}
        for name,old in self.original_snapshots.items():
            current=self.kube.get('deployment',name)
            allowed={digest(old['spec']['template']),receipt['target']['template_hashes'].get(name,digest(old['spec']['template']))}
            if current['metadata']['uid']!=old['metadata']['uid'] or current['spec'].get('replicas',1)!=0 or digest(current['spec']['template']) not in allowed:
                raise Blocked('rollout_rollback_workload_changed')
            replacement[name]=current
        self.marker=row;self.snapshots=replacement
        self.verify_final_storage()
        self.save({'kind':'rollout_partial_target_observed'})

    def restore_original_templates(self):
        self.verify_final_storage()
        # First validate the entire current ledger, before any rollback patch.
        for name,snapshot in self.snapshots.items():
            current=self.kube.get('deployment',name)
            if current['metadata']['uid']!=snapshot['metadata']['uid'] or current['spec'].get('replicas',1)!=0 or current['spec']['template']!=snapshot['spec']['template']:
                raise Blocked('rollout_rollback_workload_changed')
        for name,original in self.original_snapshots.items():
            if name==HUB:continue
            self.verify_final_storage()
            current=self.kube.get('deployment',name);snapshot=self.snapshots[name]
            if current['metadata']['uid']!=snapshot['metadata']['uid'] or current['spec'].get('replicas',1)!=0 or current['spec']['template']!=snapshot['spec']['template']:
                raise Blocked('rollout_rollback_workload_changed')
            template=copy.deepcopy(original['spec']['template'])
            for container in template['spec'].get('initContainers',[])+template['spec']['containers']:
                key=name+'/'+container['name'];image=self.old_images.get(key,'')
                if not re.fullmatch(r'[a-z0-9][a-zA-Z0-9._/:~-]{1,255}@sha256:[a-f0-9]{64}',image):raise Blocked('rollout_rollback_image_unpinned')
                container['image']=image
            authority=getattr(self,'finish_authority_guard',None)
            if authority is not None:authority()
            self.snapshots[name]=self.kube.cas('deployment',current,[
                {'op':'test','path':'/spec/replicas','value':0},
                {'op':'test','path':'/spec/template','value':snapshot['spec']['template']},
                {'op':'replace','path':'/spec/template','value':template}])
            self.save({'kind':'rollout_original_template_restored','name':name})
        self.verify_final_storage()

    def restart(self):
        if self.owned_marker()['data']['phase']!='rollout-verified':
            raise Blocked('rollout_unverified_resume_forbidden')
        if getattr(self,'renderer_transition',None) is not None and not callable(getattr(self,'renderer_post_start',None)):
            raise Blocked('renderer_restart_proof_missing')
        authority=getattr(self,'finish_authority_guard',None)
        self.seal_before_business_resume()
        if authority is None:
            from preserve import ColdRelease
            runtime=getattr(self,'closed_preservation_runtime',None)
            if type(runtime) is ColdRelease:authority=runtime.guard
        if authority is None:return super().restart()
        # The inherited User/Space bootstrap has direct template CAS calls in
        # addition to scale calls. Every one must check the same short authority
        # before mutation; subsequent readiness/final checks retain progress.
        original=self.kube
        class GuardedCAS:
            def __getattr__(self,name):return getattr(original,name)
            def cas(self,*args,**kwargs):
                authority()
                return original.cas(*args,**kwargs)
        self.kube=GuardedCAS()
        try:return super().restart()
        finally:self.kube=original

    def restart_missing(self,ledger):
        """Resume only this fresh proof's recorded missing obligations.

        Completed workloads are observed; their scale action is not replayed.
        The owned User/Space temporary override remains a typed, reversible
        startup transition, and is never accepted as a final target template.
        """
        if self.owned_marker()['data']['phase']!='rollout-verified':
            raise Blocked('rollout_unverified_resume_forbidden')
        if set(ledger.record['pending'])|set(ledger.record['completed'])!=set(self.snapshots):
            raise Blocked('finish_resume_inventory_changed')
        self.seal_before_business_resume()
        def resume(name):
            if name in ledger.record['pending']:
                row=self.kube.get('deployment',name)
                if row['spec'].get('replicas',1)!=0:raise Blocked('finish_resume_not_paused')
                self.scale(name,1)
            self.wait_ready(name)
        resume(HUB)
        user_name='voice-user';space_name='voice-space';original=None
        if user_name in ledger.record['pending'] and space_name in ledger.record['pending']:
            user=self.kube.get('deployment',user_name)
            original=copy.deepcopy(user['spec']['template'])
            containers=user['spec']['template']['spec']['containers']
            indexes=[i for i,c in enumerate(containers) if c['name']=='user']
            if len(indexes)!=1:raise Blocked('user_cycle_shape_invalid')
            index=indexes[0];env=containers[index].get('env');annotations=original.get('metadata',{}).get('annotations')
            if (not isinstance(env,list) or not isinstance(annotations,dict)
                or any(e['name']=='SPACE_GRPC_ADDR' for e in env)
                or 'voice.io/nats-user-space-bootstrap' in annotations):raise Blocked('user_cycle_already_owned')
            if 'SPACE_GRPC_ADDR' not in self.kube.get('configmap','voice-app-config')['data']:
                raise Blocked('user_cycle_config_missing')
            self.finish_authority_guard()
            self.save({'kind':'expired_finish_cycle_entered','name':user_name,'original_template_sha256':digest(original)})
            self.finish_authority_guard()
            self.snapshots[user_name]=self.kube.cas('deployment',user,[
                {'op':'test','path':'/spec/replicas','value':0},
                {'op':'test','path':'/spec/template','value':original},
                {'op':'add','path':'/spec/template/spec/containers/'+str(index)+'/env/-','value':{'name':'SPACE_GRPC_ADDR','value':''}},
                {'op':'add','path':'/spec/template/metadata/annotations/voice.io~1nats-user-space-bootstrap','value':self.operation}])
            self.save({'kind':'user_cycle_override','owner':self.operation})
        for service in LEAVES:
            name='voice-'+service
            if name in ledger.record['pending']:
                row=self.kube.get('deployment',name)
                if row['spec'].get('replicas',1)!=0:raise Blocked('finish_resume_not_paused')
                self.scale(name,1)
        self.wait_ready(user_name);self.wait_ready(space_name)
        if original is not None:
            self.finish_authority_guard();user=self.kube.get('deployment',user_name)
            self.snapshots[user_name]=self.kube.cas('deployment',user,[
                {'op':'test','path':'/spec/template','value':self.snapshots[user_name]['spec']['template']},
                {'op':'test','path':'/spec/template/metadata/annotations/voice.io~1nats-user-space-bootstrap','value':self.operation},
                {'op':'replace','path':'/spec/template','value':original}])
            self.save({'kind':'user_cycle_restored','owner':self.operation});self.finish_authority_guard()
            self.wait_ready(user_name)
        for service in LEAVES:self.wait_ready('voice-'+service)
        resume('voice-gateway')
        self.release_marker()
        return {'verified':True,'active_claim':self.final_claim['metadata']['name'],
            'generation':self.expected['generation'],'user_cycle_restored':True,'finish_only':True}

    def seal_before_business_resume(self):
        # Ordinary and both dedicated consumers share the ROOT producer;
        # absence cannot fall back to Kube readiness as preservation evidence.
        producer=getattr(self,'closed_preservation_producer',None)
        if producer is None:raise Blocked('closed_preservation_seal_required')
        result=producer.seal_before_resume(self)
        runtime=getattr(self,'closed_preservation_runtime',None)
        if runtime is None:raise Blocked('closed_preservation_release_source_required')
        runtime.arm_release(self,result)
        authority=getattr(self,'finish_authority_guard',None)
        if authority is not None:authority()
        return result

    def scale(self,name,replicas):
        finish_guard=getattr(self,'finish_guard',None)
        if finish_guard is not None and replicas!=0:finish_guard('scale',name,replicas)
        release=getattr(self,'closed_preservation_release',None)
        if release is not None and replicas!=0:release(self,'scale-intent',name)
        result=super().scale(name,replicas)
        if release is not None and replicas!=0:release(self,'scale-result',name)
        return result

    def wait_ready(self,name):
        super().wait_ready(name)
        # Inherited restart waits for HUB before scaling any application.
        # This actual-init proof therefore gates all subsequent app starts.
        if name==HUB and getattr(self,'renderer_transition',None) is not None:
            self.renderer_post_start(self)
            self.save({'kind':'rollout_renderer_live_verified'})
        finish_guard=getattr(self,'finish_guard',None)
        if finish_guard is not None:finish_guard('ready',name,None)
        release=getattr(self,'closed_preservation_release',None)
        if release is not None:release(self,'ready',name)

    def release_marker(self):
        row=self.owned_marker()
        if row['data']['phase']!='rollout-verified':raise Blocked('rollout_unverified_release_forbidden')
        core={HUB,'voice-gateway',*('voice-'+s for s in LEAVES)}
        for name in sorted(set(self.snapshots)-core):
            ledger=getattr(self,'finish_ledger',None)
            if ledger is None or name in ledger.record['pending']:self.scale(name,1)
            self.wait_ready(name)
        row=self.owned_marker()
        finish_guard=getattr(self,'finish_guard',None)
        if finish_guard is not None:finish_guard('release',None,None)
        release=getattr(self,'closed_preservation_release',None)
        if release is not None:release(self,'release-intent')
        self.marker=self.kube.cas('configmap',row,[
            {'op':'test','path':'/data/dataPVC','value':self.expected['source_claim']},
            {'op':'replace','path':'/data/phase','value':'active'},
            {'op':'remove','path':'/data/knownRolloutOperation'}])
        if getattr(self,'finish_authority_guard',None) is not None:
            self.finish_released_marker=copy.deepcopy(self.marker)
        self.save({'kind':'rollout_released','marker':self.marker})
        if release is not None:release(self,'release-result')
        final_guard=getattr(self,'finish_authority_guard',None)
        if final_guard is not None:final_guard()

    def new_claim(self,*args):raise Blocked('rollout_pvc_allocation_forbidden')
    def select_claim(self):raise Blocked('rollout_pvc_selection_forbidden')

    def verify_final_storage(self):
        self.verify_closed()
        self.verify_selected_storage_identity()

    def verify_selected_storage_identity(self):
        """Same selected physical store without asserting writer closure.

        Renderer input and post-start checks use this identity-only guard.
        Native operations and paused CAS still require verify_final_storage.
        """
        namespace=self.kube.get('namespace',guard.NS);marker=self.kube.get('configmap',MARKER)
        claim=self.kube.get('pvc',self.final_claim['metadata']['name'])
        pv=self.kube.get('pv',claim['spec']['volumeName'])
        if (namespace['metadata']['uid']!=self.expected['namespace_uid']
            or marker['metadata']['uid']!=self.expected['marker_uid']
            or marker['data'].get('generation')!=self.expected['generation']
            or marker['data'].get('dataPVC')!=self.expected['source_claim']
            or claim['metadata']['uid']!=self.expected['source_claim_uid']
            or claim['metadata']['uid']!=self.final_claim['metadata']['uid']
            or pv_storage_path(pv,claim['metadata']['uid'],claim['metadata']['name'],self.expected['source_pv_uid'])!=str(self.final_path)
            or any(pv['spec'].get(k)!=self.final_pv['spec'].get(k) for k in ('local','hostPath'))):
            raise Blocked('rollout_selected_storage_identity_changed')
        # Every ancestor of the exact UID-bound source is a trusted directory;
        # never follow a provisioner path through an untrusted symlink.
        descriptor=self.selected_store_descriptor()
        fd=os.open('/',os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
        try:
            parts=self.final_path.parts[1:]
            for index,name in enumerate(parts):
                child=os.open(name,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=fd)
                os.close(fd);fd=child;s=os.fstat(fd)
                if index==len(parts)-1:
                    try:verify_selected_store_leaf(s,descriptor)
                    except Blocked:raise Blocked('rollout_selected_store_custody_invalid') from None
                elif s.st_uid!=0 or s.st_mode&0o022:
                    raise Blocked('rollout_selected_store_parent_untrusted')
        finally:os.close(fd)

    def selected_store_descriptor(self):
        captured=self.snapshots[HUB];current=self.kube.get('deployment',HUB)
        if (current['metadata']['uid']!=captured['metadata']['uid']
            or current['metadata'].get('namespace')!=captured['metadata'].get('namespace')
            or current['spec']!=captured['spec']):
            raise Blocked('selected_store_hub_authority_changed')
        return selected_store_descriptor(captured,self.final_claim,self.final_pv)
