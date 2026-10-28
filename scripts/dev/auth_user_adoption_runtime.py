"""Concrete fixed-namespace backend for auth_user_adoption; not a generic runner."""
import base64
import copy
import datetime
import hashlib
import json
import pathlib
import re
import time
import urllib.parse
import urllib.request
from auth_user_adoption import (RecoveryRequired, LABELS, sql_phases, operation_secret,
                               deployment_plan, terminal_job, old_incarnations_drained, Journal)

QUALIFIED_TOOLS={
    'postgres':'postgres@sha256:44c4ee9810eff91f7eab4d822642e01115b1a9eccce4bcbdde7604752d68eac6',
    'flyway':'flyway/flyway@sha256:c8cf894f9b63e8b3a50bb7a89f65b84dbd147d3203d8bc502f7c570b7d440895',
    'migrate':'migrate/migrate@sha256:f21c436af23c282f4516b00ba3e93bccf5c5fe5cd52530fd5c319a936998f539'}


def digest(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True, separators=(',', ':')).encode()).hexdigest()


def validate_admission(manifest, evidence):
    if (not re.fullmatch(r'[0-9a-f]{40}',str(manifest.get('source_sha','')))
            or not re.fullmatch(r'[0-9a-f]{64}',str(manifest.get('operator_sha256','')))
            or set(manifest.get('products',{}))!={'auth','user'}
            or manifest.get('tools')!=QUALIFIED_TOOLS
            or manifest.get('recovery_mode') not in ('qualified_image_restore','reviewed_forward_only')):
        raise RecoveryRequired('candidate/source/recovery admission incomplete')
    for service,ref in manifest['products'].items():
        if not re.fullmatch(r'[a-z0-9][a-z0-9./:_-]*@sha256:[0-9a-f]{64}',ref):
            raise RecoveryRequired('immutable candidate refs required')
        ids = manifest.get('runtime_ids',{}).get(service)
        if not isinstance(ids,list) or not ids or any(not re.fullmatch(
                r'(?:docker-pullable://)?[a-z0-9][a-z0-9./:_-]*@sha256:[0-9a-f]{64}|(?:containerd://)?sha256:[0-9a-f]{64}',item) for item in ids):
            raise RecoveryRequired('qualified node/runtime identities missing')
    required={'image_provenance','runtime_roles','recovery','operator_review'}
    if set(evidence)!=required:
        raise RecoveryRequired('required qualification records missing')
    for kind,record in evidence.items():
        if (record.get('kind')!=kind or record.get('status')!='PASS'
                or record.get('source_sha')!=manifest['source_sha']
                or record.get('products')!=manifest['products']
                or record.get('operator_sha256')!=manifest['operator_sha256']
                or not record.get('reference')):
            raise RecoveryRequired('qualification is stale, partial or wrong scope')
    if (evidence['operator_review'].get('reviewer')!='Dimas-8'
            or evidence['recovery'].get('reviewer')!='Dimas-8'
            or evidence['recovery'].get('recovery_mode')!=manifest['recovery_mode']):
        raise RecoveryRequired('independent exact recovery/operator review absent')
    image=evidence['image_provenance']
    if (image.get('runtime_ids')!=manifest['runtime_ids'] or image.get('node_accessibility') is not True
            or image.get('artifact_scope') not in ('published_oci','qualified_node_import')):
        raise RecoveryRequired('actual node-accessible immutable candidate proof missing')
    if (manifest['recovery_mode']=='qualified_image_restore'
            and evidence['recovery'].get('old_artifact_scope')!='original_oci'):
        raise RecoveryRequired('recovered executable is not original OCI rollback qualification')


def validate_defaulted_job(authored,defaulted):
    defaults={'/spec':{'parallelism':1,'completions':1,'completionMode':'NonIndexed','suspend':False,
                      'podReplacementPolicy':'TerminatingOrFailed'},
              '/spec/template/metadata':{'creationTimestamp':None},
              '/spec/template/spec':{'dnsPolicy':'ClusterFirst','schedulerName':'default-scheduler',
                  'serviceAccountName':'default','serviceAccount':'default','securityContext':{},
                  'enableServiceLinks':True,'preemptionPolicy':'PreemptLowerPriority'},
              '/spec/template/spec/containers/*':{'imagePullPolicy':'IfNotPresent',
                  'terminationMessagePath':'/dev/termination-log','terminationMessagePolicy':'File'},
              '/spec/template/spec/volumes/*/configMap':{'defaultMode':420}}
    def compare(before,after,path):
        if isinstance(before,dict):
            if not isinstance(after,dict):
                raise RecoveryRequired('dry-run field type changed')
            for key,value in before.items():
                if key not in after:
                    raise RecoveryRequired('dry-run authored field missing')
                compare(value,after[key],path+'/'+key)
            for key in set(after)-set(before):
                if key not in defaults.get(path,{}) or after[key]!=defaults[path][key]:
                    raise RecoveryRequired('unrecognized server default or admission mutation')
        elif isinstance(before,list):
            if not isinstance(after,list) or len(before)!=len(after):
                raise RecoveryRequired('dry-run collection changed')
            for left,right in zip(before,after):
                compare(left,right,path+'/*')
        elif before!=after:
            raise RecoveryRequired('dry-run authored value changed')
    compare(authored['spec'],defaulted['spec'],'/spec')
    if defaulted.get('metadata',{}).get('annotations',{}).get('voice.dev/adoption-operation')!=authored['metadata']['annotations']['voice.dev/adoption-operation']:
        raise RecoveryRequired('dry-run operation identity changed')


def resource_card():
    base = 'http://192.168.0.109:30990/api/projects/voice/resources/DEV'
    with urllib.request.urlopen(base, timeout=10) as response:
        card = json.load(response)
    query = urllib.parse.urlencode({'actor':'Dimas-16','claim':card.get('claim',''),
                                   'revision':card['revision']})
    with urllib.request.urlopen(base + '/check?' + query, timeout=10) as response:
        json.load(response)
    if card.get('phase') in ('deploy','verify'):
        with urllib.request.urlopen('http://192.168.0.109:30990/api/projects/voice/tasks/A1-DEV-AUTH-USER-CREDENTIAL-ADOPTION',timeout=10) as response:
            task=json.load(response)
        allowed=('dev_deploying',) if card['phase']=='deploy' else ('dev_ready_for_test','dev_test','stg_ready_for_deploy')
        if (task.get('status') not in allowed or task.get('owner')!='Dimas-16' or task.get('assignee')!='Dimas-16'):
            raise RecoveryRequired('actual adoption task deploy gates not satisfied')
    return card


def _get(kube, kind, name=None):
    return kube.call('get', kind, *([name] if name else []), '--ignore-not-found=true', '-o', 'json')


def _referenced_bindings(value):
    refs = set()
    if isinstance(value, dict):
        for key, item in value.items():
            if key in ('secretKeyRef','secretRef'):
                refs.add(('secret',item['name']))
            elif key in ('configMapKeyRef','configMapRef'):
                refs.add(('configmap',item['name']))
            elif key == 'secret' and isinstance(item, dict) and 'secretName' in item:
                refs.add(('secret',item['secretName']))
            elif key == 'configMap' and isinstance(item, dict) and 'name' in item:
                refs.add(('configmap',item['name']))
            refs.update(_referenced_bindings(item))
    elif isinstance(value, list):
        for item in value:
            refs.update(_referenced_bindings(item))
    return refs


