"""Closed Creator environment resolver; private values stay in caller memory."""
import base64
import copy
import json
import re
import secrets
import time

import full_cohort_schema_metadata as metadata
import realtime_p3_create_only_transport as private
import space_creator_runtime_profile_auth as auth
import writer_cut


FIELDS={
 'space':('USER_GRPC_ADDR','SOCIAL_GRPC_ADDR','ROLE_PRINCIPAL_GRPC_ADDR','ROLE_PRINCIPAL_TLS_CA_FILE',
          'ROLE_PRINCIPAL_TLS_SERVER_NAME','SPACE_ROLE_CLIENT_CERT_FILE','SPACE_ROLE_CLIENT_KEY_FILE',
          'SPACE_CREATOR_AUTH_GRPC_ADDR','SPACE_CREATOR_AUTH_TLS_CA_FILE','SPACE_CREATOR_AUTH_TLS_SERVER_NAME',
          'SPACE_CREATOR_AUTH_CLIENT_CERT_FILE','SPACE_CREATOR_AUTH_CLIENT_KEY_FILE','SPACE_GATEWAY_LIFECYCLE_GRPC_LISTEN',
          'SPACE_GATEWAY_LIFECYCLE_TLS_CERT_FILE','SPACE_GATEWAY_LIFECYCLE_TLS_KEY_FILE',
          'SPACE_GATEWAY_LIFECYCLE_CLIENT_CA_FILE','SPACE_GATEWAY_PRINCIPAL_JWKS_URL',
          'SPACE_GATEWAY_PRINCIPAL_JWKS_CA_FILE','SPACE_GATEWAY_PRINCIPAL_REDIS_ADDR',
          'SPACE_GATEWAY_PRINCIPAL_REDIS_PASSWORD','SPACE_LIFECYCLE_ENABLED'),
 'gateway':('GATEWAY_AUTH_MODE','GATEWAY_JWKS_URL','GATEWAY_JWT_ISSUER','GATEWAY_JWT_AUDIENCE',
            'GATEWAY_SESSION_EPOCH_STRICT','GATEWAY_REDIS_ADDR','GATEWAY_REDIS_PASSWORD',
            'GATEWAY_SPACE_LIFECYCLE_GRPC_ADDR','GATEWAY_SPACE_LIFECYCLE_TLS_CA_FILE',
            'GATEWAY_SPACE_LIFECYCLE_TLS_SERVER_NAME','GATEWAY_SPACE_LIFECYCLE_CLIENT_CERT_FILE',
            'GATEWAY_SPACE_LIFECYCLE_CLIENT_KEY_FILE','GATEWAY_PRINCIPAL_SIGNING_KEYS_DIR','GATEWAY_PRINCIPAL_ACTIVE_KID'),
 'auth':('AUTH_SESSION_FLOOR_GRPC_PORT','AUTH_GRPC_TLS_CERT_FILE','AUTH_GRPC_TLS_KEY_FILE',
         'AUTH_SESSION_FLOOR_TLS_CLIENT_CA_FILE','S2S_JWKS_URLS_JSON'),
 'role':('ROLE_PRINCIPAL_GRPC_LISTEN','ROLE_PRINCIPAL_TLS_CERT_FILE','ROLE_PRINCIPAL_TLS_KEY_FILE',
         'ROLE_PRINCIPAL_CLIENT_CA_FILE','ROLE_PRINCIPAL_REPLAY_REDIS_ADDR','ROLE_PRINCIPAL_REPLAY_REDIS_PASSWORD',
         'S2S_JWKS_URLS_JSON'),
 'user':(), 'social':(),
}
EXPECTED={'USER_GRPC_ADDR':'voice-user:9090','SOCIAL_GRPC_ADDR':'voice-social:9090',
          'GATEWAY_JWT_ISSUER':'voice-auth','GATEWAY_JWT_AUDIENCE':'voice-client',
          'GATEWAY_JWKS_URL':'http://voice-auth:8080/api/v1/auth/.well-known/jwks.json',
          'AUTH_SESSION_FLOOR_GRPC_PORT':'9092','AUTH_GRPC_TLS_CERT_FILE':'/etc/voice/media/server/tls.crt',
          'AUTH_GRPC_TLS_KEY_FILE':'/etc/voice/media/server/tls.key',
          'AUTH_SESSION_FLOOR_TLS_CLIENT_CA_FILE':'/etc/voice/media/ca/ca.crt','ROLE_PRINCIPAL_GRPC_LISTEN':':9091'}
