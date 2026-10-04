"""Human-root Kubernetes adapter for the NATS-only maintenance boundary.

Every mutation is bound to observed UID/resourceVersion. Authentication and
other data stores are neither changed nor recreated by this adapter.
"""
import json
import os
from pathlib import Path
import re
import stat
import subprocess
import time

from controller import Blocked
from commands import capture
from docker_runtime import NATS_IMAGE

LEAVES = ('analytics','auth','bot','chat','file','matchmaking','messaging',
          'moderation','notification','realtime','role','search','social',
          'space','story','subscription','user','voice')
HUB = 'voice-nats-pvc-candidate'
MARKER = 'voice-nats-generation'
NAMESPACE = 'voice-staging'

# Fixed typed output: historical ReplicaSet templates can exceed the capture
# cap. They are irrelevant to physical Pod ownership/PVC writer detection.
FENCE_TEMPLATE = ('{"items":[{{range $i,$r := .items}}{{if $i}},{{end}}'
    '{"kind":{{printf "%q" $r.kind}},"metadata":{"uid":{{printf "%q" $r.metadata.uid}},'
    '"ownerReferences":[{{range $j,$o := $r.metadata.ownerReferences}}{{if $j}},{{end}}'
    '{"uid":{{printf "%q" $o.uid}}}{{end}}]},"spec":{"volumes":['
    '{{range $j,$v := $r.spec.volumes}}{{if $j}},{{end}}'
    '{"persistentVolumeClaim":{"claimName":{{if $v.persistentVolumeClaim}}'
    '{{printf "%q" $v.persistentVolumeClaim.claimName}}{{else}}""{{end}}}}'
    '{{end}}]}}{{end}}]}')


def fence_objects(kube):
    result=kube.run(['get','pods,replicasets','-o','go-template='+FENCE_TEMPLATE])
    rows=result.get('items') if isinstance(result,dict) and set(result)=={'items'} else None
    if not isinstance(rows,list) or len(rows)>2048:raise Blocked('fence_projection_invalid')
    uid=re.compile(r'[a-f0-9]{8}(?:-[a-f0-9]{4}){3}-[a-f0-9]{12}')
    for row in rows:
        try:
            if set(row)!={'kind','metadata','spec'} or set(row['metadata'])!={'uid','ownerReferences'} or set(row['spec'])!={'volumes'}:raise ValueError()
            if row['kind'] not in ('Pod','ReplicaSet') or not uid.fullmatch(row['metadata']['uid']):raise ValueError()
            owners=row['metadata'].get('ownerReferences',[]);volumes=row.get('spec',{}).get('volumes',[])
            if not isinstance(owners,list) or len(owners)>16 or not isinstance(volumes,list) or len(volumes)>64:raise ValueError()
            if any(set(o)!={'uid'} or not uid.fullmatch(o['uid']) for o in owners):raise ValueError()
            for volume in volumes:
                if set(volume)!={'persistentVolumeClaim'} or set(volume['persistentVolumeClaim'])!={'claimName'}:raise ValueError()
                claim=volume.get('persistentVolumeClaim',{}).get('claimName','')
                if not isinstance(claim,str) or len(claim)>253 or (claim and not re.fullmatch(r'[a-z0-9][a-z0-9.-]*',claim)):raise ValueError()
        except (KeyError,TypeError,AttributeError,ValueError):raise Blocked('fence_projection_invalid') from None
    return rows


