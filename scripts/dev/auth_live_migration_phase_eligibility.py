"""Closed Auth configuration checks, used only with fenced Kubernetes snapshots.

These pure checks are not admission evidence; the phase must bind their inputs
and result to its protected intent before acting.
"""
import re
import shlex
import base64
import posixpath
import copy
import subprocess
import hashlib
import json
import publish_candidate
import cohort_readback as readback
from frontend_runtime_executor import KubectlTransport


class EligibilityError(RuntimeError):
    pass


def _source_tree(root, commit):
    if not isinstance(commit,str) or not re.fullmatch(r'[0-9a-f]{40}',commit): raise ValueError()
    def git(ref):
        p=subprocess.run(['git','-C',str(root),'rev-parse','--verify',ref+':src/backend/auth'],
                         stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,timeout=20,check=True)
        result=p.stdout.decode('ascii').strip()
        if not re.fullmatch(r'[0-9a-f]{40}',result): raise ValueError()
        return result
    tree=git(commit)
    if tree!=git('d11f510e50a4a86bfa2cf8707a8b087efe7d77f8'): raise ValueError()
    return tree


def observe_candidate(phase, drain, source_root, actor, token):
    """Produce a guarded private candidate snapshot; phase must persist and recheck it."""
    try:
        p=phase.phases
        if type(p.client) is not KubectlTransport or p.client.journal is not p.journal or p.fence.ledger.client is not p.client:
            raise ValueError()
        plan=copy.deepcopy(p.intent['auth_live_candidate'])
        if (p.journal.intent!=p.intent or set(plan)!={'deployment','platform_digest'}
                or set(plan['deployment'])!={'uid','generation','resourceVersion','spec'}): raise ValueError()
        expected=plan['deployment']; context=p.fence.context
        readback._guard(phase,drain)
        tree=_source_tree(source_root,context['commit'])
        image=context['images']['voice-auth/auth']
        target=candidate_target(expected['spec'],image)
        auth=next(c for c in target['template']['spec']['containers'] if c['name']=='auth')
        def deployment():
            readback._guard(phase,drain)
            obj=p.client.call('get','deployment','voice-auth','-o','json')
            readback._guard(phase,drain)
            meta=obj['metadata']
            if (meta.get('name')!='voice-auth' or meta.get('namespace')!='voice-dev'
                    or meta.get('uid')!=expected['uid'] or meta.get('generation')!=expected['generation']
                    or not isinstance(meta.get('resourceVersion'),str) or not meta['resourceVersion']
                    or obj['spec']!=expected['spec']): raise ValueError()
            return {'uid':meta['uid'],'generation':meta['generation'],'resourceVersion':meta['resourceVersion']}
        first=deployment()
        index=image.rsplit('@',1)[1]
        registry=publish_candidate.Registry(actor,token)
        readback._guard(phase,drain)
        image_config=read_image_config(registry,index,plan['platform_digest'])
        readback._guard(phase,drain)
        guard=lambda: readback._guard(phase,drain)
        values,refs=read_references(p.client,auth,guard)
        effective=resolve_environment(auth,values,image_config['env'])
        for key in ('PATH','JAVA_HOME'):
            if effective.get(key)!=image_config['env'].get(key): raise ValueError()
        policy=check_environment(effective)
        values2,refs2=read_references(p.client,auth,guard)
        if values!=values2 or refs!=refs2 or p.intent['auth_live_candidate']!=plan or p.journal.intent!=p.intent:
            raise ValueError()
        last=deployment()
        encoded=json.dumps(target,sort_keys=True,separators=(',',':')).encode()
        return {'source_sha':context['commit'],'auth_tree':tree,'image':image,'platform_digest':plan['platform_digest'],
                'config_digest':image_config['config_digest'],'deployment':last,'initial_resourceVersion':first['resourceVersion'],
                'target_spec_sha256':hashlib.sha256(encoded).hexdigest(),'references':refs,'policy':policy}
    except BaseException:
        raise EligibilityError('candidate Auth configuration not admitted') from None


def candidate_target(spec, image):
    try:
        if (not isinstance(image,str) or not re.fullmatch(r'ghcr\.io/poryadok/voiceroot/auth@sha256:[0-9a-f]{64}',image)
                or type(spec.get('replicas')) is not int or spec['replicas']!=0): raise ValueError()
        target=copy.deepcopy(spec)
        pod=target['template']['spec']
        if pod.get('initContainers'): raise ValueError()
        containers=pod['containers']
        matches=[c for c in containers if c.get('name')=='auth']
        if len(matches)!=1: raise ValueError()
        matches[0]['image']=image
        check_container(matches[0])
        return target
    except BaseException:
        raise EligibilityError('unsupported Auth target') from None


