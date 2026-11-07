"""Fixed C1 release composition.  This module does not acquire a claim."""
import copy
import os
from pathlib import Path
import select
import stat
import sys
import threading
import time

import canonical_application_package as canonical
import dev_peer_issuer_client as issuer_client
import dev_peer_issuer_preflight as issuer
import dev_realtime_p3_issuer_capability as realtime_capability
import dev_realtime_p3_issuer_client as realtime_client
import full_cohort_backup_phase as backup
import full_cohort_supervisor as supervisor
import space_creator_runtime_profile_delegated as delegated
import space_creator_runtime_profile_target_plan as target_plan
import space_creator_runtime_profile_trust_effect as trust_effect
import space_creator_runtime_profile_config as creator_config
import space_creator_runtime_profile_jdk as target_jdk
import space_creator_peer_phase as peer_phase
import space_native_tool_identity as tool
import space_role_peer_create as role
import realtime_p3_create_only as realtime_p3
import story_media_backup


class DriverError(RuntimeError): pass


def _check_shared_row(fence,row,revision,reads,expected,receiver):
    if (not fence.owned(row) or row.get('status')!='active' or
            not isinstance(revision,tuple) or len(revision)!=2 or any(not isinstance(x,str) or not x for x in revision) or
            any(read!=(row,revision) for read in reads) or
            any(row.get(key)!=value for key,value in expected.items()) or
            receiver.ledger in row or receiver.effects_key in row): raise DriverError('creator ledger changed')


def _ensure_private(parent, name):
    if name not in ('voice-dev-peer-issuer','voice-realtime-p3','preflight','operations'): raise DriverError('directory name invalid')
    with story_media_backup.private_directory(parent) as fd:
        try: os.mkdir(name,0o700,dir_fd=fd); os.fsync(fd)
        except FileExistsError: pass
        child=os.open(name,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=fd)
        try:
            current=os.fstat(child); path=os.stat(name,dir_fd=fd,follow_symlinks=False)
            if (not stat.S_ISDIR(current.st_mode) or current.st_uid!=os.geteuid() or
                    stat.S_IMODE(current.st_mode)!=0o700 or (current.st_dev,current.st_ino)!=(path.st_dev,path.st_ino)):
                raise DriverError('private directory unavailable')
            again=os.stat(name,dir_fd=fd,follow_symlinks=False)
            if (again.st_dev,again.st_ino)!=(current.st_dev,current.st_ino): raise DriverError('private directory changed')
        finally: os.close(child)


def _bases():
    home=Path('/home/pmd'); root=home/'voice-dev-peer-issuer'; p3=home/'voice-realtime-p3'
    _ensure_private(home,'voice-dev-peer-issuer')
    _ensure_private(root,'preflight'); _ensure_private(root,'operations')
    _ensure_private(home,'voice-realtime-p3'); _ensure_private(p3,'operations')
    if (issuer.SOCKET_ROOT!=root/'preflight' or realtime_capability.SOCKET_ROOT!=issuer.SOCKET_ROOT or
            delegated.ROOT!=root/'operations' or realtime_p3.SOCKET_PARENT!=p3/'operations'): raise DriverError('socket roots changed')


def _directory(base, suffix, profile=None):
    """Create only this operation's private socket directories once."""
    with story_media_backup.private_directory(base) as fd:
        os.mkdir(suffix, 0o700, dir_fd=fd); os.fsync(fd)
    with story_media_backup.private_directory(base / suffix) as fd:
        if profile is not None:
            os.mkdir(profile, 0o700, dir_fd=fd); os.fsync(fd)
        info=os.fstat(fd); return (info.st_dev,info.st_ino)


def _realtime_directory(suffix, identity):
    with story_media_backup.private_directory(issuer.SOCKET_ROOT / suffix) as fd:
        info=os.fstat(fd)
        if (info.st_dev,info.st_ino)!=identity: raise DriverError('preflight directory changed')
        os.mkdir('realtime',0o700,dir_fd=fd); os.fsync(fd)
        after=os.stat(issuer.SOCKET_ROOT / suffix,follow_symlinks=False)
        if (after.st_dev,after.st_ino)!=identity: raise DriverError('preflight directory changed')