def capture(kube, binding_keys=None):
    namespace = _get(kube, 'namespace', 'voice-dev')
    meta = namespace.get('metadata', {})
    if (not meta.get('uid') or any(meta.get('labels', {}).get(k) != v for k,v in LABELS.items())):
        raise RecoveryRequired('DEV namespace marker invalid')
    workloads = _get(kube, 'deployments,statefulsets')['items']
    full = {w['metadata']['name']:w for w in workloads}
    if not {'voice-auth','voice-user','voice-space','voice-postgres'}.issubset(full):
        raise RecoveryRequired('required DEV workloads missing')
    pods = _get(kube, 'pods')['items']
    space = full['voice-space']
    selected = [p for p in pods if all(p['metadata'].get('labels', {}).get(k) == v
                for k,v in space['spec']['selector']['matchLabels'].items())]
    def ready(p):
        return (not p['metadata'].get('deletionTimestamp') and p['status'].get('phase') == 'Running'
                and any(c.get('type') == 'Ready' and c.get('status') == 'True' for c in p['status'].get('conditions', [])))
    if not selected or not all(ready(p) for p in selected):
        raise RecoveryRequired('actual configured Space peer not ready')
    peer = {'service': _get(kube, 'service', 'voice-space')['spec'],
            'service_uid': _get(kube, 'service', 'voice-space')['metadata']['uid'],
            'pods': [{'uid':p['metadata']['uid'],'ip':p['status'].get('podIP'),
                      'owners':p['metadata'].get('ownerReferences', []),
                      'restarts':{s['name']:s['restartCount'] for s in p['status'].get('containerStatuses', [])}}
                     for p in sorted(selected,key=lambda p:p['metadata']['name'])], 'ready':True,
            'endpoints': sorted([{'uid':e['metadata']['uid'],'ports':e.get('ports', []),'endpoints':e.get('endpoints', [])}
                for e in _get(kube, 'endpointslices')['items']
                if e['metadata'].get('labels', {}).get('kubernetes.io/service-name') == 'voice-space'], key=lambda e:e['uid'])}
    mail = _get(kube,'service','voice-dev-mail')
    peer['mail_service'] = {'uid':mail['metadata']['uid'],'spec':mail['spec']}
    refs = binding_keys
    if refs is None:
        refs = sorted(set.union(*[_referenced_bindings(full[n]['spec']['template']['spec'])
                                 for n in ('voice-auth','voice-user','voice-space','voice-postgres')]))
    bindings = {}
    for kind,name in refs:
        obj = _get(kube, kind, name)
        if not obj.get('metadata', {}).get('uid'):
            raise RecoveryRequired('original credential/config binding missing')
        bindings[kind + '/' + name] = {'uid':obj['metadata']['uid'], 'rv':obj['metadata']['resourceVersion'],
                                       'hash':digest({'data':obj.get('data', {}),'binaryData':obj.get('binaryData', {})})}
    storage = {}
    for pvc in _get(kube, 'persistentvolumeclaims')['items']:
        pv = _get(kube, 'persistentvolume', pvc['spec']['volumeName'])
        storage[pvc['metadata']['name']] = {'uid':pvc['metadata']['uid'],'spec':pvc['spec'],
            'pv_uid':pv['metadata']['uid'],'pv_spec':pv['spec']}
    snapshot = {'namespace':{'uid':meta['uid'],'labels':meta['labels']}, 'storage':storage,
        'bindings':bindings,'peer':peer,'workloads':{name:{'uid':w['metadata']['uid'],'spec':w['spec']}
            for name,w in sorted(full.items())},
        'ready':{name:w.get('status', {}).get('readyReplicas', 0) >= w['spec'].get('replicas', 1)
                 for name,w in full.items()}}
    return snapshot, full, pods


def database_state(kube):
    cluster = kube.query('postgres', """SELECT json_build_object(
      'major',current_setting('server_version_num')::int/10000,
      'admin',json_build_object('user',current_user,'superuser',(SELECT rolsuper FROM pg_roles WHERE rolname=current_user)),
      'owners',(SELECT json_object_agg(datname,pg_get_userbyid(datdba)) FROM pg_database WHERE datname IN ('auth_db','user_db')),
      'logins',(SELECT json_agg(rolname ORDER BY rolname) FROM pg_roles WHERE rolcanlogin),
      'roles',(SELECT coalesce(json_object_agg(r.rolname,json_build_object('oid',r.oid,'login',r.rolcanlogin,'superuser',r.rolsuper,'createdb',r.rolcreatedb,'createrole',r.rolcreaterole,'inherit',r.rolinherit,'replication',r.rolreplication,'bypassrls',r.rolbypassrls,'owned_objects',(SELECT count(*) FROM pg_shdepend d WHERE d.refobjid=r.oid AND d.deptype='o'))),'{}'::json) FROM pg_roles r WHERE rolname IN ('auth_runtime','user_runtime')),
      'memberships',(SELECT coalesce(json_agg(json_build_object('role',pg_get_userbyid(roleid),'member',pg_get_userbyid(member))),'[]'::json) FROM pg_auth_members WHERE roleid IN(SELECT oid FROM pg_roles WHERE rolname IN ('auth_runtime','user_runtime')) OR member IN(SELECT oid FROM pg_roles WHERE rolname IN ('auth_runtime','user_runtime'))),
      'sessions',(SELECT coalesce(json_agg(json_build_object('pid',pid,'backend_start',backend_start,'role',usename,'database',datname,'client_address',client_addr::text,'application',application_name,'state',state)),'[]'::json) FROM pg_stat_activity WHERE (datname IN ('auth_db','user_db') OR application_name LIKE 'voice-au-%' OR application_name='voice-adoption-backup') AND application_name <> 'voice-adoption-observe'));
    """)
    cluster['schemas'], cluster['objects'], cluster['acls'] = {}, {}, {}
    cluster['consumers'],cluster['unexpected_schemas'],cluster['event_triggers']={},{},{}
    for db in ('auth_db','user_db'):
        cat = kube.query(db, """SELECT json_build_object('schema_owner',(SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname='public'),
          'objects',(SELECT coalesce(json_object_agg(c.relname,pg_get_userbyid(c.relowner)),'{}'::json) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN('r','p','S','v','m')),
          'relation_acls',(SELECT coalesce(json_object_agg(c.relname,c.relacl::text),'{}'::json) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN('r','p','S')),
          'defaults',(SELECT coalesce(json_agg(json_build_object('owner',pg_get_userbyid(defaclrole),'namespace',defaclnamespace,'type',defaclobjtype,'acl',defaclacl::text) ORDER BY pg_get_userbyid(defaclrole),defaclnamespace,defaclobjtype),'[]'::json) FROM pg_default_acl),
          'unexpected_schemas',(SELECT coalesce(json_agg(nspname),'[]'::json) FROM pg_namespace WHERE nspname NOT IN('public','information_schema') AND nspname NOT LIKE 'pg_%'),
          'event_triggers',(SELECT coalesce(json_agg(evtname),'[]'::json) FROM pg_event_trigger),
          'consumers',(SELECT coalesce(json_agg(json_build_object('name',r.rolname,'schema_create',has_schema_privilege(r.oid,'public','CREATE'),'read_tables',(SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN('r','p','v','m') AND has_table_privilege(r.oid,c.oid,'SELECT')),'write_tables',(SELECT count(*) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN('r','p','v','m') AND has_table_privilege(r.oid,c.oid,'INSERT,UPDATE,DELETE')))),'[]'::json) FROM pg_roles r WHERE r.rolcanlogin AND r.rolname NOT IN('voice','auth_runtime','user_runtime')));""")
        cluster['schemas'][db] = cat['schema_owner']
        cluster['objects'].update({db+'/'+name:owner for name,owner in cat['objects'].items()})
        cluster['acls'][db] = {'relations':cat['relation_acls'],'defaults':cat['defaults']}
        cluster['consumers'][db]=cat['consumers']
        cluster['unexpected_schemas'][db]=cat['unexpected_schemas']
        cluster['event_triggers'][db]=cat['event_triggers']
    cluster['auth_history'] = kube.query('auth_db', "SELECT coalesce(json_agg(json_build_object('version',version,'script',script,'checksum',checksum,'success',success) ORDER BY installed_rank),'[]'::json) FROM flyway_schema_history")
    cluster['user_history'] = kube.query('user_db', "SELECT json_build_object('version',max(version),'dirty',bool_or(dirty),'rows',count(*)) FROM schema_migrations")
    cluster['auth_ddl_gate']=None
    if 'auth_identity_ddl_gate' in cluster['event_triggers']['auth_db']:
        root=pathlib.Path(__file__).resolve().parents[2]
        cluster['auth_ddl_gate']=kube.query('auth_db',(root/'src/backend/auth/src/main/resources/roleidentityauthority/gate-catalog.sql').read_text(encoding='utf-8'))
    return cluster


