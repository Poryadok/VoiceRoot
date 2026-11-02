"""Source-derived pre-drain Auth/Gateway targets; no effects or readiness."""
import copy
import json
from pathlib import Path
import re
import secrets
import shlex
import urllib.parse

import auth_live_migration_phase_eligibility as auth
import realtime_p3_adoption as env
import space_creator_runtime_profile as creator
import full_cohort_inventory_phase as inventory
import full_cohort_schema_metadata as metadata
import publish_candidate
import space_creator_runtime_profile_auth as shared
import space_creator_runtime_profile_config as observed_config
import space_creator_runtime_profile_jdk as jdk
import dev_peer_issuer_preflight as preflight
import writer_cut

GATEWAY_JWKS=creator.GATEWAY_JWKS
SPACE_JWKS='https://voice-space:8443/.well-known/jwks.json'
TRUST='/etc/voice/media/trust/truststore.p12'
TRUST_OPTIONS=['-Djavax.net.ssl.trustStore='+TRUST,'-Djavax.net.ssl.trustStorePassword=changeit',
               '-Djavax.net.ssl.trustStoreType=PKCS12']
PRIVATE_ENV={'SPRING_DATASOURCE_PASSWORD','AUTH_TOTP_ENCRYPTION_KEY','ACCOUNT_DELETE_TOKEN_SECRET','AUTH_RESEND_API_KEY',
             'SPRING_DATA_REDIS_PASSWORD','GATEWAY_REDIS_PASSWORD','GATEWAY_DATABASE_URL','GATEWAY_STATIC_TOKENS_JSON'}


def _unique(pairs):
    out={}
    for key,value in pairs:
        if key in out: raise ValueError()
        out[key]=value
    return out


def _no_literal_credentials(container):
    for item in container.get('env',[]):
        value=item.get('value'); name=item.get('name')
        if name=='GATEWAY_STATIC_TOKENS_JSON' and value=='{}': continue
        if name in PRIVATE_ENV and value not in (None,''): raise ValueError()


def _entry(container,name,value,config_keys,*,replace=False):
    entries=container.setdefault('env',[])
    if not isinstance(entries,list) or any(not isinstance(x,dict) or not isinstance(x.get('name'),str) for x in entries): raise ValueError()
    matches=[x for x in entries if x['name']==name]
    if len(matches)>1: raise ValueError()
    desired={'name':name,'value':value}
    if matches:
        if matches[0]==desired: return
        if not replace or set(matches[0])!={'name','value'}: raise ValueError()
        entries[entries.index(matches[0])]=desired
    else:
        if name in config_keys and not replace: raise ValueError()
        entries.append(desired)


def _check_auth_overrides(effective):
    fixed={'AUTH_SESSION_FLOOR_GRPC_PORT','AUTH_GRPC_TLS_CERT_FILE','AUTH_GRPC_TLS_KEY_FILE',
           'AUTH_SESSION_FLOOR_TLS_CLIENT_CA_FILE','S2S_JWKS_URLS_JSON','S2S_JWKS_REFRESH_AFTER','S2S_JWKS_HARD_EXPIRY'}
    canonical={re.sub(r'[^a-z0-9]','',name.lower()):name for name in fixed}
    for name in effective:
        normalized=re.sub(r'[^a-z0-9]','',name.lower())
        if normalized in ('s2ssigningkeypem','s2ssigningkid'): raise ValueError()
        redis_allowed={'springdataredishost':'SPRING_DATA_REDIS_HOST','springdataredisport':'SPRING_DATA_REDIS_PORT',
                       'springdataredispassword':'SPRING_DATA_REDIS_PASSWORD'}
        if normalized.startswith(('springdataredis','springredis')) and redis_allowed.get(normalized)!=name: raise ValueError()
        if (normalized.startswith(('authsessionfloor','authgrpctls','s2sjwks'))
                and (normalized not in canonical or name!=canonical[normalized])): raise ValueError()


