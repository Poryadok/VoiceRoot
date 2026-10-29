"""Guarded P1 Auth truststore comparison; no target spec or peer readiness."""
import copy
import hashlib
import json
import os
import re
import secrets
import selectors
import subprocess
import time

import full_cohort_schema_metadata as metadata
import space_creator_runtime_profile_jdk as jdk
import space_creator_runtime_profile_material as material
import space_creator_runtime_profile_trust as trust
import writer_cut

STEP='space-creator-target-trust'
LEDGER='space_creator_target_trust'
TMPFS='rw,noexec,nosuid,nodev,size=2m,mode=0700,uid=65532,gid=65532'
SCRIPT=('umask 077; cat > /scratch/store; /usr/bin/openssl pkcs12 -in /scratch/store -passin pass:changeit -noout; exec /opt/java/openjdk/bin/keytool -list -v '
        '-keystore /scratch/store -storetype PKCS12 -storepass changeit -J-Duser.language=en -J-Duser.country=US')


class TargetTrustError(RuntimeError): pass


def _preflight_layout(phase):
    import space_p3_issuer_preflight as old
    import dev_peer_issuer_preflight as peer
    if type(phase) is old.IssuerPreflightPhase:
        return old,old.STEP,old.LEDGER
    if type(phase) is peer.PeerIssuerPreflightPhase and phase.profile=='creator':
        return peer,phase.step,phase.ledger
    raise ValueError()


def _input(args,payload):
    if not isinstance(payload,bytes) or not 0<len(payload)<=1048576: raise ValueError()
    process=subprocess.Popen(args,stdin=subprocess.PIPE,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL)
    selector=selectors.DefaultSelector(); result=bytearray(); remaining=memoryview(payload); deadline=time.monotonic()+30
    try:
        os.set_blocking(process.stdout.fileno(),False); os.set_blocking(process.stdin.fileno(),False)
        selector.register(process.stdout,selectors.EVENT_READ); selector.register(process.stdin,selectors.EVENT_WRITE)
        while selector.get_map():
            left=deadline-time.monotonic()
            if left<=0: raise TimeoutError()
            events=selector.select(left)
            if not events: raise TimeoutError()
            for event,_ in events:
                if event.fileobj is process.stdout:
                    chunk=os.read(process.stdout.fileno(),min(65536,1048577-len(result)))
                    if not chunk: selector.unregister(process.stdout); process.stdout.close()
                    else:
                        result.extend(chunk)
                        if len(result)>1048576: raise ValueError()
                else:
                    written=os.write(process.stdin.fileno(),remaining[:65536]); remaining=remaining[written:]
                    if not remaining: selector.unregister(process.stdin); process.stdin.close()
        if process.wait(timeout=max(.01,deadline-time.monotonic()))!=0: raise ValueError()
        return bytes(result)
    finally:
        selector.close()
        if process.poll() is None: process.kill(); process.wait()
        for stream in (process.stdin,process.stdout):
            if stream is not None and not stream.closed: stream.close()


