"""Journal the actual Space public signer set before cutting its writer."""
import base64
import copy
import hashlib
from pathlib import Path
import secrets

import frontend_runtime_executor
import full_cohort_package_phase
import media_operation_guard
import realtime_config_transition_space_plan as plan
import realtime_p3_public_inputs as public
import writer_cut

STEP='realtime-space-public-plan'


class PlanPhaseError(RuntimeError): pass


def _pod_matches(desired,actual):
    expected=[x for x in desired.get('containers',[]) if x.get('name')=='space']
    running=[x for x in actual.get('containers',[]) if x.get('name')=='space']
    if len(expected)!=1 or len(running)!=1: return False
    old,new=expected[0],copy.deepcopy(running[0]); volumes=copy.deepcopy(actual.get('volumes',[]))
    mounts=new.get('volumeMounts',[])
    injected=[m for m in mounts if m.get('mountPath')=='/var/run/secrets/kubernetes.io/serviceaccount']
    if len(injected)>1: return False
    if injected:
        mount=injected[0]; name=mount.get('name','')
        if (set(mount)!={'name','mountPath','readOnly'} or mount.get('readOnly') is not True
                or not name.startswith('kube-api-access-') or desired.get('automountServiceAccountToken') is False): return False
        selected=[v for v in volumes if v.get('name')==name]
        if len(selected)!=1 or set(selected[0])!={'name','projected'}: return False
        projected=selected[0]['projected']; sources=projected.get('sources',[])
        if (set(projected)!={'sources','defaultMode'} or projected['defaultMode']!=420 or len(sources)!=3
                or [set(s) for s in sources]!=[{'serviceAccountToken'},{'configMap'},{'downwardAPI'}]): return False
        token=sources[0]['serviceAccountToken']
        if (set(token)!={'path','expirationSeconds'} or token.get('path')!='token'
                or type(token.get('expirationSeconds')) is not int or not 600<=token['expirationSeconds']<=86400
                or sources[1]['configMap']!={'name':'kube-root-ca.crt','items':[{'key':'ca.crt','path':'ca.crt'}]}
                or sources[2]['downwardAPI']!={'items':[{'path':'namespace','fieldRef':{'apiVersion':'v1','fieldPath':'metadata.namespace'}}]}): return False
        new['volumeMounts']=[m for m in mounts if m!=mount]; volumes=[v for v in volumes if v.get('name')!=name]
    return (all(old.get(k,[])==new.get(k,[]) for k in ('env','envFrom','volumeMounts'))
            and old.get('image')==new.get('image') and desired.get('volumes',[])==volumes)