def render_auth(source,image,sources,image_env,config_keys):
    """Construct a private target from a current source; caller authenticates inputs."""
    if (source.get('kind')!='Deployment' or source.get('apiVersion')!='apps/v1'
            or source.get('metadata',{}).get('name')!='voice-auth'
            or source.get('metadata',{}).get('namespace')!='voice-dev'): raise ValueError()
    spec=copy.deepcopy(source['spec']); spec['replicas']=0
    target=auth.candidate_target(spec,image); pod=target['template']['spec']
    container=next(x for x in pod['containers'] if x.get('name')=='auth')
    _no_literal_credentials(container)
    effective=auth.resolve_environment(container,sources,image_env); auth.check_environment(effective); route=auth.check_database_route(effective)
    if any(effective.get(key)!=image_env.get(key) for key in ('PATH','JAVA_HOME')): raise ValueError()
    _check_auth_overrides(effective)
    def can_replace(name):
        existing=[x for x in container.get('env',[]) if x.get('name')==name]
        return not effective.get(name) or (len(existing)==1 and set(existing[0])=={'name','value'})
    options=shlex.split(effective.get('JAVA_TOOL_OPTIONS',''))
    for option in options:
        if option.startswith('-Djavax.net.ssl.') and option not in TRUST_OPTIONS: raise ValueError()
    if len(options)!=len(set(options)): raise ValueError()
    if not all(option in options for option in TRUST_OPTIONS):
        if not can_replace('JAVA_TOOL_OPTIONS'): raise ValueError()
        options=[option for option in options if option not in TRUST_OPTIONS]+TRUST_OPTIONS
        _entry(container,'JAVA_TOOL_OPTIONS',' '.join(options),config_keys,replace=True)
    raw=effective.get('S2S_JWKS_URLS_JSON','')
    jwks=json.loads(raw,object_pairs_hook=_unique) if raw else {}
    if not isinstance(jwks,dict) or any(not isinstance(k,str) or not isinstance(v,str) for k,v in jwks.items()): raise ValueError()
    for key,value in jwks.items():
        parsed=urllib.parse.urlsplit(value)
        if (not re.fullmatch(r'[A-Za-z0-9][A-Za-z0-9._-]{0,127}',key) or parsed.scheme!='https'
                or not parsed.hostname or parsed.username is not None or parsed.fragment or parsed.query): raise ValueError()
    complete=all(jwks.get(key)==value for key,value in (('gateway',GATEWAY_JWKS),('space',SPACE_JWKS)))
    for key,value in (('gateway',GATEWAY_JWKS),('space',SPACE_JWKS)):
        if key in jwks and jwks[key]!=value: raise ValueError()
        jwks[key]=value
    if not complete:
        if not can_replace('S2S_JWKS_URLS_JSON'): raise ValueError()
        _entry(container,'S2S_JWKS_URLS_JSON',json.dumps(jwks,sort_keys=True,separators=(',',':')),config_keys,replace=True)
    for name,value in (('AUTH_SESSION_FLOOR_GRPC_PORT','9092'),('AUTH_GRPC_TLS_CERT_FILE','/etc/voice/media/server/tls.crt'),
                       ('AUTH_GRPC_TLS_KEY_FILE','/etc/voice/media/server/tls.key'),
                       ('AUTH_SESSION_FLOOR_TLS_CLIENT_CA_FILE','/etc/voice/media/ca/ca.crt'),
                       ('SPRING_DATA_REDIS_HOST','voice-redis'),('SPRING_DATA_REDIS_PORT','6379')):
        if name in effective and effective[name]!=value: raise ValueError()
        if effective.get(name)==value: continue
        _entry(container,name,value,config_keys)
    creator._mount(pod,container,'media-server','/etc/voice/media/server','voice-auth-media-server-tls',('tls.crt','tls.key'))
    creator._mount(pod,container,'media-ca','/etc/voice/media/ca','voice-media-ca',('ca.crt',))
    creator._mount(pod,container,'media-trust','/etc/voice/media/trust','voice-media-jvm-trust',('truststore.p12',))
    desired_port={'name':'session-floor','containerPort':9092,'protocol':'TCP'}
    found=[]
    for candidate in pod['containers']+pod.get('initContainers',[]):
        for port in candidate.get('ports',[]):
            if port.get('name')=='session-floor' or port.get('containerPort')==9092:
                if candidate is not container or port!=desired_port: raise ValueError()
                found.append(port)
    if len(found)>1: raise ValueError()
    if not found: container.setdefault('ports',[]).append(desired_port)
    security=pod.setdefault('securityContext',{})
    if 'fsGroup' in security and security['fsGroup']!=65532: raise ValueError()
    security['fsGroup']=65532
    return target,route


