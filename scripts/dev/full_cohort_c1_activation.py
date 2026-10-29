"""WIP C1 continuation helpers; no complete activation entrypoint yet."""
import os
from pathlib import Path
import re
import stat

import canonical_application_package as canonical
import dev_peer_issuer_preflight as issuer
import full_cohort_supervisor as supervisor_module
import realtime_config_transition_authority as config_authority
import space_creator_runtime_profile_delegated as delegated
import space_creator_runtime_profile_target_trust as target_trust
import space_creator_runtime_profile_trust_effect as trust_effect
import space_fresh_install_phase as fresh
import space_role_peer_create as role
import story_media_backup


class ActivationError(RuntimeError): pass


def _dev(supervisor):
    return (type(supervisor) is supervisor_module.Supervisor and supervisor.context.get('project')=='voice' and
            supervisor.context.get('resource')=='DEV' and bool(re.fullmatch(r'media-c1-[a-f0-9]{32}',supervisor.context.get('operation_id',''))))


def _profile_directory(supervisor,profile):
    """Reserve one fixed later-profile socket directory without adoption."""
    if not _dev(supervisor) or profile not in ('space','role'):
        raise ActivationError('profile authority unavailable')
    context=supervisor.context
    match=re.fullmatch(r'media-c1-([a-f0-9]{32})',context.get('operation_id',''))
    if not match or context.get('project')!='voice' or context.get('resource')!='DEV' or issuer.SOCKET_ROOT!=Path('/home/pmd/voice-dev-peer-issuer/preflight'):
        raise ActivationError('profile directory unavailable')
    try:
        supervisor._guard(); parent=issuer.SOCKET_ROOT/match[1]
        with story_media_backup.private_directory(parent) as fd:
            before=os.fstat(fd); visible=os.stat(parent,follow_symlinks=False)
            if ((before.st_dev,before.st_ino)!=(visible.st_dev,visible.st_ino) or
                    before.st_uid!=os.geteuid() or stat.S_IMODE(before.st_mode)!=0o700): raise ValueError()
            os.mkdir(profile,0o700,dir_fd=fd); os.fsync(fd)
            child=os.open(profile,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=fd)
            try:
                current=os.fstat(child); named=os.stat(profile,dir_fd=fd,follow_symlinks=False)
                after=os.fstat(fd); parent_visible=os.stat(parent,follow_symlinks=False)
                if (not stat.S_ISDIR(current.st_mode) or current.st_uid!=os.geteuid() or
                        stat.S_IMODE(current.st_mode)!=0o700 or (current.st_dev,current.st_ino)!=(named.st_dev,named.st_ino) or
                        (before.st_dev,before.st_ino)!=(after.st_dev,after.st_ino) or
                        (before.st_dev,before.st_ino)!=(parent_visible.st_dev,parent_visible.st_ino)): raise ValueError()
            finally: os.close(child)
        supervisor._guard()
    except BaseException:
        raise ActivationError('profile directory outcome unavailable; reconcile') from None


def _later_receiver(driver,token,profile):
    """Construct the fixed fresh post-effect preflight and its matching receiver."""
    import full_cohort_c1_driver as driver_module
    if type(driver) is not driver_module.C1Driver or profile not in ('space','role'):
        raise ActivationError('profile receiver authority unavailable')
    s=driver.supervisor; source=str(canonical.ROOT)
    if not _dev(s) or not isinstance(token,str) or not token:
        raise ActivationError('profile receiver authority unavailable')
    try:
        s._guard()
        j,f,c=s._open(profile+'-delegated-preflight-journal',{'dev_peer_issuer_preflight':{
            'source_root':source,'source_sha':s.context['commit'],'profile':profile}})
        phase=issuer.PeerIssuerPreflightPhase(j,f,c,s.lock,s.namespace_uid,s.package,s.cut,source,token,profile)
        driver_module._handshake(phase,token)
        j,f,c=s._open(profile+'-delegated-receiver-journal',{'peer_receiver':{
            'profile':profile,'source_sha':s.context['commit']}})
        if profile=='space':
            receiver=delegated.SpaceReceiver(j,f,c,s.lock,s.namespace_uid,s.package,s.cut,phase,source,token)
            result=receiver.run()
            if delegated.verify_completed(receiver,token)!=result: raise ValueError()
        else:
            receiver=role.Session(j,f,c,s.lock,s.namespace_uid,s.package,phase,canonical.ROOT,token)
            receiver.prepare(); result=receiver.run()
            if role.verify_completed(receiver,token)!=result: raise ValueError()
        s._guard()
        return receiver
    except BaseException:
        raise ActivationError('profile receiver outcome unavailable; reconcile') from None


def _target_trust(driver,token):
    """Materialize and verify the exact target trust before Creator effects."""
    import full_cohort_c1_driver as driver_module
    if type(driver) is not driver_module.C1Driver or type(driver.trust) is not trust_effect.TrustEffectPhase:
        raise ActivationError('target trust authority unavailable')
    s=driver.supervisor
    if (not _dev(s) or driver.trust.preflight is not driver.preflight or
            driver.trust.package is not s.package or driver.trust.inventory is not s.inventory or
            driver.trust.jdk is not s.target_jdk or driver.trust.fence.context!=s.context or
            driver.trust.lock!=s.lock or driver.trust.namespace_uid!=s.namespace_uid):
        raise ActivationError('target trust authority unavailable')
    try:
        s._guard(); driver.trust.run(token); trust_effect.verify_current(driver.trust,token)
        j,f,c=s._open('creator-target-trust-journal',{})
        phase=target_trust.TargetTrustPhase(j,f,c,s.lock,s.namespace_uid,s.package,s.inventory,s.target_jdk,driver.preflight)
        receipt=phase.run(token)
        if target_trust.verify_current(phase,token)!=receipt: raise ValueError()
        s._guard()
        return phase
    except BaseException:
        raise ActivationError('target trust outcome unavailable; reconcile') from None


def _creator_and_space(driver,token):
    """Run the fixed C1 peer order, followed by the authorized DEV Space install."""
    import full_cohort_c1_driver as driver_module
    if (type(driver) is not driver_module.C1Driver or not _dev(driver.supervisor) or type(driver.receiver) is not delegated.CreatorReceiver or
            driver.receiver.preflight is not driver.preflight or type(driver.supervisor.space) is not fresh.SpaceFreshInstallPhase):
        raise ActivationError('Creator and Space authority unavailable')
    try:
        trust=_target_trust(driver,token)
        creator_receipt=driver.receiver.run()
        if delegated.verify_completed(driver.receiver,token)!=creator_receipt: raise ValueError()
        space_receiver=_later_receiver(driver,token,'space')
        role_receiver=_later_receiver(driver,token,'role')
        space_authority=config_authority.bind_delegated_space(driver.supervisor.cut,trust,driver.receiver,space_receiver,role_receiver,token)
        space_receipt=driver.supervisor.space.run()
        if fresh.verify_completed(driver.supervisor.space,token)!=space_receipt: raise ValueError()
        return trust,space_receiver,role_receiver,space_authority
    except BaseException:
        raise ActivationError('Creator and Space outcome unavailable; reconcile') from None
