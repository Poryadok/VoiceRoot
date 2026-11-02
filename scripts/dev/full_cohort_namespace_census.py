"""Source-bound pre-backup namespace census for a fenced cohort operation.

An owned cut may change only its documented Deployment targets; retained
application Pods and their configuration references remain bound. New Jobs,
including migration Jobs, are outside this pre-migration observation.
External writers and forensic provenance are not established here.
"""
import re
import copy
import hashlib
import json
import secrets


class CensusError(RuntimeError):
    pass


APP_NAMES = frozenset(('user','social','role','space','chat','file','messaging','matchmaking','search',
    'notification','realtime','bot','subscription','moderation','story','auth','voice','gameintegration',
    'analytics','gateway','web','admin','developer-portal'))
INFRA_NAMES = frozenset((('Deployment','voice-clamav'),('Deployment','voice-dev-mail'),
    ('Deployment','voice-livekit'),('Deployment','voice-nats-pvc-candidate'),('Deployment','voice-redis'),
    ('StatefulSet','voice-clickhouse'),('StatefulSet','voice-minio'),('StatefulSet','voice-postgres')))


def validate_workload_names(workload_uids):
    expected = {'Deployment/voice-' + name for name in APP_NAMES} | {kind + '/' + name for kind, name in INFRA_NAMES}
    if (not isinstance(workload_uids, dict) or set(workload_uids) != expected or
            any(not isinstance(x,str) or not x for x in workload_uids.values()) or
            len(set(workload_uids.values())) != len(workload_uids)):
        raise CensusError('namespace workload inventory invalid')


def digest(value):
    return hashlib.sha256(json.dumps(value,sort_keys=True,separators=(',',':')).encode()).hexdigest()


def validate_cut_baseline(original, current, proof):
    """Only accept spec changes explicitly supplied by a verified cut proof."""
    try:
        expected = copy.deepcopy(original)
        writers = proof['writers']
        names = set()
        if not isinstance(writers,list): raise ValueError()
        for writer in writers:
            key = writer['kind'] + '/' + writer['name']
            if (key in names or key not in expected['workload_uids'] or
                    writer['uid'] != expected['workload_uids'][key] or not isinstance(writer['spec'],dict)):
                raise ValueError()
            names.add(key)
            expected['workload_spec_sha256'][key] = digest(writer['spec'])
        if current != expected: raise ValueError()
    except (KeyError,TypeError,ValueError):
        raise CensusError('cut baseline changed') from None


_ROLES = {
    'voice-postgres': ('postgres', 'docker.io/library/postgres', [], {'POSTGRES_USER', 'POSTGRES_PASSWORD', 'POSTGRES_DB'}),
    'voice-redis': ('redis', 'docker.io/library/redis', ['redis-server', '--appendonly', 'yes'], set()),
    'voice-nats-pvc-candidate': ('nats', 'nats:2.12.12-alpine', ['-c', '/etc/nats/nats.conf'], set()),
    'voice-livekit': ('livekit', 'docker.io/livekit/livekit-server', ['--config', '/etc/livekit.yaml', '--bind', '0.0.0.0'], set()),
    'voice-clamav': ('clamav', 'docker.io/clamav/clamav', [], set()),
    'voice-clickhouse': ('clickhouse', 'docker.io/clickhouse/clickhouse-server', [], {'CLICKHOUSE_USER', 'CLICKHOUSE_PASSWORD', 'CLICKHOUSE_DB'}),
    'voice-minio': ('minio', 'ghcr.io/poryadok/voiceroot/minio', ['server', '/data', '--console-address', ':9001'], {'MINIO_API_CORS_ALLOW_ORIGIN', 'MINIO_ROOT_USER', 'MINIO_ROOT_PASSWORD'}),
    'voice-dev-mail': ('mail', 'node:24-alpine', [], set()),
}


