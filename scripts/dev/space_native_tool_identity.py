"""Source-bound Space native installer identity; no deployment or DB access.

The source inventory helper validates bytes but does not authenticate its input.
Production admission must obtain source from the protected canonical package.
"""
import argparse
import base64
import copy
import hashlib
import importlib.util
import json
import os
from pathlib import Path
import re
import subprocess
import stat
import tempfile
import time
import zipfile

_migration_path = Path(__file__).resolve().with_name('migrations.py')
_migration_spec = importlib.util.spec_from_file_location('_voice_dev_native_migrations', _migration_path)
if _migration_spec is None or _migration_spec.loader is None:
    raise ImportError('fixed DEV migration source unavailable')
migrations = importlib.util.module_from_spec(_migration_spec)
_migration_spec.loader.exec_module(migrations)


class ToolIdentityError(RuntimeError): pass


# Reviewed semantics establish that the fixed no-network error is reached only
# after these source-owned gates. A change requires a new semantic review.
GATE_SOURCE_SHA256={
    'src/backend/space/internal/nativeinstaller/runner.go':'ac4de9aba8e642b46aa8ad09d1dbb70054fb7cd7bb6bf9ddfa60d3ef1c0ff15c',
    'src/backend/space/internal/store/native_catalog_rowjson_types.go':'110c2a4c5f70cdc955ba4cb9f8d7cee424ec15176a7ab70c9c863c0d4f0dbc25',
    'src/backend/space/cmd/native-migrate/main.go':'6ac8c507850e5408761253f9613df19119dcf6ba6afacb457fdd13b5c589fc6b',
}


def ci_identity(env,run):
    """Check authenticated in-progress master push before the tool build."""
    try:
        source=env['GITHUB_SHA']; run_id=env['GITHUB_RUN_ID']; attempt=env['GITHUB_RUN_ATTEMPT']
        if (env['GITHUB_REPOSITORY']!='Poryadok/VoiceRoot' or env['GITHUB_EVENT_NAME']!='push'
            or env['GITHUB_REF']!='refs/heads/master' or not re.fullmatch(r'[a-f0-9]{40}',source)
            or any(not isinstance(v,str) or not re.fullmatch(r'[1-9][0-9]{0,19}',v) for v in (run_id,attempt))): raise ValueError()
        if (run.get('id')!=int(run_id) or run.get('run_attempt')!=int(attempt) or run.get('head_sha')!=source
            or run.get('head_branch')!='master' or run.get('event')!='push'
            or run.get('path','').split('@',1)[0]!='.github/workflows/ci.yml' or run.get('status')!='in_progress'
            or run.get('conclusion') is not None or run.get('repository',{}).get('full_name')!='Poryadok/VoiceRoot'
            or run.get('head_repository',{}).get('full_name')!='Poryadok/VoiceRoot'): raise ValueError()
        return {'source_sha':source,'run_id':int(run_id),'run_attempt':int(attempt),
                'tag':'ghcr.io/poryadok/voiceroot/space-native-installer:source-'+source+'-'+run_id+'-'+attempt}
    except Exception: raise ToolIdentityError('Space native CI identity invalid') from None


def content_identity(inventory,binary,driver,sql):
    """Bind extracted actual bytes; embedded strings are evidence, not execution proof."""
    try:
        if (not isinstance(binary,bytes) or not binary.startswith(b'\x7fELF') or not isinstance(driver,bytes)
            or not driver.startswith(b'\x7fELF') or not isinstance(sql,dict) or not sql): raise ValueError()
        source=inventory['source_sha']; build=inventory['build_source_sha256']; driver_hash=hashlib.sha256(driver).hexdigest()
        if (not re.fullmatch(r'[a-f0-9]{40}',source) or not re.fullmatch(r'[a-f0-9]{64}',build)
            or not re.fullmatch(r'[a-f0-9]{64}',inventory['sql_inventory_sha256'])
            or any(value.encode() not in binary for value in (source,build,driver_hash))): raise ValueError()
        if (set(sql)!=set(inventory['sql_files']) or any(not isinstance(v,bytes) for v in sql.values())
            or {name:hashlib.sha256(value).hexdigest() for name,value in sql.items()}!=inventory['sql_files']): raise ValueError()
        return {'executable_sha256':hashlib.sha256(binary).hexdigest(),'migrator_sha256':driver_hash,
                'sql_inventory_sha256':inventory['sql_inventory_sha256'],'embedded_values_observed':True,
                'runtime_receipt_verified':False}
    except Exception: raise ToolIdentityError('Space native image content mismatch') from None


