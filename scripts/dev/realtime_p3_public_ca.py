"""Prepare only the two public CA inputs for missing Space issuer material."""
import hashlib
import json
import os
from pathlib import Path
import re
import stat

import full_cohort_schema_metadata as common
import realtime_p3_public_inputs as public
from publish_candidate import source_snapshot

NAMES={'voice-principal-ca':'principal-ca.pem','voice-media-ca':'media-ca.pem'}


def capture(source_root,source_sha,source_authority,namespace_uid,claim_guard,destination,
            run_bytes=common.linux_private_bytes):
    """Capture only public CA PEM; result is preparation, not P3 admission."""
    dirfd=None; fds={}
    try:
        if not isinstance(source_sha,str) or not re.fullmatch(r'[a-f0-9]{40}',source_sha) or not namespace_uid: raise ValueError()
        def guard():
            common._fence(claim_guard)
            try:
                raw=common._external(run_bytes,['kubectl','--kubeconfig','/home/pmd/.kube/config','--context','default',
                    '--request-timeout=15s','get','namespace','voice-dev','-o','json'])
                meta=json.loads(raw)['metadata']
                if meta.get('name')!='voice-dev' or meta.get('uid')!=namespace_uid: raise ValueError()
            except Exception: raise common.GuardLost('namespace_guard_failed') from None
            common._fence(claim_guard)
        def authorize():
            if common._external(source_authority,source_sha) is False: raise ValueError()
        guard(); authorize(); snapshot=source_snapshot(Path(source_root),source_sha)
        manifest=public._source_contract(source_root,snapshot); refs={}; values={}
        for name in NAMES:
            refs[name]=public._meta(run_bytes,guard,'secret',name)
            values[name]=public._value(run_bytes,guard,'secret',name,'ca.crt')
            public._public_pem(values[name])
        def recheck():
            for name in NAMES:
                if public._meta(run_bytes,guard,'secret',name)!=refs[name]: raise ValueError()
            authorize()
            if source_snapshot(Path(source_root),source_sha)!=snapshot: raise ValueError()
            guard()
        recheck(); directory=Path(destination)
        if not directory.is_absolute() or directory.resolve()!=directory: raise ValueError()
        dirfd=os.open(directory,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW); info=os.fstat(dirfd)
        if info.st_uid!=os.geteuid() or stat.S_IMODE(info.st_mode)!=0o700 or os.listdir(dirfd): raise ValueError()
        hashes={}
        for name,filename in NAMES.items():
            fd=os.open(filename,os.O_RDWR|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600,dir_fd=dirfd); fds[filename]=fd
            remaining=memoryview(values[name])
            while remaining:
                count=os.write(fd,remaining)
                if count<=0: raise ValueError()
                remaining=remaining[count:]
            os.fsync(fd); path=Path('/proc')/str(os.getpid())/'fd'/str(fd)
            public._openssl('x509','-in',path,'-noout','-checkend','86400')
            if b'CA:TRUE' not in public._openssl('x509','-in',path,'-noout','-ext','basicConstraints'): raise ValueError()
            hashes[name]=hashlib.sha256(public._openssl('x509','-in',path,'-outform','DER')).hexdigest()
        os.fsync(dirfd); recheck(); now=os.stat(directory,follow_symlinks=False)
        if (now.st_dev,now.st_ino)!=(info.st_dev,info.st_ino): raise ValueError()
        for name,filename in NAMES.items():
            held=os.fstat(fds[filename]); current=os.stat(filename,dir_fd=dirfd,follow_symlinks=False)
            if ((held.st_dev,held.st_ino)!=(current.st_dev,current.st_ino) or held.st_nlink!=1
                    or stat.S_IMODE(held.st_mode)!=0o600 or held.st_uid!=os.geteuid()
                    or os.pread(fds[filename],32769,0)!=values[name]): raise ValueError()
        guard()
        return {'status':'observed','source_sha':source_sha,'namespace_uid':namespace_uid,
                'source_manifest_sha256':manifest,'objects':refs,'public_der_sha256':hashes,
                'scope':'public-ca-preparation-only','admission_qualified':False}
    except common.GuardLost: raise
    except Exception: raise common.Hold('public_ca_capture_unavailable') from None
    finally:
        if dirfd is not None: os.close(dirfd)
        for fd in fds.values(): os.close(fd)