def backup_catalog_queries():
    common="""SELECT json_build_object('database_owner',(SELECT pg_get_userbyid(datdba) FROM pg_database WHERE datname=current_database()),
      'schema_owner',(SELECT pg_get_userbyid(nspowner) FROM pg_namespace WHERE nspname='public'),
      'schema_acl',(SELECT nspacl::text FROM pg_namespace WHERE nspname='public'),
      'objects',(SELECT coalesce(json_agg(json_build_object('name',c.relname,'kind',c.relkind,'owner',pg_get_userbyid(c.relowner),'acl',c.relacl::text) ORDER BY c.relname),'[]'::json) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='public' AND c.relkind IN('r','p','S','v','m')),
      'defaults',(SELECT coalesce(json_agg(json_build_object('owner',pg_get_userbyid(d.defaclrole),'schema',n.nspname,'type',d.defaclobjtype,'acl',d.defaclacl::text) ORDER BY pg_get_userbyid(d.defaclrole),n.nspname,d.defaclobjtype),'[]'::json) FROM pg_default_acl d LEFT JOIN pg_namespace n ON n.oid=d.defaclnamespace),
      'history', HISTORY);"""
    return {'auth_db':common.replace('HISTORY',"(SELECT coalesce(json_agg(json_build_object('version',version,'script',script,'checksum',checksum,'success',success) ORDER BY installed_rank),'[]'::json) FROM flyway_schema_history)"),
            'user_db':common.replace('HISTORY',"(SELECT json_agg(json_build_object('version',version,'dirty',dirty) ORDER BY version) FROM schema_migrations)")}


def backup_catalog(kube):
    return {db:kube.query(db,sql) for db,sql in backup_catalog_queries().items()}


def source_config(root, operation):
    operation_name(operation)
    root = pathlib.Path(root)
    data = {}
    for prefix, folder in [('auth', root/'src/backend/auth/src/main/resources/db/migration'),
                           ('user', root/'src/backend/migrations/user_db')]:
        for path in sorted(folder.glob('*.sql')):
            data[prefix + '-' + path.name] = path.read_text(encoding='utf-8')
    phases = sql_phases(root)
    data.update({name + '.sql': sql for name, sql in phases.items()})
    if not any(k.startswith('auth-V') for k in data) or not any(k.startswith('user-') for k in data):
        raise RecoveryRequired('migration sources missing')
    if len(json.dumps(data).encode()) > 900000:
        raise RecoveryRequired('migration source bundle exceeds bounded ConfigMap size')
    return {'apiVersion': 'v1', 'kind': 'ConfigMap', 'immutable': True,
            'metadata': metadata('voice-auth-user-sql-' + operation, operation), 'data': data}


def operation_name(operation):
    if not isinstance(operation, str) or not re.fullmatch(r'[a-z0-9][a-z0-9-]{0,30}', operation):
        raise RecoveryRequired('invalid operation identity')


def metadata(name, operation):
    return {'name': name, 'namespace': 'voice-dev', 'labels': dict(LABELS),
            'annotations': {'voice.dev/adoption-operation': operation}}


def validate_ddl_gate(gate,root=None):
    root=pathlib.Path(root) if root is not None else pathlib.Path(__file__).resolve().parents[2]
    sql=(root/'src/backend/auth/src/main/resources/db/migration/V31__auth_identity_ddl_barrier.sql').read_text(encoding='utf-8')
    body=re.search(r'LANGUAGE plpgsql SECURITY INVOKER AS \$\$(.*?)\$\$;',sql,re.S)
    tags=re.search(r'WHEN TAG IN \((.*?)\)\s+EXECUTE FUNCTION public\.auth_identity_ddl_gate\(\);',sql,re.S)
    if body is None or tags is None:
        raise RecoveryRequired('fixed V31 canonical gate source changed')
    expected_tags=re.findall(r"'([^']+)'",tags.group(1))
    if (not isinstance(gate,dict) or any(gate.get(k) is not True for k in
            ('relation_ok','acl_ok','function_ok','binding_ok','major_ok'))
            or any(gate.get(k)!='voice' for k in ('barrier_owner','function_owner','event_owner'))
            or gate.get('event_enabled')!='O' or gate.get('function_source')!=body.group(1)
            or sorted(gate.get('event_tags',[]))!=sorted(expected_tags)):
        raise RecoveryRequired('canonical V31 gate owner/function/binding/tags/ACL altered')


def database_guard(original, current, steps,root=None):
    if current.get('admin')!=original.get('admin') or current.get('major')!=original.get('major'):
        raise RecoveryRequired('migration admin authority or PG major changed')
    if any(current.get('unexpected_schemas',{}).get(db,[]) for db in ('auth_db','user_db')):
        raise RecoveryRequired('unrecognized schema')
    events=current.get('event_triggers',{})
    if events.get('user_db',[]):
        raise RecoveryRequired('unrecognized User event trigger')
    auth_events=events.get('auth_db',[])
    has_v31=any(row['version']=='31' for row in current['auth_history'])
    if auth_events or has_v31:
        if 'migrate_auth' not in steps or not has_v31 or auth_events!=['auth_identity_ddl_gate']:
            raise RecoveryRequired('unexpected Auth event trigger transition')
        validate_ddl_gate(current.get('auth_ddl_gate'),root)
    for consumers in current.get('consumers',{}).values():
        validate_consumers(consumers)
    if current['owners'] != original['owners'] or current['schemas'] != original['schemas']:
        raise RecoveryRequired('database or schema owner changed')
    for name, owner in original['objects'].items():
        if current['objects'].get(name) != owner:
            raise RecoveryRequired('original relation missing or owner changed')
    for name, owner in current['objects'].items():
        if name not in original['objects']:
            migration = 'migrate_auth' if name.startswith('auth_db/') else 'migrate_user'
            if owner != 'voice' or migration not in steps:
                raise RecoveryRequired('unexpected new relation authority')
    roles_expected = 'roles' in steps
    expected_roles = {'auth_runtime', 'user_runtime'} if roles_expected else set()
    if set(current['roles']) not in (set(), expected_roles) or (not roles_expected and current['roles']):
        raise RecoveryRequired('foreign or partial runtime roles')
    if steps.get('roles', {}).get('status') == 'done' and set(current['roles']) != expected_roles:
        raise RecoveryRequired('completed runtime roles missing')
    for props in current['roles'].values():
        if (props.get('login') is not True or props.get('owned_objects') != 0
                or any(props.get(flag) is not False for flag in
                       ('superuser','createdb','createrole','inherit','replication','bypassrls'))):
            raise RecoveryRequired('unexpected runtime role authority')
    if current['memberships']:
        raise RecoveryRequired('runtime role memberships not admitted')
    if set(current['logins']) != set(original['logins']) | set(current['roles']):
        raise RecoveryRequired('unrecognized affected login')
    initial = original['auth_history']
    history = current['auth_history']
    if history[:len(initial)] != initial or any(not r['success'] for r in history):
        raise RecoveryRequired('Auth historical checksum or success changed')
    versions = [r['version'] for r in history[len(initial):]]
    allowed = [[], ['28','31','32']] if 'migrate_auth' in steps else [[]]
    if versions not in allowed or (steps.get('migrate_auth', {}).get('status') == 'done' and not versions):
        raise RecoveryRequired('Auth migration version unexpected')
    user = current['user_history']
    allowed_user = [original['user_history']['version']]
    if 'migrate_user' in steps:
        allowed_user.append(22)
    if user['dirty'] or user['version'] not in allowed_user:
        raise RecoveryRequired('User migration dirty or unexpected')
    if steps.get('migrate_user', {}).get('status') == 'done' and user['version'] != 22:
        raise RecoveryRequired('completed User migration missing')


