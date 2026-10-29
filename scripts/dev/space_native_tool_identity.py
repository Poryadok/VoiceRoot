"""Source-bound Space native installer identity; no deployment or DB access.

The source inventory helper validates bytes but does not authenticate its input.
Production admission must obtain source from the protected canonical package.
"""
import argparse
import base64
import copy
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import stat
import tempfile
import time
import zipfile

import migrations


class ToolIdentityError(RuntimeError): pass


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
                                                           'embedded_values_observed','runtime_receipt_verified','help_exit_code'}
            or any(not isinstance(content[k],str) or not re.fullmatch(r'[a-f0-9]{64}',content[k])
                   for k in ('executable_sha256','migrator_sha256','sql_inventory_sha256'))
            or content['sql_inventory_sha256']!=inventory['sql_inventory_sha256']
            or content['embedded_values_observed'] is not True or content['runtime_receipt_verified'] is not False
            or type(content['help_exit_code']) is not int or content['help_exit_code']!=0): raise ValueError()
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
            for name,path in (('native-migrate','/usr/local/bin/native-migrate'),('migrate','/usr/local/bin/migrate'),('migrations','/migrations')):
                common.command(['docker','cp',container+':'+path,str(dest/name)])
            def read(path,limit):
                if path.is_symlink() or not path.is_file() or not 0<path.stat().st_size<=limit: raise ValueError()
                return path.read_bytes()
            binary=read(dest/'native-migrate',100<<20); driver=read(dest/'migrate',100<<20)
            directory=dest/'migrations'
            if directory.is_symlink() or not directory.is_dir(): raise ValueError()
            entries=list(directory.iterdir())
            if len(entries)>512: raise ValueError()
            if any(p.is_symlink() or not p.is_file() or p.suffix!='.sql' for p in entries): raise ValueError()
            sql={p.name:read(p,2<<20) for p in entries}
            proof=content_identity(inventory,binary,driver,sql)
    finally:
        common.command(['docker','rm',container])
    _run_help(reference)
    proof['help_exit_code']=0
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


def source_inventory(root,snapshot):
    """Cross-check the fixed installer inputs against a verified source snapshot."""
    try:
        root=Path(root); files=snapshot['files']; source=snapshot['source_sha']
        if (not isinstance(source,str) or not re.fullmatch(r'[0-9a-f]{40}',source) or source=='0'*40
            or not isinstance(files,dict)): raise ValueError()
        def read(relative):
            path=root/relative
            if path.is_symlink() or not path.is_file() or not path.resolve().is_relative_to(root.resolve()): raise ValueError()
            raw=path.read_bytes()
            if hashlib.sha256(raw).hexdigest()!=files.get(relative): raise ValueError()
            return raw
        backend=root/'src/backend'
        gos={p.relative_to(root).as_posix() for p in backend.rglob('*.go') if not p.name.endswith('_test.go')}
        expected_gos={p for p in files if p.startswith('src/backend/') and p.endswith('.go') and not p.endswith('_test.go')}
        if not gos or gos!=expected_gos: raise ValueError()
        for name in gos: read(name)
        sql_dir=root/'src/backend/migrations/space_db'
        sql_paths={p.relative_to(root).as_posix() for p in sql_dir.glob('*.sql')}
        expected_sql={p for p in files if p.startswith('src/backend/migrations/space_db/') and p.endswith('.sql')}
        if not sql_paths or sql_paths!=expected_sql: raise ValueError()
        sql={Path(path).name:hashlib.sha256(read(path)).hexdigest() for path in sql_paths}
        if len(sql)!=len(sql_paths): raise ValueError()
        manifest_path='src/backend/space/internal/nativeinstaller/source_manifest.go'
        manifest=read(manifest_path).decode('utf-8')
        found=re.findall(r'^\s*"([^"/]+\.sql)":\s*"([a-f0-9]{64})",\s*$',manifest,re.M)
        inventory=re.findall(r'^const InventorySHA256 = "([a-f0-9]{64})"$',manifest,re.M)
        if len(inventory)!=1 or len(found)!=len(dict(found)) or dict(found)!=sql: raise ValueError()
        dockerfile='src/backend/space/Dockerfile.native-migrate'; read(dockerfile)
        return {'source_sha':source,'build_source_sha256':migrations.space_source_digest(root),
                'sql_inventory_sha256':inventory[0],'dockerfile_sha256':files[dockerfile],
                'source_manifest_sha256':files[manifest_path],'sql_files':dict(sorted(sql.items()))}
    except Exception: raise ToolIdentityError('Space native source inventory not qualified') from None


if __name__=='__main__': raise SystemExit(main())