def check_container(container):
    try:
        if (not isinstance(container,dict) or container.get('name')!='auth'
                or container.get('command') not in (None,[]) or container.get('args') not in (None,[])
                or container.get('workingDir') not in (None,'/') or container.get('volumeDevices')): raise ValueError()
        forbidden=('/config','/auth.jar','/usr','/opt/java','/etc/java',
                   '/application.yml','/application.yaml','/application.properties')
        mounts=container.get('volumeMounts',[])
        if not isinstance(mounts,list): raise ValueError()
        for mount in mounts:
            path=mount['mountPath']
            if not isinstance(path,str) or not path.startswith('/') or posixpath.normpath(path)!=path: raise ValueError()
            if any(path==x or path.startswith(x+'/') or x.startswith(path.rstrip('/')+'/') for x in forbidden): raise ValueError()
    except BaseException:
        raise EligibilityError('unsupported candidate Auth container') from None


def read_references(client, container, guard):
    """Read all referenced objects twice under guard; values remain private."""
    try:
        names=set()
        for item in container.get('envFrom',[]):
            for field,kind in (('configMapRef','configmap'),('secretRef','secret')):
                if field in item: names.add((kind,item[field]['name']))
        for item in container.get('env',[]):
            for field,kind in (('configMapKeyRef','configmap'),('secretKeyRef','secret')):
                if field in item.get('valueFrom',{}): names.add((kind,item['valueFrom'][field]['name']))
        if len(names)>128 or any(not isinstance(n,str) or not re.fullmatch(r'[a-z0-9]([-a-z0-9.]*[a-z0-9])?',n) or len(n)>253 for _,n in names):
            raise ValueError()
        def capture():
            snapshots={}
            for kind,name in sorted(names):
                if guard() is False: raise ValueError()
                obj=client.call('get',kind,name,'-o','json')
                if guard() is False: raise ValueError()
                meta=obj['metadata']
                if (meta.get('name')!=name or meta.get('namespace')!='voice-dev'
                        or any(not isinstance(meta.get(k),str) or not meta[k] for k in ('uid','resourceVersion'))
                        or obj.get('binaryData')): raise ValueError()
                data=obj.get('data',{})
                if not isinstance(data,dict) or len(data)>4096: raise ValueError()
                values={}
                for k,v in data.items():
                    if not isinstance(k,str) or not isinstance(v,str): raise ValueError()
                    values[k]=base64.b64decode(v,validate=True).decode('utf-8') if kind=='secret' else v
                    if '\x00' in values[k]: raise ValueError()
                snapshots[(kind,name)]=(meta['uid'],meta['resourceVersion'],values)
            return snapshots
        first=capture(); second=capture()
        if first!=second: raise ValueError()
        return ({key:entry[2] for key,entry in first.items()},
                [{'kind':key[0],'name':key[1],'uid':entry[0],'resourceVersion':entry[1]} for key,entry in first.items()])
    except BaseException:
        raise EligibilityError('Auth environment references unavailable') from None


def read_image_config(registry, index_digest, platform_digest):
    """Read content-addressed Auth config; caller binds digests to protected package."""
    try:
        if any(not isinstance(value,str) or not re.fullmatch(r'sha256:[0-9a-f]{64}',value)
               for value in (index_digest,platform_digest)): raise ValueError()
        repository='poryadok/voiceroot/auth'
        index=publish_candidate.decode_manifest(registry.get(repository,'manifests/'+index_digest),index_digest)
        if publish_candidate.platform_manifest(index)!=platform_digest: raise ValueError()
        matches=[x for x in index['manifests'] if x.get('digest')==platform_digest
                 and x.get('platform',{}).get('os')=='linux' and x.get('platform',{}).get('architecture')=='amd64'
                 and not x.get('platform',{}).get('variant')]
        if len(matches)!=1 or type(matches[0].get('size')) is not int: raise ValueError()
        raw=registry.get(repository,'manifests/'+platform_digest)
        if len(raw)!=matches[0]['size']: raise ValueError()
        manifest=publish_candidate.decode_manifest(raw,platform_digest)
        descriptor=manifest['config']
        if not isinstance(descriptor,dict) or not re.fullmatch(r'sha256:[0-9a-f]{64}',descriptor.get('digest','')):
            raise ValueError()
        config_raw=registry.get(repository,'blobs/'+descriptor['digest'])
        if type(descriptor.get('size')) is not int or descriptor['size']!=len(config_raw): raise ValueError()
        image=publish_candidate.decode_manifest(config_raw,descriptor['digest'])
        config=image['config']
        if (manifest.get('schemaVersion')!=2 or image.get('os')!='linux' or image.get('architecture')!='amd64'
                or config.get('Entrypoint')!=['java','-jar','/auth.jar'] or config.get('Cmd') not in (None,[])
                or config.get('WorkingDir')!='/'): raise ValueError()
        env={}
        for row in config.get('Env',[]):
            if not isinstance(row,str) or '=' not in row: raise ValueError()
            key,value=row.split('=',1)
            if not key or key in env: raise ValueError()
            env[key]=value
        return {'env':env,'config_digest':descriptor['digest']}
    except BaseException:
        raise EligibilityError('candidate image configuration unavailable') from None