def validate_consumers(consumers):
    if (len(consumers)!=1 or any(c.get('name')!='gameintegration_runtime'
            or c.get('read_tables')!=0 or c.get('write_tables')!=0
            or c.get('schema_create') is not False for c in consumers)):
        raise RecoveryRequired('unrecognized existing Auth/User grant consumer')


def _validate_projection(kube,planned,own_secret,forbidden_values,service):
    cache={('secret',own_secret['metadata']['name']):own_secret}
    forbidden_keys=({'POSTGRES_PASSWORD','USER_DB_PASSWORD','USER_DATABASE_URL'} if service=='Auth'
                    else {'POSTGRES_PASSWORD','AUTH_DB_PASSWORD','AUTH_DATABASE_URL'})
    forbidden_database=b'user_db' if service=='Auth' else b'auth_db'
    def values(kind,name):
        if (kind,name) not in cache:
            cache[kind,name]=_get(kube,kind,name)
        data=cache[kind,name].get('data',{})
        return {key:base64.b64decode(value) if kind=='secret' else value.encode()
                for key,value in data.items()}
    def check(key,value):
        if key in forbidden_keys or forbidden_database in value.lower() or any(secret and secret in value for secret in forbidden_values):
            raise RecoveryRequired(service+' credential exposure outside own database boundary')
    spec=planned['spec']['template']['spec']
    for container in spec['containers']:
        for entry in container.get('env',[]):
            if 'value' in entry:
                check(entry['name'],entry['value'].encode())
            for key,kind in [('secretKeyRef','secret'),('configMapKeyRef','configmap')]:
                ref=entry.get('valueFrom',{}).get(key)
                if ref:
                    data=values(kind,ref['name'])
                    if ref['key'] not in data and not ref.get('optional'):
                        raise RecoveryRequired('Auth projected environment key missing')
                    if ref['key'] in data:
                        check(ref['key'],data[ref['key']])
        for source in container.get('envFrom',[]):
            for key,kind in [('secretRef','secret'),('configMapRef','configmap')]:
                if key in source:
                    for name,value in values(kind,source[key]['name']).items():
                        check(name,value)
    for volume in spec.get('volumes',[]):
        refs=[]
        if 'secret' in volume:
            refs.append(volume['secret'])
        refs += [s['secret'] for s in volume.get('projected',{}).get('sources',[]) if 'secret' in s]
        for ref in refs:
            name=ref.get('secretName',ref.get('name'))
            data=values('secret',name)
            keys=[item['key'] for item in ref['items']] if 'items' in ref else list(data)
            if any(key not in data for key in keys) and not ref.get('optional'):
                raise RecoveryRequired('Auth projected Secret item missing')
            for key in keys:
                if key in data:
                    check(key,data[key])


def validate_auth_projection(kube,planned,own_secret,forbidden_values):
    return _validate_projection(kube,planned,own_secret,forbidden_values,'Auth')


def validate_user_projection(kube,planned,own_secret,forbidden_values):
    return _validate_projection(kube,planned,own_secret,forbidden_values,'User')


def job_plan(step, operation, images, secret, sources):
    operation_name(operation)
    if step not in ('migrate_auth', 'migrate_user', 'roles', 'grants_auth', 'grants_user'):
        raise RecoveryRequired('unsupported adoption Job')
    if set(images) != {'postgres', 'flyway', 'migrate'} or any(not re.fullmatch(
            r'[a-z0-9][a-z0-9./:_-]*@sha256:[0-9a-f]{64}', ref) for ref in images.values()):
        raise RecoveryRequired('immutable tool images required')
    if (sources.get('kind') != 'ConfigMap' or sources.get('immutable') is not True
            or sources.get('metadata', {}).get('annotations', {}).get('voice.dev/adoption-operation') != operation):
        raise RecoveryRequired('exact owned migration sources required')
    items = [{'key':key, 'path':('auth/' + key[5:] if key.startswith('auth-') else
                               'user/' + key[5:] if key.startswith('user-') else key)}
             for key in sorted(sources['data'])]
    name = 'voice-au-' + operation + '-' + step.replace('_', '-')
    ref = lambda key, source='voice-app-secrets': {'secretKeyRef': {'name': source, 'key': key}}
    env = [{'name': 'PGPASSWORD', 'valueFrom': ref('POSTGRES_PASSWORD')},
           {'name': 'POSTGRES_USER', 'value': 'voice'},
           {'name': 'PGAPPNAME', 'value': name},
           {'name': 'PGCONNECT_TIMEOUT', 'value': '5'},
           {'name': 'PGOPTIONS', 'value': '-c statement_timeout=60000 -c lock_timeout=5000 -c idle_in_transaction_session_timeout=60000'}]
    image = images['postgres']
    mounts = [{'name': 'sql', 'mountPath': '/sql', 'readOnly': True},
              {'name': 'tmp', 'mountPath': '/tmp'}]
    if step == 'migrate_auth':
        image = images['flyway']
        options = urllib.parse.urlencode({'connectTimeout':'5', 'socketTimeout':'70',
                   'ApplicationName':name, 'options':'-c statement_timeout=60000 -c lock_timeout=5000'})
        env += [{'name':'FLYWAY_USER','value':'voice'}, {'name':'FLYWAY_PASSWORD','valueFrom':ref('POSTGRES_PASSWORD')},
                {'name':'FLYWAY_URL','value':'jdbc:postgresql://voice-postgres:5432/auth_db?' + options}]
        script = 'exec /flyway/flyway -connectRetries=0 -validateMigrationNaming=true -baselineOnMigrate=false -outOfOrder=false -locations=filesystem:/sql/auth -target=32 migrate >/dev/null 2>&1'
    elif step == 'migrate_user':
        image = images['migrate']
        script = ('exec migrate -path=/sql/user -lock-timeout=10 -database '
                  '"postgres://${POSTGRES_USER}:${PGPASSWORD}@voice-postgres:5432/user_db?sslmode=disable&connect_timeout=5&x-statement-timeout=60000&application_name=' + name +
                  '&options=-c%20statement_timeout%3D60000%20-c%20lock_timeout%3D5000" goto 22 >/dev/null 2>&1')
    else:
        env += [{'name':'AUTH_DB_PASSWORD','valueFrom':ref('AUTH_DB_PASSWORD',secret)},
                {'name':'USER_DB_PASSWORD','valueFrom':ref('USER_DB_PASSWORD',secret)}]
        database = {'roles':'postgres', 'grants_auth':'auth_db', 'grants_user':'user_db'}[step]
        script = 'exec psql -X -qAt -v ON_ERROR_STOP=1 -h voice-postgres -U voice -d ' + database + ' -f /sql/' + step + '.sql >/dev/null 2>&1'
    # manualSelector avoids generated controller-UID selectors, so an exact
    # server-dry-run defaulted spec can be bound before atomic CREATE.
    labels = dict(LABELS, **{'voice.dev/adoption-job': name})
    return {'apiVersion':'batch/v1', 'kind':'Job', 'metadata':metadata(name,operation),
            'spec':{'manualSelector':True, 'selector':{'matchLabels':{'voice.dev/adoption-job':name}},
                    'backoffLimit':0, 'activeDeadlineSeconds':120,
                    'template':{'metadata':{'labels':labels}, 'spec':{
                        'restartPolicy':'Never', 'automountServiceAccountToken':False, 'terminationGracePeriodSeconds':5,
                        'containers':[{'name':'operation', 'image':image, 'command':['/bin/sh','-ec'], 'args':[script],
                            'env':env, 'volumeMounts':mounts,
                            'securityContext':{'allowPrivilegeEscalation':False, 'readOnlyRootFilesystem':True,
                                               'runAsNonRoot':True, 'runAsUser':1000, 'capabilities':{'drop':['ALL']}},
                            'resources':{'requests':{'cpu':'100m','memory':'128Mi'},'limits':{'cpu':'1','memory':'512Mi'}}}],
                        'volumes':[{'name':'sql','configMap':{'name':sources['metadata']['name'], 'items':items}},
                                   {'name':'tmp','emptyDir':{}}]}}}}