class TargetTrustPhase(jdk.TargetJdkPhase):
    def __init__(self,journal,fence,client,lock,namespace_uid,package_phase,inventory_phase,jdk_phase,preflight_phase):
        super().__init__(journal,fence,client,lock,namespace_uid,package_phase,inventory_phase)
        try: _preflight_layout(preflight_phase)
        except BaseException: raise TargetTrustError('target trust identity unavailable') from None
        if (type(jdk_phase) is not jdk.TargetJdkPhase or
                jdk_phase.package is not package_phase or jdk_phase.inventory is not inventory_phase or
                preflight_phase.package is not package_phase or any(x.fence.context!=fence.context or x.lock!=lock or
                x.namespace_uid!=namespace_uid for x in (jdk_phase,preflight_phase))): raise TargetTrustError('target trust identity unavailable')
        self.jdk,self.preflight=jdk_phase,preflight_phase; self._target_refs=(jdk_phase,preflight_phase)

    def _locals(self,row,jdk_digest,preflight_digest):
        try:
            _,pre_step,pre_ledger=_preflight_layout(self.preflight)
            if self._target_refs!=(self.jdk,self.preflight): raise ValueError()
            self._local(); self.jdk._local(); self.preflight._local()
            local=self.jdk.journal.state['steps'][jdk.STEP]; marker={k:v for k,v in local.items() if k!='receipt'}
            prelocal=self.preflight.journal.state['steps'][pre_step]
            if (marker!=self.jdk.record or row.get(jdk.LEDGER)!=marker or marker.get('status')!='done' or
                marker.get('receipt_sha256')!=jdk_digest or marker.get('receipt_sha256')!=writer_cut.digest(local.get('receipt')) or
                row.get(pre_ledger)!=self.preflight.record or self.preflight.record.get('receipt_sha256')!=preflight_digest or
                writer_cut.digest(prelocal.get('receipt'))!=preflight_digest): raise ValueError()
        except BaseException: raise metadata.GuardLost('target trust custody unavailable') from None

    def _dependencies(self,token):
        try:
            preflight,_,_=_preflight_layout(self.preflight)
            if self._target_refs!=(self.jdk,self.preflight): raise ValueError()
            self._guard(); row,rev=self._ledger()
            roots=jdk.verify_completed(self.jdk,token); source=preflight.verify_current(self.preflight,token)
            self._guard()
            if self._ledger()!=(row,rev) or self.jdk._ledger()!=(row,rev) or self.preflight.fence.ledger.read()!=(row,rev): raise ValueError()
            self._locals(row,writer_cut.digest(roots),writer_cut.digest(source))
            return roots,source
        except BaseException: raise metadata.GuardLost('target trust dependency unavailable') from None

    def _state(self,value):
        self._guard(); row,rev=self._ledger(); marker=row.get(LEDGER)
        if not isinstance(marker,dict) or marker.get('status')!='pending' or self.journal.state['steps'].get(STEP)!=marker: raise ValueError()
        new=copy.deepcopy(marker); new['tool']=copy.deepcopy(value)
        self.journal.state['steps'][STEP]=copy.deepcopy(new); self.journal.save(); self._guard()
        current,version=self._ledger()
        if current!=row or version!=rev: raise ValueError()
        current[LEDGER]=copy.deepcopy(new); self.fence.ledger.write(version,current); self.record=copy.deepcopy(new); self._guard()
        if self._ledger()[0].get(LEDGER)!=new: raise ValueError()

    def _capture(self,token):
        roots,source=self._dependencies(token)
        trust_id,store=self._secret('voice-media-jvm-trust','truststore.p12')
        ca_id,ca=self._secret('voice-media-ca','ca.crt'); ca_sha=hashlib.sha256(material._cert_der(ca)).hexdigest()
        if source.get('ca')!={'uid':ca_id['uid'],'resourceVersion':ca_id['resourceVersion'],'der_sha256':ca_sha}: raise ValueError()
        info=json.loads(self._docker(['image','inspect',roots['image']]))
        jdk._image_info(info,roots['image'],roots['config_digest'])
        name='voice-creator-auth-trust-'+secrets.token_hex(16)
        self._state({'status':'create-pending','name':name,'image_id':roots['config_digest']})
        cid=self._docker(['create','-i','--name',name,'--network','none','--read-only','--cap-drop','ALL',
             '--security-opt','no-new-privileges','--tmpfs','/scratch:'+TMPFS,'--user','65532:65532',
             '--label','voice.test.owner='+name,'--entrypoint','/bin/sh',roots['config_digest'],'-ec',SCRIPT],256).decode('ascii').strip()
        if not re.fullmatch(r'[a-f0-9]{64}',cid): raise ValueError()
        info=json.loads(self._docker(['inspect',cid])); item=info[0] if isinstance(info,list) and len(info)==1 else {}
        host=item.get('HostConfig',{}); cfg=item.get('Config',{})
        if (item.get('Id')!=cid or item.get('Image')!=roots['config_digest'] or cfg.get('Labels',{}).get('voice.test.owner')!=name or
                cfg.get('Entrypoint')!=['/bin/sh'] or cfg.get('Cmd')!=['-ec',SCRIPT] or cfg.get('User')!='65532:65532' or
                cfg.get('OpenStdin') is not True or
                host.get('NetworkMode')!='none' or host.get('ReadonlyRootfs') is not True or host.get('CapDrop')!=['ALL'] or
                host.get('SecurityOpt')!=['no-new-privileges'] or host.get('Tmpfs')!={'/scratch':TMPFS} or host.get('PortBindings') or item.get('Mounts')): raise ValueError()
        self._state({'status':'start-pending','name':name,'image_id':roots['config_digest'],'id':cid})
        error=None; facts=None
        try:
            self._guard(); listing=_input(['docker','start','-ai',cid],store); self._guard()
            facts=trust.compare_roots(set(roots['root_fingerprints']),listing,ca_sha)
            status=json.loads(self._docker(['inspect',cid])); item=status[0] if isinstance(status,list) and len(status)==1 else {}
            if item.get('Id')!=cid or item.get('State',{}).get('Status')!='exited' or item.get('State',{}).get('ExitCode')!=0: raise ValueError()
        except BaseException as failure:error=failure
        if isinstance(error,metadata.GuardLost): raise error
        try:
            self._state({'status':'remove-pending','name':name,'image_id':roots['config_digest'],'id':cid})
            check=json.loads(self._docker(['inspect',cid])); item=check[0] if isinstance(check,list) and len(check)==1 else {}
            if item.get('Id')!=cid or item.get('Image')!=roots['config_digest'] or item.get('Config',{}).get('Labels',{}).get('voice.test.owner')!=name: raise ValueError()
            self._docker(['rm','-f','-v',cid],256)
            if cid in self._docker(['ps','-a','--no-trunc','--format','{{.ID}}']).decode('ascii').splitlines(): raise ValueError()
            self._state({'status':'removed','name':name,'image_id':roots['config_digest'],'id':cid})
        except BaseException as cleanup:
            if isinstance(error,metadata.GuardLost) or isinstance(cleanup,metadata.GuardLost): raise metadata.GuardLost('target trust cleanup guard unavailable') from None
            raise TargetTrustError('target trust cleanup uncertain') from None
        if error is not None: raise error
        if (self._secret('voice-media-jvm-trust','truststore.p12')!=(trust_id,store) or self._secret('voice-media-ca','ca.crt')!=(ca_id,ca) or
                self._dependencies(token)!=(roots,source)): raise ValueError()
        return {'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit'],
                'namespace_uid':self.namespace_uid,'jdk_receipt_sha256':writer_cut.digest(roots),
                'preflight_receipt_sha256':writer_cut.digest(source),'trust_secret':trust_id,'media_ca':source['ca'],
                'root_count':facts['root_count'],'store_count':facts['store_count'],
                'scope':'p1-auth-public-truststore-only-no-target-spec-or-peer-readiness'}

    def run(self,token):
        pending=False
        try:
            self._guard(); self._dependencies(token); row,rev=self._ledger()
            if STEP in self.journal.state['steps'] or LEDGER in row: raise ValueError()
            marker={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit']}
            pending=True; self.journal.state['steps'][STEP]=copy.deepcopy(marker); self.journal.save(); self._guard()
            current,version=self._ledger()
            if current!=row or version!=rev: raise ValueError()
            current[LEDGER]=copy.deepcopy(marker); current['step']=STEP+'-pending'; self.fence.ledger.write(version,current); self.record=marker
            receipt=self._capture(token); self._guard(); current,version=self._ledger()
            if current.get(LEDGER)!=self.record or self.journal.state['steps'].get(STEP)!=self.record or self.record.get('tool',{}).get('status')!='removed': raise ValueError()
            done=self.record|{'status':'done','receipt_sha256':writer_cut.digest(receipt)}
            self.journal.state['steps'][STEP]=copy.deepcopy(done)|{'receipt':copy.deepcopy(receipt)}; self.journal.save()
            current[LEDGER]=copy.deepcopy(done); current['step']=STEP+'-done'; self.fence.ledger.write(version,current); self.record=done
            self._guard()
            final,_=self._ledger()
            if final.get(LEDGER)!=done: raise ValueError()
            self._locals(final,receipt['jdk_receipt_sha256'],receipt['preflight_receipt_sha256'])
            if self.record!=done or self.journal.state['steps'].get(STEP)!=done|{'receipt':receipt}: raise ValueError()
            return copy.deepcopy(receipt)
        except BaseException as error:
            if pending:
                self.poisoned=True
                try:self.fence.uncertain('target Auth trust observation uncertain')
                except BaseException:pass
            if isinstance(error,metadata.GuardLost): raise
            raise TargetTrustError('target Auth trust observation unavailable') from None


def verify_current(phase,token):
    """Recheck protected target trust metadata while original preflight is current."""
    try:
        if type(phase) is not TargetTrustPhase: raise ValueError()
        phase._guard(); row,rev=phase._ledger(); local=copy.deepcopy(phase.journal.state['steps'][STEP]); receipt=local['receipt']
        marker={k:v for k,v in local.items() if k!='receipt'}
        if marker!=phase.record or marker.get('status')!='done' or marker.get('receipt_sha256')!=writer_cut.digest(receipt) or row.get(LEDGER)!=marker: raise ValueError()
        roots,source=phase._dependencies(token); trust_id,_=phase._secret('voice-media-jvm-trust','truststore.p12')
        ca_id,ca=phase._secret('voice-media-ca','ca.crt'); ca_sha=hashlib.sha256(material._cert_der(ca)).hexdigest()
        if (receipt['jdk_receipt_sha256']!=writer_cut.digest(roots) or receipt['preflight_receipt_sha256']!=writer_cut.digest(source) or
                receipt['trust_secret']!=trust_id or receipt['media_ca']!=source['ca'] or
                source['ca']!={'uid':ca_id['uid'],'resourceVersion':ca_id['resourceVersion'],'der_sha256':ca_sha}): raise ValueError()
        phase._guard(); after,end=phase._ledger()
        if after!=row or end!=rev or phase.journal.state['steps'].get(STEP)!=local: raise ValueError()
        phase._locals(after,receipt['jdk_receipt_sha256'],receipt['preflight_receipt_sha256'])
        if phase.record!=marker or phase.journal.state['steps'].get(STEP)!=local: raise metadata.GuardLost('target trust record changed')
        return copy.deepcopy(receipt)
    except metadata.GuardLost: raise
    except BaseException: raise TargetTrustError('target Auth trust receipt unavailable') from None


def verify_successor(phase,token,creator_session):
    """Recheck trust after the exact guarded five-Secret creation successor."""
    try:
        import space_creator_runtime_profile_create as creator
        import space_p3_issuer_preflight as preflight
        if (type(phase) is not TargetTrustPhase or type(creator_session) is not creator.Session or
                creator_session.preflight is not phase.preflight or creator_session.package is not phase.package or
                creator_session.fence.context!=phase.fence.context or creator_session.lock!=phase.lock or
                creator_session.namespace_uid!=phase.namespace_uid): raise ValueError()
        phase._guard(); row,rev=phase._ledger(); local=copy.deepcopy(phase.journal.state['steps'][STEP]); receipt=local['receipt']
        marker={k:v for k,v in local.items() if k!='receipt'}
        if marker!=phase.record or marker.get('status')!='done' or marker.get('receipt_sha256')!=writer_cut.digest(receipt) or row.get(LEDGER)!=marker: raise ValueError()
        original=copy.deepcopy(phase.preflight.journal.state['steps'][preflight.STEP]['receipt'])
        try:roots=jdk.verify_completed(phase.jdk,token); issued=creator.verify_completed(creator_session,token)
        except BaseException:raise metadata.GuardLost('target trust successor dependency unavailable') from None
        if (writer_cut.digest(original)!=receipt['preflight_receipt_sha256'] or writer_cut.digest(roots)!=receipt['jdk_receipt_sha256'] or
                issued.get('preflight_sha256')!=receipt['preflight_receipt_sha256'] or issued.get('operation_id')!=receipt['operation_id'] or
                issued.get('source_sha')!=receipt['source_sha'] or issued.get('namespace_uid')!=receipt['namespace_uid']): raise ValueError()
        trust_id,_=phase._secret('voice-media-jvm-trust','truststore.p12'); ca_id,ca=phase._secret('voice-media-ca','ca.crt')
        ca_sha=hashlib.sha256(material._cert_der(ca)).hexdigest()
        if (receipt['trust_secret']!=trust_id or receipt['media_ca']!=original['ca'] or
                original['ca']!={'uid':ca_id['uid'],'resourceVersion':ca_id['resourceVersion'],'der_sha256':ca_sha}): raise ValueError()
        phase._guard(); after,end=phase._ledger()
        try:
            if after!=row or end!=rev or creator_session.fence.ledger.read()!=(row,rev): raise ValueError()
            phase._locals(after,receipt['jdk_receipt_sha256'],receipt['preflight_receipt_sha256']); creator_session._local()
        except BaseException:raise metadata.GuardLost('target trust successor authority changed') from None
        if (phase.record!=marker or phase.journal.state['steps'].get(STEP)!=local or
                creator_session.record.get('receipt')!=issued or after.get(creator.LEDGER)!=creator_session.record or
                after.get(creator.EFFECTS)!=creator_session.effects): raise metadata.GuardLost('target trust successor record changed')
        return copy.deepcopy(receipt)
    except metadata.GuardLost: raise
    except BaseException: raise TargetTrustError('target Auth trust successor unavailable') from None


def verify_fixed9_successor(phase,token,creator,space,role_phase):
    """Recheck P1 trust after the exact Creator, Space and Role transitions."""
    import space_creator_runtime_profile_fixed9_successor as successor
    try:
        if type(phase) is not TargetTrustPhase: raise ValueError()
        phase._guard(); row,rev=phase._ledger(); local=copy.deepcopy(phase.journal.state['steps'][STEP]); receipt=local['receipt']
        marker={k:v for k,v in local.items() if k!='receipt'}
        if marker!=phase.record or marker.get('status')!='done' or marker.get('receipt_sha256')!=writer_cut.digest(receipt) or row.get(LEDGER)!=marker: raise ValueError()
        roots=jdk.verify_completed(phase.jdk,token); original=successor.verify(phase,token,creator,space,role_phase)
        trust_id,_=phase._secret('voice-media-jvm-trust','truststore.p12'); ca_id,ca=phase._secret('voice-media-ca','ca.crt')
        if (writer_cut.digest(roots)!=receipt['jdk_receipt_sha256'] or writer_cut.digest(original)!=receipt['preflight_receipt_sha256'] or
                receipt['trust_secret']!=trust_id or receipt['media_ca']!=original['ca'] or
                original['ca']!={'uid':ca_id['uid'],'resourceVersion':ca_id['resourceVersion'],'der_sha256':hashlib.sha256(material._cert_der(ca)).hexdigest()}): raise ValueError()
        phase._guard(); after,end=phase._ledger()
        if (after,end)!=(row,rev) or successor.verify(phase,token,creator,space,role_phase)!=original: raise ValueError()
        phase._locals(after,receipt['jdk_receipt_sha256'],receipt['preflight_receipt_sha256'])
        if phase.record!=marker or phase.journal.state['steps'].get(STEP)!=local: raise metadata.GuardLost('target trust record changed')
        return copy.deepcopy(receipt)
    except metadata.GuardLost: raise
    except BaseException: raise TargetTrustError('fixed peer target trust successor unavailable') from None
