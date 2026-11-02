"""Guarded Creator material producer; not runtime or release qualification."""
import copy
import hashlib
import ipaddress
import json
from pathlib import Path
import re
import secrets

import canonical_application_package as canonical
import frontend_runtime_executor as transport
import full_cohort_package_phase as package
import full_cohort_schema_metadata as metadata
import media_operation_guard
import media_operation_journal
import publish_candidate
import realtime_config_transition_authority as authority
import realtime_p3_create_only_transport as private
import resource_fencing
import space_creator_runtime_profile_material as material
import writer_cut


class ProfileError(RuntimeError): pass


STEP='space-creator-material'
LEDGER='space_creator_material'
PINS={
 'src/backend/space/creator_runtime.go':'bb7b7c60daceac65413870dad04e821b1a50c4cfc531927553befb4aac2326ea',
 'src/backend/space/internal/lifecycleprincipal/runtime.go':'58593d8821d53daa8113e0d05b7f380bf0b9480a84a416a07c55d8dd46eef0c8',
 'src/backend/gateway/space_lifecycle_transport.go':'4d25c95b8e51ef5d1f83c046f237df9288e1099299b717940e60d9288d8d13bb',
 'src/backend/gateway/principal_issuer.go':'0029af8557ea2b910c790fc5dae878fbc3881f31792d0db905602484cf51e554',
 'src/backend/auth/src/main/java/voice/backend/auth/principal/AuthPrincipalTransport.java':'e884dfed6c9d36c152992320373792ba236f182dda2392ca603d392881a3acfb',
 'src/backend/space/principal_runtime.go':'f6cf15e8a36e5eb27126813dd42e189bda68b8a7825fe890bc87d09f6c865cd3',
 'src/backend/space/main.go':'0305026e763ee771c7893ade9c24aa0455490dd96e07a42915a42883a6057ce8',
 'src/backend/space/internal/grpcsvc/public_creator.go':'232ebedfffb4c46f779851263ec80ee3b17966efc7379ec61da44382e8f7f86e',
 'src/backend/space/internal/grpcsvc/lifecycle_public_join.go':'106aa26670a296fc7392475ab94ddd4f9eba88344fcc8b8487d187d2e51e26fd',
 'src/backend/space/internal/grpcsvc/invite_permission.go':'7b3ffbaeb76f110e3596a27e6b9fe763c0b7927d2ef3dc9e5974f8b4979e68e4',
 'src/backend/space/internal/grpcsvc/join.go':'5e5b7d4a2401e3dfc74d8275d7678ceb594a3721ecf84f98615ec3b2ce88e8ff',
 'src/backend/space/internal/grpcsvc/privacy.go':'bc51a29157ab297822141f2e0a372754ac7f6337dd43173e512cfe2a58631d4f',
}