def guard_snapshot(original, current, expected, resource):
    for key in ('namespace', 'storage', 'bindings', 'peer'):
        if current.get(key) != original.get(key):
            raise RecoveryRequired('original namespace/storage/bindings/peer changed')
    if (resource.get('owner') != 'Dimas-16' or resource.get('state') != 'held'
            or resource.get('phase') not in expected.get('allowed_phases',('deploy',))
            or resource.get('tasks') != ['A1-DEV-AUTH-USER-CREDENTIAL-ADOPTION']):
        raise RecoveryRequired('DEV workflow authority changed')
    for key in ('claim', 'generation', 'window', 'candidate'):
        if resource.get(key) != expected['resource'].get(key):
            raise RecoveryRequired('DEV claim/window/candidate changed')
    if set(current['workloads']) != set(original['workloads']):
        raise RecoveryRequired('workload package changed')
    for name, before in original['workloads'].items():
        after = expected['workloads'].get(name, before) if name in ('voice-auth', 'voice-user') else before
        if current['workloads'][name] not in (before, after):
            raise RecoveryRequired('workload identity or full spec drift')


class Runtime:
    """Each external effect has a deterministic identity and read-only reconciler.

    There is no general shell hook, reset, password ALTER, rollback or force-unlock.
    Terminal success retains the common lock until separately guarded finish.
    """
    def __init__(self, journal, kube, root, resource=resource_card):
        self.journal, self.kube, self.root, self.resource = journal, kube, pathlib.Path(root), resource
        self.intent = journal.state['intent']
        self.operation = self.intent['operation']
        operation_name(self.operation)
        self.sources = source_config(root,self.operation)
        self.secret = operation_secret(self.operation,journal.state['credentials'])
        self.lock = {'apiVersion':'v1','kind':'ConfigMap',
            'metadata':metadata('voice-dev-image-update-lock',self.operation),
            'data':{'operation':self.operation,'actor':'Dimas-16',
                    'claim':self.intent['resource']['claim'], 'generation':str(self.intent['resource']['generation']),
                    'window':self.intent['resource']['window'],'intent_sha256':digest(self.intent)}}

    def guard(self, state):
        if 'manifest' in self.intent:
            verify_package(self.root,self.intent['manifest'])
        expiry = datetime.datetime.fromisoformat(self.intent['expires_at'])
        if expiry.tzinfo is None or datetime.datetime.now(datetime.timezone.utc) >= expiry:
            raise RecoveryRequired('operation execution window expired')
        if digest(self.sources) != self.intent['sources_sha256']:
            raise RecoveryRequired('source SQL bundle changed')
        current, full, pods = capture(self.kube, [tuple(k.split('/',1)) for k in self.intent['baseline']['bindings']])
        expected = {'resource':self.intent['resource'], 'workloads':copy.deepcopy(self.intent['baseline']['workloads'])}
        if getattr(self,'finishing',False) or state.get('finish',{}).get('status') in ('pending','done'):
            expected['allowed_phases']=('deploy','verify')
        for service in ('auth','user'):
            if 'roll_'+service in state['steps']:
                original = self.intent['original_deployments'][service]
                _, wanted = deployment_plan(original,service,self.intent['products'][service],self.secret['metadata']['name'])
                expected['workloads']['voice-'+service] = {'uid':original['metadata']['uid'],'spec':wanted['spec']}
        guard_snapshot(self.intent['baseline'],current,expected,self.resource())
        for name,ready in current['ready'].items():
            if name not in ('voice-auth','voice-user') and not ready:
                raise RecoveryRequired('unrelated workload is not ready')
            if name in ('voice-auth','voice-user') and not ready and 'roll_'+name.removeprefix('voice-') not in state['steps']:
                raise RecoveryRequired('original app became unready before its rollout')
        db = database_state(self.kube)
        database_guard(self.intent['database'],db,state['steps'],self.root)
        for step,database in [('grants_auth','auth_db'),('grants_user','user_db')]:
            saved=state['steps'].get(step)
            if saved and saved['status']=='done':
                if digest(db['acls'][database])!=saved['receipt'].get('catalog_sha256'):
                    raise RecoveryRequired('completed grant catalog/default privileges drifted')
        role_receipt=state['steps'].get('roles',{})
        if role_receipt.get('status')=='done':
            if {name:props['oid'] for name,props in db['roles'].items()}!=role_receipt['receipt']['role_oids']:
                raise RecoveryRequired('original created runtime role identities changed')
        app_ips = {p['status'].get('podIP') for p in pods if any(
            all(p['metadata'].get('labels', {}).get(k)==v for k,v in full[n]['spec']['selector']['matchLabels'].items())
            for n in ('voice-auth','voice-user'))}
        owned_apps = {'voice-au-'+self.operation+'-'+s.replace('_','-') for s in
                      ('migrate_auth','migrate_user','roles','grants_auth','grants_user') if s in state['steps']}
        if 'backup' in state['steps']:
            owned_apps.add('voice-adoption-backup')
        for session in db['sessions']:
            if session['application'] in owned_apps:
                continue
            if session['role'] != 'voice' and session['role'] not in ('auth_runtime','user_runtime'):
                raise RecoveryRequired('unrecognized connected Auth/User consumer')
            if (session.get('client_address') or '').split('/')[0] not in app_ips:
                raise RecoveryRequired('unattributed affected database session')
        if 'lock' in state['steps']:
            actual = _get(self.kube,'configmap','voice-dev-image-update-lock')
            saved = state['steps']['lock']
            finishing = state.get('finish', {}).get('status') in ('pending','done')
            if (not actual and saved['status']=='done' and not finishing):
                raise RecoveryRequired('completed common updater lock missing')
            if actual and (actual.get('data') != self.lock['data'] or
                    (saved['status']=='done' and actual['metadata']['uid']!=saved['receipt']['uid'])):
                raise RecoveryRequired('common updater lock changed')
        for step,body in [('sources',self.sources),('secret',self.secret)]:
            saved = state['steps'].get(step)
            if saved and saved['status']=='done':
                actual = self._object_receipt(body)
                if actual.get('status')!='complete' or actual['receipt']!=saved['receipt']:
                    raise RecoveryRequired('owned immutable source/Secret changed')
        for step in ('migrate_auth','migrate_user','roles','grants_auth','grants_user'):
            saved=state['steps'].get(step)
            if saved and saved['status']=='done':
                job=_get(self.kube,'job',state['plans'][step]['metadata']['name'])
                if (not job or job['metadata']['uid']!=saved['receipt']['uid']
                        or job['spec']!=state['plans'][step]['spec']):
                    raise RecoveryRequired('original terminal Job identity/spec changed')

    def _object_receipt(self, body):
        actual = _get(self.kube,body['kind'].lower(),body['metadata']['name'])
        if not actual:
            return {'status':'not_started'}
        if (actual['metadata'].get('annotations', {}).get('voice.dev/adoption-operation')!=self.operation
                or actual.get('data')!=body.get('data') or actual.get('immutable')!=body.get('immutable')
                or actual['metadata'].get('deletionTimestamp')):
            return {'status':'conflict'}
        return {'status':'complete','receipt':{'uid':actual['metadata']['uid'],
                                             'data_sha256':digest(actual.get('data', {}))}}

    def reconcile(self, step, state):
        if step=='lock':
            return self._object_receipt(self.lock)
        if step=='sources':
            return self._object_receipt(self.sources)
        if step=='secret':
            return self._object_receipt(self.secret)
        if step=='backup':
            from auth_user_adoption_backup import qualified_backup
            proof = qualified_backup(self.journal)
            sessions = database_state(self.kube)['sessions']
            if proof and not any(s['application']=='voice-adoption-backup' for s in sessions):
                return {'status':'complete','receipt':proof}
            if self.journal.read_private('backup-manifest.json',optional=True) is not None:
                return {'status':'running'}
            return {'status':'not_started'}
        if step in ('migrate_auth','migrate_user','roles','grants_auth','grants_user'):
            body = state['plans'][step]
            actual = _get(self.kube,'job',body['metadata']['name'])
            if not actual:
                return {'status':'not_started'}
            db = database_state(self.kube)
            sessions = [s for s in db['sessions'] if s['application']==body['metadata']['name']]
            result = terminal_job(actual,body['spec'],self.operation,sessions,db['user_history']['dirty'])
            if result['status']=='complete':
                if step=='migrate_auth' and [r['version'] for r in db['auth_history']][-3:]!=['28','31','32']:
                    return {'status':'conflict'}
                if step=='migrate_user' and db['user_history']['version']!=22:
                    return {'status':'conflict'}
                if step=='roles':
                    if set(db['roles'])!={'auth_runtime','user_runtime'}:
                        return {'status':'conflict'}
                    result['receipt']['role_oids'] = {r:p['oid'] for r,p in db['roles'].items()}
                if step.startswith('grants_'):
                    database = 'auth_db' if step=='grants_auth' else 'user_db'
                    result['receipt']['catalog_sha256'] = digest(db['acls'][database])
            return result
        if step in ('roll_auth','roll_user'):
            service = step.removeprefix('roll_')
            original = self.intent['original_deployments'][service]
            actual = _get(self.kube,'deployment','voice-'+service)
            _,wanted = deployment_plan(original,service,self.intent['products'][service],self.secret['metadata']['name'])
            if actual['metadata']['uid']!=original['metadata']['uid']:
                return {'status':'conflict'}
            if actual['spec']==original['spec']:
                return {'status':'not_started'}
            if actual['spec']!=wanted['spec']:
                return {'status':'conflict'}
            if not self._rollout_ready(service,actual):
                return {'status':'running'}
            return {'status':'complete','receipt':{'uid':actual['metadata']['uid'],
                'spec_sha256':digest(actual['spec']), 'image':self.intent['products'][service]}}
        if step=='verify':
            proof = self.journal.read_private('verification.json',optional=True)
            if proof is None:
                return {'status':'not_started'}
            self.verify_isolation()
            return {'status':'complete','receipt':{'verification_sha256':digest(proof)}}
        raise RecoveryRequired('unknown adoption step')

    def _rollout_ready(self,service,deployment):
        status = deployment.get('status', {})
        replicas = deployment['spec'].get('replicas',1)
        if (status.get('observedGeneration')!=deployment['metadata']['generation'] or
            any(status.get(k,0)!=replicas for k in ('replicas','updatedReplicas','readyReplicas','availableReplicas'))):
            return False
        selected = [p for p in _get(self.kube,'pods')['items'] if all(
            p['metadata'].get('labels', {}).get(k)==v for k,v in deployment['spec']['selector']['matchLabels'].items())]
        if len(selected)!=replicas:
            return False
        for pod in selected:
            if pod['metadata'].get('deletionTimestamp'):
                return False
            statuses = pod.get('status', {}).get('containerStatuses', [])
            main = [s for s in statuses if s['name']==service]
            if (not statuses or not all(s.get('ready') for s in statuses) or len(main)!=1
                    or main[0].get('imageID') not in self.intent['runtime_ids'][service]):
                return False
        return True

    def execute(self,step,state):
        if step in ('lock','sources','secret'):
            body = {'lock':self.lock,'sources':self.sources,'secret':self.secret}[step]
            self.kube.call('create','-f','-','-o','json',body=body)
        elif step=='backup':
            from auth_user_adoption_backup import export_backup
            catalog=backup_catalog(self.kube)
            if catalog!=self.intent['backup_catalog']:
                raise RecoveryRequired('original backup catalog changed before export')
            export_backup(self.journal,self.kube,catalog)
            if backup_catalog(self.kube)!=catalog:
                raise RecoveryRequired('backup schema/owner/ACL/history changed during export')
        elif step in ('migrate_auth','migrate_user','roles','grants_auth','grants_user'):
            self.kube.call('create','-f','-','-o','json',body=state['plans'][step])
            self._wait(step,state)
        elif step in ('roll_auth','roll_user'):
            service = step.removeprefix('roll_')
            current = _get(self.kube,'deployment','voice-'+service)
            original = self.intent['original_deployments'][service]
            if current['spec']!=original['spec'] or current['metadata']['uid']!=original['metadata']['uid']:
                raise RecoveryRequired('original rollout CAS target changed')
            patch,_ = deployment_plan(current,service,self.intent['products'][service],self.secret['metadata']['name'])
            self.kube.call('patch','deployment','voice-'+service,'--type=json','-o','json',body=patch)
            self._wait(step,state)
        elif step=='verify':
            self.verify_isolation()
            from auth_user_adoption_smoke import public_smoke
            proof = public_smoke(self.journal,self.kube)
            profile=str(__import__('uuid').UUID(proof['profile_id']))
            account=str(__import__('uuid').UUID(proof['account_id']))
            row=self.kube.query('user_db',"SELECT json_build_object('profile_id',id::text,'account_id',account_id::text) FROM profiles WHERE id='"+profile+"'::uuid")
            if row!={'profile_id':profile,'account_id':account}:
                raise RecoveryRequired('owned public JWT profile not bound to User row')
            proof['user_row_binding']=True
            self.journal.write_private('verification.json',proof)
        else:
            raise RecoveryRequired('unknown adoption effect')

    def _wait(self,step,state):
        deadline = time.monotonic()+125
        stable=0
        while time.monotonic()<deadline:
            self.guard(state)
            result = self.reconcile(step,state)
            if result['status']=='complete':
                stable+=1
                if not step.startswith('roll_') or stable>=3:
                    return
                time.sleep(2)
                continue
            stable=0
            if result['status']!='running':
                raise RecoveryRequired('owned effect failed or conflicting')
            time.sleep(2)
        raise RecoveryRequired('client wait expired; reconcile actual server/Job')

    def verify_isolation(self):
        # Old Pods/ReplicaSets and every original/new old-IP admin session must
        # drain. No arbitrary voice session is ever terminated by this tool.
        pods = _get(self.kube,'pods')['items']
        replicasets = _get(self.kube,'replicasets')['items']
        db = database_state(self.kube)
        for service in ('auth','user'):
            if not self._rollout_ready(service,_get(self.kube,'deployment','voice-'+service)):
                raise RecoveryRequired('actual final source-qualified rollout no longer ready')
        projected_pods = [{'uid':p['metadata']['uid']} for p in pods]
        projected_rs = [{'uid':r['metadata']['uid'],'replicas':r['spec'].get('replicas',0)} for r in replicasets]
        if not old_incarnations_drained(self.intent['old_incarnations'],projected_pods,projected_rs,db['sessions']):
            raise RecoveryRequired('old admin-bearing incarnation or session not drained')
        for role,database in [('auth_runtime','auth_db'),('user_runtime','user_db')]:
            if not any(s['role']==role and s['database']==database for s in db['sessions']):
                raise RecoveryRequired('actual own-role application sessions absent')
        # Canonical barrier gate checks direct runtime ACLs as well as owners.
        for service,database,opposite,ledger in [('auth','auth_db','user_db','flyway_schema_history'),
                                                ('user','user_db','auth_db','schema_migrations')]:
            role = service+'_runtime'
            sql = "SELECT json_build_object('role',current_user,'connect',has_database_privilege(current_user,current_database(),'CONNECT'),'opposite',has_database_privilege(current_user,'"+opposite+"','CONNECT'),'create',has_schema_privilege(current_user,'public','CREATE'),'temp',has_database_privilege(current_user,current_database(),'TEMP'),'ledger_read',has_table_privilege(current_user,'public."+ledger+"','SELECT'),'ledger_write',has_table_privilege(current_user,'public."+ledger+"','INSERT,UPDATE,DELETE,TRUNCATE'));"
            proof = self.kube.runtime_query(role,database,self.journal.state['credentials'][service],sql,opposite)
            if (proof.get('role')!=role or proof.get('connect') is not True or proof.get('opposite') is not False
                    or proof.get('create') is not False or proof.get('temp') is not (service=='user')
                    or proof.get('ledger_read') is not True or proof.get('ledger_write') is not False):
                raise RecoveryRequired('actual runtime credential privilege boundary invalid')
        gate = self.kube.runtime_query('auth_runtime','auth_db',self.journal.state['credentials']['auth'],
            (self.root/'src/backend/auth/src/main/resources/roleidentityauthority/gate-catalog.sql').read_text(encoding='utf-8'))
        validate_ddl_gate(gate,self.root)
        if not gate.get('acl_ok') or not gate.get('runtime_ok'):
            raise RecoveryRequired('canonical barrier ACL invalid')

    def finish(self):
        self.finishing=True
        state = self.journal.state
        if any(state['steps'].get(s,{}).get('status')!='done' for s in
               ('lock','backup','sources','secret','migrate_auth','migrate_user','roles','grants_auth','grants_user','roll_auth','roll_user','verify')):
            raise RecoveryRequired('unfinished effect prevents lock release')
        self.guard(state)
        self.verify_isolation()
        actual = _get(self.kube,'configmap','voice-dev-image-update-lock')
        if (not actual and state.get('finish',{}).get('status') in ('pending','done')
                and state['finish'].get('lock_uid')==state['steps']['lock']['receipt']['uid']):
            # The DELETE response may have been lost. Exact terminal absence,
            # unchanged candidate and drained old incarnations are reobserved;
            # never issue another delete or remove a later owner's lock.
            state['finish']['status']='done'
            self.journal.save()
            return
        if not actual or actual['metadata']['uid']!=state['steps']['lock']['receipt']['uid']:
            raise RecoveryRequired('owned lock release target absent or changed')
        state['finish'] = {'status':'pending','lock_uid':actual['metadata']['uid']}
        self.journal.save()
        options = {'apiVersion':'v1','kind':'DeleteOptions','preconditions':{
            'uid':actual['metadata']['uid'],'resourceVersion':actual['metadata']['resourceVersion']}}
        self.kube.call('delete','--raw=/api/v1/namespaces/voice-dev/configmaps/voice-dev-image-update-lock',
                       '-f','-',body=options)
        self.guard(state)
        if _get(self.kube,'configmap','voice-dev-image-update-lock'):
            raise RecoveryRequired('owned lock deletion not terminal')
        state['finish']['status']='done'
        self.journal.save()

    def abort(self):
        state=self.journal.state
        if any(step in state['steps'] for step in ('migrate_auth','migrate_user','roles','grants_auth','grants_user','roll_auth','roll_user','verify')):
            raise RecoveryRequired('data/rollout effect intent exists; automatic abort refused')
        self.guard(state)
        db=database_state(self.kube)
        if any(s['application']=='voice-adoption-backup' or s['application'].startswith('voice-au-'+self.operation)
               for s in db['sessions']):
            raise RecoveryRequired('owned server operation remains active')
        lock=_get(self.kube,'configmap','voice-dev-image-update-lock')
        if lock:
            if lock.get('data')!=self.lock['data']:
                raise RecoveryRequired('abort cannot delete another updater lock')
            state['finish']={'status':'pending','lock_uid':lock['metadata']['uid'],'kind':'unchanged-data-abort'}
            self.journal.save()
            self.kube.call('delete','--raw=/api/v1/namespaces/voice-dev/configmaps/voice-dev-image-update-lock','-f','-',
                body={'apiVersion':'v1','kind':'DeleteOptions','preconditions':{
                    'uid':lock['metadata']['uid'],'resourceVersion':lock['metadata']['resourceVersion']}})
        self.guard(state)
        if _get(self.kube,'configmap','voice-dev-image-update-lock'):
            raise RecoveryRequired('abort lock deletion not terminal')
        state['aborted']=True
        self.journal.save()


