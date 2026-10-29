"""Pre-drain Auth trust observation; no issuer or HTTPS readiness claim."""
import base64
import copy
import hashlib
import json
from pathlib import PurePosixPath
import secrets
import shlex

import frontend_runtime_executor as transport
import full_cohort_inventory_phase as inventory
import full_cohort_package_phase as package
import full_cohort_schema_metadata as metadata
import media_operation_guard
import media_operation_journal
import realtime_p3_create_only_transport as private
import resource_fencing
import space_creator_runtime_profile_material as material
import space_creator_runtime_profile_trust as trust
import space_native_tool_identity as native
import writer_cut

STEP='space-creator-auth-trust'
LEDGER='space_creator_auth_trust'
TRUST='/etc/voice/media/trust/truststore.p12'
JDK='/opt/java/openjdk/lib/security/cacerts'


class AuthTrustError(RuntimeError): pass


def _options(value):
    options=shlex.split(value or '')
    expected=['-Djavax.net.ssl.trustStore='+TRUST,'-Djavax.net.ssl.trustStorePassword=changeit',
              '-Djavax.net.ssl.trustStoreType=PKCS12','-XX:MaxRAMPercentage=75.0','-XX:InitialRAMPercentage=25.0']
    if sorted(options)!=sorted(expected): raise ValueError()


def _mount(pod):
    containers=[x for x in pod['spec']['containers'] if x.get('name')=='auth']
    if len(containers)!=1: raise ValueError()
    c=containers[0]
    mounts=[x for x in c.get('volumeMounts',[]) if x.get('mountPath')=='/etc/voice/media/trust']
    if len(mounts)!=1 or mounts[0]!={'name':'media-trust','mountPath':'/etc/voice/media/trust','readOnly':True}: raise ValueError()
    volumes=[x for x in pod['spec'].get('volumes',[]) if x.get('name')=='media-trust']
    wanted={'secretName':'voice-media-jvm-trust','defaultMode':288,
            'items':[{'key':'truststore.p12','path':'truststore.p12'}]}
    if len(volumes)!=1 or volumes[0].get('secret')!=wanted: raise ValueError()
    for mount in c.get('volumeMounts',[]):
        path=mount.get('mountPath','')
        if not isinstance(path,str) or not path.startswith('/') or path.startswith('//') or str(PurePosixPath(path))!=path or '..' in path.split('/'): raise ValueError()
        if mount is not mounts[0] and (path=='/' or path.startswith('/etc/voice/media/trust/') or
                '/etc/voice/media/trust'.startswith(path.rstrip('/')+'/')): raise ValueError()
        if any(target==path or target.startswith(path.rstrip('/')+'/') or path.startswith(target+'/')
               for target in ('/opt/java/openjdk','/usr','/bin','/lib','/lib64')): raise ValueError()


