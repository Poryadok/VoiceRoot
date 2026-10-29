"""Source-bound Space native installer identity; no deployment or DB access.

The source inventory helper validates bytes but does not authenticate its input.
Production admission must obtain source from the protected canonical package.
"""
import argparse
import base64
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time

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
    result=subprocess.run(['docker','run','--rm','--network','none','--read-only','--cap-drop','ALL',
        '--security-opt','no-new-privileges','--pids-limit','64','--memory','128m','--cpus','1',reference,'--help'],
        stdin=subprocess.DEVNULL,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,timeout=30)
    if result.returncode!=0: raise ToolIdentityError('Space native executable help failed')
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


if __name__=='__main__': raise SystemExit(main())


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
