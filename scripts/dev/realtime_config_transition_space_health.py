"""Post-resume Space Pod HTTPS/JWKS health bound to its public plan."""
import base64
import copy
import hashlib
import json
import secrets

import frontend_runtime_executor
import full_cohort_image_apply
import full_cohort_package_phase
import media_operation_guard
import realtime_config_transition_space as space
import realtime_config_transition_authority as peer_authority
import realtime_config_transition_space_plan_phase as planned
import realtime_config_transition_space_service as space_service
import realtime_p3_public_inputs as public
import writer_cut

STEP='realtime-space-health'
LEDGER='realtime_space_health'


class HealthError(RuntimeError): pass


def _bound_public(expected,observation):
    for name,identity in (('voice-media-ca',expected['media_ca']),('voice-space-media-server-tls',expected['tls'])):
        if observation['public_objects'][name]!={'uid':identity['uid'],'resource_version':identity['resourceVersion']}: raise ValueError()


class SpaceHealthPhase:
    def __init__(self,journal,fence,client,lock,namespace_uid,package,cut,plan_phase,image_phase,config_journal):
        self.journal,self.fence,self.client,self.package,self.cut=journal,fence,client,package,cut
        self.plan_phase,self.image_phase,self.config_journal=plan_phase,image_phase,config_journal
        self.lock,self.namespace_uid=copy.deepcopy(lock),namespace_uid
        self.intent=copy.deepcopy(journal.intent); self.plan=self.intent.get('realtime_space_health'); self.record=None
        if (type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal or fence.ledger.client is not client
                or type(package) is not full_cohort_package_phase.PackagePhase or type(cut) is not writer_cut.WriterCut
                or type(image_phase) is not full_cohort_image_apply.FullCohortImageApply
                or any(x.fence.context!=fence.context for x in (package,cut,image_phase))
                or any(x.lock!=lock or x.namespace_uid!=namespace_uid for x in (package,cut,image_phase))
                or self.intent.get('resource_context')!=fence.context or self.intent.get('namespace_uid')!=namespace_uid
                or not isinstance(self.plan,dict) or set(self.plan)!={'source_root','space_pod','destination'}): raise HealthError('Space health identity unavailable')
        if type(plan_phase) is planned.SpacePlanPhase:
            if plan_phase.cut is not cut: raise HealthError('Space plan identity unavailable')
        elif type(plan_phase) is peer_authority.BoundFixed9SpaceAuthority:
            if plan_phase.cut is not cut: raise HealthError('Space peer identity unavailable')
        else:
            import space_p3_issuer_create as issuer
            if (type(plan_phase) is not issuer.Session or plan_phase.fence.context!=fence.context
                    or plan_phase.lock!=lock or plan_phase.namespace_uid!=namespace_uid): raise HealthError('Space issuer identity unavailable')

    def _guard(self):
        if self.journal.poisoned or self.journal.intent!=self.intent or self.journal.state.get('intent')!=self.intent: raise ValueError()
        media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
        if self.record is not None:
            row,_=self.fence.ledger.read(); local=self.journal.state['steps'].get(STEP)
            if row.get(LEDGER)!=self.record or not isinstance(local,dict) or {k:v for k,v in local.items() if k!='receipt'}!=self.record: raise ValueError()
            if self.record['status']=='done' and writer_cut.digest(local.get('receipt'))!=self.record['receipt_sha256']: raise ValueError()

    def _sources(self,token):
        self._guard(); self.image_phase._guard()
        if type(self.plan_phase) is planned.SpacePlanPhase:
            expected=planned.verify(self.plan_phase,self.cut,token)
        elif type(self.plan_phase) is peer_authority.BoundFixed9SpaceAuthority:
            expected,_=peer_authority._space(self.cut,self.plan_phase,token)
            if (expected.get('schema')!='voice-dev-space-delegated-v1' or
                    expected.get('operation_id')!=self.fence.context['operation_id'] or
                    expected.get('source_sha')!=self.fence.context['commit'] or
                    expected.get('namespace_uid')!=self.namespace_uid): raise ValueError()
        else:
            import space_p3_issuer_create as issuer
            expected=issuer.verify_completed(self.plan_phase)
            row,revision=self.fence.ledger.read()
            if (self.plan_phase.fence.ledger.read()!=(row,revision)
                    or expected.get('schema')!='voice-dev-space-p3-issuer-v1'
                    or expected.get('operation_id')!=self.fence.context['operation_id']
                    or expected.get('source_sha')!=self.fence.context['commit']
                    or expected.get('namespace_uid')!=self.namespace_uid
                    or expected.get('scope')!='space-two-secret-create-only'): raise ValueError()
        package=self.package.verify_current(token); proof=self.cut.current_resumed_proof()
        spaces=[x for x in proof['controllers'] if x.get('kind')=='Deployment' and x.get('name')=='voice-space']
        if len(spaces)!=1 or type(spaces[0].get('replicas')) is not int or spaces[0]['replicas']<1: raise ValueError()
        return expected,package,spaces[0]

    def _pod(self,resumed):
        self._guard(); pod=self.client.call('get','pod',self.plan['space_pod'],'-o','json'); self._guard()
        if not self.image_phase._valid_pod('space',pod,resumed['spec']): raise ValueError()
        status=[x for x in pod['status']['containerStatuses'] if x.get('name')=='space']
        if len(status)!=1: raise ValueError()
        meta=pod['metadata']
        return {'name':meta['name'],'uid':meta['uid'],'resourceVersion':meta['resourceVersion'],
                'container_id':status[0]['containerID'],'image_id':status[0]['imageID'],'restart_count':status[0]['restartCount']}

    def _service(self,observation):
        service_proof=space_service.verify_space_service(self.cut,self.config_journal)
        service_identity=observation['identity']['service']
        if (service_proof['receipt']['uid']!=service_identity['uid']
                or service_proof['receipt']['resourceVersion']!=service_identity['resource_version']): raise ValueError()
        for kind,key in (('service','service'),('endpoints','endpoints')):
            self._guard(); obj=self.client.call('get',kind,'voice-space','-o','json'); self._guard()
            meta=obj.get('metadata',{}); expected=observation['identity'][key]
            expected_rv=expected['resource_version'] if key=='service' else expected['resourceVersion']
            if (meta.get('name')!='voice-space' or meta.get('namespace')!='voice-dev' or meta.get('deletionTimestamp')
                    or meta.get('uid')!=expected['uid'] or meta.get('resourceVersion')!=expected_rv): raise ValueError()

    def _service_local(self):
        return copy.deepcopy((self.config_journal.intent,self.config_journal.state.get('intent'),
                              self.config_journal.state['steps'].get(space_service.STEP)))

    def _refs(self,observation):
        for name,identity in observation['public_objects'].items():
            if public._meta(public.common.linux_private_bytes,self._guard,'secret',name)!=identity: raise ValueError()
        self._service(observation)

    def run(self,token):
        pending=False
        try:
            service_local=self._service_local()
            expected,package,resumed=self._sources(token)
            if type(self.plan_phase) is planned.SpacePlanPhase and self.plan['source_root']!=self.plan_phase.plan['source_root']: raise ValueError()
            pod=self._pod(resumed); row,rev=self.fence.ledger.read()
            if STEP in self.journal.state['steps'] or LEDGER in row: raise ValueError()
            record={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],
                    'source_sha':self.fence.context['commit'],'namespace_uid':self.namespace_uid}
            pending=True; self.journal.state['steps'][STEP]=copy.deepcopy(record); self.journal.save()
            current,version=self.fence.ledger.read()
            if current!=row or version!=rev: raise ValueError()
            current[LEDGER]=copy.deepcopy(record); current['step']=STEP+'-pending'
            self.fence.ledger.write(rev,current); self.record=copy.deepcopy(record); self._guard()
            def authority(sha):
                if sha!=self.fence.context['commit']: raise ValueError()
                self.package.verify_current(token)
            result=space.capture(self.plan['source_root'],self.fence.context['commit'],authority,self.plan['space_pod'],
                                 self.namespace_uid,self._guard,self.plan['destination'])
            if result['identity']['workload']['pod_uid']!=pod['uid']: raise ValueError()
            if type(self.plan_phase) is peer_authority.BoundFixed9SpaceAuthority:
                _bound_public(expected,result)
            self._service(result)
            planned_document=(json.loads(base64.b64decode(expected['observation']['jwks_bytes'],validate=True))
                              if type(self.plan_phase) is planned.SpacePlanPhase else expected['public_jwks'])
            # Both parsers enforce the source-defined closed two-key RSA shape.
            if planned_document!=json.loads(result['jwks_bytes']): raise ValueError()
            if self._pod(resumed)!=pod: raise ValueError()
            next_expected,next_package,next_resumed=self._sources(token)
            if next_expected!=expected or next_package!=package or next_resumed!=resumed: raise ValueError()
            receipt={'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit'],
                'namespace_uid':self.namespace_uid,'plan_sha256':writer_cut.digest(expected),'resumed':copy.deepcopy(resumed),
                'pod':pod,'https':{**result,'jwks_bytes':base64.b64encode(result['jwks_bytes']).decode()},
                'scope':'space-resumed-pod-https-not-service-dataplane'}
            self._guard(); row,rev=self.fence.ledger.read()
            done=record|{'status':'done','receipt_sha256':writer_cut.digest(receipt)}
            self.journal.state['steps'][STEP]=copy.deepcopy(done)|{'receipt':copy.deepcopy(receipt)}; self.journal.save()
            current,version=self.fence.ledger.read()
            if current!=row or version!=rev or row.get(LEDGER)!=self.record: raise ValueError()
            current[LEDGER]=copy.deepcopy(done); current['step']=STEP+'-done'; self.fence.ledger.write(rev,current)
            self.record=copy.deepcopy(done); self._guard(); final_row,final_rev=self.fence.ledger.read()
            local=copy.deepcopy(self.journal.state['steps'][STEP])
            next_expected,next_package,next_resumed=self._sources(token)
            if next_expected!=expected or next_package!=package or next_resumed!=resumed: raise ValueError()
            if self._pod(resumed)!=pod: raise ValueError()
            self._refs(result); self._guard()
            after,end=self.fence.ledger.read()
            if (after!=final_row or end!=final_rev or self.journal.state['steps'][STEP]!=local
                    or self._service_local()!=service_local): raise ValueError()
            return copy.deepcopy(receipt)
        except BaseException:
            if pending:
                try: self.fence.uncertain('Space health outcome uncertain')
                except BaseException: pass
            raise HealthError('Space health unavailable') from None


