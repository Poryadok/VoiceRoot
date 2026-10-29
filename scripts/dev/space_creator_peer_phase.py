"""Fenced static peer targets; material and runtime readiness are separate."""
import copy
import hashlib
import re
import secrets

import canonical_application_package as canonical
import full_cohort_inventory_phase as inventory
import full_cohort_schema_metadata as metadata
import realtime_config_transition as p3
import realtime_p3_create_only_source as source
import space_creator_peer_role_target as role_target
import space_creator_peer_target as peer_target
import space_creator_runtime_profile as creator
import space_creator_runtime_profile_auth as auth
import space_creator_runtime_profile_config as config
import space_native_tool_identity as native
import writer_cut


STEP='space-creator-peer-plan'
LEDGER='space_creator_peer_plan'
REVIEWED_SOURCE='550fadb5a44aad58c158686cf4b559dbc364eefb'
SOURCE_PATHS=(
    'src/backend/space/main.go','src/backend/space/creator_runtime.go',
    'src/backend/space/internal/grpcsvc/member_authority.go',
    'src/backend/space/internal/grpcsvc/public_creator.go','src/backend/space/internal/grpcsvc/invite_permission.go',
    'src/backend/space/internal/grpcsvc/join.go','src/backend/space/internal/grpcsvc/privacy.go',
    'src/backend/space/internal/s2s/user_profiles.go','src/backend/space/internal/s2s/social_blocks.go',
    'src/backend/pkg/roleoverrideprincipal/runtime.go','src/backend/pkg/roleoverrideprincipal/principal.go',
    'src/backend/pkg/roleoverrideprincipal/interceptor.go',
    'src/backend/role/main.go','src/backend/role/member_authority_runtime.go',
    'src/backend/role/principal_issuer.go',
    'src/backend/role/internal/grpcsvc/member_admission.go','src/backend/role/internal/grpcsvc/ordinary_scope.go',
    'src/backend/role/internal/principalruntime/config.go','src/backend/user/main.go',
    'src/backend/user/internal/grpcsvc/user.go','src/backend/social/main.go',
    'src/backend/social/internal/grpcsvc/social_blocks.go')


class PeerPlanError(RuntimeError):pass


def _literal(container,name,data):
    entries=container.get('env',[]); sources=container.get('envFrom',[])
    if (not isinstance(entries,list) or not isinstance(sources,list) or any(
            not isinstance(item,dict) or not isinstance(item.get('name'),str) for item in entries) or
            len({item['name'] for item in entries})!=len(entries) or
            any(item!={'configMapRef':{'name':'voice-app-config'}} for item in sources)):
        raise ValueError('peer listener source unavailable')
    selected=[item for item in entries if item['name']==name]
    if selected:
        item=selected[0]
        if set(item)=={'name','value'} and isinstance(item['value'],str) and '$(' not in item['value']:
            return item['value']
        ref=item.get('valueFrom',{}).get('configMapKeyRef',{})
        if (set(item)=={'name','valueFrom'} and set(item['valueFrom'])=={'configMapKeyRef'} and
                set(ref)=={'name','key'} and ref['name']=='voice-app-config' and ref['key']==name and
                isinstance(data.get(name),str)):return data[name]
        raise ValueError('peer listener override unsupported')
    return data.get(name) if sources else None


def _container(deployment,service):
    pod=deployment['spec']['template']['spec']
    found=[item for item in pod.get('containers',[]) if item.get('name')==service]
    if len(found)!=1 or found[0].get('command') or found[0].get('args'):raise ValueError('peer container invalid')
    return found[0]


def _target(source_deployment,service,image,config_spec=None):
    if not re.fullmatch(r'ghcr\.io/poryadok/voiceroot/'+service+r'@sha256:[0-9a-f]{64}',image):
        raise ValueError('peer image unavailable')
    config_spec=copy.deepcopy(config_spec if config_spec is not None else source_deployment['spec'])
    config_spec['replicas']=0
    image_spec=copy.deepcopy(config_spec); container=_container({'spec':image_spec},service); container['image']=image
    return {'source_uid':source_deployment['metadata']['uid'],'source_generation':source_deployment['metadata']['generation'],
            'source_spec_sha256':writer_cut.digest(source_deployment['spec']),
            'config_target':config_spec,'image_target':image_spec}