def validate_infra(name, podspec, mail_source=None, mail_expected=None):
    """Check a fixed role shape; further custody and source checks are mandatory."""
    try:
        role, repo, args, env = _ROLES[name]
        if (not isinstance(podspec, dict) or any(podspec.get(k,False) is not False for k in ('hostNetwork','hostPID','hostIPC')) or
                podspec.get('ephemeralContainers', [])):
            raise ValueError()
        containers = podspec['containers']
        if not isinstance(containers, list) or len(containers) != 1 or containers[0].get('name') != role:
            raise ValueError()
        item = containers[0]
        if item.get('lifecycle') or item.get('volumeDevices'):
            raise ValueError()
        for probe in ('startupProbe','readinessProbe','livenessProbe'):
            execution=item.get(probe,{}).get('exec')
            if execution and not (name=='voice-postgres' and probe=='readinessProbe' and
                                  execution=={'command':['pg_isready','-U','voice','-d','voice']}):
                raise ValueError()
        image = item.get('image')
        if name == 'voice-dev-mail':
            if image != repo or item.get('command') != ['node', '/stub/server.mjs']:
                raise ValueError()
            mounts = item.get('volumeMounts', [])
            if len(mounts) != 1 or mounts[0] != {'name': 'script', 'mountPath': '/stub', 'readOnly': True}:
                raise ValueError()
            volumes = [v for v in podspec.get('volumes', []) if v.get('name') == 'script']
            if (len(volumes) != 1 or volumes[0].get('configMap', {}).get('name') != 'voice-dev-mail' or
                    volumes[0].get('configMap', {}).get('items') or volumes[0].get('configMap', {}).get('optional',False) is not False or
                    not isinstance(mail_expected, str) or not mail_expected or mail_source != {'server.mjs': mail_expected}):
                raise ValueError()
        elif (not isinstance(image, str) or
              not re.fullmatch(re.escape(repo) + r'(?::[^@\s]+)?@sha256:[0-9a-f]{64}', image) or item.get('command')):
            raise ValueError()
        if item.get('args', []) != args or item.get('envFrom', []):
            raise ValueError()
        entries = item.get('env', [])
        if not isinstance(entries, list) or any(not isinstance(x, dict) or not isinstance(x.get('name'), str) for x in entries):
            raise ValueError()
        names = [x['name'] for x in entries]
        if len(names) != len(set(names)) or set(names) != env:
            raise ValueError()
        fixed_env={
            'voice-postgres':{'POSTGRES_USER':{'valueFrom':{'configMapKeyRef':{'name':'voice-app-config','key':'POSTGRES_USER'}}},
                'POSTGRES_PASSWORD':{'valueFrom':{'secretKeyRef':{'name':'voice-app-secrets','key':'POSTGRES_PASSWORD'}}},
                'POSTGRES_DB':{'value':'voice'}},
            'voice-clickhouse':{'CLICKHOUSE_USER':{'value':'default'},
                'CLICKHOUSE_PASSWORD':{'valueFrom':{'secretKeyRef':{'name':'voice-app-secrets','key':'CLICKHOUSE_PASSWORD'}}},
                'CLICKHOUSE_DB':{'value':'voice'}},
        }
        if name in fixed_env and any({k:v for k,v in x.items() if k!='name'} != fixed_env[name][x['name']] for x in entries):
            raise ValueError()
        if name=='voice-minio':
            values={x['name']:{k:v for k,v in x.items() if k!='name'} for x in entries}
            if (values['MINIO_ROOT_USER']!={'valueFrom':{'secretKeyRef':{'name':'voice-minio-credentials','key':'MINIO_ROOT_USER'}}} or
                    values['MINIO_ROOT_PASSWORD']!={'valueFrom':{'secretKeyRef':{'name':'voice-minio-credentials','key':'MINIO_ROOT_PASSWORD'}}} or
                    set(values['MINIO_API_CORS_ALLOW_ORIGIN'])!={'value'} or
                    not isinstance(values['MINIO_API_CORS_ALLOW_ORIGIN']['value'],str)):
                raise ValueError()
        fixed_mounts={
            'voice-postgres':[{'name':'pgdata','mountPath':'/var/lib/postgresql/data'}],
            'voice-clickhouse':[{'name':'chdata','mountPath':'/var/lib/clickhouse'}],
            'voice-minio':[{'name':'data','mountPath':'/data'}],
            'voice-redis':[], 'voice-clamav':[],
            'voice-livekit':[{'name':'livekit-config','mountPath':'/etc/livekit.yaml','subPath':'livekit.yaml','readOnly':True}],
        }
        if name in fixed_mounts and item.get('volumeMounts',[])!=fixed_mounts[name]: raise ValueError()
        if name=='voice-livekit':
            volumes=[v for v in podspec.get('volumes',[]) if v.get('name')=='livekit-config']
            if (len(volumes)!=1 or volumes[0].get('configMap',{}).get('name')!='voice-livekit-config' or
                    volumes[0].get('configMap',{}).get('items') or
                    volumes[0].get('configMap',{}).get('optional',False) is not False): raise ValueError()
        if name == 'voice-nats-pvc-candidate':
            fixed = 'nats:2.12.12-alpine@sha256:2ca98656a279b2d88cfdf2b8c3f0d5d7f3941ae9dc2ab12ebaa92d83e0f4ccdb'
            init = podspec.get('initContainers', [])
            if image != fixed or not isinstance(init, list) or len(init) != 1:
                raise ValueError()
            renderer = init[0]
            security={'allowPrivilegeEscalation':False,'readOnlyRootFilesystem':True,'runAsNonRoot':True,
                      'runAsUser':10000,'runAsGroup':10000,'capabilities':{'drop':['ALL']},
                      'seccompProfile':{'type':'RuntimeDefault'}}
            expected_args = ['/run/nats/input/operator.jwt', '/run/nats/input/account.jwt',
                '/run/nats/input/system-account.jwt', '/run/nats/input/account.public',
                '/run/nats/input/system-account.public', '/run/nats/template/nats.conf',
                '/run/nats-rendered/config/nats.conf']
            if (renderer.get('name') != 'nats-config-renderer' or renderer.get('args') != expected_args or
                    renderer.get('command') or renderer.get('env', []) or renderer.get('envFrom', []) or
                    renderer.get('securityContext')!=security or item.get('securityContext')!=security or
                    renderer.get('lifecycle') or renderer.get('volumeDevices') or
                    any(renderer.get(probe,{}).get('exec') for probe in ('startupProbe','readinessProbe','livenessProbe')) or
                    not re.fullmatch(r'ghcr\.io/poryadok/voiceroot/nats-hub-config-renderer(?::[^@\s]+)?@sha256:[0-9a-f]{64}', renderer.get('image', ''))):
                raise ValueError()
            required_mounts = {('nats-resolver-input', '/run/nats/input', True),
                ('nats-config-template', '/run/nats/template', True),
                ('nats-rendered-config', '/run/nats-rendered', False),
                ('nats-operator-jwt', '/etc/nats/jwt', True), ('nats-hub-tls', '/etc/nats/tls', True)}
            mounts = renderer.get('volumeMounts', [])
            if (len(mounts) != len(required_mounts) or
                    any(set(m) - {'name','mountPath','readOnly'} for m in mounts) or
                    {(m.get('name'),m.get('mountPath'),m.get('readOnly',False)) for m in mounts} != required_mounts):
                raise ValueError()
            expected_main = [
                {'name':'jsdata','mountPath':'/data'},
                {'name':'nats-rendered-config','mountPath':'/etc/nats/nats.conf','subPath':'config/nats.conf','readOnly':True},
                {'name':'nats-operator-jwt','mountPath':'/etc/nats/jwt','readOnly':True},
                {'name':'nats-hub-tls','mountPath':'/etc/nats/tls','readOnly':True}]
            if item.get('volumeMounts', []) != expected_main:
                raise ValueError()
            volumes = {v.get('name'):v for v in podspec.get('volumes', [])}
            if (len(volumes) != len(podspec.get('volumes', [])) or
                    set(volumes)!={'jsdata','nats-rendered-config','nats-config-template','nats-resolver-input',
                                  'nats-operator-jwt','nats-hub-tls'} or
                    volumes.get('jsdata',{}).get('persistentVolumeClaim',{}).get('claimName') != 'voice-nats-jsdata' or
                    volumes.get('nats-config-template',{}).get('configMap',{}).get('name') != 'voice-nats-hub-config' or
                    volumes.get('nats-config-template',{}).get('configMap',{}).get('items') or
                    volumes.get('nats-config-template',{}).get('configMap',{}).get('optional',False) is not False or
                    volumes.get('nats-resolver-input',{}).get('secret',{}).get('secretName') != 'voice-nats-operator' or
                    volumes.get('nats-operator-jwt',{}).get('secret',{}).get('secretName') != 'voice-nats-operator' or
                    volumes.get('nats-hub-tls',{}).get('secret',{}).get('secretName') != 'voice-nats-hub-tls' or
                    volumes.get('nats-rendered-config',{}).get('emptyDir',{}).get('medium') != 'Memory'):
                raise ValueError()
            expected_items={'nats-resolver-input':['operator.jwt','account.jwt','system-account.jwt','account.public','system-account.public'],
                            'nats-operator-jwt':['operator.jwt'],'nats-hub-tls':['tls.crt','tls.key','ca.crt']}
            for volume,keys in expected_items.items():
                secret=volumes[volume]['secret']
                if (secret.get('optional',False) is not False or secret.get('defaultMode')!=288 or
                        secret.get('items') != [{'key':key,'path':key} for key in keys]): raise ValueError()
        elif podspec.get('initContainers', []):
            raise ValueError()
        return True
    except (KeyError, TypeError, ValueError):
        raise CensusError('infrastructure role unavailable') from None