def render_gateway(source,image,config_keys):
    if not isinstance(image,str) or not re.fullmatch(r'ghcr\.io/poryadok/voiceroot/gateway@sha256:[0-9a-f]{64}',image): raise ValueError()
    obj=copy.deepcopy(source); obj['spec']['replicas']=0
    pod=obj['spec']['template']['spec']; containers=[x for x in pod['containers'] if x.get('name')=='gateway']
    if len(containers)!=1 or pod.get('initContainers'): raise ValueError()
    container=containers[0]
    _no_literal_credentials(container)
    if container.get('command') not in (None,[]) or container.get('args') not in (None,[]) or container.get('workingDir') not in (None,'/') or container.get('volumeDevices'): raise ValueError()
    for mount in container.get('volumeMounts',[]): creator._check_path([mount],'/gateway')
    container['image']=image
    _,target=creator.render_gateway(obj,config_keys)
    return target


def _check_gateway_environment(effective):
    if effective.get('GATEWAY_AUTH_MODE','').lower()=='static': raise ValueError()
    for key in ('GATEWAY_JWKS_URL','GATEWAY_JWT_ISSUER','GATEWAY_JWT_AUDIENCE'):
        if effective.get(key)!=observed_config.EXPECTED[key]: raise ValueError()
    if effective.get('GATEWAY_REDIS_ADDR')!='voice-redis:6379': raise ValueError()
    raw=effective.get('GATEWAY_STATIC_TOKENS_JSON','')
    if raw and json.loads(raw,object_pairs_hook=_unique)!={}: raise ValueError()
    if any(key.startswith('LD_') for key in effective): raise ValueError()


def _gateway_image(registry,image,platform):
    repository='poryadok/voiceroot/gateway'; index=image.rsplit('@',1)[1]
    manifest=publish_candidate.decode_manifest(registry.get(repository,'manifests/'+index),index)
    if publish_candidate.platform_manifest(manifest)!=platform: raise ValueError()
    matches=[x for x in manifest['manifests'] if x.get('digest')==platform and x.get('platform',{}).get('os')=='linux'
             and x.get('platform',{}).get('architecture')=='amd64' and not x.get('platform',{}).get('variant')]
    if len(matches)!=1 or type(matches[0].get('size')) is not int: raise ValueError()
    raw=registry.get(repository,'manifests/'+platform)
    if len(raw)!=matches[0]['size']: raise ValueError()
    child=publish_candidate.decode_manifest(raw,platform)
    if child.get('schemaVersion')!=2: raise ValueError()
    desc=child['config']; digest=desc['digest']
    if not re.fullmatch(r'sha256:[a-f0-9]{64}',digest): raise ValueError()
    data=registry.get(repository,'blobs/'+digest)
    if type(desc.get('size')) is not int or len(data)!=desc['size']: raise ValueError()
    config=publish_candidate.decode_manifest(data,digest); runtime=config['config']
    if (config.get('os')!='linux' or config.get('architecture')!='amd64' or runtime.get('Entrypoint')!=['/gateway']
            or runtime.get('Cmd') not in (None,[]) or runtime.get('WorkingDir') not in (None,'/')
            or runtime.get('Volumes') not in (None,{})): raise ValueError()
    result={}
    for item in runtime.get('Env',[]):
        if not isinstance(item,str) or '=' not in item: raise ValueError()
        key,value=item.split('=',1)
        if not key or key in result: raise ValueError()
        result[key]=value
    return {'env':result,'config_digest':digest}


