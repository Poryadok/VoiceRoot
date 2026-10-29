"""Private journaled pre-cut Space HTTPS observation and post-cut verifier."""
import base64
import copy
import hashlib
from pathlib import Path
import secrets

import frontend_runtime_executor
import full_cohort_package_phase
import media_operation_guard
import realtime_config_transition_space as space
import realtime_p3_public_inputs as public
import writer_cut

STEP='realtime-space-https'


class SpacePhaseError(RuntimeError): pass


class SpacePhase:
    def __init__(self,journal,fence,client,lock,namespace_uid,package):
        self.journal,self.fence,self.client,self.package=journal,fence,client,package
        self.lock,self.namespace_uid=copy.deepcopy(lock),namespace_uid
        self.intent=copy.deepcopy(journal.intent); self.plan=self.intent.get('realtime_space_https')
        if (type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal
                or fence.ledger.client is not client or type(package) is not full_cohort_package_phase.PackagePhase
                or package.fence.context != fence.context or package.lock != lock or package.namespace_uid != namespace_uid
                or self.intent.get('resource_context') != fence.context or self.intent.get('namespace_uid') != namespace_uid
                or not isinstance(self.plan,dict) or set(self.plan) != {'source_root','space_pod','destination'}
                or not Path(self.plan['source_root']).is_absolute() or not Path(self.plan['destination']).is_absolute()):
            raise SpacePhaseError('Space phase identity unavailable')
        self.record=None

    def _guard(self):
        if self.journal.poisoned or self.journal.intent!=self.intent or self.journal.state.get('intent')!=self.intent: raise ValueError()
        media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
        if self.record is not None:
            row,_=self.fence.ledger.read()
            local=self.journal.state['steps'].get(STEP)
            if row.get('realtime_space_https')!=self.record or not isinstance(local,dict): raise ValueError()
            if {k:v for k,v in local.items() if k!='receipt'}!=self.record: raise ValueError()
            if self.record['status']=='done' and writer_cut.digest(local.get('receipt'))!=self.record['receipt_sha256']: raise ValueError()

    def run(self,token):
        pending=False
        try:
            self._guard(); self.package.verify_current(token); self._guard()
            row,rev=self.fence.ledger.read()
            if STEP in self.journal.state['steps'] or 'realtime_space_https' in row: raise ValueError()
            record={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],
                    'source_sha':self.fence.context['commit'],'namespace_uid':self.namespace_uid}
            pending=True; self.journal.state['steps'][STEP]=copy.deepcopy(record); self.journal.save()
            now,version=self.fence.ledger.read()
            if now!=row or version!=rev: raise ValueError()
            now['realtime_space_https']=copy.deepcopy(record); now['step']=STEP+'-pending'
            self.fence.ledger.write(rev,now); self.record=copy.deepcopy(record); self._guard()
            def authority(sha):
                if sha!=self.fence.context['commit']: raise ValueError()
                self.package.verify_current(token)
            result=space.capture(self.plan['source_root'],self.fence.context['commit'],authority,self.plan['space_pod'],
                                 self.namespace_uid,self._guard,self.plan['destination'])
            self._guard(); deployment=self.client.call('get','deployment','voice-space','-o','json'); self._guard()
            meta=deployment['metadata']; identity=result['identity']['workload']
            if (deployment.get('apiVersion')!='apps/v1' or deployment.get('kind')!='Deployment'
                    or meta.get('name')!='voice-space' or meta.get('namespace')!='voice-dev' or meta.get('deletionTimestamp')
                    or meta.get('uid')!=identity['deployment_uid'] or meta.get('resourceVersion')!=identity['deployment_rv']
                    or type(meta.get('generation')) is not int): raise ValueError()
            receipt={'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit'],
                     'namespace_uid':self.namespace_uid,'observation':{**result,'jwks_bytes':base64.b64encode(result['jwks_bytes']).decode()},
                     'deployment':{'uid':meta['uid'],'resourceVersion':meta['resourceVersion'],
                                   'generation':meta['generation'],'spec':copy.deepcopy(deployment['spec'])}}
            self._guard(); row,rev=self.fence.ledger.read()
            if row.get('realtime_space_https')!=self.record: raise ValueError()
            done=record|{'status':'done','receipt_sha256':writer_cut.digest(receipt)}
            self.journal.state['steps'][STEP]=copy.deepcopy(done)|{'receipt':copy.deepcopy(receipt)}; self.journal.save()
            current,version=self.fence.ledger.read()
            if current!=row or version!=rev: raise ValueError()
            current['realtime_space_https']=copy.deepcopy(done); current['step']=STEP+'-done'
            self.fence.ledger.write(rev,current); self.record=copy.deepcopy(done); self._guard()
            return copy.deepcopy(receipt)
        except BaseException:
            if pending:
                try: self.fence.uncertain('Space HTTPS observation uncertain')
                except BaseException: pass
            raise SpacePhaseError('Space HTTPS observation unavailable') from None


def verify(phase,cut,token):
    """Verify the captured source and cut predecessor without contacting a drained Pod."""
    try:
        if (type(phase) is not SpacePhase or type(cut) is not writer_cut.WriterCut
                or phase.fence.context!=cut.fence.context or phase.lock!=cut.lock or phase.namespace_uid!=cut.namespace_uid): raise ValueError()
        phase._guard(); cut._guard(); phase.package.verify_current(token)
        row,rev=phase.fence.ledger.read(); other,other_rev=cut.fence.ledger.read()
        if row!=other or rev!=other_rev or phase.record is None or phase.record.get('status')!='done': raise ValueError()
        local=copy.deepcopy(phase.journal.state['steps'][STEP]); receipt=local['receipt']; dep=receipt['deployment']
        if (receipt.get('operation_id')!=phase.fence.context['operation_id'] or receipt.get('source_sha')!=phase.fence.context['commit']
                or receipt.get('namespace_uid')!=phase.namespace_uid): raise ValueError()
        observed=receipt['observation']
        raw=base64.b64decode(observed['jwks_bytes'],validate=True)
        if (not raw or len(raw)>65536 or hashlib.sha256(raw).hexdigest()!=observed['jwks_sha256']
                or observed.get('scope')!='space-pod-https-through-kubernetes-tunnel'
                or observed.get('source_sha')!=receipt['source_sha'] or observed.get('namespace_uid')!=phase.namespace_uid): raise ValueError()
        matches=[x for x in cut.admission['controllers'] if x.get('kind')=='Deployment' and x.get('metadata',{}).get('name')=='voice-space']
        if len(matches)!=1: raise ValueError()
        actual=matches[0]; meta=actual['metadata']
        if (meta.get('uid')!=dep['uid'] or meta.get('generation')!=dep['generation'] or actual.get('spec')!=dep['spec']): raise ValueError()
        # Public Secrets and Service are required to retain the captured identity.
        guard=phase._guard
        for name,expected in observed['public_objects'].items():
            if public._meta(public.common.linux_private_bytes,guard,'secret',name)!=expected: raise ValueError()
        service=phase.client.call('get','service','voice-space','-o','json')['metadata']
        expected=observed['identity']['service']
        if service.get('uid')!=expected['uid'] or service.get('resourceVersion')!=expected['resource_version']: raise ValueError()
        phase._guard(); cut._guard(); after,end=phase.fence.ledger.read()
        if after!=row or end!=rev or phase.journal.state['steps'][STEP]!=local: raise ValueError()
        return copy.deepcopy(receipt)
    except Exception:
        raise SpacePhaseError('Space HTTPS predecessor unavailable') from None