def verify_completed(phase,cut,token):
    """Revalidate durable health without requiring the whole cohort drained."""
    try:
        if type(phase) is not SpaceHealthPhase or phase.cut is not cut: raise ValueError()
        service_local=phase._service_local(); phase._guard(); expected,package,resumed=phase._sources(token)
        row,rev=phase.fence.ledger.read(); local=copy.deepcopy(phase.journal.state['steps'][STEP]); receipt=local['receipt']
        if (phase.record is None or phase.record.get('status')!='done' or receipt['resumed']!=resumed
                or receipt['plan_sha256']!=writer_cut.digest(expected) or receipt['pod']!=phase._pod(resumed)
                or receipt.get('scope')!='space-resumed-pod-https-not-service-dataplane'): raise ValueError()
        observation=receipt['https']; raw=base64.b64decode(observation['jwks_bytes'],validate=True)
        if type(phase.plan_phase) is peer_authority.BoundFixed9SpaceAuthority: _bound_public(expected,observation)
        if hashlib.sha256(raw).hexdigest()!=observation['jwks_sha256']: raise ValueError()
        phase._refs(observation)
        phase._guard(); after,end=phase.fence.ledger.read()
        if after!=row or end!=rev or phase.journal.state['steps'][STEP]!=local or phase._service_local()!=service_local: raise ValueError()
        return copy.deepcopy(receipt)
    except Exception: raise HealthError('Space health unavailable') from None