STEP='space-creator-target-plan'
LEDGER='space_creator_target_plan'


class TargetPlanError(RuntimeError): pass


class TargetPlanPhase(shared.AuthTrustPhase):
    """Durable private target plan rooted in actual current typed dependencies."""
    def __init__(self,journal,fence,client,lock,namespace_uid,package,inventory_phase,config_phase,jdk_phase,preflight_phase,trust_phase,source_root):
        super().__init__(journal,fence,client,lock,namespace_uid,package,inventory_phase)
        import space_creator_runtime_profile_trust_effect as trust
        if (type(config_phase) is not observed_config.CreatorConfigPhase or type(jdk_phase) is not jdk.TargetJdkPhase
                or type(preflight_phase) is not preflight.PeerIssuerPreflightPhase or preflight_phase.profile!='creator'
                or type(trust_phase) is not trust.TrustEffectPhase
                or any(x.fence.context!=fence.context or x.lock!=lock or x.namespace_uid!=namespace_uid
                       for x in (config_phase,jdk_phase,preflight_phase,trust_phase))
                or any(x.package is not package for x in (config_phase,jdk_phase,preflight_phase,trust_phase))
                or config_phase.inventory is not inventory_phase or jdk_phase.inventory is not inventory_phase
                or trust_phase.inventory is not inventory_phase or trust_phase.jdk is not jdk_phase
                or trust_phase.preflight is not preflight_phase or not Path(source_root).is_absolute()
                or journal.intent.get('creator_target_plan')!={'source_root':str(source_root),'source_sha':fence.context['commit']}): raise TargetPlanError('target plan identity unavailable')
        self.config,self.jdk,self.preflight,self.trust,self.source_root=config_phase,jdk_phase,preflight_phase,trust_phase,source_root
        self._source_root=source_root
        self._deps=(config_phase,jdk_phase,preflight_phase,trust_phase)
        self._dep_refs=tuple((x.journal,x.fence,x.client,x.fence.ledger) for x in self._deps)
        self._dep_custody=tuple(shared.native.SpaceNativeToolPhase._custody(x.journal) for x in self._deps)

    def _local(self):
        super()._local()
        if (self.source_root!=self._source_root or getattr(self,'_deps',None)!=(self.config,self.jdk,self.preflight,self.trust)
                or any(any(a is not b for a,b in zip((x.journal,x.fence,x.client,x.fence.ledger),expected))
                       for x,expected in zip(self._deps,self._dep_refs))
                or any(x.client.journal is not x.journal or x.fence.ledger.client is not x.client for x in self._deps)
                or tuple(shared.native.SpaceNativeToolPhase._custody(x.journal) for x in self._deps)!=self._dep_custody): raise metadata.GuardLost('target dependency custody changed')

    def _evidence(self,token):
        import space_creator_runtime_profile_trust_effect as trust
        self._guard(); row,rev=self._ledger()
        package=self._package(token); observed=self._inventory(token); config=observed_config.verify_completed(self.config,token)
        roots=jdk.verify_completed(self.jdk,token); issuer=preflight.verify_current(self.preflight,token); prepared=trust.verify_prepared(self.trust,token)
        required={'voice-auth-media-server-tls':{'tls.crt','tls.key'},'voice-gateway-media-client-tls':{'tls.crt','tls.key'},
                  'voice-gateway-principal-signing':{'current.pem','next.pem','active-kid'}}
        available=issuer.get('creator_secrets',{})
        for name,keys in required.items():
            item=available.get(name)
            if item=={'state':'absent'}: continue
            if (not isinstance(item,dict) or set(item)!={'state','uid','resourceVersion','keys'} or item.get('state')!='present'
                    or any(not isinstance(item.get(k),str) or not item[k] for k in ('uid','resourceVersion'))
                    or item.get('keys')!=sorted(keys)): raise ValueError()
        if (prepared.get('media_ca')!=issuer.get('ca') or prepared.get('jdk_receipt_sha256')!=writer_cut.digest(roots)
                or prepared.get('issuer_receipt_sha256')!=writer_cut.digest(issuer)
                or roots.get('image')!=package['images']['voice-auth/auth']): raise ValueError()
        self._guard()
        if self._ledger()!=(row,rev) or any(x.fence.ledger.read()!=(row,rev) for x in self._deps): raise ValueError()
        return package,observed,config,roots,issuer,prepared

    def _capture(self,token,actor):
        package,observed,config,roots,issuer,prepared=self._evidence(token)
        source=publish_candidate.source_snapshot(Path(self.source_root),self.fence.context['commit'])
        self._guard(); _,controllers,projection=inventory.inventory(self.client); self._guard()
        if projection!=observed['projection']: raise ValueError()
        configmap=self.client.call('get','configmap','voice-app-config','-o','json'); self._guard()
        meta=configmap['metadata']; data=configmap.get('data',{})
        if (meta.get('name')!='voice-app-config' or meta.get('namespace')!='voice-dev' or meta.get('deletionTimestamp')
                or any(not isinstance(meta.get(k),str) or not meta[k] for k in ('uid','resourceVersion')) or not isinstance(data,dict)): raise ValueError()
        keys=sorted(data)
        registry=publish_candidate.Registry(actor,token)
        auth_image=package['images']['voice-auth/auth']; gateway_image=package['images']['voice-gateway/gateway']
        auth_info=auth.read_image_config(registry,auth_image.rsplit('@',1)[1],package['platform_digests']['voice-auth/auth'])
        gateway_info=_gateway_image(registry,gateway_image,package['platform_digests']['voice-gateway/gateway'])
        if auth_info['config_digest']!=roots['config_digest']: raise ValueError()
        targets={}; references={}; environments={}
        for service in ('auth','gateway'):
            obj=controllers['voice-'+service]; containers=[x for x in obj['spec']['template']['spec']['containers'] if x.get('name')==service]
            if len(containers)!=1: raise ValueError()
            values,refs=auth.read_references(self.client,containers[0],self._guard); references[service]=refs
            environments[service]=values
            if service=='auth': target,route=render_auth(obj,auth_image,values,auth_info['env'],keys)
            else:
                effective=auth.resolve_environment(containers[0],values,gateway_info['env'])
                _check_gateway_environment(effective)
                target=render_gateway(obj,gateway_image,keys)
            config_target=copy.deepcopy(target)
            next(x for x in config_target['template']['spec']['containers'] if x.get('name')==service)['image']=containers[0]['image']
            targets[service]={'source':copy.deepcopy(obj),'config_target':config_target,'image_target':target}
        self._guard(); other=self.client.call('get','configmap','voice-app-config','-o','json')
        if other!=configmap: raise ValueError()
        for service in ('auth','gateway'):
            container=next(x for x in controllers['voice-'+service]['spec']['template']['spec']['containers'] if x.get('name')==service)
            if auth.read_references(self.client,container,self._guard)!=(environments[service],references[service]): raise ValueError()
        if (publish_candidate.source_snapshot(Path(self.source_root),self.fence.context['commit'])!=source
                or self._evidence(token)!=(package,observed,config,roots,issuer,prepared)): raise ValueError()
        snapshot={'operation_id':self.fence.context['operation_id'],'source_sha':self.fence.context['commit'],
                  'namespace_uid':self.namespace_uid,'source_tree_sha256':source['git_tree_sha256'],
                  'package_sha256':writer_cut.digest(package),'inventory_sha256':writer_cut.digest(observed),
                  'config_sha256':writer_cut.digest(config),'jdk_sha256':writer_cut.digest(roots),
                  'preflight_sha256':writer_cut.digest(issuer),'trust_plan_sha256':writer_cut.digest(prepared),
                  'image_configs':{'auth':auth_info['config_digest'],'gateway':gateway_info['config_digest']},
                  'target_sha256':{name:{key:writer_cut.digest(value[key]) for key in ('config_target','image_target')}
                                   for name,value in targets.items()},
                  'references_sha256':writer_cut.digest(references),'auth_database':route,'scope':'private-predrain-target-plan-not-runtime-readiness'}
        return snapshot,targets

    def run(self,token,actor):
        pending=False
        try:
            self._guard(); self._evidence(token); row,rev=self._ledger()
            if STEP in self.journal.state['steps'] or LEDGER in row: raise ValueError()
            marker={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],
                    'source_sha':self.fence.context['commit']}
            pending=True; self.journal.state['steps'][STEP]=copy.deepcopy(marker); self.journal.save()
            current,version=self._ledger()
            if current!=row or version!=rev: raise ValueError()
            current[LEDGER]=copy.deepcopy(marker); current['step']=STEP+'-pending'; self.fence.ledger.write(version,current)
            self.record=copy.deepcopy(marker); anchor,version=self._ledger(); receipt,targets=self._capture(token,actor)
            self._guard(); current,end=self._ledger()
            if current!=anchor or end!=version or self.journal.state['steps'].get(STEP)!=marker: raise ValueError()
            done=marker|{'status':'done','receipt_sha256':writer_cut.digest(receipt),'targets_sha256':writer_cut.digest(targets)}
            local=done|{'receipt':copy.deepcopy(receipt),'private_targets':copy.deepcopy(targets)}
            self.journal.state['steps'][STEP]=copy.deepcopy(local); self.journal.save()
            current[LEDGER]=copy.deepcopy(done); current['step']=STEP+'-done'; self.fence.ledger.write(end,current); self.record=copy.deepcopy(done)
            self._guard(); final,_=self._ledger(); self._local()
            if final.get(LEDGER)!=done or self.journal.state['steps'].get(STEP)!=local: raise ValueError()
            return copy.deepcopy(receipt)
        except BaseException as error:
            if pending:
                self.poisoned=True
                try:self.fence.uncertain('Creator target planning uncertain')
                except BaseException:pass
            if isinstance(error,metadata.GuardLost): raise
            raise TargetPlanError('Creator target plan unavailable') from None