def _handshake(phase, token, *, realtime=False):
    if realtime:
        if type(phase) is not realtime_capability.RealtimeP3Capability: raise DriverError('Realtime capability required')
        handle=realtime_client.new_capability_trigger(phase,token)
    else:
        handle=issuer_client.new_preflight_trigger(phase,token)
    phase.prepare(); failure=[]
    def trigger():
        try: handle.run()
        except BaseException as error: failure.append(error)
    worker=threading.Thread(target=trigger,daemon=False)
    try: worker.start()
    except BaseException:
        handle.cancel(); phase._uncertain(); phase.close(); raise
    finishing=False; finished=False
    try:
        deadline=time.monotonic()+300
        while True:
            if failure: raise DriverError('issuer preflight trigger failed')
            remaining=deadline-time.monotonic()
            if remaining<=0: raise DriverError('issuer preflight timed out')
            if select.select([phase.listener],[],[],min(remaining,0.5))[0]: break
            if not worker.is_alive(): raise DriverError('issuer preflight ended without a connection')
        finishing=True; receipt=phase.finish(); finished=True; worker.join(300)
        if worker.is_alive() or failure: raise DriverError('issuer preflight outcome uncertain')
        proof=(realtime_capability.verify_current(phase,token) if realtime else issuer.verify_current(phase,token))
        if proof!=receipt: raise DriverError('issuer preflight changed')
        return receipt
    except BaseException:
        try: handle.cancel()
        except BaseException: pass
        warning=time.monotonic()+30
        while worker.is_alive():
            worker.join(1)
            if worker.is_alive() and time.monotonic()>=warning:
                print('DEV_RESOURCE_RECOVERY_REQUIRED issuer trigger still active; retain claim',file=sys.stderr,flush=True)
                warning=time.monotonic()+30
        if not finishing or finished: phase._uncertain()
        try: phase.close()
        except BaseException: pass
        raise