def oci_identity(source,index_digest,index_raw,manifest_raw,config_raw):
    """Verify registry bytes and exact linux/amd64 tool image configuration."""
    import publish_candidate as common
    try:
        def unique(pairs):
            result={}
            for key,value in pairs:
                if key in result: raise ValueError()
                result[key]=value
            return result
        for raw in (index_raw,manifest_raw,config_raw): json.loads(raw,object_pairs_hook=unique)
        index=common.decode_manifest(index_raw,index_digest)
        platform=common.platform_manifest(index)
        manifest=common.decode_manifest(manifest_raw,platform)
        config_digest=manifest['config']['digest']
        config=common.decode_manifest(config_raw,config_digest)
        runtime=config['config']
        env=runtime.get('Env') or []
        if (not isinstance(env,list) or any(not isinstance(v,str) or '=' not in v for v in env)
            or len({v.split('=',1)[0] for v in env})!=len(env)
            or any(v.split('=',1)[0].startswith(('PG','SPACE_NATIVE_','LD_')) for v in env)): raise ValueError()
        if (not isinstance(source,str) or not re.fullmatch(r'[a-f0-9]{40}',source)
            or config.get('os')!='linux' or config.get('architecture')!='amd64'
            or runtime.get('User')!='65532:65532' or runtime.get('Entrypoint')!=['/usr/local/bin/native-migrate']
            or runtime.get('Labels',{}).get('org.opencontainers.image.revision')!=source): raise ValueError()
        return {'index_digest':index_digest,'platform_digest':platform,'config_digest':config_digest,
                'immutable_image':'ghcr.io/poryadok/voiceroot/space-native-installer@'+platform}
    except Exception: raise ToolIdentityError('Space native OCI identity invalid') from None


def validate_publication(receipt,identity,inventory):
    """Validate the fixed CI receipt; authenticated run/artifact is external."""
    try:
        fields={'schema','source_sha','run_id','run_attempt','tag','index_digest','platform_digest','config_digest',
                'immutable_image','source_inventory','request','content','runtime_qualified','deployment_performed'}
        if (not isinstance(receipt,dict) or set(receipt)!=fields or receipt['schema']!='voice-space-native-tool-publication-v1'
            or any(receipt[k]!=identity[k] for k in ('source_sha','run_id','run_attempt','tag'))
            or receipt['source_inventory']!=inventory or receipt['runtime_qualified'] is not False
            or receipt['deployment_performed'] is not False): raise ValueError()
        if any(not isinstance(receipt[k],str) or not re.fullmatch(r'sha256:[0-9a-f]{64}',receipt[k])
               for k in ('index_digest','platform_digest','config_digest')): raise ValueError()
        if receipt['immutable_image']!='ghcr.io/poryadok/voiceroot/space-native-installer@'+receipt['platform_digest']: raise ValueError()
        request=receipt['request']
        if (not isinstance(request,dict) or set(request)!={'schema','cycle','request_id'}
            or request['schema']!='voice-full-application-build-request-v1' or type(request['cycle']) is not int
            or request['cycle'] not in (1,2) or not isinstance(request['request_id'],str)
            or not re.fullmatch(r'[a-f0-9]{32}',request['request_id'])): raise ValueError()
        content=receipt['content']
        if (not isinstance(content,dict) or set(content)!={'executable_sha256','migrator_sha256','sql_inventory_sha256',
                                                           'embedded_values_observed','runtime_receipt_verified','help_exit_code','compiled_gate_probe','closure'}
            or any(not isinstance(content[k],str) or not re.fullmatch(r'[a-f0-9]{64}',content[k])
                   for k in ('executable_sha256','migrator_sha256','sql_inventory_sha256'))
            or content['sql_inventory_sha256']!=inventory['sql_inventory_sha256']
            or content['embedded_values_observed'] is not True or content['runtime_receipt_verified'] is not False
            or type(content['help_exit_code']) is not int or content['help_exit_code']!=0
            or content['compiled_gate_probe']!='native51-source-gates-reached-v1'
            or not isinstance(content['closure'],dict) or content['closure'].get('schema')!='voice-space-native-local-closure-v1'): raise ValueError()
        return copy.deepcopy(receipt)
    except Exception: raise ToolIdentityError('Space native publication receipt invalid') from None


def tool_job(jobs,source,run_id,attempt):
    try:
        rows=[job for job in jobs if job.get('name')=='Space native tool image']
        if len(rows)!=1: raise ValueError()
        job=rows[0]
        if (job.get('run_id')!=run_id or job.get('run_attempt')!=attempt or job.get('head_sha')!=source
            or type(job.get('id')) is not int or job.get('status')!='completed' or job.get('conclusion')!='success'): raise ValueError()
        for name in ('Verify Space native tool source','Capture Space native tool identity'):
            steps=[step for step in job.get('steps',[]) if step.get('name')==name]
            if len(steps)!=1 or steps[0].get('conclusion')!='success': raise ValueError()
        return copy.deepcopy(job)
    except Exception: raise ToolIdentityError('Space native canonical CI job invalid') from None


def _ci_context():
    """Authenticate this exact full-request master push and its checkout."""
    import canonical_application_package as canonical
    import publish_candidate as common
    import source_authority as authority
    token=os.environ.get('GH_TOKEN',''); actor=os.environ.get('GITHUB_ACTOR','')
    if not token or not actor: raise ToolIdentityError('Space native CI credentials unavailable')
    root=Path(__file__).resolve().parents[2]; headers=authority._headers(token); deadline=time.monotonic()+120
    run_id=os.environ.get('GITHUB_RUN_ID','')
    if not re.fullmatch(r'[1-9][0-9]{0,19}',run_id): raise ToolIdentityError('Space native CI run invalid')
    run=authority._get(authority.API+'/actions/runs/'+run_id,headers,deadline)
    identity=ci_identity(os.environ,run); sha=identity['source_sha']
    authority._head(headers,sha,deadline)
    snapshot=common.source_snapshot(root,sha); inventory=source_inventory(root,snapshot)
    raw=(root/canonical.REQUEST).read_bytes()
    if len(raw)>4096 or snapshot['files'].get(canonical.REQUEST)!=hashlib.sha256(raw).hexdigest(): raise ToolIdentityError('Space native request mismatch')
    commit=authority._get(authority.API+'/git/commits/'+sha,headers,deadline)
    parents=commit.get('parents')
    if commit.get('sha')!=sha or not isinstance(parents,list) or not parents: raise ToolIdentityError('Space native parent unavailable')
    old_oid=canonical.tree_blob(headers,parents[0]['sha'],canonical.REQUEST,deadline); previous=None
    if old_oid is not None:
        blob=authority._get(authority.API+'/git/blobs/'+old_oid,headers,deadline)
        if blob.get('sha')!=old_oid or blob.get('encoding')!='base64': raise ToolIdentityError('Space native prior request unavailable')
        previous=base64.b64decode(''.join(blob.get('content','').split()),validate=True)
        if len(previous)>4096 or canonical.blob_oid(previous)!=old_oid: raise ToolIdentityError('Space native prior request mismatch')
    request=canonical.validate_request(raw,previous)
    return root,identity,snapshot,inventory,request,common.Registry(actor,token)