KEYS=frozenset(name for names in FIELDS.values() for name in names)|{'REDIS_ADDR','active-kid'}


def resolve(snapshot,variable,read_key):
    """Resolve Kubernetes precedence privately; reader is fixed by phase implementation."""
    env=snapshot.get('env',[]); sources=snapshot.get('envFrom',[])
    if not isinstance(env,list) or not isinstance(sources,list): raise ValueError()
    selected=[entry for entry in env if isinstance(entry,dict) and entry.get('name')==variable]
    if len(selected)>1: raise ValueError()
    references=[]
    def ref(kind,spec,key):
        allowed={'configmap':{'voice-app-config'},'secret':{'voice-app-secrets','voice-space-principal-signing','voice-gateway-principal-signing'}}
        if not isinstance(spec,dict) or spec.get('name') not in allowed[kind] or key not in KEYS: raise ValueError()
        answer=read_key(kind,spec['name'],key)
        if answer is None:
            if spec.get('optional') is True: return None
            raise ValueError()
        if (set(answer)!={'uid','resource_version','value'} or not answer['uid'] or not answer['resource_version'] or
                not (answer['value'] is None or isinstance(answer['value'],str))): raise ValueError()
        references.append({'kind':kind,'name':spec['name'],'key':key,'uid':answer['uid'],'resourceVersion':answer['resource_version']})
        return answer['value']
    if selected:
        entry=selected[0]
        if set(entry)=={'name','value'} and isinstance(entry['value'],str) and '$(' not in entry['value']:
            return entry['value'],[{'kind':'literal'}]
        source=entry.get('valueFrom',{})
        if set(entry)!={'name','valueFrom'} or not isinstance(source,dict) or len(source)!=1: raise ValueError()
        kind,spec=next(iter(source.items())); mapping={'configMapKeyRef':'configmap','secretKeyRef':'secret'}
        if kind not in mapping: raise ValueError()
        value=ref(mapping[kind],spec,spec.get('key'))
        if value is None and spec.get('optional') is not True: raise ValueError()
        return value,references
    value=None
    for source in sources:
        if not isinstance(source,dict) or source.get('prefix','')!='': raise ValueError()
        source={k:v for k,v in source.items() if k!='prefix'}
        if len(source)!=1: raise ValueError()
        kind,spec=next(iter(source.items())); mapping={'configMapRef':'configmap','secretRef':'secret'}
        if kind not in mapping: raise ValueError()
        found=ref(mapping[kind],spec,variable)
        if found is not None: value=found
    return value,references


def category(service,name,value):
    if value is None: return 'absent'
    if name.endswith('PASSWORD'): return 'empty' if not value else 'nonempty'
    if name=='GATEWAY_AUTH_MODE': return 'static' if value.lower()=='static' else 'nonstatic'
    if name=='GATEWAY_SESSION_EPOCH_STRICT': return {'true':'enabled','false':'disabled'}.get(value,'invalid')
    if name=='SPACE_LIFECYCLE_ENABLED':
        return ('enabled' if value.strip() in ('1','t','T','true','TRUE','True') else
                'disabled' if value.strip() in ('','0','f','F','false','FALSE','False') else 'invalid')
    if name=='S2S_JWKS_URLS_JSON':
        parsed=json.loads(value)
        if not isinstance(parsed,dict): raise ValueError()
        return 'expected' if parsed.get('space')=='https://voice-space:8443/.well-known/jwks.json' else 'different'
    if name in EXPECTED: return 'expected' if value==EXPECTED[name] else 'different'
    return 'present'


STEP='space-creator-effective-config'
LEDGER='space_creator_effective_config'