class C1Driver:
    """Retain real phase facades for the one source-owned operation."""
    def __init__(self, operation):
        if type(operation) is not supervisor.Supervisor or not operation.context['operation_id'].startswith('media-c1-'):
            raise DriverError('C1 supervisor required')
        self.supervisor=operation; self.started=False; self.invoked=False
        self.preflight=self.receiver=self.trust=self.target=self.auth_preparation=self.p3_capability=None

    def prepare(self,token,actor,run_id,run_attempt,nodes):
        """Capture current source and build pre-drain inputs, without admission."""
        if self.invoked: raise DriverError('C1 driver already attempted; reconcile')
        self.invoked=True; s=self.supervisor
        try:
            s.prepare(token,actor,run_id,run_attempt,nodes)
            s.prepare_execution(token,actor)
            self.auth_preparation=backup.AuthToolPreparation(s,token)
            return self._prepare_creator(token)
        except BaseException:
            raise DriverError('C1 preparation incomplete; reconcile') from None

    def _prepare_creator(self, token):
        """Build the first portion of the pre-drain plan; this is not admission."""
        s=self.supervisor
        if not self.invoked or self.started: raise DriverError('C1 preparation unavailable or already attempted')
        self.started=True
        try:
            authority=s._creator_authority(); s._creator_guard(token)
            s.prepare_rootless_creator_base(token); s.prepare_peer_targets(token)
            base=(s.creator_config,s.target_jdk,s.peer_target)
            base_local=tuple((p,p.journal,p.fence,p.client,p.fence.ledger,copy.deepcopy(p.journal.intent),
                              copy.deepcopy(p.journal.state['steps']),copy.deepcopy(p.record),tool.SpaceNativeToolPhase._custody(p.journal))
                             for p in base)
            predecessors=copy.deepcopy((s.package.journal.state['steps'],s.inventory.journal.state['steps']))
            source=str(canonical.ROOT); suffix=s.context['operation_id'].removeprefix('media-c1-')
            _bases()
            preflight_directory=_directory(issuer.SOCKET_ROOT,suffix,'creator')
            self._peer_directory=_directory(delegated.ROOT,suffix)
            self._p3_directory=_directory(realtime_p3.SOCKET_PARENT,suffix)
            self._directory_identity=(suffix,preflight_directory,self._peer_directory,self._p3_directory)
            directory_identity=self._directory_identity
            j,f,c=s._open('creator-delegated-preflight-journal',{'dev_peer_issuer_preflight':{
                'source_root':source,'source_sha':s.context['commit'],'profile':'creator'}})
            self.preflight=issuer.PeerIssuerPreflightPhase(j,f,c,s.lock,s.namespace_uid,s.package,s.cut,source,token,'creator')
            receipt=_handshake(self.preflight,token)
            _realtime_directory(suffix,preflight_directory)
            self.p3_capability=realtime_capability.RealtimeP3Capability(self.preflight,token)
            capability=_handshake(self.p3_capability,token,realtime=True)
            if (not isinstance(capability.get('objects'),dict) or
                    set(capability['objects'])!={kind+'/'+name for kind,name in realtime_capability.metadata.MEMBERS} or
                    any(value!={'state':'absent'} for value in capability['objects'].values())):
                raise ValueError()
            j,f,c=s._open('creator-delegated-receiver-journal',{'peer_receiver':{
                'profile':'creator','source_sha':s.context['commit']}})
            self.receiver=delegated.CreatorReceiver(j,f,c,s.lock,s.namespace_uid,s.package,s.cut,self.preflight,source,token)
            existing=delegated.inspect_existing(self.receiver,token)
            space_existing=delegated.inspect_space_from_creator(self.receiver,token)
            role_existing=role.inspect_role_from_creator(self.receiver,token)
            j,f,c=s._open('creator-trust-effect-journal',{})
            self.trust=trust_effect.TrustEffectPhase(j,f,c,s.lock,s.namespace_uid,s.package,s.inventory,s.target_jdk,self.preflight)
            plan=self.trust.prepare(token)
            j,f,c=s._open('creator-target-plan-journal',{'creator_target_plan':{
                'source_root':source,'source_sha':s.context['commit']}})
            self.target=target_plan.TargetPlanPhase(j,f,c,s.lock,s.namespace_uid,s.package,s.inventory,
                s.creator_config,s.target_jdk,self.preflight,self.trust,source)
            target=self.target.run(token,s._execution_actor)
            phases=(self.preflight,self.receiver,self.trust,self.target)
            auth=self.auth_preparation
            if type(auth) is not backup.AuthToolPreparation or auth.supervisor is not s: raise ValueError()
            def capture(p):
                return (p,p.journal,p.fence,p.client,p.fence.ledger,copy.deepcopy(p.journal.intent),
                        copy.deepcopy(p.journal.state['steps']),copy.deepcopy(p.record),tool.SpaceNativeToolPhase._custody(p.journal))
            if issuer.verify_current(phases[0],token)!=receipt: raise ValueError()
            preflight_evidence=capture(phases[0])
            capability_phase=self.p3_capability
            if (type(capability_phase) is not realtime_capability.RealtimeP3Capability or
                    realtime_capability.verify_current(capability_phase,token)!=capability): raise ValueError()
            capability_record=copy.deepcopy(capability_phase.record); capability_refs=capability_phase._refs
            if delegated.inspect_existing(phases[1],token)!=existing: raise ValueError()
            receiver_evidence=capture(phases[1])
            receiver_material=(copy.deepcopy(phases[1].initial),bytes(phases[1].media))
            if (delegated.inspect_space_from_creator(phases[1],token)!=space_existing or
                    role.inspect_role_from_creator(phases[1],token)!=role_existing): raise ValueError()
            if trust_effect.verify_prepared(phases[2],token)!=plan: raise ValueError()
            trust_evidence=capture(phases[2])
            if target_plan.verify_current(phases[3],token,s._execution_actor)['receipt']!=target: raise ValueError()
            evidence=(preflight_evidence,receiver_evidence,trust_evidence,capture(phases[3]))
            auth_receipt=auth.verify_current(token)
            auth_fixed=(auth.supervisor,auth.package,copy.deepcopy(auth.context),auth.actor,copy.deepcopy(auth._receipt))
            s._creator_guard(token)
            reads=[p.fence.ledger.read() for p in base+phases]
            row,revision=s.fence.ledger.read()
            expected={creator_config.LEDGER:base_local[0][7],target_jdk.LEDGER:base_local[1][7],
                      peer_phase.LEDGER:base_local[2][7],phases[0].ledger:preflight_evidence[7],
                      realtime_capability.LEDGER:capability_record,trust_effect.LEDGER:trust_evidence[7],target_plan.LEDGER:evidence[3][7]}
            _check_shared_row(s.fence,row,revision,reads,expected,phases[1])
            s._execution_final(token,authority)
            if (any(a is not b for a,b in zip(base,(s.creator_config,s.target_jdk,s.peer_target))) or
                    self._directory_identity is not directory_identity or
                    directory_identity!=(suffix,preflight_directory,self._peer_directory,self._p3_directory) or
                    self.p3_capability is not capability_phase or capability_phase.creator_preflight is not phases[0] or
                    capability_phase.token!=token or capability_phase._refs is not capability_refs or capability_phase.record!=capability_record or
                    self.auth_preparation is not auth or auth.supervisor is not auth_fixed[0] or auth.package is not auth_fixed[1] or
                    (auth.context,auth.actor,auth._receipt)!=(auth_fixed[2],auth_fixed[3],auth_fixed[4]) or auth_receipt!=auth._receipt or
                    (s.package.journal.state['steps'],s.inventory.journal.state['steps'])!=predecessors or
                    any(a is not b for a,b in zip(phases,(self.preflight,self.receiver,self.trust,self.target))) or
                    (self.receiver.initial,self.receiver.media)!=receiver_material or
                    any(p.journal is not j or p.fence is not f or p.client is not c or f.ledger is not ledger or
                        c.journal is not j or ledger.client is not c or f.context!=s.context or
                        p.lock!=s.lock or p.namespace_uid!=s.namespace_uid or j.poisoned or j.intent!=intent or
                        j.state.get('intent')!=intent or j.state['steps']!=steps or p.record!=record or
                        tool.SpaceNativeToolPhase._custody(j)!=custody
                        for p,j,f,c,ledger,intent,steps,record,custody in base_local+evidence)): raise ValueError()
            return {'operation_id':s.context['operation_id'],'scope':'creator-inputs-only'}
        except BaseException:
            raise DriverError('C1 creator preparation incomplete; reconcile') from None