def operator_fingerprint(root):
    root=pathlib.Path(root)
    names=['scripts/dev/auth_user_adoption'+suffix for suffix in
           ('.py','_runtime.py','_backup.py','_smoke.py','_restore.py')]
    names+=['docker/postgres/adopt-auth-user-runtime-roles.sql',
            'docker/postgres/provision-auth-user-runtime-roles.sql',
            'src/backend/auth/src/main/resources/roleidentityauthority/gate-catalog.sql']
    for folder in ('src/backend/auth/src/main/resources/db/migration','src/backend/migrations/user_db'):
        files=sorted((root/folder).glob('*.sql'))
        if not files:
            raise RecoveryRequired('frozen schema source files missing')
        names.extend(path.relative_to(root).as_posix() for path in files)
    records={}
    for name in sorted(names):
        path=root/name
        if path.is_symlink() or not path.is_file():
            raise RecoveryRequired('frozen source must be regular files')
        records[name]={'sha256':hashlib.sha256(path.read_bytes()).hexdigest(),
                       'mode':path.stat().st_mode & 0o777}
    return digest(records)


def verify_package(root,manifest):
    if operator_fingerprint(root)!=manifest.get('operator_sha256'):
        raise RecoveryRequired('frozen operator package changed')
    evidence={}
    for kind,binding in manifest.get('evidence',{}).items():
        path=pathlib.Path(binding['path'])
        if path.is_symlink() or path.stat().st_size>2*1024*1024:
            raise RecoveryRequired('qualification artifact unsafe or oversized')
        raw=path.read_bytes()
        if hashlib.sha256(raw).hexdigest()!=binding['sha256']:
            raise RecoveryRequired('qualification artifact bytes changed')
        evidence[kind]=json.loads(raw)
    validate_admission(manifest,evidence)


