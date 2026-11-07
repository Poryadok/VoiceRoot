"""Authenticate the finished C1 activation while retaining its live fence."""
import copy
import re
import secrets

import frontend_runtime_executor
import full_cohort_activation_plan as activation
import full_cohort_image_apply as images
import full_cohort_package_phase as package
import media_operation_guard
import media_operation_journal
import media_backup_upload_runner
import full_cohort_chat_health as chat_health
import realtime_config_transition_space_health as space_health
import realtime_jwks_publisher_health as publisher_health
import resource_fencing
import space_native_tool_identity as custody
import writer_cut


STEP='full-cohort-release'
LEDGER='full_cohort_release'


class ReleaseError(RuntimeError): pass


class ReleasePhase:
    """Bind current post-resume health to the actual package and operation."""
    def __init__(self,journal,fence,client,lock,namespace_uid,package_phase,image_phase,activation_phase,cut):
        if (type(journal) is not media_operation_journal.Journal or type(fence) is not resource_fencing.Fence or
                type(fence.ledger) is not resource_fencing.KubeLedger or
                type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal or
                fence.ledger.client is not client or type(package_phase) is not package.PackagePhase or
                type(image_phase) is not images.FullCohortImageApply or type(activation_phase) is not activation.ActivationPhase or
                type(cut) is not writer_cut.WriterCut or activation_phase.package is not package_phase or
                activation_phase.image is not image_phase or activation_phase.cut is not cut or
                image_phase.admission.cut is not cut or journal.intent.get('resource_context')!=fence.context or
                journal.intent.get('namespace_uid')!=namespace_uid or
                not re.fullmatch(r'media-c1-[a-f0-9]{32}',fence.context.get('operation_id',''))):
            raise ReleaseError('release identity unavailable')
        self.journal,self.fence,self.client=journal,fence,client
        self.lock,self.namespace_uid=copy.deepcopy(lock),namespace_uid
        self.package,self.image,self.activation,self.cut=package_phase,image_phase,activation_phase,cut
        self.context=copy.deepcopy(fence.context); self.intent=copy.deepcopy(journal.intent); self.record=None; self.poisoned=False
        classes={'space_health':space_health.SpaceHealthPhase,'publisher_health':publisher_health.PublisherHealthPhase,
                 'chat_health':chat_health.ChatHealthPhase}
        if not isinstance(cut.activation,dict) or not isinstance(cut.activation.get('health'),dict) or set(cut.activation['health'])!=set(classes):
            raise ReleaseError('release health unavailable')
        self._activation_binding=cut.activation; self._health_binding=cut.activation['health']; self._health=[]
        self._activation_values=copy.deepcopy({k:v for k,v in cut.activation.items() if k not in ('phase','health')})
        for name,cls in classes.items():
            binding=self._health_binding[name]
            if not isinstance(binding,dict) or type(binding.get('phase')) is not cls or not isinstance(binding.get('token'),str):
                raise ReleaseError('release health identity unavailable')
            self._health.append((name,binding['phase'],copy.deepcopy({k:v for k,v in binding.items() if k!='phase'}),
                                 copy.deepcopy(getattr(binding['phase'],'record',None))))
        phases=(self,package_phase,image_phase,activation_phase,cut)+tuple(item[1] for item in self._health)
        journals=[p.journal for p in phases]
        if len({id(j) for j in journals})!=len(journals): raise ReleaseError('release journals must be distinct')
        self._facades=tuple((p,p.journal,p.fence,p.client,p.fence.ledger,p.fence.check,p.fence.on_uncertain,copy.deepcopy(p.journal.intent),
                             custody.SpaceNativeToolPhase._custody(p.journal)) for p in phases)
        self._refs=(self.package,self.image,self.activation,self.cut)
        self._local()

    def _local(self):
        if (self.poisoned or any(a is not b for a,b in zip((self.package,self.image,self.activation,self.cut),self._refs)) or
                self.fence.context!=self.context or self.journal.intent!=self.intent or self.journal.state.get('intent')!=self.intent or
                self.activation.package is not self.package or self.activation.image is not self.image or
                self.activation.cut is not self.cut or self.image.admission.cut is not self.cut): raise ValueError()
        if (self.cut.activation is not self._activation_binding or self.cut.activation.get('phase') is not self.activation or
                {k:v for k,v in self.cut.activation.items() if k not in ('phase','health')}!=self._activation_values or
                self.cut.activation.get('health') is not self._health_binding or
                set(self._health_binding)!={x[0] for x in self._health}): raise ValueError()
        for name,phase,details,record in self._health:
            binding=self._health_binding.get(name)
            if (not isinstance(binding,dict) or binding.get('phase') is not phase or
                    {k:v for k,v in binding.items() if k!='phase'}!=details or getattr(phase,'record',None)!=record): raise ValueError()
        for phase,j,f,c,ledger,check,uncertain,intent,held in self._facades:
            if (type(j) is not media_operation_journal.Journal or type(f) is not resource_fencing.Fence or
                    type(c) is not frontend_runtime_executor.KubectlTransport or type(ledger) is not resource_fencing.KubeLedger or
                    type(check) is not resource_fencing.ResourceCheck or check is not self.fence.check or
                    check.context!=self.context or check.endpoint!=media_backup_upload_runner.TT or
                    f.check is not check or f.on_uncertain is not uncertain or
                    getattr(uncertain,'__self__',None) is not check or
                    getattr(uncertain,'__func__',None) is not resource_fencing.ResourceCheck.mark_uncertain or
                    phase.journal is not j or phase.fence is not f or phase.client is not c or f.ledger is not ledger or
                    c.journal is not j or ledger.client is not c or f.context!=self.context or phase.lock!=self.lock or
                    phase.namespace_uid!=self.namespace_uid or getattr(phase,'poisoned',False) or j.poisoned or j.intent!=intent or j.state.get('intent')!=intent or
                    custody.SpaceNativeToolPhase._custody(j)!=held): raise ValueError()

    def _guard(self):
        self._local(); media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid); self._local()

    def _ledger(self):
        observations=[p.fence.ledger.read() for p,*_ in self._facades]
        row,revision=self.fence.ledger.read()
        if (not self.fence.owned(row) or row.get('status')!='active' or
                not isinstance(revision,tuple) or len(revision)!=2 or any(not isinstance(x,str) or not x for x in revision) or
                any(item!=(row,revision) for item in observations)): raise ValueError()
        return row,revision

    def _observe(self,token):
        self._guard(); before,before_revision=self._ledger()
        steps=tuple(copy.deepcopy(j.state['steps']) for _,j,*_ in self._facades)
        pkg=self.package.verify_current(token)
        if (not isinstance(pkg,dict) or not re.fullmatch(r'[a-f0-9]{64}',pkg.get('package_sha256','')) or
                self.cut.drained or not self.cut.started or self.cut.activation is None or
                self.cut.activation.get('phase') is not self.activation or self.cut.activation.get('token')!=token or
                any(details.get('token')!=token for _,_,details,_ in self._health)): raise ValueError()
        stage=self.cut.verify_activation_stage()
        if stage.get('stage')!='done': raise ValueError()
        self.cut._activation_verify_sources()
        resumed=self.cut.current_resumed_proof(); controllers=resumed.get('controllers')
        expected={(x['kind'],x['metadata']['name']) for x in self.cut.admission['controllers']}
        if (not isinstance(controllers,list) or len(controllers)!=len(expected) or
                {(x.get('kind'),x.get('name')) for x in controllers}!=expected): raise ValueError()
        observed=self.image.verify_after_resume()
        if not isinstance(observed,dict) or set(observed)!=set(images.CATALOG): raise ValueError()
        self.cut._activation_verify_sources()
        if self.cut.verify_activation_stage()!=stage or self.package.verify_current(token)!=pkg: raise ValueError()
        self._guard(); row,revision=self._ledger(); self._local()
        if (row!=before or revision!=before_revision or
                any(j.state['steps']!=saved for (_,j,*_),saved in zip(self._facades,steps))): raise ValueError()
        return {'operation_id':self.context['operation_id'],'source_sha':self.context['commit'],
                'namespace_uid':self.namespace_uid,'package_sha256':pkg['package_sha256']},row,revision,steps[1:]

    def run(self,token):
        pending=False
        try:
            receipt,row,revision,dependencies=self._observe(token)
            if STEP in self.journal.state['steps'] or LEDGER in row: raise ValueError()
            marker={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.context['operation_id']}
            pending=True; self.journal.state['steps'][STEP]=copy.deepcopy(marker); self.journal.save(); self._guard()
            if self._ledger()!=(row,revision): raise ValueError()
            row[LEDGER]=copy.deepcopy(marker); row['step']=STEP+'-pending'; self.fence.ledger.write(revision,row)
            self.record=copy.deepcopy(marker); current,current_row,current_revision,current_dependencies=self._observe(token)
            if (current!=receipt or current_dependencies!=dependencies or current_row.get(LEDGER)!=marker or
                    self.journal.state['steps'].get(STEP)!=marker): raise ValueError()
            done=marker|{'status':'done','receipt_sha256':writer_cut.digest(receipt)}
            local=done|{'receipt':copy.deepcopy(receipt)}; self.journal.state['steps'][STEP]=copy.deepcopy(local); self.journal.save()
            current_row[LEDGER]=copy.deepcopy(done); current_row['step']=STEP+'-done'; self.fence.ledger.write(current_revision,current_row)
            self.record=copy.deepcopy(done); self._guard(); final,_=self._ledger(); self._local()
            if (final!=current_row or final.get(LEDGER)!=done or self.journal.state['steps'].get(STEP)!=local or self.record!=done or
                    any(j.state['steps']!=saved for (_,j,*_),saved in zip(self._facades[1:],dependencies))): raise ValueError()
            return copy.deepcopy(receipt)
        except BaseException:
            if pending:
                self.poisoned=True
                try:self.fence.uncertain('C1 release qualification uncertain')
                except BaseException:pass
            raise ReleaseError('C1 release qualification unavailable') from None


def verify_completed(phase,token):
    try:
        if type(phase) is not ReleasePhase: raise ValueError()
        phase._guard(); row,revision=phase._ledger(); local=copy.deepcopy(phase.journal.state['steps'][STEP])
        receipt=local['receipt']; marker={k:v for k,v in local.items() if k!='receipt'}
        if (marker!=phase.record or marker.get('status')!='done' or marker.get('receipt_sha256')!=writer_cut.digest(receipt) or
                row.get(LEDGER)!=marker): raise ValueError()
        current,after,end,_=phase._observe(token)
        if current!=receipt or after!=row or end!=revision or phase.journal.state['steps'].get(STEP)!=local: raise ValueError()
        phase._local()
        if phase.record!=marker or phase.journal.state['steps'].get(STEP)!=local: raise ValueError()
        return copy.deepcopy(receipt)
    except BaseException: raise ReleaseError('C1 release proof unavailable') from None
