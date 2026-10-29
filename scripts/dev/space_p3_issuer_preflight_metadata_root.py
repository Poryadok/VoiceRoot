"""Fixed root-side Creator Secret metadata, no value output or mutation."""
import hashlib
import os
from pathlib import Path
import re
import selectors
import stat
import subprocess
import time

import realtime_p3_missing_issuer as custody

K3S=Path('/usr/local/bin/k3s')
KUBECONFIG=Path('/etc/rancher/k3s/k3s.yaml')
K3S_SHA256='f95350e3997165974d1d1406f01aabaf869aed7ca790cc2d83a92a82f26a06c5'
MEMBERS={'voice-space-media-client-tls':{'tls.crt','tls.key'},
         'voice-gateway-media-client-tls':{'tls.crt','tls.key'},
         'voice-auth-media-server-tls':{'tls.crt','tls.key'},
         'voice-role-media-server-tls':{'tls.crt','tls.key'},
         'voice-gateway-principal-signing':{'current.pem','next.pem','active-kid'}}
TEMPLATE='{{.metadata.uid}}{{"\\n"}}{{.metadata.resourceVersion}}{{"\\n"}}{{.metadata.deletionTimestamp}}{{"\\n"}}{{range $k,$v := .data}}{{printf "%s\\n" $k}}{{end}}'


def _open(path,limit):
    custody._check_ancestors(path.parent)
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    info=os.fstat(fd)
    if (not stat.S_ISREG(info.st_mode) or info.st_uid!=0 or info.st_nlink!=1
            or info.st_mode & 0o022 or not 0<info.st_size<=limit): os.close(fd); raise ValueError()
    return fd,(info.st_dev,info.st_ino,info.st_size,info.st_mtime_ns,info.st_ctime_ns)


def _digest(fd,size):
    digest=hashlib.sha256(); offset=0
    while offset<size:
        data=os.pread(fd,min(131072,size-offset),offset)
        if not data: raise ValueError()
        digest.update(data); offset+=len(data)
    return digest.hexdigest()


def _read(binary,config,args):
    command=['/proc/self/fd/'+str(binary),'kubectl','--kubeconfig','/proc/self/fd/'+str(config),
             '--context','default','--request-timeout=15s',*args]
    process=subprocess.Popen(command,pass_fds=(binary,config),stdin=subprocess.DEVNULL,stdout=subprocess.PIPE,
                             stderr=subprocess.DEVNULL,env={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'})
    selector=selectors.DefaultSelector(); output=bytearray()
    try:
        os.set_blocking(process.stdout.fileno(),False); selector.register(process.stdout,selectors.EVENT_READ)
        deadline=time.monotonic()+30
        while selector.get_map():
            timeout=deadline-time.monotonic()
            if timeout<=0: raise TimeoutError()
            events=selector.select(timeout)
            if not events: raise TimeoutError()
            for key,_ in events:
                data=os.read(key.fd,8193-len(output))
                if not data: selector.unregister(process.stdout)
                else:
                    output.extend(data)
                    if len(output)>8192: raise ValueError()
        if process.wait(timeout=max(0.01,deadline-time.monotonic()))!=0: raise ValueError()
        return bytes(output)
    finally:
        selector.close()
        if process.poll() is None: process.kill()
        process.wait(); process.stdout.close()


def observe(namespace_uid):
    """Return only five fixed presence/identity/key-name categories."""
    if os.geteuid()!=0 or not re.fullmatch(r'[A-Za-z0-9.-]{1,128}',namespace_uid): raise ValueError()
    binary,binary_id=_open(K3S,512*1024*1024)
    try:
        config,config_id=_open(KUBECONFIG,1024*1024)
        try:
            if _digest(binary,binary_id[2])!=K3S_SHA256: raise ValueError()
            def snapshot():
                ns=_read(binary,config,['get','namespace','voice-dev','-o','jsonpath={.metadata.uid}']).decode('ascii')
                if ns!=namespace_uid: raise ValueError()
                result={}
                for name,keys in MEMBERS.items():
                    raw=_read(binary,config,['-n','voice-dev','get','secret',name,'--ignore-not-found=true','-o','go-template='+TEMPLATE])
                    if not raw: result[name]={'state':'absent'}; continue
                    lines=raw.decode('ascii').splitlines()
                    if len(lines)<3 or any(not re.fullmatch(r'[A-Za-z0-9.-]{1,128}',x) for x in lines[:2]) or lines[2] not in ('','<no value>'): raise ValueError()
                    seen=lines[3:]
                    if len(seen)!=len(set(seen)) or set(seen)!=keys: raise ValueError()
                    result[name]={'state':'present','uid':lines[0],'resourceVersion':lines[1],'keys':sorted(seen)}
                return result
            result=snapshot()
            if (snapshot()!=result or not custody._unchanged(binary,binary_id) or not custody._unchanged(config,config_id)
                    or (K3S.lstat().st_dev,K3S.lstat().st_ino)!=binary_id[:2]
                    or (KUBECONFIG.lstat().st_dev,KUBECONFIG.lstat().st_ino)!=config_id[:2]
                    or _digest(binary,binary_id[2])!=K3S_SHA256): raise ValueError()
            return result
        finally: os.close(config)
    finally: os.close(binary)