class CreatorProfilePhase:
    """One protected observation over exact current material and source."""
    def __init__(self,journal,fence,client,lock,namespace_uid,cut,package_phase,space_phase,source_root,token):
        if (type(journal) is not media_operation_journal.Journal or type(fence) is not resource_fencing.Fence or
                type(client) is not transport.KubectlTransport or client.journal is not journal or
                getattr(fence.ledger,'client',None) is not client or type(cut) is not writer_cut.WriterCut or
                type(package_phase) is not package.PackagePhase or cut.fence.context!=fence.context or
                getattr(cut.fence.ledger,'client',None) is not cut.client or
                getattr(package_phase.fence.ledger,'client',None) is not package_phase.client or
                package_phase.fence.context!=fence.context or cut.lock!=lock or package_phase.lock!=lock or
                cut.namespace_uid!=namespace_uid or package_phase.namespace_uid!=namespace_uid or
                journal.intent.get('resource_context')!=fence.context or journal.intent.get('namespace_uid')!=namespace_uid or
                not isinstance(token,str) or not token or Path(source_root).resolve()!=canonical.ROOT.resolve()):
            raise ProfileError('Creator profile identity unavailable')
        self.journal,self.fence,self.client,self.cut,self.package,self.space_phase=journal,fence,client,cut,package_phase,space_phase
        self.lock,self.namespace_uid,self.source_root,self.token=copy.deepcopy(lock),namespace_uid,Path(source_root).resolve(),token
        self.intent=copy.deepcopy(journal.intent); self.record=None; self.poisoned=False
        self._refs=(journal,fence,client,cut,package_phase,space_phase,self.source_root,token)

    def _guard(self):
        if (self.poisoned or self.journal.poisoned or self.journal.intent!=self.intent or self.journal.state.get('intent')!=self.intent or
                self._refs!=(self.journal,self.fence,self.client,self.cut,self.package,self.space_phase,self.source_root,self.token)):
            raise metadata.GuardLost('Creator profile identity changed')
        metadata._fence(lambda:media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid))

    def _ledger(self):
        try:
            row,rev=self.fence.ledger.read()
            if (not self.fence.owned(row) or row.get('status')!='active' or not isinstance(rev,tuple) or len(rev)!=2 or
                    any(not isinstance(v,str) or not v for v in rev) or
                    self.cut.fence.ledger.read()!=(row,rev) or self.package.fence.ledger.read()!=(row,rev)): raise ValueError()
            return row,rev
        except BaseException: raise metadata.GuardLost('Creator profile ledger changed') from None

    def _source(self):
        try:self.package.verify_current(self.token)
        except BaseException:raise metadata.GuardLost('Creator package authority unavailable') from None
        snapshot=publish_candidate.source_snapshot(self.source_root,self.fence.context['commit'])
        if snapshot.get('source_sha')!=self.fence.context['commit']: raise ValueError()
        for name,expected in PINS.items():
            path=self.source_root/name
            if path.is_symlink() or not path.is_file() or snapshot.get('files',{}).get(name)!=expected or hashlib.sha256(path.read_bytes()).hexdigest()!=expected:
                raise ValueError()

    def _get_secret(self,name):
        if name not in material.SECRETS: raise ValueError()
        self._guard(); raw=private._run(['get','secret',name,'--ignore-not-found=true','-o','json']); self._guard()
        if not raw: raise ValueError()
        return json.loads(raw)

    def _redis(self):
        def get(kind,name):
            self._guard(); value=self.client.call('get',kind,name,'-o','json'); self._guard(); return value
        service=get('service','voice-redis'); endpoints=get('endpoints','voice-redis')
        sm,em=service['metadata'],endpoints['metadata']; spec=service['spec']; subsets=endpoints.get('subsets',[])
        if (sm.get('namespace')!='voice-dev' or em.get('namespace')!='voice-dev' or sm.get('name')!='voice-redis' or
                em.get('name')!='voice-redis' or sm.get('deletionTimestamp') or em.get('deletionTimestamp') or
                any(not isinstance(x.get(k),str) or not x[k] for x in (sm,em) for k in ('uid','resourceVersion')) or
                spec.get('selector')!={'app':'voice-redis'} or spec.get('type','ClusterIP')!='ClusterIP' or
                not isinstance(spec.get('clusterIP'),str) or spec['clusterIP'] in ('','None') or
                len(subsets)!=1 or subsets[0].get('notReadyAddresses') or len(subsets[0].get('addresses',[]))!=1 or
                len(spec.get('ports',[]))!=1 or spec['ports'][0].get('port')!=6379 or
                spec['ports'][0].get('targetPort',6379)!=6379 or spec['ports'][0].get('protocol','TCP')!='TCP' or
                len(subsets[0].get('ports',[]))!=1 or subsets[0]['ports'][0].get('port')!=6379 or
                subsets[0]['ports'][0].get('protocol','TCP')!='TCP'): raise ValueError()
        address=subsets[0]['addresses'][0]; ref=address['targetRef']; name=ref['name']
        if ref.get('kind')!='Pod' or ref.get('namespace','voice-dev')!='voice-dev' or not re.fullmatch(r'[a-z0-9]([-a-z0-9]*[a-z0-9])?',name): raise ValueError()
        pod=get('pod',name); pm=pod['metadata']; statuses=[x for x in pod.get('status',{}).get('containerStatuses',[]) if x.get('name')=='redis']
        if (pm.get('uid')!=ref.get('uid') or not pm.get('resourceVersion') or pm.get('deletionTimestamp') or
                pm.get('labels',{}).get('app')!='voice-redis' or pod.get('status',{}).get('podIP')!=address.get('ip') or len(statuses)!=1 or statuses[0].get('ready') is not True or
                not statuses[0].get('containerID') or type(statuses[0].get('restartCount')) is not int): raise ValueError()
        pod_ip=address.get('ip')
        if not isinstance(pod_ip,str) or str(ipaddress.ip_address(pod_ip))!=pod_ip: raise ValueError()
        snapshot={'service':{'uid':sm['uid'],'resourceVersion':sm['resourceVersion']},
                  'endpoints':{'uid':em['uid'],'resourceVersion':em['resourceVersion']},
                  'pod':{'uid':pm['uid'],'resourceVersion':pm['resourceVersion'],'name':name,
                         'container_id':statuses[0]['containerID'],'restart_count':statuses[0]['restartCount']},
                  'scope':'redis-pod-ip-only-service-path-unverified'}
        commands=[(('PING',),b'PONG'),(('ACL','WHOAMI'),b'default'),
          (('ACL','DRYRUN','default','SET','principal:space:gateway-lifecycle:gateway:fixture','1','NX','PX','1000'),b'OK'),
          (('ACL','DRYRUN','default','GET','auth:session:min_epoch:00000000-0000-0000-0000-000000000001'),b'OK')]
        for args,expected in commands:
            self._guard(); raw=metadata.linux_private_bytes(transport.KubectlTransport.PREFIX+['exec',name,'-c','redis','--',
                'env','-u','REDISCLI_AUTH','redis-cli','--raw','-h',pod_ip,'-p','6379',*args]); self._guard()
            if raw.strip()!=expected: raise ValueError()
        if get('service','voice-redis')!=service or get('endpoints','voice-redis')!=endpoints or get('pod',name)!=pod: raise ValueError()
        return snapshot

    def _observe(self):
        self._guard(); metadata._fence(self.cut._guard); self._source()
        try:proof,jwks=authority._space(self.cut,self.space_phase,self.token)
        except BaseException:raise metadata.GuardLost('Space issuer authority unavailable') from None
        objects={name:self._get_secret(name) for name in sorted(material.SECRETS)}
        result=material.validate(objects,jwks)
        if proof.get('schema')=='voice-dev-space-p3-issuer-v1':
            if (result['objects']['voice-space-principal-signing']!=proof['signer'] or
                    any(result['objects']['voice-space-media-server-tls'][k]!=proof['tls'][k] for k in ('uid','resourceVersion')) or
                    result['certificate_sha256']['voice-space-media-server-tls']!=proof['tls']['certificate_sha256'] or
                    any(result['objects']['voice-media-ca'][k]!=proof['media_ca'][k] for k in ('uid','resourceVersion')) or
                    result['media_ca_sha256']!=proof['media_ca']['der_sha256']): raise ValueError()
        else:
            observation=proof['observation']; signer=observation['signer']['secret']; sources=observation['public_sources']
            if (result['objects']['voice-space-principal-signing']!={'uid':signer['uid'],'resourceVersion':signer['resource_version']} or
                    result['objects']['voice-media-ca']!={'uid':sources['voice-media-ca']['uid'],'resourceVersion':sources['voice-media-ca']['resource_version']} or
                    result['objects']['voice-space-media-server-tls']!={'uid':sources['voice-space-media-server-tls']['uid'],
                         'resourceVersion':sources['voice-space-media-server-tls']['resource_version']}): raise ValueError()
        redis=self._redis()
        try:issuer_after=authority._space(self.cut,self.space_phase,self.token)
        except BaseException:raise metadata.GuardLost('Space issuer authority unavailable') from None
        if {name:self._get_secret(name) for name in sorted(material.SECRETS)}!=objects or issuer_after!=(proof,jwks): raise ValueError()
        self._source(); metadata._fence(self.cut._guard); self._guard()
        return {'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit'],
                'namespace_uid':self.namespace_uid,'material':result,'redis':redis,
                'space_issuer_scope':proof['scope'],'runtime_qualified':False,
                'scope':'creator-source-material-and-redis-only'}

    def run(self):
        pending=False
        try:
            self._guard(); metadata._fence(self.cut._guard); self._source(); row,rev=self._ledger()
            if STEP in self.journal.state['steps'] or LEDGER in row: raise ValueError()
            marker={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],
                    'source_sha':self.fence.context['commit']}
            pending=True; self.journal.state['steps'][STEP]=copy.deepcopy(marker); self.journal.save(); self._guard()
            current,actual=self._ledger()
            if current!=row or actual!=rev: raise ValueError()
            current[LEDGER]=copy.deepcopy(marker); current['step']=STEP+'-pending'; self.fence.ledger.write(actual,current)
            self.record=copy.deepcopy(marker); anchor,anchor_rev=self._ledger()
            if anchor.get(LEDGER)!=marker: raise ValueError()
            receipt=self._observe(); self._guard(); current,actual=self._ledger()
            if current!=anchor or actual!=anchor_rev or self.journal.state['steps'].get(STEP)!=marker: raise ValueError()
            done=marker|{'status':'done','receipt_sha256':writer_cut.digest(receipt)}
            self.journal.state['steps'][STEP]=copy.deepcopy(done)|{'receipt':copy.deepcopy(receipt)}; self.journal.save()
            current[LEDGER]=copy.deepcopy(done); current['step']=STEP+'-done'; self.fence.ledger.write(actual,current)
            self.record=copy.deepcopy(done); self._guard()
            if self._ledger()[0].get(LEDGER)!=done: raise ValueError()
            return copy.deepcopy(receipt)
        except BaseException as error:
            if pending:
                self.poisoned=True
                try:self.fence.uncertain('Creator material observation uncertain')
                except BaseException:pass
            if isinstance(error,metadata.GuardLost): raise
            raise ProfileError('Creator material observation unavailable') from None


def verify_completed(phase,token):
    try:
        if type(phase) is not CreatorProfilePhase or token!=phase.token: raise ValueError()
        phase._guard(); row,rev=phase._ledger(); local=copy.deepcopy(phase.journal.state['steps'][STEP]); receipt=local['receipt']
        marker={k:v for k,v in local.items() if k!='receipt'}
        if marker!=phase.record or marker.get('status')!='done' or marker.get('receipt_sha256')!=writer_cut.digest(receipt) or row.get(LEDGER)!=marker: raise ValueError()
        if phase._observe()!=receipt: raise ValueError()
        phase._guard(); after,end=phase._ledger()
        if after!=row or end!=rev or phase.journal.state['steps'].get(STEP)!=local: raise ValueError()
        return copy.deepcopy(receipt)
    except metadata.GuardLost: raise
    except BaseException: raise ProfileError('Creator material receipt unavailable') from None