class Kube:
    def __init__(self, run=None): self.run = run or self._run

    @staticmethod
    def _run(args, body=None, timeout=30):
        output=capture(['/usr/local/bin/k3s','kubectl','--namespace',NAMESPACE,*args],
            body=b'' if body is None else json.dumps(body).encode(),timeout=timeout,
            env={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'})
        return json.loads(output)

    def get(self, kind, name): return self.run(['get',kind,name,'-o','json'])

    def secret_meta(self, name):
        values=self.run(['get','secret',name,'-o',
            "jsonpath-as-json={.metadata['uid','resourceVersion']}"])
        if not isinstance(values,list) or len(values)!=2 or any(not isinstance(v,str) or not v for v in values):
            raise Blocked('secret_metadata_projection_invalid')
        return dict(zip(('uid','resourceVersion'),values))

    def cas(self, kind, row, changes):
        metadata = row['metadata']
        patch = [{'op':'test','path':'/metadata/uid','value':metadata['uid']},
                 {'op':'test','path':'/metadata/resourceVersion','value':metadata['resourceVersion']},*changes]
        return self.run(['patch',kind,metadata['name'],'--type=json','--patch',json.dumps(patch),'-o','json'])


class Staging:
    def __init__(self, kube, operation, expected, save):
        if not re.fullmatch(r'[a-z0-9]{8,32}',operation): raise Blocked('operation_identity_invalid')
        self.kube=kube; self.operation=operation; self.expected=expected; self.save=save
        self.snapshots={}; self.marker=None

    def competing_controllers(self):
        rows=self.kube.run(['get','hpa,jobs,deployments','-o','json'])['items']
        for row in rows:
            if row['kind']=='HorizontalPodAutoscaler' and row['spec']['scaleTargetRef'].get('name') in self.snapshots:
                raise Blocked('fenced_workload_autoscaler_present')
            if row['kind']=='Job' and row.get('status',{}).get('active',0) and 'nats' in row['metadata']['name']:
                raise Blocked('active_nats_job_present')
            if row['kind']=='Deployment' and row['metadata']['name'] not in self.snapshots and row['spec'].get('replicas',1):
                if any(c.get('image')==NATS_IMAGE for c in row['spec']['template']['spec'].get('containers',[])):
                    raise Blocked('alternate_nats_hub_present')

    def preflight(self):
        namespace=self.kube.get('namespace',NAMESPACE)
        marker=self.kube.get('configmap',MARKER)
        if namespace['metadata']['uid']!=self.expected['namespace_uid'] or marker['metadata']['uid']!=self.expected['marker_uid']:
            raise Blocked('staging_identity_changed')
        data=marker['data']
        if data.get('phase')!='active' or data.get('generation')!=self.expected['generation'] or 'knownBaselineOperation' in data:
            raise Blocked('staging_marker_changed')
        self.marker=marker
        for name in (HUB,'voice-gateway',*('voice-'+s for s in LEAVES)):
            row=self.kube.get('deployment',name)
            if row['metadata']['uid']!=self.expected['deployment_uids'][name]:
                raise Blocked('staging_deployment_changed')
            if row['spec'].get('replicas',1)!=1:
                raise Blocked('staging_replica_baseline_changed')
            if name in self.snapshots and row['spec']['template']!=self.snapshots[name]['spec']['template']:
                raise Blocked('staging_template_changed_during_proof')
            self.snapshots[name]=row
        hub=self.snapshots[HUB]
        containers=hub['spec']['template']['spec']['containers']
        if not any(c['image']==NATS_IMAGE for c in containers): raise Blocked('staging_image_changed')
        if 'secret_refs' in self.expected:
            for name,wanted in self.expected['secret_refs'].items():
                spec=self.snapshots[name]['spec']['template']['spec']
                refs={v['secret']['secretName'] for v in spec.get('volumes',[]) if 'secret' in v and v['secret']['secretName'].startswith('voice-nats-')}
                if refs!=set(wanted): raise Blocked('staging_credential_reference_changed')
        claims=[v['persistentVolumeClaim']['claimName'] for v in hub['spec']['template']['spec']['volumes'] if v['name']=='jsdata' and 'persistentVolumeClaim' in v]
        if claims!=[self.expected['source_claim']]: raise Blocked('staging_claim_changed')
        claim=self.kube.get('pvc',claims[0]); pv=self.kube.get('pv',claim['spec']['volumeName'])
        if claim['metadata']['uid']!=self.expected['source_claim_uid'] or pv['metadata']['uid']!=self.expected['source_pv_uid']:
            raise Blocked('staging_storage_changed')
        if data.get('dataPVC',claims[0])!=claims[0]: raise Blocked('staging_marker_claim_mismatch')
        self.source_claim=claim
        self.competing_controllers()
        service=self.kube.get('service','voice-nats')
        if hasattr(self,'service') and service!=self.service:
            raise Blocked('nats_service_changed')
        self.service=service
        self.save({'kind':'staging_preflight','source_claim':claims[0],
                   'source_claim_uid':claim['metadata']['uid'],
                   'replicas':{n:r['spec'].get('replicas',1) for n,r in self.snapshots.items()}})
        return {'verified':True}

    def maintenance(self):
        row=self.kube.get('configmap',MARKER)
        if row['metadata']['uid']!=self.marker['metadata']['uid'] or row['metadata']['resourceVersion']!=self.marker['metadata']['resourceVersion']:
            raise Blocked('staging_marker_race')
        self.marker=self.kube.cas('configmap',row,[
            {'op':'test','path':'/data/phase','value':'active'},
            {'op':'add','path':'/data/phase','value':'known-baseline-maintenance'},
            {'op':'add','path':'/data/knownBaselineOperation','value':self.operation}])
        self.save({'kind':'maintenance_owned'})

    def owned_marker(self):
        row=self.kube.get('configmap',MARKER)
        if (row['metadata']['uid']!=self.marker['metadata']['uid'] or
                row['metadata']['resourceVersion']!=self.marker['metadata']['resourceVersion'] or
                row['data'].get('knownBaselineOperation')!=self.operation or
                row['data'].get('phase')!='known-baseline-maintenance'):
            raise Blocked('maintenance_ownership_changed')
        return row

    def scale(self, name, replicas):
        self.owned_marker()
        row=self.kube.get('deployment',name)
        if row['metadata']['uid']!=self.snapshots[name]['metadata']['uid'] or row['spec']['template']!=self.snapshots[name]['spec']['template']:
            raise Blocked('staging_workload_race')
        result=self.kube.cas('deployment',row,[{'op':'add','path':'/spec/replicas','value':replicas}])
        self.snapshots[name]=result
        self.save({'kind':'workload_scaled','name':name,'replicas':replicas})

    def no_pods(self, names, timeout=90):
        wanted={self.snapshots[n]['metadata']['uid'] for n in names}
        deadline=time.monotonic()+timeout
        while time.monotonic()<deadline:
            for name in names:
                row=self.kube.get('deployment',name)
                if row['metadata']['uid']!=self.snapshots[name]['metadata']['uid'] or row['spec']['template']!=self.snapshots[name]['spec']['template'] or row['spec'].get('replicas',1)!=0:
                    raise Blocked('fenced_workload_changed')
            rows=fence_objects(self.kube)
            claims={self.expected.get('source_claim')}
            if hasattr(self,'final_claim'):claims.add(self.final_claim['metadata']['name'])
            if any(r['kind']=='Pod' and any(v.get('persistentVolumeClaim',{}).get('claimName') in claims for v in r['spec'].get('volumes',[])) for r in rows):
                time.sleep(0.5);continue
            replicasets={r['metadata']['uid'] for r in rows if r['kind']=='ReplicaSet' and any(o['uid'] in wanted for o in r['metadata'].get('ownerReferences',[]))}
            if not any(r['kind']=='Pod' and any(o['uid'] in replicasets for o in r['metadata'].get('ownerReferences',[])) for r in rows):
                return
            time.sleep(0.5)
        raise Blocked('staging_fence_timeout')

    def verify_closed(self):
        self.owned_marker();self.competing_controllers();self.no_pods(tuple(self.snapshots))
        if self.kube.get('service','voice-nats')!=self.service:raise Blocked('nats_service_changed')
        return {'verified':True,'hub_zero_pods':True}

    def fence(self):
        self.maintenance() # CAS before the first workload mutation.
        names=('voice-gateway',*('voice-'+s for s in LEAVES),HUB)
        for name in names:
            self.scale(name,0)
            self.save({'kind':'workload_fenced','name':name,'replicas':0})
        self.no_pods(names)
        self.verify_closed()
        return {'verified':True,'workloads':list(names),'hub_zero_pods':True}

    def new_claim(self, date):
        if not re.fullmatch(r'[0-9]{8}',date): raise Blocked('claim_date_invalid')
        self.owned_marker(); self.no_pods(tuple(self.snapshots))
        name='voice-nats-jsdata-d'+date+self.operation[:8]
        source=self.source_claim['spec']
        if source.get('storageClassName')!='local-path' or source.get('accessModes')!=['ReadWriteOnce']:
            raise Blocked('storage_class_unsupported')
        claim=self.kube.run(['create','-f','-','-o','json'],{
            'apiVersion':'v1','kind':'PersistentVolumeClaim',
            'metadata':{'name':name,'namespace':NAMESPACE,'labels':{'voice.known-nats.operation':self.operation}},
            'spec':{'storageClassName':'local-path','accessModes':['ReadWriteOnce'],
                    'resources':{'requests':{'storage':source['resources']['requests']['storage']}}}})
        self.final_claim=claim
        binder='voice-known-'+self.operation+'-binder'
        pod=self.kube.run(['create','-f','-','-o','json'],{
            'apiVersion':'v1','kind':'Pod',
            'metadata':{'name':binder,'namespace':NAMESPACE,'labels':{'voice.known-nats.operation':self.operation}},
            'spec':{'automountServiceAccountToken':False,'restartPolicy':'Never',
                'nodeSelector':{'kubernetes.io/hostname':'pmdebook'},
                'securityContext':{'runAsNonRoot':True,'runAsUser':65532,'runAsGroup':65532},
                'containers':[{'name':'binder','image':NATS_IMAGE,'command':['/bin/sleep','3600'],
                    'securityContext':{'allowPrivilegeEscalation':False,'readOnlyRootFilesystem':True,'capabilities':{'drop':['ALL']}},
                    'resources':{'requests':{'cpu':'10m','memory':'16Mi'},'limits':{'cpu':'100m','memory':'32Mi'}},
                    'volumeMounts':[{'name':'jsdata','mountPath':'/data'}]}],
                'volumes':[{'name':'jsdata','persistentVolumeClaim':{'claimName':name}}]}})
        self.save({'kind':'claim_allocating','name':name,'uid':claim['metadata']['uid'],
                   'binder':binder,'binder_uid':pod['metadata']['uid']})
        deadline=time.monotonic()+90
        while time.monotonic()<deadline:
            self.owned_marker()
            current=self.kube.get('pvc',name)
            if current['metadata']['uid']!=claim['metadata']['uid']: raise Blocked('final_claim_changed')
            if current.get('status',{}).get('phase')=='Bound': break
            time.sleep(0.5)
        else: raise Blocked('final_claim_bind_timeout')
        pv=self.kube.get('pv',current['spec']['volumeName'])
        if pv['spec'].get('claimRef',{}).get('uid')!=claim['metadata']['uid']:
            raise Blocked('final_pv_claim_mismatch')
        expected='/var/lib/rancher/k3s/storage/pvc-'+claim['metadata']['uid']+'_'+NAMESPACE+'_'+name
        if pv['spec'].get('hostPath',{}).get('path')!=expected:
            raise Blocked('final_pv_path_unsupported')
        # Delete only the exact allocation Pod, then prove no Pod mounts the
        # final claim before handing its directory to the isolated broker.
        self.kube.run(['delete','--raw','/api/v1/namespaces/'+NAMESPACE+'/pods/'+binder,'-f','-'],
            {'apiVersion':'v1','kind':'DeleteOptions','preconditions':{'uid':pod['metadata']['uid']}})
        deadline=time.monotonic()+90
        while time.monotonic()<deadline:
            pods=self.kube.run(['get','pods','-o','json'])['items']
            if not any(any(v.get('persistentVolumeClaim',{}).get('claimName')==name for v in p['spec'].get('volumes',[])) for p in pods): break
            time.sleep(0.5)
        else: raise Blocked('final_claim_writer_remains')
        path=Path(expected)
        for part in (Path('/var'),Path('/var/lib'),Path('/var/lib/rancher'),Path('/var/lib/rancher/k3s'),Path('/var/lib/rancher/k3s/storage'),path):
            s=os.lstat(part)
            if not stat.S_ISDIR(s.st_mode) or s.st_uid!=0 or (part!=path and s.st_mode&0o022):
                raise Blocked('final_pv_custody_invalid')
        if any(path.iterdir()): raise Blocked('final_claim_not_empty')
        # The local-path provisioner may create its new empty child 0777. Only
        # this UID-bound fresh empty claim is hardened; no old store is changed.
        os.chmod(path,0o700)
        self.final_claim=current; self.final_path=path
        self.final_pv=pv
        self.save({'kind':'final_claim_bound','name':name,'uid':claim['metadata']['uid'],
                   'pv_uid':pv['metadata']['uid'],'path':expected})
        return path

    def verify_final_storage(self):
        self.verify_closed()
        claim=self.kube.get('pvc',self.final_claim['metadata']['name'])
        pv=self.kube.get('pv',claim['spec']['volumeName'])
        if claim['metadata']['uid']!=self.final_claim['metadata']['uid'] or pv['metadata']['uid']!=self.final_pv['metadata']['uid'] or pv['spec'].get('claimRef',{}).get('uid')!=claim['metadata']['uid'] or pv['spec'].get('hostPath',{}).get('path')!=str(self.final_path):
            raise Blocked('closed_final_storage_changed')

    def select_claim(self):
        self.owned_marker(); self.no_pods(tuple(self.snapshots))
        row=self.kube.get('deployment',HUB)
        if row['metadata']['uid']!=self.snapshots[HUB]['metadata']['uid'] or row['spec']['template']!=self.snapshots[HUB]['spec']['template']:
            raise Blocked('hub_claim_patch_race')
        volumes=row['spec']['template']['spec']['volumes']
        indexes=[i for i,v in enumerate(volumes) if v['name']=='jsdata']
        if len(indexes)!=1: raise Blocked('hub_claim_shape_invalid')
        path='/spec/template/spec/volumes/'+str(indexes[0])+'/persistentVolumeClaim/claimName'
        self.snapshots[HUB]=self.kube.cas('deployment',row,[
            {'op':'test','path':path,'value':self.expected['source_claim']},
            {'op':'replace','path':path,'value':self.final_claim['metadata']['name']}])
        marker=self.owned_marker()
        self.marker=self.kube.cas('configmap',marker,[{'op':'add','path':'/data/dataPVC','value':self.final_claim['metadata']['name']}])
        self.save({'kind':'final_claim_selected','name':self.final_claim['metadata']['name']})

    def wait_ready(self, name, timeout=300):
        deadline=time.monotonic()+timeout
        while time.monotonic()<deadline:
            self.owned_marker()
            row=self.kube.get('deployment',name)
            if row['metadata']['uid']!=self.snapshots[name]['metadata']['uid'] or row['spec']['template']!=self.snapshots[name]['spec']['template']:
                raise Blocked('restart_workload_changed')
            state=row.get('status',{})
            if (state.get('observedGeneration',0)>=row['metadata']['generation'] and
                    state.get('readyReplicas',0)==1 and state.get('updatedReplicas',0)==1 and
                    state.get('availableReplicas',0)==1):
                return
            time.sleep(0.5)
        raise Blocked('restart_readiness_timeout')

    def restart(self):
        self.owned_marker(); self.no_pods(tuple(self.snapshots))
        claim=self.kube.get('pvc',self.final_claim['metadata']['name'])
        if claim['metadata']['uid']!=self.final_claim['metadata']['uid']:
            raise Blocked('restart_final_claim_changed')
        self.scale(HUB,1); self.wait_ready(HUB)
        # Existing reviewed bootstrap pattern breaks the User/Space startup
        # cycle with an owned temporary empty address, then restores it.
        user=self.kube.get('deployment','voice-user')
        containers=user['spec']['template']['spec']['containers']
        indexes=[i for i,c in enumerate(containers) if c['name']=='user']
        if len(indexes)!=1: raise Blocked('user_cycle_shape_invalid')
        index=indexes[0]; env=containers[index].get('env')
        annotations=user['spec']['template']['metadata'].get('annotations')
        if not isinstance(env,list) or not isinstance(annotations,dict) or any(e['name']=='SPACE_GRPC_ADDR' for e in env) or 'voice.io/nats-user-space-bootstrap' in annotations:
            raise Blocked('user_cycle_already_owned')
        config=self.kube.get('configmap','voice-app-config')
        if 'SPACE_GRPC_ADDR' not in config['data']: raise Blocked('user_cycle_config_missing')
        envpath='/spec/template/spec/containers/'+str(index)+'/env'
        annotation='/spec/template/metadata/annotations/voice.io~1nats-user-space-bootstrap'
        original=self.snapshots['voice-user']['spec']['template']
        self.snapshots['voice-user']=self.kube.cas('deployment',user,[
            {'op':'test','path':'/spec/template','value':original},
            {'op':'add','path':envpath+'/-','value':{'name':'SPACE_GRPC_ADDR','value':''}},
            {'op':'add','path':annotation,'value':self.operation}])
        self.save({'kind':'user_cycle_override','owner':self.operation})
        for service in LEAVES: self.scale('voice-'+service,1)
        self.wait_ready('voice-user'); self.wait_ready('voice-space')
        self.owned_marker()
        user=self.kube.get('deployment','voice-user')
        self.snapshots['voice-user']=self.kube.cas('deployment',user,[
            {'op':'test','path':annotation,'value':self.operation},
            {'op':'test','path':'/spec/template','value':self.snapshots['voice-user']['spec']['template']},
            {'op':'replace','path':'/spec/template','value':original}])
        self.wait_ready('voice-user')
        for service in LEAVES: self.wait_ready('voice-'+service)
        self.scale('voice-gateway',1); self.wait_ready('voice-gateway')
        marker=self.owned_marker()
        self.marker=self.kube.cas('configmap',marker,[
            {'op':'test','path':'/data/dataPVC','value':self.final_claim['metadata']['name']},
            {'op':'replace','path':'/data/phase','value':'active'},
            {'op':'remove','path':'/data/knownBaselineOperation'}])
        return {'verified':True,'active_claim':self.final_claim['metadata']['name'],
                'generation':self.expected['generation'],'user_cycle_restored':True}

    def refence(self):
        # Used only on an incomplete owned restart. No rollback, data replay,
        # credential changes, or mutation after maintenance ownership is lost.
        self.owned_marker()
        for name in ('voice-gateway',*('voice-'+s for s in LEAVES),HUB): self.scale(name,0)
        self.no_pods(tuple(self.snapshots))
        self.verify_closed()
        return {'verified':True,'hub_zero_pods':True}
