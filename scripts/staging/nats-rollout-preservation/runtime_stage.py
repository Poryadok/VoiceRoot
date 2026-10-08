"""Root-owned rollout adapter reusing the tested physical fence/startup cycle.

This adapter has no PVC allocation, selection, bootstrap or archive replay path.
The same live-selected claim is retained through target application and rollback.
"""
import copy
import os
import re
from pathlib import Path
import stat
import guard
from stage_runtime import Staging, HUB, MARKER, LEAVES, pv_storage_path, selected_store_descriptor, verify_selected_store_leaf
from controller import Blocked
from apply import digest

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
        names=tuple(n for n in self.snapshots if n!=HUB)+(HUB,)
        for name in names:self.scale(name,0)
        self.no_pods(names);self.verify_closed()
        return {'verified':True,'workloads':list(names),'hub_zero_pods':True}

    def refence(self):
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
        return super().restart()

    def wait_ready(self,name):
        super().wait_ready(name)
        # Inherited restart waits for HUB before scaling any application.
        # This actual-init proof therefore gates all subsequent app starts.
        if name==HUB and getattr(self,'renderer_transition',None) is not None:
            self.renderer_post_start(self)
            self.save({'kind':'rollout_renderer_live_verified'})

    def release_marker(self):
        row=self.owned_marker()
        if row['data']['phase']!='rollout-verified':raise Blocked('rollout_unverified_release_forbidden')
        core={HUB,'voice-gateway',*('voice-'+s for s in LEAVES)}
        for name in sorted(set(self.snapshots)-core):
            self.scale(name,1);self.wait_ready(name)
        row=self.owned_marker()
        self.marker=self.kube.cas('configmap',row,[
            {'op':'test','path':'/data/dataPVC','value':self.expected['source_claim']},
            {'op':'replace','path':'/data/phase','value':'active'},
            {'op':'remove','path':'/data/knownRolloutOperation'}])
        self.save({'kind':'rollout_released','marker':self.marker})

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