STEP = 'full-cohort-namespace-census'


class NamespaceCensus:
    """Protected original namespace census, rechecked against an owned cut.

    This is not a complete operator or a proof of external writer absence.
    """
    def __init__(self,journal,fence,client,lock,namespace_uid,package,inventory):
        import frontend_runtime_executor
        import full_cohort_inventory_phase
        import full_cohort_package_phase
        import media_operation_journal
        import resource_fencing
        if (type(journal) is not media_operation_journal.Journal or
                type(client) is not frontend_runtime_executor.KubectlTransport or
                type(fence) is not resource_fencing.Fence or type(fence.ledger) is not resource_fencing.KubeLedger or
                type(package) is not full_cohort_package_phase.PackagePhase or
                type(inventory) is not full_cohort_inventory_phase.InventoryPhase or inventory.package is not package or
                client.journal is not journal or fence.ledger.client is not client or
                journal.intent.get('resource_context') != fence.context or journal.intent.get('namespace_uid') != namespace_uid):
            raise CensusError('census identity invalid')
        self.journal,self.fence,self.client,self.package,self.inventory = journal,fence,client,package,inventory
        self._journal,self._fence,self._client,self._package,self._inventory = journal,fence,client,package,inventory
        self.lock,self.namespace_uid = copy.deepcopy(lock),namespace_uid
        self.intent = copy.deepcopy(journal.intent)
        self.poisoned = False
        for phase in (package,inventory):
            if phase.fence.context != fence.context or phase.lock != self.lock or phase.namespace_uid != namespace_uid:
                raise CensusError('census authority differs')

    def _guard(self):
        import media_operation_guard
        if (self.poisoned or self.journal is not self._journal or self.fence is not self._fence or
                self.client is not self._client or self.package is not self._package or self.inventory is not self._inventory or
                self.journal.poisoned or self.journal.intent != self.intent or
                self.journal.dirfd is None or self.journal.lockfd is None or
                self.journal.state.get('intent') != self.intent or self.client.journal is not self.journal or
                self.fence.ledger.client is not self.client or self.intent.get('resource_context')!=self.fence.context or
                self.intent.get('namespace_uid')!=self.namespace_uid or self.inventory.package is not self.package):
            raise CensusError('census identity changed')
        media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)
        row,revision = self.fence.ledger.read()
        if (not self.fence.owned(row) or row.get('status') != 'active' or
                not isinstance(revision,tuple) or len(revision)!=2 or
                any(not isinstance(x,str) or not x for x in revision)):
            raise CensusError('census ledger unavailable')
        for phase in (self.package,self.inventory):
            if (phase.journal.poisoned or phase.journal.dirfd is None or phase.journal.lockfd is None or
                    phase.fence.ledger.read() != (row,revision)): raise CensusError('census ledger differs')
        return row,revision

    def _read(self,resource):
        import full_cohort_schema_metadata as schema
        paths={'deployments':('/apis/apps/v1','Deployment'), 'statefulsets':('/apis/apps/v1','StatefulSet'),
               'replicasets':('/apis/apps/v1','ReplicaSet'), 'daemonsets':('/apis/apps/v1','DaemonSet'),
               'jobs':('/apis/batch/v1','Job'), 'cronjobs':('/apis/batch/v1','CronJob'),
               'horizontalpodautoscalers':('/apis/autoscaling/v2','HorizontalPodAutoscaler'),
               'pods':('/api/v1','Pod'), 'pvcs':('/api/v1','PersistentVolumeClaim')}
        if resource == 'namespace': path='/api/v1/namespaces/voice-dev'
        else:
            prefix,_=paths[resource]; name='persistentvolumeclaims' if resource=='pvcs' else resource
            path=prefix+'/namespaces/voice-dev/'+name+'?limit=500'
        self._guard()
        raw=schema._bounded_process(['kubectl','--kubeconfig','/home/pmd/.kube/config','--context','default',
                '--namespace','voice-dev','--request-timeout=15s','get','--raw',path],8*1024*1024)
        self._guard(); result=json.loads(raw)
        if resource == 'namespace':
            meta=result.get('metadata',{}); labels=meta.get('labels',{})
            if (meta.get('name')!='voice-dev' or meta.get('uid')!=self.namespace_uid or meta.get('deletionTimestamp') or
                    labels.get('voice.environment')!='dev' or labels.get('app.kubernetes.io/managed-by')!='voice-dev'):
                raise CensusError('namespace changed')
        else:
            import media_live_baseline as baseline
            kind=paths[resource][1]
            if not isinstance(result,dict) or result.get('kind')!=kind+'List' or not isinstance(result.get('items'),list):
                raise CensusError('namespace list invalid')
            baseline._list_version(result)
            for row in result['items']:
                if not isinstance(row,dict) or row.get('kind',kind)!=kind: raise CensusError('namespace item invalid')
                row['kind']=kind
            baseline._items(result)
        return result

    def _config(self,name):
        import full_cohort_schema_metadata as schema
        if not isinstance(name,str) or not re.fullmatch(r'[a-z0-9]([-a-z0-9.]*[a-z0-9])?',name): raise CensusError('reference invalid')
        self._guard(); raw=schema._bounded_process(['kubectl','--kubeconfig','/home/pmd/.kube/config',
            '--context','default','--namespace','voice-dev','--request-timeout=15s','get','configmap',name,'-o','json'],1048576)
        self._guard(); value=json.loads(raw); meta=value.get('metadata',{})
        if (value.get('kind')!='ConfigMap' or meta.get('name')!=name or meta.get('namespace')!='voice-dev' or
                not meta.get('uid') or not meta.get('resourceVersion') or meta.get('deletionTimestamp')):
            raise CensusError('config reference unavailable')
        return value

    def _secret_metadata(self,name):
        import full_cohort_schema_metadata as schema
        if not isinstance(name,str) or not re.fullmatch(r'[a-z0-9]([-a-z0-9.]*[a-z0-9])?',name): raise CensusError('reference invalid')
        template='{{.metadata.uid}}{{"\\t"}}{{.metadata.resourceVersion}}{{"\\n"}}{{range $k, $v := .data}}{{$k}}{{"\\n"}}{{end}}'
        self._guard(); raw=schema._bounded_process(['kubectl','--kubeconfig','/home/pmd/.kube/config',
            '--context','default','--namespace','voice-dev','--request-timeout=15s','get','secret',name,
            '-o','go-template='+template],65536); self._guard()
        rows=raw.decode('utf8').splitlines(); fields=rows[0].split('\t')
        if len(fields)!=2 or not all(fields) or len(rows[1:])!=len(set(rows[1:])): raise CensusError('secret metadata unavailable')
        return {'uid':fields[0],'resourceVersion':fields[1],'keys':sorted(rows[1:])}

    def _snapshot(self,token):
        import canonical_application_package as canonical
        import full_cohort_schema_metadata as schema
        import media_live_baseline as baseline
        import publish_candidate
        from pathlib import Path
        self._guard(); package=self.package.verify_current(token)
        source=publish_candidate.source_snapshot(canonical.ROOT,self.fence.context['commit'])
        mail=(Path(canonical.ROOT)/'docker/verification-stub/server.mjs').read_bytes()
        if hashlib.sha256(mail).hexdigest()!=source['files'].get('docker/verification-stub/server.mjs'):
            raise CensusError('mail source changed')
        docs={}
        def reader(context,namespace,resource):
            if context!='default' or namespace!='voice-dev': raise CensusError('baseline target invalid')
            docs[resource]=self._read(resource)
            return copy.deepcopy(docs[resource])
        current=baseline.collect('default','voice-dev',reader=reader)
        if current['namespace_uid']!=self.namespace_uid: raise CensusError('namespace identity changed')
        validate_workload_names(current['workload_uids'])
        for name in ('daemonsets','cronjobs','horizontalpodautoscalers'):
            if self._read(name)['items']: raise CensusError('unsupported namespace controller')
        workloads={x['kind']+'/'+x['metadata']['name']:x for kind in ('deployments','statefulsets') for x in docs[kind]['items']}
        if set(workloads)!=set(current['workload_uids']): raise CensusError('workload inventory changed')
        configs=set(); secrets_=set(); config_keys={}; secret_keys={}
        def refs(value):
            if isinstance(value,dict):
                for key,child in value.items():
                    if key in ('configMap','configMapRef','configMapKeyRef') and isinstance(child,dict) and child.get('name'):
                        configs.add(child['name'])
                        if key=='configMapKeyRef' and child.get('key'): config_keys.setdefault(child['name'],set()).add(child['key'])
                        for item in child.get('items',[]):
                            if item.get('key'): config_keys.setdefault(child['name'],set()).add(item['key'])
                    if key in ('secret','secretRef','secretKeyRef') and isinstance(child,dict):
                        name=child.get('name',child.get('secretName'))
                        if name:
                            secrets_.add(name)
                            if key=='secretKeyRef' and child.get('key'): secret_keys.setdefault(name,set()).add(child['key'])
                            for item in child.get('items',[]):
                                if item.get('key'): secret_keys.setdefault(name,set()).add(item['key'])
                    refs(child)
            elif isinstance(value,list):
                for child in value: refs(child)
        for obj in workloads.values(): refs(obj['spec']['template']['spec'])
        for name,key in (('voice-dev-mail','server.mjs'),('voice-nats-hub-config','nats.conf'),
                         ('voice-livekit-config','livekit.yaml')):
            config_keys.setdefault(name,set()).add(key)
        config_values={name:self._config(name) for name in sorted(configs)}
        config_meta={name:{'uid':v['metadata']['uid'],'resourceVersion':v['metadata']['resourceVersion'],
                     'data_sha256':digest({'data':v.get('data',{}),'binaryData':v.get('binaryData',{})})}
                     for name,v in config_values.items()}
        secret_meta={name:self._secret_metadata(name) for name in sorted(secrets_)}
        if (any(not keys.issubset(config_values[name].get('data',{})) for name,keys in config_keys.items()) or
                any(not keys.issubset(secret_meta[name]['keys']) for name,keys in secret_keys.items())):
            raise CensusError('referenced key unavailable')
        for kind,name in INFRA_NAMES:
            spec=workloads[kind+'/'+name]['spec']['template']['spec']
            mail_data=config_values.get('voice-dev-mail',{}).get('data') if name=='voice-dev-mail' else None
            if name=='voice-dev-mail' and config_values.get('voice-dev-mail',{}).get('binaryData'):
                raise CensusError('mail source changed')
            validate_infra(name,spec,mail_data,mail.decode('utf8') if name=='voice-dev-mail' else None)
        by_uid={obj['metadata']['uid']:key for key,obj in workloads.items()}
        replicas={}
        for obj in docs['replicasets']['items']:
            owner=baseline._owner(obj)
            if owner['kind']!='Deployment' or owner['uid'] not in by_uid: raise CensusError('replicaset owner invalid')
            replicas[obj['metadata']['uid']]=by_uid[owner['uid']]
        details={kind+'/'+name:[] for kind,name in INFRA_NAMES}
        application_pods={'Deployment/voice-'+name:[] for name in APP_NAMES}
        active_counts={key:0 for key in workloads}
        def process_shape(container):
            mounts=[]
            for m in container.get('volumeMounts',[]):
                if (m.get('mountPath')=='/var/run/secrets/kubernetes.io/serviceaccount' and
                        m.get('name','').startswith('kube-api-access-') and m.get('readOnly') is True and
                        set(m)=={'name','mountPath','readOnly'}):
                    continue
                mounts.append(m)
            return {k:copy.deepcopy(container.get(k,[])) for k in ('command','args','env','envFrom','volumeDevices')} | {
                k:copy.deepcopy(container.get(k)) for k in ('lifecycle','readinessProbe','livenessProbe','startupProbe')} | {
                'name':container.get('name'),'image':container.get('image'),'volumeMounts':mounts}
        for pod in docs['pods']['items']:
            status=pod.get('status',{}); meta=pod.get('metadata',{})
            if status.get('phase') in ('Succeeded','Failed'): continue
            owner=baseline._owner(pod)
            key=replicas.get(owner['uid']) if owner['kind']=='ReplicaSet' else by_uid.get(owner['uid']) if owner['kind']=='StatefulSet' else None
            if key not in workloads or meta.get('deletionTimestamp') or status.get('phase')!='Running':
                raise CensusError('active pod ownership invalid')
            spec=pod.get('spec',{}); expected=workloads[key]['spec']['template']['spec']
            if any(spec.get(k,False) is not False for k in ('hostNetwork','hostPID','hostIPC')) or spec.get('ephemeralContainers',[]):
                raise CensusError('pod network invalid')
            actual=spec.get('containers'); intended=expected.get('containers')
            if (not isinstance(actual,list) or not isinstance(intended,list) or
                    sorted((process_shape(x) for x in actual),key=lambda x:x['name']) !=
                    sorted((process_shape(x) for x in intended),key=lambda x:x['name'])):
                raise CensusError('pod process differs from controller')
            actual_init=spec.get('initContainers',[]); intended_init=expected.get('initContainers',[])
            if ([process_shape(x) for x in actual_init] != [process_shape(x) for x in intended_init] or
                    (key!='Deployment/voice-nats-pvc-candidate' and actual_init)):
                raise CensusError('pod init process invalid')
            if key in details:
                requested={m['name'] for c in intended+intended_init for m in c.get('volumeMounts',[])}
                actual_volumes={v.get('name'):v for v in spec.get('volumes',[])}
                expected_volumes={v.get('name'):v for v in expected.get('volumes',[])}
                if len(actual_volumes)!=len(spec.get('volumes',[])) or len(expected_volumes)!=len(expected.get('volumes',[])):
                    raise CensusError('pod volume identity invalid')
                claims={'StatefulSet/voice-postgres':('pgdata','pgdata-voice-postgres-0'),
                        'StatefulSet/voice-clickhouse':('chdata','chdata-voice-clickhouse-0'),
                        'StatefulSet/voice-minio':('data','data-voice-minio-0')}
                for volume in requested:
                    if key in claims and volume==claims[key][0]:
                        target=actual_volumes.get(volume,{}).get('persistentVolumeClaim',{})
                        if target.get('claimName')!=claims[key][1] or target.get('readOnly',False) is not False:
                            raise CensusError('pod PVC differs')
                    elif actual_volumes.get(volume)!=expected_volumes.get(volume) or volume not in expected_volumes:
                        raise CensusError('pod volume differs')
            states=status.get('containerStatuses',[]); init_states=status.get('initContainerStatuses',[])
            if (not isinstance(states,list) or len(states)!=len(actual) or
                    {x.get('name') for x in states}!={x.get('name') for x in actual} or
                    len({x.get('name') for x in states})!=len(states) or
                    any(x.get('ready') is not True or not isinstance(x.get('state',{}).get('running'),dict) or
                        not isinstance(x.get('containerID'),str) or not x['containerID'] or
                        not isinstance(x.get('imageID'),str) or not x['imageID'] or
                        type(x.get('restartCount')) is not int or x['restartCount']<0 for x in states) or
                    not isinstance(init_states,list) or len(init_states)!=len(actual_init) or
                    {x.get('name') for x in init_states}!={x.get('name') for x in actual_init} or
                    any(type(x.get('state',{}).get('terminated',{}).get('exitCode')) is not int or
                        x['state']['terminated']['exitCode']!=0 or not x.get('imageID') or
                        not x.get('containerID') or type(x.get('restartCount')) is not int or
                        x['restartCount']<0 for x in init_states)):
                raise CensusError('pod runtime identity invalid')
            active_counts[key]+=1
            if key in details or key in application_pods:
                if not isinstance(spec.get('nodeName'),str) or not spec['nodeName']:
                    raise CensusError('workload node invalid')
                item={'name':meta['name'],'uid':meta['uid'],'owner_uid':owner['uid'],'node':spec['nodeName'],
                    'spec_sha256':digest(spec),
                    'containers':sorted(({'name':x['name'],'container_id':x['containerID'],'image_id':x['imageID'],
                        'restart_count':x['restartCount']} for x in states),key=lambda x:x['name']),
                    'init':sorted(({'name':x['name'],'image_id':x['imageID'],'container_id':x['containerID'],
                        'restart_count':x['restartCount']} for x in init_states),key=lambda x:x['name'])}
                (details if key in details else application_pods)[key].append(item)
        for key,obj in workloads.items():
            replicas=obj['spec'].get('replicas')
            if type(replicas) is not int or replicas<0 or active_counts[key]!=replicas:
                raise CensusError('controller pod count changed')
        for value in details.values(): value.sort(key=lambda x:x['uid'])
        for value in application_pods.values(): value.sort(key=lambda x:x['uid'])
        jobs=sorted(({'uid':x['metadata']['uid'],'resourceVersion':x['metadata']['resourceVersion'],
                      'sha256':digest({'spec':x.get('spec'),'status':x.get('status')})} for x in docs['jobs']['items']),
                    key=lambda x:x['uid'])
        generations={key:obj['metadata'].get('generation') for key,obj in workloads.items()}
        if any(type(x) is not int for x in generations.values()): raise CensusError('controller generation invalid')
        self._guard()
        if self.package.verify_current(token)!=package or publish_candidate.source_snapshot(canonical.ROOT,self.fence.context['commit'])!=source:
            raise CensusError('census source changed')
        return {'baseline':current,'generations':generations,'infra_pods':details,'application_pods':application_pods,'jobs':jobs,
                'configmaps':config_meta,'secrets':secret_meta,'source_sha':self.fence.context['commit'],
                'package_sha256':package['package_sha256'],'namespace_uid':self.namespace_uid}

    def run(self,token):
        pending=False
        try:
            row,rev=self._guard()
            if STEP in self.journal.state['steps'] or 'full_cohort_namespace_census' in row: raise ValueError()
            inventory=self.inventory.verify_current(token)
            first=self._snapshot(token)
            if self._snapshot(token)!=first or self.inventory.verify_current(token)!=inventory: raise ValueError()
            if first['package_sha256']!=inventory['package_sha256']: raise ValueError()
            if set(inventory['projection'])!=APP_NAMES: raise ValueError()
            for name in APP_NAMES:
                key='Deployment/voice-'+name
                actual=first['application_pods'][key]
                entry=inventory['projection'][name]
                if (entry['uid']!=first['baseline']['workload_uids'][key] or
                        entry['generation']!=first['generations'][key] or
                        entry['spec_sha256']!=first['baseline']['workload_spec_sha256'][key] or
                        entry['replicas']!=len(actual)): raise ValueError()
                historical=entry['pods']
                projected=sorted(({'name':p['name'],'uid':p['uid'],'replicaset_uid':p['owner_uid'],
                    'containers':p['containers']} for p in actual),key=lambda p:p['uid'])
                if projected!=sorted(historical,key=lambda p:p['uid']): raise ValueError()
            marker={'status':'pending','nonce':secrets.token_hex(16),'operation_id':self.fence.context['operation_id'],
                    'source_sha':self.fence.context['commit']}
            pending=True; self.journal.state['steps'][STEP]=copy.deepcopy(marker); self.journal.save()
            current,revision=self._guard()
            if current!=row or revision!=rev or self.journal.state['steps'].get(STEP)!=marker: raise ValueError()
            current['full_cohort_namespace_census']=copy.deepcopy(marker); current['step']=STEP+'-pending'
            self.fence.ledger.write(revision,current)
            if self._snapshot(token)!=first or self.inventory.verify_current(token)!=inventory: raise ValueError()
            current,revision=self._guard()
            if current.get('full_cohort_namespace_census')!=marker or self.journal.state['steps'].get(STEP)!=marker:
                raise ValueError()
            receipt={'operation_id':marker['operation_id'],'inventory_sha256':digest(inventory),**first}
            done=marker|{'status':'done','receipt_sha256':digest(receipt)}
            local=done|{'receipt':copy.deepcopy(receipt)}
            self.journal.state['steps'][STEP]=copy.deepcopy(local); self.journal.save()
            current['full_cohort_namespace_census']=copy.deepcopy(done); current['step']=STEP+'-done'
            self.fence.ledger.write(revision,current)
            final,_=self._guard()
            if final.get('full_cohort_namespace_census')!=done or self.journal.state['steps'].get(STEP)!=local:
                raise ValueError()
            return copy.deepcopy(receipt)
        except BaseException:
            if pending:
                self.poisoned=True
                try: self.fence.uncertain('namespace census outcome uncertain')
                except BaseException: pass
            raise CensusError('namespace census unavailable') from None

    def verify_current(self,token,cut=None):
        try:
            local=copy.deepcopy(self.journal.state['steps'][STEP]); receipt=local['receipt']
            marker={k:v for k,v in local.items() if k!='receipt'}
            row,_=self._guard()
            if (set(marker)!={'status','nonce','operation_id','source_sha','receipt_sha256'} or
                    marker.get('status')!='done' or marker.get('receipt_sha256')!=digest(receipt) or
                    marker.get('operation_id')!=self.fence.context['operation_id'] or
                    marker.get('source_sha')!=self.fence.context['commit'] or
                    row.get('full_cohort_namespace_census')!=marker): raise ValueError()
            inventory=self.inventory.verify_receipt(token)
            if digest(inventory)!=receipt['inventory_sha256']: raise ValueError()
            proof=None
            if cut is None:
                if self.inventory.verify_current(token)!=inventory: raise ValueError()
            else:
                import writer_cut
                if (type(cut) is not writer_cut.WriterCut or cut.fence.context!=self.fence.context or
                        cut.lock!=self.lock or cut.namespace_uid!=self.namespace_uid or
                        cut.journal.poisoned or cut.journal.dirfd is None or cut.journal.lockfd is None or
                        cut.fence.ledger.read()!=self.fence.ledger.read()): raise ValueError()
                proof=cut.current_cut_proof()
            observed=self._snapshot(token)
            if self._snapshot(token)!=observed: raise ValueError()
            old={k:v for k,v in receipt.items() if k not in ('operation_id','inventory_sha256')}
            if proof is None:
                if observed!=old: raise ValueError()
            else:
                expected=copy.deepcopy(old)
                for writer in proof['writers']:
                    key=writer['kind']+'/'+writer['name']
                    if (key not in expected['application_pods'] or writer['uid']!=old['baseline']['workload_uids'].get(key) or
                            writer['spec'].get('replicas')!=0):
                        raise ValueError()
                    expected['generations'][key]=writer['generation']
                    expected['application_pods'][key]=[]
                validate_cut_baseline(old['baseline'],observed['baseline'],proof)
                expected['baseline']=copy.deepcopy(observed['baseline'])
                if observed!=expected or cut.current_cut_proof()!=proof: raise ValueError()
            final,_=self._guard()
            if (final.get('full_cohort_namespace_census')!=marker or self.journal.state['steps'].get(STEP)!=local or
                    self.inventory.verify_receipt(token)!=inventory or
                    (cut is not None and cut.current_cut_proof()!=proof)): raise ValueError()
            final,_=self._guard()
            if final.get('full_cohort_namespace_census')!=marker or self.journal.state['steps'].get(STEP)!=local:
                raise ValueError()
            return copy.deepcopy(receipt)
        except BaseException:
            raise CensusError('namespace census no longer current') from None