def _stored(phase):
    phase._guard(); row,rev=phase._ledger(); local=copy.deepcopy(phase.journal.state['steps'][STEP])
    marker={k:v for k,v in local.items() if k not in ('receipt','private_targets')}
    if (marker!=phase.record or marker.get('status')!='done' or row.get(LEDGER)!=marker
            or marker.get('receipt_sha256')!=writer_cut.digest(local.get('receipt'))
            or marker.get('targets_sha256')!=writer_cut.digest(local.get('private_targets'))): raise ValueError()
    return row,rev,local


def verify_current(phase,token,actor):
    """Return private exact targets while original inventory and refs are current."""
    try:
        if type(phase) is not TargetPlanPhase: raise ValueError()
        row,rev,local=_stored(phase); receipt,targets=phase._capture(token,actor)
        if receipt!=local['receipt'] or targets!=local['private_targets']: raise ValueError()
        phase._guard(); after,end=phase._ledger(); phase._local()
        if after!=row or end!=rev or phase.journal.state['steps'].get(STEP)!=local: raise ValueError()
        return copy.deepcopy({'receipt':receipt,'targets':targets})
    except metadata.GuardLost: raise
    except BaseException: raise TargetPlanError('Creator current target unavailable') from None


def verify_receipt(phase,token):
    """Reopen the protected pre-drain plan, without claiming current Pods."""
    try:
        if type(phase) is not TargetPlanPhase: raise ValueError()
        row,rev,local=_stored(phase); phase._package(token)
        historical=observed_config.verify_receipt(phase.config,token)
        if writer_cut.digest(historical)!=local['receipt']['config_sha256']: raise ValueError()
        phase._guard(); after,end=phase._ledger(); phase._local()
        if after!=row or end!=rev or phase.journal.state['steps'].get(STEP)!=local: raise ValueError()
        return copy.deepcopy({'receipt':local['receipt'],'targets':local['private_targets']})
    except metadata.GuardLost: raise
    except BaseException: raise TargetPlanError('Creator historical target unavailable') from None