def ci_preflight():
    root,identity,snapshot,inventory,request,registry=_ci_context()
    repository='poryadok/voiceroot/space-native-installer'; tag=identity['tag'].rsplit(':',1)[1]
    if registry.get(repository,'manifests/'+tag,absent=True) is not None: raise ToolIdentityError('Space native publication tag already exists')
    return True


def _run_help(reference):
    """Run the fixed no-network probe and remove its exact owned container."""
    import publish_candidate as common
    container=common.command(['docker','create','--network','none','--read-only','--cap-drop','ALL',
        '--security-opt','no-new-privileges','--pids-limit','64','--memory','128m','--cpus','1',reference,'--help']).decode().strip()
    if not re.fullmatch(r'[a-f0-9]{64}',container): raise ToolIdentityError('Space native help container invalid')
    try:
        result=subprocess.run(['docker','start','-a',container],stdin=subprocess.DEVNULL,stdout=subprocess.DEVNULL,
                              stderr=subprocess.DEVNULL,timeout=30)
        if result.returncode!=0: raise ToolIdentityError('Space native executable help failed')
    finally:
        common.command(['docker','rm','--force',container])


def _run_compiled_probe(reference,build_source_sha256):
    """Reach the source-owned gates with no network and no real database."""
    import publish_candidate as common
    import resource
    if not isinstance(build_source_sha256,str) or not re.fullmatch(r'[a-f0-9]{64}',build_source_sha256):
        raise ToolIdentityError('Space native probe source invalid')
    uri='postgres://fixture:fixture@127.0.0.1:1/postgres?sslmode=disable&connect_timeout=2'
    container=common.command(['docker','create','--network','none','--read-only','--cap-drop','ALL',
        '--security-opt','no-new-privileges','--pids-limit','64','--memory','128m','--cpus','1',
        '--env','SPACE_NATIVE_ADMIN_URL='+uri,'--env','SPACE_NATIVE_REQUIRED_SOURCE_SHA256='+build_source_sha256,
        reference,'--mode=provision']).decode().strip()
    if not re.fullmatch(r'[a-f0-9]{64}',container): raise ToolIdentityError('Space native probe container invalid')
    try:
        with tempfile.TemporaryFile() as stdout,tempfile.TemporaryFile() as stderr:
            def limit(): resource.setrlimit(resource.RLIMIT_FSIZE,(8192,8192))
            result=subprocess.run(['docker','start','-a',container],stdin=subprocess.DEVNULL,stdout=stdout,stderr=stderr,
                                  timeout=30,preexec_fn=limit)
            stdout.seek(0); stderr.seek(0)
            if (result.returncode!=1 or stdout.read(8193)!=b''
                or stderr.read(8193)!=b'administrative database unavailable\n'): raise ToolIdentityError('Space native compiled gates not reached')
    finally:
        common.command(['docker','rm','--force',container])
    return 'native51-source-gates-reached-v1'


def _extract_image(reference,config_digest,inventory,root):
    import publish_candidate as common
    common.command(['docker','pull','--platform','linux/amd64',reference])
    actual=common.command(['docker','image','inspect',reference,'--format','{{.Id}}']).decode().strip()
    common.validate_config_identity(config_digest,actual)
    container=common.command(['docker','create','--platform','linux/amd64','--network','none','--entrypoint','/bin/true',reference]).decode().strip()
    if not re.fullmatch(r'[a-f0-9]{64}',container): raise ToolIdentityError('Space native inspection container invalid')
    try:
        with tempfile.TemporaryDirectory(prefix='voice-native-tool-') as folder:
            dest=Path(folder)
            for name,path in (('native-migrate','/usr/local/bin/native-migrate'),('migrate','/usr/local/bin/migrate'),('migrations','/migrations'),('closure.json','/usr/local/share/voice-native-closure.json')):
                common.command(['docker','cp',container+':'+path,str(dest/name)])
            def read(path,limit):
                if path.is_symlink() or not path.is_file() or path.stat().st_nlink!=1 or not 0<path.stat().st_size<=limit: raise ValueError()
                return path.read_bytes()
            binary=read(dest/'native-migrate',100<<20); driver=read(dest/'migrate',100<<20)
            directory=dest/'migrations'
            if directory.is_symlink() or not directory.is_dir(): raise ValueError()
            entries=list(directory.iterdir())
            if len(entries)>512: raise ValueError()
            if any(p.is_symlink() or not p.is_file() for p in entries): raise ValueError()
            sql={p.name:read(p,2<<20) for p in entries if p.suffix=='.sql'}
            other={p.name:hashlib.sha256(read(p,2<<20)).hexdigest() for p in entries if p.suffix!='.sql'}
            if other!=inventory['other_files']: raise ValueError()
            proof=content_identity(inventory,binary,driver,sql)
            proof['closure']=normalize_closure(read(dest/'closure.json',2<<20),root,common.source_snapshot(root,inventory['source_sha']))
    finally:
        common.command(['docker','rm',container])
    _run_help(reference)
    proof['help_exit_code']=0
    proof['compiled_gate_probe']=_run_compiled_probe(reference,inventory['build_source_sha256'])
    return proof