def package_images(workloads,pods):
    images={}
    for name,w in workloads.items():
        selected=[p for p in pods if all(p['metadata'].get('labels',{}).get(k)==v
            for k,v in w['spec']['selector']['matchLabels'].items())]
        for container in w['spec']['template']['spec']['containers']:
            ref=container['image']
            if '@sha256:' not in ref:
                actual={s['imageID'].removeprefix('docker-pullable://') for p in selected
                    for s in p.get('status',{}).get('containerStatuses',[]) if s['name']==container['name']}
                if len(actual)!=1:
                    raise RecoveryRequired('baseline image identity ambiguous')
                ref=next(iter(actual))
            if not re.fullmatch(r'[a-z0-9][a-z0-9./:_-]*@sha256:[0-9a-f]{64}',ref):
                raise RecoveryRequired('known immutable baseline image map required')
            images[name+'-'+container['name']]=ref
    return images


def prepare(root,directory,manifest_path,kube,resource=resource_card):
    manifest=json.loads(pathlib.Path(manifest_path).read_text(encoding='utf-8'))
    verify_package(root,manifest)  # before every live read/dry-run; no unqualified apply
    operation=manifest['operation'];operation_name(operation)
    card=resource()
    if (card.get('state')!='held' or card.get('owner')!='Dimas-16'
            or card.get('phase')!='prepare'
            or card.get('tasks')!=['A1-DEV-AUTH-USER-CREDENTIAL-ADOPTION']):
        raise RecoveryRequired('exclusive actual DEV prepare window required')
    original,workloads,pods=capture(kube)
    if original['namespace']['uid']!='15950fc8-0392-4053-9156-e95e78b596f9' or not all(original['ready'].values()):
        raise RecoveryRequired('exact existing DEV baseline not settled')
    images=package_images(workloads,pods)
    wanted=dict(images)
    for service,ref in manifest['products'].items():
        wanted['voice-'+service+'-'+service]=ref
    if card.get('observed',{}).get('images')!=images or card.get('candidate',{}).get('images')!=wanted:
        raise RecoveryRequired('actual baseline or full mixed candidate differs from resource')
    for service in ('auth','user'):
        deployment_plan(workloads['voice-'+service],service,manifest['products'][service],'voice-auth-user-'+operation)
    if _get(kube,'configmap','voice-dev-image-update-lock'):
        raise RecoveryRequired('common updater lock already occupied')
    admin_values=_get(kube,'secret','voice-app-secrets').get('data',{})
    admin_password=base64.b64decode(admin_values['POSTGRES_PASSWORD'])
    if not re.fullmatch(rb'[A-Za-z0-9_-]{32,128}',admin_password):
        raise RecoveryRequired('legacy migration URL password requires separately supported encoding; no rotation permitted')
    if any(j.get('status',{}).get('active',0) for j in _get(kube,'jobs')['items']):
        raise RecoveryRequired('another active Job requires reconciliation')
    database=database_state(kube)
    if (database['owners']!={'auth_db':'voice','user_db':'voice'} or database['roles']
            or database['major']!=16 or database['admin']!={'user':'voice','superuser':True}
            or set(database['logins'])!={'voice','gameintegration_runtime'}
            or [r['version'] for r in database['auth_history']]!=[str(i) for i in range(1,27)]
            or any(not r['success'] for r in database['auth_history'])
            or database['user_history']!={'version':19,'dirty':False,'rows':1}):
        raise RecoveryRequired('reviewed existing schema/role baseline differs')
    database_guard(database,database,{})
    old={'pods':[],'sessions':[s for s in database['sessions'] if s['database'] in ('auth_db','user_db')]}
    rss={r['metadata']['uid']:r for r in _get(kube,'replicasets')['items']}
    for service in ('auth','user'):
        d=workloads['voice-'+service]
        for pod in pods:
            if not all(pod['metadata'].get('labels',{}).get(k)==v for k,v in d['spec']['selector']['matchLabels'].items()):
                continue
            owners=[o for o in pod['metadata'].get('ownerReferences',[]) if o.get('controller') and o.get('kind')=='ReplicaSet']
            if len(owners)!=1 or owners[0]['uid'] not in rss or not any(
                o.get('uid')==d['metadata']['uid'] and o.get('controller') for o in rss[owners[0]['uid']]['metadata'].get('ownerReferences',[])):
                raise RecoveryRequired('original app Pod/ReplicaSet owner chain ambiguous')
            old['pods'].append({'uid':pod['metadata']['uid'],'ip':pod['status']['podIP'],'rs_uid':owners[0]['uid']})
    if len(old['pods'])!=2 or any((s.get('client_address') or '').split('/')[0] not in {p['ip'] for p in old['pods']}
        or s['role']!='voice' for s in old['sessions']):
        raise RecoveryRequired('unrecognized active affected consumer')
    sources=source_config(root,operation)
    intent={'operation':operation,'manifest':manifest,'products':manifest['products'],'runtime_ids':manifest['runtime_ids'],
            'resource':{k:card[k] for k in ('claim','generation','window','candidate')},
            'baseline':original,'database':database,'old_incarnations':old,'backup_catalog':backup_catalog(kube),
            'original_deployments':{s:workloads['voice-'+s] for s in ('auth','user')},
            'sources_sha256':digest(sources),
            'expires_at':(datetime.datetime.now(datetime.timezone.utc)+datetime.timedelta(minutes=15)).isoformat()}
    journal=Journal(directory,intent)
    try:
        if journal.state['steps']:
            raise RecoveryRequired('prepare cannot replace an existing operation')
        own_secret=operation_secret(operation,journal.state['credentials'])
        planned_auth=deployment_plan(workloads['voice-auth'],'auth',manifest['products']['auth'],own_secret['metadata']['name'])[1]
        forbidden=[admin_password,journal.state['credentials']['user'].encode()]
        if 'USER_DB_PASSWORD' in admin_values:
            forbidden.append(base64.b64decode(admin_values['USER_DB_PASSWORD']))
        validate_auth_projection(kube,planned_auth,own_secret,forbidden)
        planned_user=deployment_plan(workloads['voice-user'],'user',manifest['products']['user'],own_secret['metadata']['name'])[1]
        user_forbidden=[admin_password,journal.state['credentials']['auth'].encode()]
        if 'AUTH_DB_PASSWORD' in admin_values:
            user_forbidden.append(base64.b64decode(admin_values['AUTH_DB_PASSWORD']))
        validate_user_projection(kube,planned_user,own_secret,user_forbidden)
        for service in ('auth','user'):
            patch,expected=deployment_plan(workloads['voice-'+service],service,manifest['products'][service],own_secret['metadata']['name'])
            dry=kube.call('patch','deployment','voice-'+service,'--type=json','--dry-run=server','-o','json',body=patch)
            if dry.get('spec')!=expected['spec'] or dry['metadata']['uid']!=expected['metadata']['uid']:
                raise RecoveryRequired('server admission changed planned app credential boundary')
        plans={}
        for step in ('migrate_auth','migrate_user','roles','grants_auth','grants_user'):
            authored=job_plan(step,operation,manifest['tools'],'voice-auth-user-'+operation,sources)
            defaulted=kube.call('create','--dry-run=server','-f','-','-o','json',body=authored)
            validate_defaulted_job(authored,defaulted)
            plans[step]={'apiVersion':'batch/v1','kind':'Job','metadata':authored['metadata'],'spec':defaulted['spec']}
        journal.state['plans']=plans
        journal.save()
        fresh,_,_=capture(kube,[tuple(k.split('/',1)) for k in original['bindings']])
        if fresh!=original:
            raise RecoveryRequired('original preparation snapshot drifted')
        latest=resource()
        if any(latest.get(k)!=card.get(k) for k in ('owner','claim','generation','window','candidate','phase')):
            raise RecoveryRequired('prepare authority changed')
        journal.write_private('baseline-observation.json',{'state':'known','images':images,
            'evidence':'Fresh read-only existing Auth/User adoption baseline; no DB/workload/Secret writes. Old backend source remains unknown.'})
        return journal
    except BaseException:
        journal.close()
        raise