class SpacePlanPhase:
    def __init__(self,journal,fence,client,lock,namespace_uid,package,cut):
        self.journal,self.fence,self.client,self.package,self.cut=journal,fence,client,package,cut
        self.lock,self.namespace_uid=copy.deepcopy(lock),namespace_uid
        self.intent=copy.deepcopy(journal.intent); self.plan=self.intent.get('realtime_space_public_plan'); self.record=None
        if (type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal or fence.ledger.client is not client
                or type(package) is not full_cohort_package_phase.PackagePhase or type(cut) is not writer_cut.WriterCut
                or package.fence.context!=fence.context or cut.fence.context!=fence.context
                or package.lock!=lock or cut.lock!=lock or package.namespace_uid!=namespace_uid or cut.namespace_uid!=namespace_uid
                or self.intent.get('resource_context')!=fence.context or self.intent.get('namespace_uid')!=namespace_uid
                or not isinstance(self.plan,dict) or set(self.plan)!={'source_root','space_pod','destination'}
                or not Path(self.plan['source_root']).is_absolute() or not Path(self.plan['destination']).is_absolute()):
            raise PlanPhaseError('Space public plan identity unavailable')

    def _guard(self):
        if self.journal.poisoned or self.journal.intent!=self.intent or self.journal.state.get('intent')!=self.intent: raise ValueError()
        media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
        if self.record is not None:
            row,_=self.fence.ledger.read(); local=self.journal.state['steps'].get(STEP)
            if (row.get('realtime_space_public_plan')!=self.record or not isinstance(local,dict)
                    or {k:v for k,v in local.items() if k!='receipt'}!=self.record): raise ValueError()
            if self.record['status']=='done' and writer_cut.digest(local.get('receipt'))!=self.record['receipt_sha256']: raise ValueError()

    def _refs(self,observation):
        if public._signer_identity(public.common.linux_private_bytes,self._guard)!=observation['signer']['secret']: raise ValueError()
        for name,identity in observation['public_sources'].items():
            kind='configmap' if name=='voice-app-config' else 'secret'
            if public._meta(public.common.linux_private_bytes,self._guard,kind,name)!=identity: raise ValueError()

    def run(self,token):
        pending=False
        try:
            self._guard(); self.cut._guard(); self.package.verify_current(token)
            row,rev=self.fence.ledger.read(); other,other_rev=self.cut.fence.ledger.read()
            if row!=other or rev!=other_rev or STEP in self.journal.state['steps'] or 'realtime_space_public_plan' in row: raise ValueError()
            record={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],
                    'source_sha':self.fence.context['commit'],'namespace_uid':self.namespace_uid}
            pending=True; self.journal.state['steps'][STEP]=copy.deepcopy(record); self.journal.save()
            now,version=self.fence.ledger.read()
            if now!=row or version!=rev: raise ValueError()
            now['realtime_space_public_plan']=copy.deepcopy(record); now['step']=STEP+'-pending'
            self.fence.ledger.write(rev,now); self.record=copy.deepcopy(record); self._guard()
            def authority(sha):
                if sha!=self.fence.context['commit']: raise ValueError()
                self.package.verify_current(token)
            result=plan.capture(self.plan['source_root'],self.fence.context['commit'],authority,self.plan['space_pod'],
                                self.namespace_uid,self._guard,self.plan['destination'])
            self._guard(); self.cut._guard()
            deployment=self.client.call('get','deployment','voice-space','-o','json')
            pod=self.client.call('get','pod',self.plan['space_pod'],'-o','json'); self._guard()
            meta=deployment['metadata']; identity=result['space_identity']
            if (deployment.get('apiVersion')!='apps/v1' or deployment.get('kind')!='Deployment'
                    or meta.get('name')!='voice-space' or meta.get('namespace')!='voice-dev' or meta.get('deletionTimestamp')
                    or meta.get('uid')!=identity['deployment_uid'] or meta.get('resourceVersion')!=identity['deployment_rv']
                    or type(meta.get('generation')) is not int or pod['metadata'].get('uid')!=identity['pod_uid']
                    or pod['metadata'].get('resourceVersion')!=identity['pod_rv']): raise ValueError()
            if not _pod_matches(deployment['spec']['template']['spec'],pod['spec']): raise ValueError()
            matches=[x for x in self.cut.admission['controllers'] if x.get('kind')=='Deployment' and x.get('metadata',{}).get('name')=='voice-space']
            if len(matches)!=1 or matches[0].get('spec')!=deployment['spec'] or any(matches[0]['metadata'].get(k)!=meta.get(k) for k in ('uid','generation')): raise ValueError()
            receipt={'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit'],'namespace_uid':self.namespace_uid,
                'observation':{**result,'jwks_bytes':base64.b64encode(result['jwks_bytes']).decode()},
                'deployment':{'uid':meta['uid'],'generation':meta['generation'],'spec':copy.deepcopy(deployment['spec'])}}
            self._refs(result); self._guard(); row,rev=self.fence.ledger.read()
            done=record|{'status':'done','receipt_sha256':writer_cut.digest(receipt)}
            self.journal.state['steps'][STEP]=copy.deepcopy(done)|{'receipt':copy.deepcopy(receipt)}; self.journal.save()
            current,version=self.fence.ledger.read()
            if current!=row or version!=rev or row.get('realtime_space_public_plan')!=self.record: raise ValueError()
            current['realtime_space_public_plan']=copy.deepcopy(done); current['step']=STEP+'-done'
            self.fence.ledger.write(rev,current); self.record=copy.deepcopy(done); self._guard(); self._refs(result); self._guard()
            return copy.deepcopy(receipt)
        except BaseException:
            if pending:
                try: self.fence.uncertain('Space public plan uncertain')
                except BaseException: pass
            raise PlanPhaseError('Space public plan unavailable') from None


def verify(phase,cut,token):
    """Reopen source-bound pre-cut public plan without touching stopped Pods."""
    try:
        if type(phase) is not SpacePlanPhase or phase.cut is not cut: raise ValueError()
        phase._guard(); cut._guard(); phase.package.verify_current(token)
        row,rev=phase.fence.ledger.read(); other,other_rev=cut.fence.ledger.read()
        if row!=other or rev!=other_rev or phase.record is None or phase.record.get('status')!='done': raise ValueError()
        local=copy.deepcopy(phase.journal.state['steps'][STEP]); receipt=local['receipt']; observation=receipt['observation']
        raw=base64.b64decode(observation['jwks_bytes'],validate=True); plan._parse(raw)
        if (not raw or len(raw)>65536 or hashlib.sha256(raw).hexdigest()!=observation['jwks_sha256']
                or observation.get('scope')!='space-current-http-pod-public-keys-not-https-readiness'
                or receipt.get('operation_id')!=phase.fence.context['operation_id']
                or receipt.get('source_sha')!=phase.fence.context['commit'] or receipt.get('namespace_uid')!=phase.namespace_uid): raise ValueError()
        dep=receipt['deployment']; matches=[x for x in cut.admission['controllers'] if x.get('kind')=='Deployment' and x.get('metadata',{}).get('name')=='voice-space']
        if len(matches)!=1 or matches[0].get('spec')!=dep['spec'] or any(matches[0]['metadata'].get(k)!=dep[k] for k in ('uid','generation')): raise ValueError()
        phase._refs(observation)
        phase._guard(); cut._guard(); after,end=phase.fence.ledger.read()
        if after!=row or end!=rev or phase.journal.state['steps'][STEP]!=local: raise ValueError()
        return copy.deepcopy(receipt)
    except Exception: raise PlanPhaseError('Space public plan unavailable') from None