class AuthTrustPhase:
    def __init__(self,journal,fence,client,lock,namespace_uid,package_phase,inventory_phase):
        if (type(journal) is not media_operation_journal.Journal or type(fence) is not resource_fencing.Fence or
            type(client) is not transport.KubectlTransport or client.journal is not journal or fence.ledger.client is not client or
            type(package_phase) is not package.PackagePhase or type(inventory_phase) is not inventory.InventoryPhase or
            inventory_phase.package is not package_phase or any(x.fence.context!=fence.context or x.lock!=lock or
            x.namespace_uid!=namespace_uid for x in (package_phase,inventory_phase)) or
            any(type(x.journal) is not media_operation_journal.Journal or x.client.journal is not x.journal or
                x.fence.ledger.client is not x.client for x in (package_phase,inventory_phase)) or
            journal.intent.get('resource_context')!=fence.context or journal.intent.get('namespace_uid')!=namespace_uid):
            raise AuthTrustError('Auth trust identity unavailable')
        self.journal,self.fence,self.client,self.package,self.inventory=journal,fence,client,package_phase,inventory_phase
        self.lock,self.namespace_uid=copy.deepcopy(lock),namespace_uid
        self.intent=copy.deepcopy(journal.intent); self.record=None; self.poisoned=False
        self._references=(journal,fence,client,fence.ledger,package_phase,package_phase.journal,package_phase.fence,
                          package_phase.client,package_phase.fence.ledger,inventory_phase,inventory_phase.journal,
                          inventory_phase.fence,inventory_phase.client,inventory_phase.fence.ledger)
        self._custodies=tuple(native.SpaceNativeToolPhase._custody(j) for j in (journal,package_phase.journal,inventory_phase.journal))

    def _local(self):
        try:
            refs=(self.journal,self.fence,self.client,self.fence.ledger,self.package,self.package.journal,self.package.fence,
                  self.package.client,self.package.fence.ledger,self.inventory,self.inventory.journal,self.inventory.fence,
                  self.inventory.client,self.inventory.fence.ledger)
            if (self.poisoned or self.journal.poisoned or self.journal.intent!=self.intent or self.journal.state.get('intent')!=self.intent or
                any(a is not b for a,b in zip(refs,self._references)) or self.client.journal is not self.journal or
                self.fence.ledger.client is not self.client or self.package.client.journal is not self.package.journal or
                self.package.fence.ledger.client is not self.package.client or self.inventory.client.journal is not self.inventory.journal or
                self.inventory.fence.ledger.client is not self.inventory.client or
                tuple(native.SpaceNativeToolPhase._custody(j) for j in (self.journal,self.package.journal,self.inventory.journal))!=self._custodies): raise ValueError()
        except BaseException: raise metadata.GuardLost('Auth trust identity changed') from None
    def _guard(self):
        self._local()
        metadata._fence(lambda:media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid))
        self._local()

    def _ledger(self):
        try:
            row,rev=self.fence.ledger.read()
            if (not self.fence.owned(row) or row.get('status')!='active' or not isinstance(rev,tuple) or len(rev)!=2 or
                any(not isinstance(x,str) or not x for x in rev) or self.inventory.fence.ledger.read()!=(row,rev) or
                self.package.fence.ledger.read()!=(row,rev)): raise ValueError()
            return row,rev
        except BaseException: raise metadata.GuardLost('Auth trust ledger changed') from None

    def _inventory(self,token):
        try:return self.inventory.verify_current(token)
        except BaseException:raise metadata.GuardLost('Auth inventory unavailable') from None

    def _package(self,token):
        try:return self.package.verify_current(token)
        except BaseException:raise metadata.GuardLost('Auth package unavailable') from None

    def _secret(self,name,key):
        if (name,key) not in (('voice-media-jvm-trust','truststore.p12'),('voice-media-ca','ca.crt')): raise ValueError()
        template='{{.metadata.uid}}{{"\\n"}}{{.metadata.resourceVersion}}{{"\\n"}}{{.metadata.deletionTimestamp}}{{"\\n"}}{{index .data "'+key+'"}}'
        self._guard(); raw=private._run(['get','secret',name,'-o','go-template='+template]); self._guard()
        parts=raw.split(b'\n',3)
        if len(parts)!=4 or not parts[0] or not parts[1] or parts[2] not in (b'',b'<no value>'): raise ValueError()
        value=base64.b64decode(parts[3],validate=True)
        if not value or len(value)>1048576: raise ValueError()
        return {'uid':parts[0].decode('ascii'),'resourceVersion':parts[1].decode('ascii')},value

    def _exec(self,pod,*args):
        self._guard(); raw=private._run(['exec',pod,'-c','auth','--',*args]); self._guard(); return raw

    def _observe(self,token):
        observed=self._inventory(token); self._package(token); self._ledger()
        trust_id,store=self._secret('voice-media-jvm-trust','truststore.p12')
        ca_id,ca=self._secret('voice-media-ca','ca.crt'); media_sha=hashlib.sha256(material._cert_der(ca)).hexdigest()
        if b'CA:TRUE' not in material._openssl(['x509','-in','{cert}','-noout','-ext','basicConstraints'],cert=ca): raise ValueError()
        material._openssl(['x509','-in','{cert}','-noout','-checkend','86400'],cert=ca)
        result=[]
        for pod_name in observed['pods']['auth']:
            snapshot=metadata.kube_workload_snapshot('auth',pod_name,metadata.linux_private_bytes,self._guard)
            before=snapshot(); self._guard(); pod=self.client.call('get','pod',pod_name,'-o','json'); self._guard()
            if before['namespace_uid']!=self.namespace_uid or pod['metadata'].get('uid')!=before['pod_uid'] or pod['metadata'].get('resourceVersion')!=before['pod_rv']:
                raise ValueError()
            matches=[p for p in observed['projection']['auth']['pods'] if p.get('name')==pod_name and p.get('uid')==before['pod_uid']]
            app=[c for c in matches[0]['containers'] if c.get('name')=='auth'] if len(matches)==1 else []
            if len(app)!=1 or app[0].get('container_id')!=before['container_id'] or app[0].get('restart_count')!=before['restart_count'] or not app[0].get('image_id'): raise ValueError()
            _mount(pod)
            reader=metadata.kube_runtime_reader('auth',pod_name,metadata.linux_private_bytes); reader.auth_policy(); _options(reader('JAVA_TOOL_OPTIONS'))
            for variable in ('_JAVA_OPTIONS','JDK_JAVA_OPTIONS'):
                raw=self._exec(pod_name,'bash','-ec','[ "$(readlink /proc/1/exe)" = /opt/java/openjdk/bin/java ] || exit 1; '+metadata.ENV_SCRIPT,'--',variable)
                if raw not in (b'M',b'P'): raise ValueError()
            expected=hashlib.sha256(store).hexdigest().encode('ascii')
            actual=self._exec(pod_name,'sha256sum',TRUST).split()
            if len(actual)!=2 or actual[0]!=expected or actual[1]!=TRUST.encode(): raise ValueError()
            def keytool(path):
                return self._exec(pod_name,'/opt/java/openjdk/bin/keytool','-list','-v','-keystore',path,'-storepass','changeit',
                                  '-J-Duser.language=en','-J-Duser.country=US')
            jdk,selected=keytool(JDK),keytool(TRUST); facts=trust.compare(jdk,selected,media_sha)
            roots=sorted(trust.parse(jdk)); facts['jdk_roots_sha256']=writer_cut.digest(roots)
            if snapshot()!=before: raise ValueError()
            result.append({'pod_uid':before['pod_uid'],'pod_rv':before['pod_rv'],'container_id':before['container_id'],
                           'restart_count':before['restart_count'],'image_id':app[0]['image_id'],'trust':facts})
        if not result or self._secret('voice-media-jvm-trust','truststore.p12')!=(trust_id,store) or self._secret('voice-media-ca','ca.crt')!=(ca_id,ca): raise ValueError()
        if self._inventory(token)!=observed: raise ValueError()
        return {'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit'],
                'namespace_uid':self.namespace_uid,'trust_secret':trust_id,'media_ca':ca_id,'pods':result,
                'scope':'auth-current-public-truststore-only-no-https-or-issuer-custody'}

    def run(self,token):
        pending=False
        try:
            self._guard(); self._inventory(token); row,rev=self._ledger()
            if STEP in self.journal.state['steps'] or LEDGER in row: raise ValueError()
            marker={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit']}
            pending=True; self.journal.state['steps'][STEP]=copy.deepcopy(marker); self.journal.save(); self._guard()
            current,version=self._ledger()
            if current!=row or version!=rev: raise ValueError()
            current[LEDGER]=copy.deepcopy(marker); current['step']=STEP+'-pending'; self.fence.ledger.write(version,current)
            self.record=copy.deepcopy(marker); anchor,version=self._ledger()
            receipt=self._observe(token); self._guard(); current,end=self._ledger()
            if current!=anchor or end!=version or self.journal.state['steps'].get(STEP)!=marker: raise ValueError()
            done=marker|{'status':'done','receipt_sha256':writer_cut.digest(receipt)}
            self.journal.state['steps'][STEP]=copy.deepcopy(done)|{'receipt':copy.deepcopy(receipt)}; self.journal.save()
            current[LEDGER]=copy.deepcopy(done); current['step']=STEP+'-done'; self.fence.ledger.write(end,current)
            self.record=copy.deepcopy(done); self._guard()
            if self._ledger()[0].get(LEDGER)!=done: raise ValueError()
            self._local()
            if self.record!=done or self.journal.state['steps'].get(STEP)!=done|{'receipt':receipt}: raise ValueError()
            return copy.deepcopy(receipt)
        except BaseException as error:
            if pending:
                self.poisoned=True
                try:self.fence.uncertain('Auth trust observation uncertain')
                except BaseException:pass
            if isinstance(error,metadata.GuardLost): raise
            raise AuthTrustError('Auth trust observation unavailable') from None


def verify_completed(phase,token):
    try:
        if type(phase) is not AuthTrustPhase: raise ValueError()
        phase._guard(); row,rev=phase._ledger(); local=copy.deepcopy(phase.journal.state['steps'][STEP]); receipt=local['receipt']
        marker={k:v for k,v in local.items() if k!='receipt'}
        if marker!=phase.record or marker.get('status')!='done' or marker.get('receipt_sha256')!=writer_cut.digest(receipt) or row.get(LEDGER)!=marker: raise ValueError()
        if phase._observe(token)!=receipt: raise ValueError()
        phase._guard(); after,end=phase._ledger()
        if after!=row or end!=rev or phase.journal.state['steps'].get(STEP)!=local: raise ValueError()
        phase._local()
        return copy.deepcopy(receipt)
    except metadata.GuardLost: raise
    except BaseException: raise AuthTrustError('Auth trust receipt unavailable') from None