def capture_ci(index_digest,output):
    """Capture actual immutable tool bytes from the protected CI build."""
    import publish_candidate as common
    if not isinstance(index_digest,str) or not re.fullmatch(r'sha256:[a-f0-9]{64}',index_digest): raise ToolIdentityError('Space native image digest invalid')
    root,identity,snapshot,inventory,request,registry=_ci_context()
    repository='poryadok/voiceroot/space-native-installer'; tag=identity['tag'].rsplit(':',1)[1]
    index_raw=registry.get(repository,'manifests/'+index_digest)
    index=common.decode_manifest(index_raw,index_digest); platform=common.platform_manifest(index)
    manifest_raw=registry.get(repository,'manifests/'+platform)
    manifest=common.decode_manifest(manifest_raw,platform); config_raw=registry.get(repository,'blobs/'+manifest['config']['digest'])
    if registry.get(repository,'manifests/'+tag)!=index_raw: raise ToolIdentityError('Space native publication tag changed')
    oci=oci_identity(identity['source_sha'],index_digest,index_raw,manifest_raw,config_raw)
    proof=_extract_image(oci['immutable_image'],oci['config_digest'],inventory,root)
    root2,identity2,snapshot2,inventory2,request2,registry2=_ci_context()
    if (identity2!=identity or snapshot2!=snapshot or inventory2!=inventory or request2!=request
        or registry2.get(repository,'manifests/'+tag)!=index_raw): raise ToolIdentityError('Space native build authority changed')
    receipt={'schema':'voice-space-native-tool-publication-v1',**identity,**oci,'source_inventory':inventory,
             'request':request,'content':proof,'runtime_qualified':False,'deployment_performed':False}
    validate_publication(receipt,identity,inventory)
    output=Path(output); temporary=Path(os.environ.get('RUNNER_TEMP',''))
    if (not output.is_absolute() or not temporary.is_absolute() or output.parent!=temporary
        or output.exists() or output.is_symlink() or temporary.is_symlink()): raise ToolIdentityError('Space native output invalid')
    raw=(json.dumps(receipt,sort_keys=True,separators=(',',':'))+'\n').encode()
    fd=os.open(output,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
    with os.fdopen(fd,'wb') as stream: stream.write(raw); stream.flush(); os.fsync(stream.fileno())
    return receipt


def authenticated_publication(token,actor,package_receipt,root):
    """Read the exact successful canonical CI tool artifact and live OCI metadata."""
    import canonical_application_package as canonical
    import publish_candidate as common
    import source_authority as authority
    import space_native_tool_identity_custody as custody
    try:
        if (not isinstance(token,str) or not token or not isinstance(actor,str) or not actor
            or Path(root).resolve()!=canonical.ROOT.resolve()): raise ValueError()
        source=package_receipt['source_sha']; run_id=package_receipt['run_id']; attempt=package_receipt['run_attempt']
        if (not re.fullmatch(r'[a-f0-9]{40}',source) or type(run_id) is not int or run_id<=0
            or type(attempt) is not int or attempt<=0): raise ValueError()
        snapshot=common.source_snapshot(Path(root),source); inventory=source_inventory(root,snapshot)
        headers=authority._headers(token); deadline=time.monotonic()+120
        repo=authority._head(headers,source,deadline)
        run=authority._get(authority.API+'/actions/runs/'+str(run_id),headers,deadline)
        canonical.validate_run(run,source,run_id,attempt)
        jobs=authority._pages(authority.API+'/actions/runs/'+str(run_id)+'/attempts/'+str(attempt)+'/jobs','jobs',headers,deadline)
        canonical.validate_jobs(jobs,source,run_id,attempt); job=tool_job(jobs,source,run_id,attempt)
        name='space-native-tool-'+str(run_id)+'-'+str(attempt)
        artifacts=authority._pages(authority.API+'/actions/runs/'+str(run_id)+'/artifacts','artifacts',headers,deadline)
        candidates=[row for row in artifacts if row.get('name')==name]
        if len(candidates)!=1 or type(candidates[0].get('id')) is not int: raise ValueError()
        artifact_id=candidates[0]['id']; artifact=authority._get(authority.API+'/actions/artifacts/'+str(artifact_id),headers,deadline)
        workflow=artifact['workflow_run']; t=authority.transport
        if (artifact.get('id')!=artifact_id or artifact.get('name')!=name or artifact.get('expired') is not False
            or workflow.get('id')!=run_id or workflow.get('head_sha')!=source or workflow.get('repository_id')!=repo['id']
            or workflow.get('head_repository_id')!=repo['id']
            or not t._timestamp(job['started_at'])<=t._timestamp(artifact['created_at'])<=t._timestamp(job['completed_at'])
            or t._timestamp(artifact['expires_at'])<=t.datetime.now(t.timezone.utc)): raise ValueError()
        with tempfile.TemporaryFile() as archive:
            count,_=authority._fetch(authority.API+'/actions/artifacts/'+str(artifact_id)+'/zip',headers,4<<20,deadline,target=archive)
            if type(artifact.get('size_in_bytes')) is not int or count!=artifact['size_in_bytes']: raise ValueError()
            archive.seek(0)
            with zipfile.ZipFile(archive) as zipped:
                members=zipped.infolist()
                if (len(members)!=1 or members[0].filename!='space-native-tool.json' or members[0].is_dir()
                    or members[0].file_size>1<<20 or stat.S_IFMT(members[0].external_attr>>16) not in (0,stat.S_IFREG)
                    or members[0].flag_bits&1): raise ValueError()
                raw=zipped.read(members[0])
        def unique(pairs):
            result={}
            for key,value in pairs:
                if key in result: raise ValueError()
                result[key]=value
            return result
        publication=json.loads(raw,object_pairs_hook=unique)
        identity={'source_sha':source,'run_id':run_id,'run_attempt':attempt,
                  'tag':'ghcr.io/poryadok/voiceroot/space-native-installer:source-'+source+'-'+str(run_id)+'-'+str(attempt)}
        validate_publication(publication,identity,inventory)
        custody._equivalent(publication['content']['closure'],snapshot)
        request=canonical.parse_request((Path(root)/canonical.REQUEST).read_bytes())
        if publication['request']!=request: raise ValueError()
        registry=common.Registry(actor,token); repository='poryadok/voiceroot/space-native-installer'
        index=registry.get(repository,'manifests/'+publication['index_digest'])
        manifest=registry.get(repository,'manifests/'+publication['platform_digest'])
        config=registry.get(repository,'blobs/'+publication['config_digest'])
        if registry.get(repository,'manifests/'+identity['tag'].rsplit(':',1)[1])!=index: raise ValueError()
        oci=oci_identity(source,publication['index_digest'],index,manifest,config)
        if any(publication[k]!=oci[k] for k in ('index_digest','platform_digest','config_digest','immutable_image')): raise ValueError()
        authority._head(headers,source,deadline)
        again=authority._get(authority.API+'/actions/runs/'+str(run_id),headers,deadline)
        canonical.validate_run(again,source,run_id,attempt)
        if common.source_snapshot(Path(root),source)!=snapshot: raise ValueError()
        return {'artifact_id':artifact_id,'artifact_sha256':hashlib.sha256(raw).hexdigest(),
                'publication':copy.deepcopy(publication)}
    except Exception: raise ToolIdentityError('Space native authenticated publication unavailable') from None


def main():
    parser=argparse.ArgumentParser(description=__doc__); sub=parser.add_subparsers(dest='command',required=True)
    sub.add_parser('ci-preflight')
    capture=sub.add_parser('capture-ci'); capture.add_argument('--index-digest',required=True); capture.add_argument('--output',required=True)
    args=parser.parse_args()
    try:
        if args.command=='ci-preflight': ci_preflight()
        else: capture_ci(args.index_digest,args.output)
    except Exception:
        print('Space native tool qualification refused')
        return 1
    print('Space native tool CI evidence captured; runtime qualification remains separate')
    return 0


def _unique_json(pairs):
    result={}
    for key,value in pairs:
        if key in result: raise ValueError()
        result[key]=value
    return result

def normalize_closure(raw,root,snapshot):
    """Bind builder-produced Go dependency metadata to authenticated source bytes."""
    try:
        if not isinstance(raw,bytes) or not 0<len(raw)<=2<<20: raise ValueError()
        text=raw.decode('utf8'); decoder=json.JSONDecoder(object_pairs_hook=_unique_json); pos=0; packages={}; files={}
        root=Path(root); tracked=snapshot['files']
        def relative(value):
            if not isinstance(value,str) or not value.startswith('/src/'): raise ValueError()
            parts=value.removeprefix('/src/').split('/')
            if not parts or any(not p or p in ('.','..') for p in parts): raise ValueError()
            return 'src/backend/'+'/'.join(parts)
        def add(name):
            p=root/name
            if (name not in tracked or p.is_symlink() or not p.is_file() or p.stat().st_nlink!=1 or
                not p.resolve().is_relative_to(root.resolve()) or hashlib.sha256(p.read_bytes()).hexdigest()!=tracked[name]): raise ValueError()
            files[name]=tracked[name]
        while True:
            pos=len(text)-len(text[pos:].lstrip())
            if pos==len(text): break
            entry,pos=decoder.raw_decode(text,pos)
            if not isinstance(entry,dict) or entry.get('Error') or entry.get('Incomplete'): raise ValueError()
            directory=entry.get('Dir','')
            if not isinstance(directory,str): raise ValueError()
            if not directory.startswith('/src/'): continue
            directory=relative(directory)
            if directory in packages: raise ValueError()
            live=root/directory
            if live.is_symlink() or not live.is_dir(): raise ValueError()
            extensions={'.go','.s','.S','.c','.cc','.cpp','.cxx','.h','.hh','.hpp','.hxx','.f','.F','.m','.mm','.syso','.swig','.swigcxx'}
            actual=sorted(p.name for p in live.iterdir() if p.suffix in extensions and not p.name.endswith('_test.go'))
            expected=sorted(name.removeprefix(directory+'/') for name in tracked if name.startswith(directory+'/')
                            and '/' not in name.removeprefix(directory+'/') and Path(name).suffix in extensions and not name.endswith('_test.go'))
            if actual!=expected or not actual: raise ValueError()
            listed=entry.get('GoFiles',[])+entry.get('CgoFiles',[])
            if not isinstance(listed,list) or not listed or any(name not in actual for name in listed): raise ValueError()
            members=[directory+'/'+name for name in actual]
            for name in members: add(name)
            packages[directory]=members
            patterns=entry.get('EmbedPatterns',[]); embedded=entry.get('EmbedFiles',[])
            if (not isinstance(patterns,list) or not isinstance(embedded,list) or sorted(patterns)!=sorted(embedded) or
                any(not isinstance(name,str) or name.startswith('all:') or any(c in name for c in '*?[]')
                    or name.startswith('/') or any(p in ('','.','..') for p in name.split('/')) for name in patterns)): raise ValueError()
            for name in embedded: add(directory+'/'+name)
            module=entry.get('Module')
            if not isinstance(module,dict): raise ValueError()
            module=module.get('Replace') or module
            module_dir=relative(module['Dir'])
            for name in ('go.mod','go.sum'):
                path=module_dir+'/'+name
                if path in tracked: add(path)
                elif name=='go.mod': raise ValueError()
        if not packages or 'src/backend/space/cmd/native-migrate' not in packages: raise ValueError()
        workspace=[name for name in ('src/backend/go.work','src/backend/go.work.sum') if name in tracked]
        for name in workspace: add(name)
        return {'schema':'voice-space-native-local-closure-v1','packages':dict(sorted(packages.items())),
                'files':dict(sorted(files.items())),'workspace':workspace,'raw_sha256':hashlib.sha256(raw).hexdigest()}
    except Exception: raise ToolIdentityError('Space native closure invalid') from None


def source_inventory(root,snapshot):
    """Cross-check the fixed installer inputs against a verified source snapshot."""
    try:
        root=Path(root); files=snapshot['files']; source=snapshot['source_sha']
        if (not isinstance(source,str) or not re.fullmatch(r'[0-9a-f]{40}',source) or source=='0'*40
            or not isinstance(files,dict)): raise ValueError()
        def read(relative):
            path=root/relative
            if path.is_symlink() or not path.is_file() or path.stat().st_nlink!=1 or not path.resolve().is_relative_to(root.resolve()): raise ValueError()
            raw=path.read_bytes()
            if hashlib.sha256(raw).hexdigest()!=files.get(relative): raise ValueError()
            return raw
        backend=root/'src/backend'
        gos={p.relative_to(root).as_posix() for p in backend.rglob('*.go') if not p.name.endswith('_test.go')}
        expected_gos={p for p in files if p.startswith('src/backend/') and p.endswith('.go') and not p.endswith('_test.go')}
        if not gos or gos!=expected_gos: raise ValueError()
        for name in gos: read(name)
        if any(files.get(name)!=digest for name,digest in GATE_SOURCE_SHA256.items()): raise ValueError()
        sql_dir=root/'src/backend/migrations/space_db'
        entries=list(sql_dir.iterdir())
        if any(p.is_symlink() or not p.is_file() for p in entries): raise ValueError()
        all_paths={p.relative_to(root).as_posix() for p in entries}
        expected_all={p for p in files if p.startswith('src/backend/migrations/space_db/')}
        if all_paths!=expected_all: raise ValueError()
        sql_paths={p for p in all_paths if p.endswith('.sql')}
        expected_sql={p for p in expected_all if p.endswith('.sql')}
        if not sql_paths or sql_paths!=expected_sql: raise ValueError()
        sql={Path(path).name:hashlib.sha256(read(path)).hexdigest() for path in sql_paths}
        other={Path(path).name:hashlib.sha256(read(path)).hexdigest() for path in all_paths-sql_paths}
        if len(sql)!=len(sql_paths): raise ValueError()
        manifest_path='src/backend/space/internal/nativeinstaller/source_manifest.go'
        manifest=read(manifest_path).decode('utf-8')
        found=re.findall(r'^\s*"([^"/]+\.sql)":\s*"([a-f0-9]{64})",\s*$',manifest,re.M)
        inventory=re.findall(r'^const InventorySHA256 = "([a-f0-9]{64})"$',manifest,re.M)
        if len(inventory)!=1 or len(found)!=len(dict(found)) or dict(found)!=sql: raise ValueError()
        dockerfile='src/backend/space/Dockerfile.native-migrate'; read(dockerfile)
        return {'source_sha':source,'build_source_sha256':migrations.space_source_digest(root),
                'sql_inventory_sha256':inventory[0],'dockerfile_sha256':files[dockerfile],
                'source_manifest_sha256':files[manifest_path],'gate_source_sha256':dict(GATE_SOURCE_SHA256),
                'sql_files':dict(sorted(sql.items())),'other_files':dict(sorted(other.items()))}
    except Exception: raise ToolIdentityError('Space native source inventory not qualified') from None


class SpaceNativeToolPhase:
    """One-shot admission of the same canonical package run's native tool."""
    STEP='space-native-tool'

    def __init__(self,journal,fence,client,lock,namespace_uid,package,actor):
        import frontend_runtime_executor
        import full_cohort_package_phase
        import media_operation_journal
        import resource_fencing
        if (type(journal) is not media_operation_journal.Journal or type(fence) is not resource_fencing.Fence
            or type(client) is not frontend_runtime_executor.KubectlTransport or client.journal is not journal
            or type(fence.ledger) is not resource_fencing.KubeLedger or getattr(fence.ledger,'client',None) is not client
            or type(package) is not full_cohort_package_phase.PackagePhase
            or type(package.fence.ledger) is not resource_fencing.KubeLedger or package.fence.ledger.client is not package.client
            or type(package.journal) is not media_operation_journal.Journal or package.client.journal is not package.journal
            or package.journal is journal
            or not isinstance(actor,str) or not actor or not isinstance(namespace_uid,str) or not namespace_uid
            or journal.intent.get('resource_context')!=fence.context or journal.intent.get('namespace_uid')!=namespace_uid
            or not re.fullmatch(r'media-c1-[a-f0-9]{32}',fence.context.get('operation_id',''))):
            raise ToolIdentityError('Space native phase identity invalid')
        self.journal,self.fence,self.client=journal,fence,client
        self.lock,self.namespace_uid=copy.deepcopy(lock),namespace_uid
        self.package,self.actor=package,actor; self._package_ref,self._actor=package,actor
        self._journal_ref,self._package_journal_ref=journal,package.journal
        self._fence_ref,self._client_ref,self._ledger_ref=fence,client,fence.ledger
        self._package_fence_ref,self._package_client_ref,self._package_ledger_ref=package.fence,package.client,package.fence.ledger
        self._journal_custody=self._custody(journal); self._package_journal_custody=self._custody(package.journal)
        self.intent=copy.deepcopy(journal.intent); self.poisoned=False

    @staticmethod
    def _custody(journal):
        import media_operation_journal
        if type(journal) is not media_operation_journal.Journal or type(journal.dirfd) is not int or type(journal.lockfd) is not int:
            raise ToolIdentityError('Space native journal custody unavailable')
        directory=os.fstat(journal.dirfd); lock=os.fstat(journal.lockfd)
        media_operation_journal.Journal._private(directory,0o700,directory=True)
        media_operation_journal.Journal._private(lock,0o600)
        path_directory=os.stat(journal.path,follow_symlinks=False); path_lock=os.stat(journal.path/'lock',follow_symlinks=False)
        if ((directory.st_dev,directory.st_ino)!=(path_directory.st_dev,path_directory.st_ino)
            or (lock.st_dev,lock.st_ino)!=(path_lock.st_dev,path_lock.st_ino)): raise ToolIdentityError('Space native journal replaced')
        return (journal.dirfd,journal.lockfd,directory.st_dev,directory.st_ino,lock.st_dev,lock.st_ino)

    def _guard(self):
        import media_operation_guard
        if (self.poisoned or self.journal is not self._journal_ref or self.package is not self._package_ref
            or self.package.journal is not self._package_journal_ref or self.journal.poisoned or self.actor!=self._actor
            or self.fence is not self._fence_ref or self.client is not self._client_ref or self.fence.ledger is not self._ledger_ref
            or self.package.fence is not self._package_fence_ref or self.package.client is not self._package_client_ref
            or self.package.fence.ledger is not self._package_ledger_ref
            or self._ledger_ref.client is not self.client or self._package_ledger_ref.client is not self.package.client
            or self.client.journal is not self.journal or self.package.client.journal is not self.package.journal
            or self.journal.intent!=self.intent
            or self.journal.state.get('intent')!=self.intent): raise ToolIdentityError('Space native phase identity changed')
        if (self._custody(self.journal)!=self._journal_custody or self._custody(self.package.journal)!=self._package_journal_custody):
            raise ToolIdentityError('Space native journal custody changed')
        media_operation_guard.verify(self.fence,self.client,self.lock,self.namespace_uid)

    def _package(self,token):
        self._guard(); package=self.package
        if (package.fence.context!=self.fence.context or package.namespace_uid!=self.namespace_uid or package.lock!=self.lock):
            raise ToolIdentityError('Space native package authority differs')
        first,rev=self.fence.ledger.read(); other,other_rev=package.fence.ledger.read()
        if first!=other or rev!=other_rev or not isinstance(rev,tuple) or len(rev)!=2 or not all(rev):
            raise ToolIdentityError('Space native package ledger differs')
        receipt=package.verify_current(token)
        after,after_rev=self.fence.ledger.read(); other_after,other_after_rev=package.fence.ledger.read()
        if (after!=first or other_after!=first or after_rev!=rev or other_after_rev!=rev
            or receipt.get('operation_id')!=self.fence.context['operation_id']
            or receipt.get('source_sha')!=self.fence.context['commit'] or receipt.get('images')!=self.fence.context['images']):
            raise ToolIdentityError('Space native package authority changed')
        self._guard(); return receipt

    def _receipt(self,package,artifact):
        pub=artifact['publication']; inventory=pub['source_inventory']; content=pub['content']
        identity={'source_sha':package['source_sha'],'run_id':package['run_id'],'run_attempt':package['run_attempt'],
                  'tag':'ghcr.io/poryadok/voiceroot/space-native-installer:source-'+package['source_sha']+'-'+str(package['run_id'])+'-'+str(package['run_attempt'])}
        validate_publication(pub,identity,inventory)
        if (inventory.get('source_sha')!=package['source_sha'] or type(artifact.get('artifact_id')) is not int or artifact['artifact_id']<=0
            or not isinstance(artifact.get('artifact_sha256'),str) or not re.fullmatch(r'[a-f0-9]{64}',artifact['artifact_sha256'])
            or pub['request']['cycle']!=1): raise ToolIdentityError('Space native artifact identity invalid')
        return {'operation_id':self.fence.context['operation_id'],'namespace_uid':self.namespace_uid,
                'source_sha':package['source_sha'],'run_id':package['run_id'],'run_attempt':package['run_attempt'],
                'package_sha256':package['package_sha256'],'artifact_id':artifact['artifact_id'],'artifact_sha256':artifact['artifact_sha256'],
                'index_digest':pub['index_digest'],'platform_digest':pub['platform_digest'],'config_digest':pub['config_digest'],
                'immutable_image':pub['immutable_image'],'build_source_sha256':inventory['build_source_sha256'],
                'sql_inventory_sha256':inventory['sql_inventory_sha256'],'dockerfile_sha256':inventory['dockerfile_sha256'],
                'source_manifest_sha256':inventory['source_manifest_sha256'],'executable_sha256':content['executable_sha256'],
                'migrator_sha256':content['migrator_sha256'],'compiled_gate_probe':content['compiled_gate_probe'],
                'closure_sha256':hashlib.sha256(json.dumps(content['closure'],sort_keys=True,separators=(',',':')).encode()).hexdigest(),
                'nodes':copy.deepcopy(package['nodes']),'runtime_receipt_verified':False}

    def run(self,token):
        import canonical_application_package as canonical
        pending=False
        try:
            package=self._package(token); self._guard(); row,revision=self.fence.ledger.read()
            if (not self.fence.owned(row) or row.get('status')!='active' or self.STEP in self.journal.state['steps']
                or 'space_native_tool' in row): raise ValueError()
            marker={'status':'pending','nonce':os.urandom(16).hex(),'operation_id':self.fence.context['operation_id'],
                    'source_sha':package['source_sha'],'run_id':package['run_id'],'run_attempt':package['run_attempt'],
                    'package_sha256':package['package_sha256']}
            pending=True; self.journal.state['steps'][self.STEP]=copy.deepcopy(marker); self.journal.save()
            self._guard(); current,rv=self.fence.ledger.read()
            if current!=row or rv!=revision: raise ValueError()
            current['space_native_tool']=copy.deepcopy(marker); current['step']=self.STEP+'-pending'
            self.fence.ledger.write(rv,current); self._guard()
            current,rv=self.fence.ledger.read()
            if current.get('space_native_tool')!=marker or self.journal.state['steps'].get(self.STEP)!=marker: raise ValueError()
            artifact=authenticated_publication(token,self.actor,package,canonical.ROOT)
            receipt=self._receipt(package,artifact)
            self._guard(); current,rv=self.fence.ledger.read()
            if (current.get('space_native_tool')!=marker or self.journal.state['steps'].get(self.STEP)!=marker
                or self._package(token)!=package): raise ValueError()
            digest=hashlib.sha256(json.dumps(receipt,sort_keys=True,separators=(',',':')).encode()).hexdigest()
            done=marker|{'status':'done','receipt_sha256':digest}
            self.journal.state['steps'][self.STEP]=copy.deepcopy(done)|{'receipt':copy.deepcopy(receipt)}; self.journal.save()
            current['space_native_tool']=copy.deepcopy(done); current['step']=self.STEP+'-done'
            self.fence.ledger.write(rv,current); self._guard()
            if self._package(token)!=package: raise ValueError()
            check,_=self.fence.ledger.read()
            if check.get('space_native_tool')!=done or self.journal.state['steps'].get(self.STEP)!=done|{'receipt':receipt}: raise ValueError()
            return copy.deepcopy(receipt)
        except BaseException:
            if pending:
                self.poisoned=True
                try: self.fence.uncertain('Space native tool admission uncertain')
                except BaseException: pass
            raise ToolIdentityError('Space native tool admission uncertain' if pending else 'Space native tool admission refused') from None


def verify_completed(phase,token):
    """Reauthenticate the immutable publication and protected terminal receipt."""
    import canonical_application_package as canonical
    try:
        if type(phase) is not SpaceNativeToolPhase: raise ValueError()
        phase._guard(); local=copy.deepcopy(phase.journal.state['steps'].get(phase.STEP))
        if not isinstance(local,dict) or not isinstance(local.get('receipt'),dict): raise ValueError()
        receipt=local['receipt']; done={k:v for k,v in local.items() if k!='receipt'}
        row,revision=phase.fence.ledger.read()
        if (done.get('status')!='done' or row.get('space_native_tool')!=done
            or done.get('receipt_sha256')!=hashlib.sha256(json.dumps(receipt,sort_keys=True,separators=(',',':')).encode()).hexdigest()): raise ValueError()
        package=phase._package(token)
        artifact=authenticated_publication(token,phase.actor,package,canonical.ROOT)
        if phase._receipt(package,artifact)!=receipt: raise ValueError()
        if phase._package(token)!=package: raise ValueError()
        phase._guard(); final,final_revision=phase.fence.ledger.read()
        if final!=row or final_revision!=revision or phase.journal.state['steps'].get(phase.STEP)!=local: raise ValueError()
        return copy.deepcopy(receipt)
    except BaseException: raise ToolIdentityError('Space native completed authority unavailable') from None


if __name__=='__main__': raise SystemExit(main())