def resolve_environment(container, sources, image_env=None):
    """Resolve a complete private snapshot; this function does not authenticate it."""
    if not isinstance(container,dict) or not isinstance(sources,dict): raise EligibilityError('invalid environment')
    if image_env is not None and (not isinstance(image_env,dict) or any(not isinstance(k,str) or not isinstance(v,str) for k,v in image_env.items())):
        raise EligibilityError('invalid image environment')
    result=dict(image_env or {})
    def source(kind, spec):
        if not isinstance(spec,dict) or set(spec)-{'name','key','optional'} or not isinstance(spec.get('name'),str):
            raise EligibilityError('unsupported environment reference')
        if type(spec.get('optional',False)) is not bool: raise EligibilityError('invalid reference')
        key=(kind,spec['name'])
        if key not in sources or not isinstance(sources[key],dict): raise EligibilityError('source snapshot missing')
        return sources[key]
    for item in container.get('envFrom',[]):
        if not isinstance(item,dict) or set(item)-{'prefix','configMapRef','secretRef'}: raise EligibilityError('invalid envFrom')
        prefix=item.get('prefix','')
        if not isinstance(prefix,str): raise EligibilityError('invalid prefix')
        refs=[(field,kind) for field,kind in (('configMapRef','configmap'),('secretRef','secret')) if field in item]
        if len(refs)!=1: raise EligibilityError('invalid envFrom reference')
        field,kind=refs[0]
        for key,value in source(kind,item[field]).items():
            if not isinstance(key,str) or not isinstance(value,str): raise EligibilityError('invalid source data')
            name=prefix+key
            if re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*',name): result[name]=value
    seen=set()
    for item in container.get('env',[]):
        if not isinstance(item,dict) or set(item)-{'name','value','valueFrom'}: raise EligibilityError('invalid environment entry')
        name=item.get('name')
        if not isinstance(name,str) or not re.fullmatch(r'[A-Za-z_][A-Za-z0-9_]*',name) or name in seen:
            raise EligibilityError('ambiguous environment')
        seen.add(name)
        if 'value' in item and 'valueFrom' not in item:
            value=item['value']
            if not isinstance(value,str) or '$(' in value: raise EligibilityError('unsupported interpolation')
        elif 'valueFrom' in item and 'value' not in item:
            ref=item['valueFrom']
            if not isinstance(ref,dict) or len(ref)!=1: raise EligibilityError('unsupported environment reference')
            field,spec=next(iter(ref.items()))
            if field not in ('configMapKeyRef','secretKeyRef'): raise EligibilityError('unsupported environment reference')
            values=source('configmap' if field=='configMapKeyRef' else 'secret',spec)
            key=spec.get('key')
            if not isinstance(key,str) or not key: raise EligibilityError('invalid source key')
            if key not in values:
                if spec.get('optional',False): continue
                raise EligibilityError('required environment missing')
            value=values[key]
            if not isinstance(value,str): raise EligibilityError('invalid source value')
        else: raise EligibilityError('unsupported environment entry')
        result[name]=value
    return result


def check_environment(env):
    if not isinstance(env, dict) or any(not isinstance(k,str) or not isinstance(v,str) for k,v in env.items()):
        raise EligibilityError('unresolved Auth environment')
    if env.get('AUTH_FLYWAY_ENABLED','').lower() != 'false' or env.get('AUTH_PERSISTENCE','jdbc') != 'jdbc':
        raise EligibilityError('unsupported Auth persistence mode')
    allowed={'SPRING_DATASOURCE_URL','SPRING_DATASOURCE_USERNAME','SPRING_DATASOURCE_PASSWORD','AUTH_PERSISTENCE'}
    for key,value in env.items():
        normalized=re.sub(r'[^a-z0-9]','',key.lower())
        if (normalized.startswith('authroleidentityselector') or normalized.startswith('springconfig')
                or key.startswith('LD_')
                or normalized.startswith('springprofiles') or normalized.startswith('springapplicationjson')
                or normalized.startswith('springflyway') or normalized.startswith('voiceauthjdbc')
                or (normalized == 'authflywayenabled' and key != 'AUTH_FLYWAY_ENABLED')
                or (normalized.startswith('springdatasource') and key not in allowed)
                or (normalized.startswith('authpersistence') and key not in allowed)
                or normalized in ('jdkjavaoptions','javaoptions','javaagent')):
            raise EligibilityError('unsupported Auth configuration override')
        if key == 'JAVA_TOOL_OPTIONS':
            try: options=shlex.split(value)
            except ValueError: raise EligibilityError('invalid JVM options') from None
            for option in options:
                if not (re.fullmatch(r'-XX:(MaxRAMPercentage|InitialRAMPercentage)=[0-9.]+',option)
                        or re.fullmatch(r'-Djavax\.net\.ssl\.(trustStore|trustStorePassword|trustStoreType)=[A-Za-z0-9_./-]+',option)):
                    raise EligibilityError('unsupported JVM option')
    return {'flyway':'disabled','persistence':'jdbc','selector':'absent'}