def verify_p3_directory(driver,token):
    """Recheck the pre-created, operation-owned Realtime P3 socket directory."""
    try:
        if type(driver) is not C1Driver or not driver.started or driver.supervisor._execution_token!=token: raise ValueError()
        s=driver.supervisor; identity=driver._directory_identity
        suffix,_,_,expected=identity
        if (suffix!=s.context['operation_id'].removeprefix('media-c1-') or expected!=driver._p3_directory or
                realtime_p3.SOCKET_PARENT!=Path('/home/pmd/voice-realtime-p3/operations')): raise ValueError()
        s._guard()
        with story_media_backup.private_directory(realtime_p3.SOCKET_PARENT/suffix) as fd:
            info=os.fstat(fd)
            if (info.st_dev,info.st_ino)!=expected: raise ValueError()
        s._guard()
        with story_media_backup.private_directory(realtime_p3.SOCKET_PARENT/suffix) as fd:
            info=os.fstat(fd)
            if (info.st_dev,info.st_ino)!=expected: raise ValueError()
        if driver._directory_identity is not identity or driver._p3_directory!=expected: raise ValueError()
        return {'operation_id':s.context['operation_id'],'scope':'owned-p3-socket-directory'}
    except BaseException: raise DriverError('Realtime P3 directory unavailable') from None
