"""Authenticated Space tool publication from a protected STAGING source capture.

This byte/provenance reader does not establish authority for its inputs. The
source-owned STAGING Session must authenticate the approved capture and guard
the fixed source directory and operation before and after calling it.
"""
import copy
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import stat
import tempfile
import time
import zipfile

import canonical_application_package as canonical
import publish_candidate as common
import source_authority as authority
import space_native_tool_identity as tool
import space_native_tool_identity_custody as custody

class StagingToolError(RuntimeError): pass

def _source(root,approved):
    root=Path(root)
    files=approved['source_files']; sha=approved['source_sha']
    if (not root.is_absolute() or root.is_symlink() or root.resolve()!=root or not root.is_dir() or
        not re.fullmatch(r'[a-f0-9]{40}',sha) or not isinstance(files,dict) or not files or len(files)>100000): raise ValueError()
    for name,digest in files.items():
        path=PurePosixPath(name) if isinstance(name,str) else None
        if (path is None or path.is_absolute() or str(path)!=name or any(p in ('','.','..') for p in path.parts) or
            not isinstance(digest,str) or not re.fullmatch(r'[a-f0-9]{64}',digest)): raise ValueError()
    observed={}; total=0
    for current,dirs,names in os.walk(root,followlinks=False):
        for name in dirs:
            p=Path(current)/name
            if p.is_symlink() or not p.is_dir(): raise ValueError()
        for name in names:
            path=Path(current)/name; relative=path.relative_to(root).as_posix()
            if relative not in files or relative in observed: raise ValueError()
            fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
            try:
                before=os.fstat(fd)
                if not stat.S_ISREG(before.st_mode) or before.st_nlink!=1 or before.st_size>512*1024**2: raise ValueError()
                total+=before.st_size
                if total>4*1024**3: raise ValueError()
                h=hashlib.sha256(); remaining=before.st_size
                while remaining:
                    chunk=os.read(fd,min(1048576,remaining))
                    if not chunk: raise ValueError()
                    h.update(chunk); remaining-=len(chunk)
                def identity(s): return (s.st_dev,s.st_ino,s.st_size,s.st_mtime_ns,s.st_ctime_ns,s.st_mode,s.st_nlink)
                if identity(before)!=identity(os.fstat(fd)) or identity(before)!=identity(os.stat(path,follow_symlinks=False)): raise ValueError()
                observed[relative]=h.hexdigest()
            finally: os.close(fd)
    if observed!=files: raise ValueError()
    return {'source_sha':sha,'files':dict(sorted(observed.items()))}

def authenticated_staging_publication(token,actor,approved,root):
    """Recheck exact source, successful same-run artifact and registry identity."""
    try:
        if (not isinstance(token,str) or not token or not isinstance(actor,str) or not actor or
            not isinstance(approved,dict) or approved.get('schema')!='voice-source-authority-v1' or
            approved.get('repository')!='Poryadok/VoiceRoot' or approved.get('verified') is not True or
            approved.get('provisional_staging') is True): raise ValueError()
        approved=copy.deepcopy(approved); source=approved['source_sha']; run_id=approved['run_id']; attempt=approved['run_attempt']
        if type(run_id) is not int or run_id<=0 or type(attempt) is not int or attempt<=0: raise ValueError()
        snapshot=_source(root,approved); inventory=tool.source_inventory(root,snapshot)
        headers=authority._headers(token); deadline=time.monotonic()+120
        master=approved.get('source_equivalence',{}).get('current_master_sha_after',source)
        if not isinstance(master,str) or not re.fullmatch(r'[a-f0-9]{40}',master): raise ValueError()
        repo=authority._head(headers,master,deadline)
        run=authority._get(authority.API+'/actions/runs/'+str(run_id),headers,deadline)
        canonical.validate_run(run,source,run_id,attempt)
        jobs=authority._pages(authority.API+'/actions/runs/'+str(run_id)+'/attempts/'+str(attempt)+'/jobs','jobs',headers,deadline)
        canonical.validate_jobs(jobs,source,run_id,attempt); job=tool.tool_job(jobs,source,run_id,attempt)
        name='space-native-tool-'+str(run_id)+'-'+str(attempt)
        artifacts=authority._pages(authority.API+'/actions/runs/'+str(run_id)+'/artifacts','artifacts',headers,deadline)
        matches=[item for item in artifacts if item.get('name')==name]
        if len(matches)!=1 or type(matches[0].get('id')) is not int: raise ValueError()
        artifact_id=matches[0]['id']; artifact=authority._get(authority.API+'/actions/artifacts/'+str(artifact_id),headers,deadline)
        workflow=artifact['workflow_run']; t=authority.transport
        if (artifact.get('id')!=artifact_id or artifact.get('name')!=name or artifact.get('expired') is not False or
            workflow.get('id')!=run_id or workflow.get('head_sha')!=source or workflow.get('repository_id')!=repo['id'] or
            workflow.get('head_repository_id')!=repo['id'] or
            not t._timestamp(job['started_at'])<=t._timestamp(artifact['created_at'])<=t._timestamp(job['completed_at']) or
            t._timestamp(artifact['expires_at'])<=t.datetime.now(t.timezone.utc)): raise ValueError()
        with tempfile.TemporaryFile() as archive:
            count,_=authority._fetch(authority.API+'/actions/artifacts/'+str(artifact_id)+'/zip',headers,4<<20,deadline,target=archive)
            if type(artifact.get('size_in_bytes')) is not int or count!=artifact['size_in_bytes']: raise ValueError()
            archive.seek(0)
            with zipfile.ZipFile(archive) as zipped:
                members=zipped.infolist()
                if (len(members)!=1 or members[0].filename!='space-native-tool.json' or members[0].is_dir() or
                    members[0].file_size>1<<20 or stat.S_IFMT(members[0].external_attr>>16) not in (0,stat.S_IFREG) or
                    members[0].flag_bits&1): raise ValueError()
                raw=zipped.read(members[0])
        publication=json.loads(raw,object_pairs_hook=tool._unique_json)
        identity={'source_sha':source,'run_id':run_id,'run_attempt':attempt,
                  'tag':'ghcr.io/poryadok/voiceroot/space-native-installer:source-'+source+'-'+str(run_id)+'-'+str(attempt)}
        tool.validate_publication(publication,identity,inventory); custody._equivalent(publication['content']['closure'],snapshot)
        if publication['request']!=canonical.parse_request((Path(root)/canonical.REQUEST).read_bytes()): raise ValueError()
        registry=common.Registry(actor,token); repository='poryadok/voiceroot/space-native-installer'
        index=registry.get(repository,'manifests/'+publication['index_digest'])
        manifest=registry.get(repository,'manifests/'+publication['platform_digest'])
        config=registry.get(repository,'blobs/'+publication['config_digest'])
        if registry.get(repository,'manifests/'+identity['tag'].rsplit(':',1)[1])!=index: raise ValueError()
        oci=tool.oci_identity(source,publication['index_digest'],index,manifest,config)
        if any(publication[k]!=oci[k] for k in ('index_digest','platform_digest','config_digest','immutable_image')): raise ValueError()
        authority._head(headers,master,deadline)
        canonical.validate_run(authority._get(authority.API+'/actions/runs/'+str(run_id),headers,deadline),source,run_id,attempt)
        if _source(root,approved)!=snapshot: raise ValueError()
        return {'artifact_id':artifact_id,'artifact_sha256':hashlib.sha256(raw).hexdigest(),'publication':copy.deepcopy(publication)}
    except Exception: raise StagingToolError('STAGING native publication unavailable') from None
