"""Bind a fixed activation order to the actual applied full-cohort image set.

Only the Space, publisher, Chat and Realtime dependency barriers are asserted
here. The ordering of the other selected writers is deterministic sequencing,
not an additional readiness prerequisite.
"""
import copy
import hashlib
import json
import secrets

import frontend_runtime_executor
import full_cohort_image_apply as images
import full_cohort_package_phase as package
import media_operation_guard
import writer_cut

STEP='full-cohort-activation-plan'


class ActivationError(RuntimeError): pass


def digest(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()


class ActivationPhase:
    def __init__(self,journal,fence,client,lock,namespace_uid,package_phase,image_phase,cut):
        self.journal,self.fence,self.client=journal,fence,client
        self.lock,self.namespace_uid=copy.deepcopy(lock),namespace_uid
        self.package,self.image,self.cut=package_phase,image_phase,cut
        self.intent=copy.deepcopy(journal.intent); self.poisoned=False
        if (type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal or
                getattr(fence.ledger,'client',None) is not client or type(package_phase) is not package.PackagePhase or
                type(image_phase) is not images.FullCohortImageApply or type(cut) is not writer_cut.WriterCut or
                image_phase.admission.cut is not cut or self.intent.get('resource_context')!=fence.context or
                self.intent.get('namespace_uid')!=namespace_uid): raise ActivationError('activation identity invalid')
        for phase in (package_phase,image_phase,cut):
            if phase.fence.context!=fence.context or phase.lock!=self.lock or phase.namespace_uid!=namespace_uid:
                raise ActivationError('activation authority differs')

    def _guard(self):
        if (self.poisoned or self.journal.poisoned or self.journal.intent!=self.intent or
                self.journal.state.get('intent')!=self.intent): raise ActivationError('activation identity changed')
        media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)

    def _same_ledger(self):
        row,revision=self.fence.ledger.read()
        for phase in (self.package,self.image,self.cut):
            if phase.fence.ledger.read()!=(row,revision): raise ActivationError('activation ledger differs')
        if not self.fence.owned(row) or row.get('status')!='active': raise ActivationError('activation authority lost')
        return row,revision

    def _image_records(self):
        lineage=self.image.verify_activation_lineage(); self._same_ledger()
        if (not isinstance(lineage,dict) or set(lineage)!={'operation_id','source_sha','images'} or
                lineage['operation_id']!=self.fence.context['operation_id'] or
                lineage['source_sha']!=self.fence.context['commit'] or not isinstance(lineage['images'],dict) or
                set(lineage['images'])!=set(images.CATALOG)): raise ActivationError('image application incomplete')
        return copy.deepcopy(lineage['images'])

    def run(self,token):
        pending=False
        try:
            self._guard(); package_receipt=self.package.verify_current(token)
            records=self._image_records(); row,revision=self._same_ledger()
            self.image._drained(row)
            if STEP in self.journal.state['steps'] or 'full_cohort_activation_plan' in row: raise ValueError()
            selected=self.cut.admission['controllers']; names=[]
            for item in selected:
                name=item.get('metadata',{}).get('name')
                if item.get('kind')!='Deployment' or name not in images.CATALOG or name in names: raise ValueError()
                names.append(name)
            special=['voice-space','voice-chat','voice-realtime']
            order=sorted(name for name in names if name not in special)+[name for name in special if name in names]
            targets=[]
            for name in order:
                applied=records[name]; actual=self.image._get('deployment',name)
                if not self.image._match(name,actual,applied['uid'],applied['spec'],generation=applied['generation']):
                    raise ValueError()
                targets.append({'kind':'Deployment','name':name,'uid':applied['uid'],
                                'generation':applied['generation'],'spec':copy.deepcopy(applied['spec'])})
            receipt={'operation_id':self.fence.context['operation_id'],'namespace_uid':self.namespace_uid,
                     'source_sha':self.fence.context['commit'],'cut_invocation':self.cut.invocation,
                     'package_sha256':package_receipt['package_sha256'],'ordered':targets,
                     'barriers':['space_health','publisher_health','chat_health']}
            marker={'status':'pending','nonce':secrets.token_hex(16),'operation_id':receipt['operation_id']}
            pending=True; self.journal.state['steps'][STEP]=copy.deepcopy(marker); self.journal.save()
            self._guard(); current,rev=self._same_ledger()
            if current!=row or rev!=revision or self.journal.state['steps'].get(STEP)!=marker: raise ValueError()
            current['full_cohort_activation_plan']=copy.deepcopy(marker); current['step']=STEP+'-pending'
            self.fence.ledger.write(rev,current); self._guard()
            if self.package.verify_current(token)!=package_receipt or self._image_records()!=records: raise ValueError()
            for target in targets:
                if not self.image._match(target['name'],self.image._get('deployment',target['name']),target['uid'],
                                         target['spec'],generation=target['generation']): raise ValueError()
            current,rev=self._same_ledger()
            if current.get('full_cohort_activation_plan')!=marker or self.journal.state['steps'].get(STEP)!=marker: raise ValueError()
            done=marker|{'status':'done','receipt_sha256':digest(receipt)}
            local=done|{'receipt':copy.deepcopy(receipt)}
            self.journal.state['steps'][STEP]=copy.deepcopy(local); self.journal.save()
            current['full_cohort_activation_plan']=copy.deepcopy(done); current['step']=STEP+'-done'
            self.fence.ledger.write(rev,current); self._guard()
            if self._same_ledger()[0].get('full_cohort_activation_plan')!=done or self.journal.state['steps'].get(STEP)!=local:
                raise ValueError()
            return copy.deepcopy(receipt)
        except BaseException:
            if pending:
                self.poisoned=True
                try: self.fence.uncertain('activation plan outcome uncertain')
                except BaseException: pass
            raise ActivationError('activation plan unavailable') from None


def verify_completed(phase,cut,token):
    """Reauthenticate the protected plan; current resumed Pods are checked by CUT."""
    try:
        if type(phase) is not ActivationPhase or phase.cut is not cut: raise ValueError()
        phase._guard(); local=copy.deepcopy(phase.journal.state['steps'][STEP]); receipt=local['receipt']
        marker={k:v for k,v in local.items() if k!='receipt'}
        row,_=phase._same_ledger()
        if (set(marker)!={'status','nonce','operation_id','receipt_sha256'} or marker['status']!='done' or
                marker['receipt_sha256']!=digest(receipt) or row.get('full_cohort_activation_plan')!=marker or
                receipt['operation_id']!=phase.fence.context['operation_id'] or receipt['source_sha']!=phase.fence.context['commit'] or
                receipt['cut_invocation']!=cut.invocation or receipt['namespace_uid']!=phase.namespace_uid or
                phase.package.verify_current(token)['package_sha256']!=receipt['package_sha256']): raise ValueError()
        phase._guard(); phase._image_records(); final,revision=phase._same_ledger()
        if (final.get('full_cohort_activation_plan')!=marker or not isinstance(revision,tuple) or len(revision)!=2 or
                any(not isinstance(v,str) or not v for v in revision) or
                phase.journal.state['steps'].get(STEP)!=local): raise ValueError()
        return copy.deepcopy(receipt)
    except BaseException:
        raise ActivationError('activation plan no longer authorized') from None