class ConfigError(RuntimeError): pass


class CreatorConfigPhase(auth.AuthTrustPhase):
    """Current pre-drain PID1 and Kubernetes source observation, not readiness."""
    def __init__(self,*args):
        super().__init__(*args); self._deadline=None

    def _guard(self):
        if self._deadline is not None and time.monotonic()>self._deadline: raise metadata.GuardLost('Creator observation deadline exceeded')
        return super()._guard()

    def _key(self,kind,name,key):
        allowed={'configmap':{'voice-app-config'},'secret':{'voice-app-secrets','voice-space-principal-signing','voice-gateway-principal-signing'}}
        if kind not in allowed or name not in allowed[kind] or key not in KEYS or not re.fullmatch(r'[A-Za-z_][A-Za-z0-9_-]*',key): raise ValueError()
        template=('{{.metadata.uid}}{{"\\n"}}{{.metadata.resourceVersion}}{{"\\n"}}{{.metadata.deletionTimestamp}}{{"\\n"}}'
                  '{{if eq (index .data "'+key+'") nil}}M{{else}}P{{index .data "'+key+'"}}{{end}}')
        self._guard(); raw=private._run(['get',kind,name,'--ignore-not-found=true','-o','go-template='+template]); self._guard()
        if not raw: return None
        parts=raw.split(b'\n',3)
        if len(parts)!=4 or not parts[0] or not parts[1] or parts[2] not in (b'',b'<no value>'): raise ValueError()
        data=parts[3]
        if data==b'M': value=None
        elif data.startswith(b'P'):
            payload=base64.b64decode(data[1:],validate=True) if kind=='secret' else data[1:]
            if len(payload)>65536: raise ValueError()
            value=payload.decode('utf-8')
        else: raise ValueError()
        return {'uid':parts[0].decode('ascii'),'resource_version':parts[1].decode('ascii'),'value':value}

    def _runtime(self,service,pod,variable):
        if service not in FIELDS or variable not in FIELDS[service]: raise ValueError()
        executable='/gateway' if service=='gateway' else '/opt/java/openjdk/bin/java' if service=='auth' else '/service'
        shell='bash' if service=='auth' else 'sh'
        script='[ "$(readlink /proc/1/exe)" = "'+executable+'" ] || exit 1; '+metadata.ENV_SCRIPT
        self._guard(); raw=private._run(['exec',pod,'-c',metadata.CONTAINERS[service],'--',shell,'-ec',script,'--',variable]); self._guard()
        if raw==b'M': return None
        if not raw.startswith(b'P') or len(raw)>65536: raise ValueError()
        return raw[1:].decode('utf-8')

    def _observe(self,token):
        observed=self._inventory(token); self._package(token); self._ledger(); workloads={}
        for service in FIELDS:
            records=[]
            for pod in observed['pods'][service]:
                snap=metadata.kube_workload_snapshot(service,pod,metadata.linux_private_bytes,self._guard)
                before=snap(); matches=[p for p in observed['projection'][service]['pods'] if p.get('name')==pod and p.get('uid')==before['pod_uid']]
                apps=[c for c in matches[0]['containers'] if c.get('name')==metadata.CONTAINERS[service]] if len(matches)==1 else []
                if before['namespace_uid']!=self.namespace_uid or len(apps)!=1 or apps[0].get('container_id')!=before['container_id'] or apps[0].get('restart_count')!=before['restart_count'] or not apps[0].get('image_id'): raise ValueError()
                if service=='auth':
                    reader=metadata.kube_runtime_reader('auth',pod,metadata.linux_private_bytes); reader.auth_policy(); reader('JAVA_TOOL_OPTIONS')
                facts={}; checked=[]
                for name in FIELDS[service]:
                    value,refs=resolve(before,name,self._key); runtime=self._runtime(service,pod,name)
                    if value!=runtime: raise ValueError()
                    facts[name]={'state':category(service,name,value),'references':refs}
                    checked.append((name,value,refs))
                if snap()!=before: raise ValueError()
                for name,value,refs in checked:
                    again,again_refs=resolve(before,name,self._key)
                    if again!=value or again_refs!=refs: raise ValueError()
                if snap()!=before: raise ValueError()
                records.append({'pod_uid':before['pod_uid'],'pod_rv':before['pod_rv'],'container_id':before['container_id'],
                                'restart_count':before['restart_count'],'image_id':apps[0]['image_id'],'fields':facts})
            if not records: raise ValueError()
            workloads[service]=records
        if self._inventory(token)!=observed: raise ValueError()
        return {'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit'],
                'namespace_uid':self.namespace_uid,'workloads':workloads,'runtime_qualified':False,
                'limits':['auth_spring_property_precedence_unverified','peer_runtime_unverified'],
                'scope':'current-selected-pid1-env-and-sources-only'}

    def run(self,token):
        pending=False; self._deadline=time.monotonic()+180
        try:
            self._guard(); self._inventory(token); row,rev=self._ledger()
            if STEP in self.journal.state['steps'] or LEDGER in row: raise ValueError()
            marker={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit']}
            pending=True; self.journal.state['steps'][STEP]=copy.deepcopy(marker); self.journal.save(); self._guard()
            current,version=self._ledger()
            if current!=row or version!=rev: raise ValueError()
            current[LEDGER]=copy.deepcopy(marker); current['step']=STEP+'-pending'; self.fence.ledger.write(version,current)
            self.record=copy.deepcopy(marker); anchor,version=self._ledger(); receipt=self._observe(token); self._guard(); current,end=self._ledger()
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
                try:self.fence.uncertain('Creator config observation uncertain')
                except BaseException:pass
            if isinstance(error,metadata.GuardLost): raise
            raise ConfigError('Creator config observation unavailable') from None
        finally:self._deadline=None


def verify_completed(phase,token):
    try:
        if type(phase) is not CreatorConfigPhase: raise ValueError()
        phase._deadline=time.monotonic()+180
        phase._guard(); row,rev=phase._ledger(); local=copy.deepcopy(phase.journal.state['steps'][STEP]); receipt=local['receipt']
        marker={k:v for k,v in local.items() if k!='receipt'}
        if marker!=phase.record or marker.get('status')!='done' or marker.get('receipt_sha256')!=writer_cut.digest(receipt) or row.get(LEDGER)!=marker: raise ValueError()
        if phase._observe(token)!=receipt: raise ValueError()
        phase._guard(); after,end=phase._ledger()
        if after!=row or end!=rev or phase.journal.state['steps'].get(STEP)!=local: raise ValueError()
        phase._local()
        return copy.deepcopy(receipt)
    except metadata.GuardLost: raise
    except BaseException: raise ConfigError('Creator config receipt unavailable') from None
    finally:
        if type(phase) is CreatorConfigPhase: phase._deadline=None


def verify_receipt(phase,token):
    """Authenticate the pre-drain observation without asserting current Pods."""
    try:
        if type(phase) is not CreatorConfigPhase: raise ValueError()
        phase._guard(); row,rev=phase._ledger(); local=copy.deepcopy(phase.journal.state['steps'][STEP]); receipt=local['receipt']
        marker={k:v for k,v in local.items() if k!='receipt'}
        if (marker!=phase.record or marker.get('status')!='done' or marker.get('receipt_sha256')!=writer_cut.digest(receipt) or
                row.get(LEDGER)!=marker or receipt.get('operation_id')!=phase.fence.context['operation_id'] or
                receipt.get('source_sha')!=phase.fence.context['commit'] or receipt.get('namespace_uid')!=phase.namespace_uid): raise ValueError()
        phase._package(token); phase._guard(); after,end=phase._ledger()
        if after!=row or end!=rev: raise ValueError()
        phase._local()
        if phase.record!=marker or phase.journal.state['steps'].get(STEP)!=local: raise metadata.GuardLost('Creator config historical record changed')
        return copy.deepcopy(receipt)
    except metadata.GuardLost: raise
    except BaseException: raise ConfigError('Creator config historical receipt unavailable') from None