class PeerTargetPhase(auth.AuthTrustPhase):
    """Capture only pre-drain static target intent from real typed predecessors."""
    def __init__(self,journal,fence,client,lock,namespace_uid,package,inventory_phase,config_phase):
        super().__init__(journal,fence,client,lock,namespace_uid,package,inventory_phase)
        if (type(config_phase) is not config.CreatorConfigPhase or config_phase.package is not package or
                config_phase.inventory is not inventory_phase or config_phase.fence.context!=fence.context or
                config_phase.lock!=lock or config_phase.namespace_uid!=namespace_uid or
                config_phase.journal in (journal,package.journal,inventory_phase.journal)):
            raise PeerPlanError('peer target authority unavailable')
        self.config=config_phase; self._config_refs=(config_phase,config_phase.journal,config_phase.fence,
            config_phase.client,config_phase.fence.ledger)
        self._config_custody=native.SpaceNativeToolPhase._custody(config_phase.journal)

    def _peer_local(self):
        self._local(); refs=(self.config,self.config.journal,self.config.fence,self.config.client,self.config.fence.ledger)
        if (any(a is not b for a,b in zip(refs,self._config_refs)) or self.config.client.journal is not self.config.journal or
                self.config.fence.ledger.client is not self.config.client or self.config.fence.context!=self.fence.context or
                self.config.lock!=self.lock or self.config.namespace_uid!=self.namespace_uid or
                native.SpaceNativeToolPhase._custody(self.config.journal)!=self._config_custody):raise ValueError()
        self.config._local()

    def _peer_guard(self):
        self._peer_local();self._guard();self._peer_local()

    def _sources(self):
        commit=self.fence.context['commit']; pins={}
        for path in SOURCE_PATHS:
            expected=source._blob(canonical.ROOT,REVIEWED_SOURCE,path)
            if source._blob(canonical.ROOT,commit,path)!=expected or source._disk(canonical.ROOT,path)!=expected:raise ValueError()
            pins[path]=hashlib.sha256(expected).hexdigest()
        return pins

    def _predecessor_local(self):
        return copy.deepcopy((self.package.journal.state['steps'],self.inventory.journal.state['steps'],
                              self.config.journal.state['steps'],self.config.record))

    def _observe(self,token):
        self._peer_guard(); own_local=copy.deepcopy((self.journal.state['steps'],self.record))
        package=self._package(token); package_local=copy.deepcopy(self.package.journal.state['steps'])
        current=self._inventory(token); inventory_local=copy.deepcopy(self.inventory.journal.state['steps'])
        observed=config.verify_completed(self.config,token)
        config_local=copy.deepcopy((self.config.journal.state['steps'],self.config.record))
        row,rev=self._ledger()
        if self.config.fence.ledger.read()!=(row,rev):raise ValueError()
        commit=self.fence.context['commit'];pins=self._sources()
        names,deployments,projection=inventory.inventory(self.client)
        if names!=current['pods'] or projection!=current['projection']:raise ValueError()
        cm=self.client.call('get','configmap','voice-app-config','-o','json'); cmmeta=cm['metadata']; data=cm.get('data',{})
        if (cm.get('apiVersion')!='v1' or cm.get('kind')!='ConfigMap' or cmmeta.get('name')!='voice-app-config' or
                cmmeta.get('namespace')!='voice-dev' or cmmeta.get('deletionTimestamp') or
                any(not isinstance(cmmeta.get(k),str) or not cmmeta[k] for k in ('uid','resourceVersion')) or
                not isinstance(data,dict) or any(not isinstance(k,str) or not isinstance(v,str) for k,v in data.items())):raise ValueError()
        for field in ('USER_GRPC_ADDR','SOCIAL_GRPC_ADDR'):
            if any(item['fields'][field]['state']!='expected' for item in observed['workloads']['space']):raise ValueError()
        databases={}
        for name in ('space','role','user','social'):
            entries=current['database_observations']['workloads'][name]
            if (not isinstance(entries,list) or len(entries)!=len(names[name]) or not entries or any(
                    entry.get('status')!='observed' or entry.get('result',{}).get('binding',{}).get('state')!='resolved' or
                    entry.get('result',{}).get('binding',{}).get('database')!=metadata.OWNERS[name] or
                    not isinstance(entry.get('result',{}).get('binding',{}).get('role'),str) or
                    not metadata.SAFE.fullmatch(entry['result']['binding']['role']) or
                    entry.get('result',{}).get('metadata',{}).get('database')!=metadata.OWNERS[name] or
                    type(entry.get('result',{}).get('metadata',{}).get('session',{}).get('database_oid')) is not int or
                    entry['result']['metadata']['session']['database_oid']<=0
                    for entry in entries)):raise ValueError('peer database route unavailable')
            databases[name]=copy.deepcopy(entries)
        for service,field in (('user','USER_GRPC_ADDR'),('social','SOCIAL_GRPC_LISTEN')):
            value=_literal(_container(deployments['voice-'+service],service),field,data)
            if value not in (None,'',':9090'):raise ValueError('peer listener target unsupported')
        keys=sorted(data)
        space=copy.deepcopy(deployments['voice-space']);space['spec']['replicas']=0
        _,spec=p3.render_space(space,keys);space['spec']=spec
        _,spec=creator.render_space(space,keys);space['spec']=spec
        _,space_spec=peer_target.render_space(space,keys)
        role=copy.deepcopy(deployments['voice-role']);role['spec']['replicas']=0
        _,role_spec=role_target.render_role(role,keys)
        specs={'space':space_spec,'role':role_spec}
        targets={name:_target(deployments['voice-'+name],name,package['images']['voice-'+name+'/'+name],specs.get(name))
                 for name in ('space','role','user','social')}
        services={}
        for name,port in (('role',9091),('user',9090),('social',9090),('auth',9092)):
            obj=self.client.call('get','service','voice-'+name,'-o','json')
            services[name]={'source':copy.deepcopy(obj),'proof':role_target.verify_service(obj,deployments['voice-'+name],port)}
        _,role_service_target=role_target.render_role_service(services['role']['source'],deployments['voice-role'])
        services['role']['target_spec']=role_service_target
        space_service=self.client.call('get','service','voice-space','-o','json')
        _,space_service_target=peer_target.render_service(space_service,deployments['voice-space']['spec']['template']['metadata']['labels'])
        self._peer_guard()
        if (inventory.inventory(self.client)!=(names,deployments,projection) or self._inventory(token)!=current or
                config.verify_completed(self.config,token)!=observed or self._package(token)!=package or
                self.client.call('get','configmap','voice-app-config','-o','json')!=cm or
                self.client.call('get','service','voice-space','-o','json')!=space_service):raise ValueError()
        for name,value in services.items():
            if self.client.call('get','service','voice-'+name,'-o','json')!=value['source']:raise ValueError()
        self._peer_guard();after,end=self._ledger()
        if after!=row or end!=rev or self.config.fence.ledger.read()!=(row,rev):raise ValueError()
        if self._sources()!=pins:raise ValueError('peer source changed')
        self._peer_local()
        if ((self.journal.state['steps'],self.record)!=own_local or
                self.package.journal.state['steps']!=package_local or self.inventory.journal.state['steps']!=inventory_local or
                (self.config.journal.state['steps'],self.config.record)!=config_local):raise ValueError()
        return {'operation_id':self.fence.context['operation_id'],'source_sha':commit,'namespace_uid':self.namespace_uid,
                'configmap':{'uid':cmmeta['uid'],'resourceVersion':cmmeta['resourceVersion']},
                'database_observations_sha256':writer_cut.digest(databases),
                'source_pins_sha256':writer_cut.digest(pins),
                'targets':targets,'services':services,'space_service':{'source':space_service,'target_spec':space_service_target},
                'runtime_qualified':False,'scope':'static-peer-targets-no-material-or-runtime-proof'}

    def run(self,token):
        pending=False
        try:
            self._peer_guard();row,rev=self._ledger()
            if STEP in self.journal.state['steps'] or LEDGER in row:raise ValueError()
            marker={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],
                    'source_sha':self.fence.context['commit']}
            pending=True;self.journal.state['steps'][STEP]=copy.deepcopy(marker);self.journal.save()
            current,version=self._ledger()
            if current!=row or version!=rev:raise ValueError()
            current[LEDGER]=copy.deepcopy(marker);current['step']=STEP+'-pending';self.fence.ledger.write(version,current)
            self.record=copy.deepcopy(marker)
            predecessor_local=self._predecessor_local()
            receipt=self._observe(token);self._peer_guard();current,version=self._ledger()
            if current.get(LEDGER)!=marker or self.journal.state['steps'].get(STEP)!=marker:raise ValueError()
            done=marker|{'status':'done','receipt_sha256':writer_cut.digest(receipt)}
            local=done|{'receipt':copy.deepcopy(receipt)};self.journal.state['steps'][STEP]=copy.deepcopy(local);self.journal.save()
            current[LEDGER]=copy.deepcopy(done);current['step']=STEP+'-done';self.fence.ledger.write(version,current);self.record=done
            self._peer_guard();row,_=self._ledger()
            if row.get(LEDGER)!=done or self.journal.state['steps'].get(STEP)!=local:raise ValueError()
            if writer_cut.digest(self._sources())!=receipt['source_pins_sha256']:raise ValueError()
            self._peer_local()
            if (self.record!=done or self.journal.state['steps'].get(STEP)!=local or
                    self._predecessor_local()!=predecessor_local):raise ValueError()
            return copy.deepcopy(receipt)
        except BaseException as error:
            if pending:
                self.poisoned=True
                try:self.fence.uncertain('peer target plan uncertain')
                except BaseException:pass
            if isinstance(error,metadata.GuardLost):raise
            raise PeerPlanError('peer target plan unavailable') from None


def verify_current(phase,token):
    try:
        if type(phase) is not PeerTargetPhase:raise ValueError()
        phase._peer_guard();row,rev=phase._ledger();local=copy.deepcopy(phase.journal.state['steps'][STEP]);receipt=local['receipt']
        predecessor_local=phase._predecessor_local()
        marker={k:v for k,v in local.items() if k!='receipt'}
        if marker!=phase.record or marker.get('status')!='done' or marker.get('receipt_sha256')!=writer_cut.digest(receipt) or row.get(LEDGER)!=marker:raise ValueError()
        if phase._observe(token)!=receipt:raise ValueError()
        phase._peer_guard();after,end=phase._ledger()
        if after!=row or end!=rev or phase.journal.state['steps'].get(STEP)!=local:raise ValueError()
        if writer_cut.digest(phase._sources())!=receipt['source_pins_sha256']:raise ValueError()
        phase._peer_local()
        if (phase.record!=marker or phase.journal.state['steps'].get(STEP)!=local or
                phase._predecessor_local()!=predecessor_local):raise ValueError()
        return copy.deepcopy(receipt)
    except metadata.GuardLost:raise
    except BaseException:raise PeerPlanError('peer target receipt unavailable') from None
