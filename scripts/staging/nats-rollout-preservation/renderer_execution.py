"""Compare two approved renderer images without a broker or selected store.

All inputs stay in a root-private scratch directory. Only an output hash is
returned. This adapter does not admit arbitrary images, commands or mounts.
"""
import json
import os
from pathlib import Path
import re
import stat
import tempfile
import uuid
from controller import Blocked
from docker_runtime import DockerRuntime, LABEL

IMAGE=re.compile(r'ghcr\.io/poryadok/voiceroot/nats-hub-config-renderer@sha256:[a-f0-9]{64}')
ARGS=('/run/nats/input/operator.jwt','/run/nats/input/account.jwt',
    '/run/nats/input/system-account.jwt','/run/nats/input/account.public',
    '/run/nats/input/system-account.public','/run/nats/template/nats.conf',
    '/run/nats-rendered/config/nats.conf')

def read_output(path):
    fd=os.open(path,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        row=os.fstat(fd)
        if (not stat.S_ISREG(row.st_mode) or row.st_uid!=65532 or row.st_nlink!=1
            or stat.S_IMODE(row.st_mode)!=0o400 or not 1<=row.st_size<=2<<20):
            raise Blocked('renderer_output_custody_invalid')
        raw=os.read(fd,row.st_size+1)
        if len(raw)!=row.st_size:raise Blocked('renderer_output_changed')
        return raw
    finally:os.close(fd)

def create(runtime,image,mounts,suffix):
    if not IMAGE.fullmatch(image) or suffix not in ('old','target'):
        raise Blocked('renderer_image_scope_invalid')
    rows=json.loads(runtime.run(['image','inspect',image]))
    if len(rows)!=1 or image not in rows[0].get('RepoDigests',[]):
        raise Blocked('renderer_image_digest_invalid')
    name='voice-known-'+runtime.operation+'-'+suffix
    args=['create','--name',name,'--label',LABEL+'='+runtime.operation,
        '--network','none','--user','65532:65532','--read-only','--cap-drop','ALL',
        '--security-opt','no-new-privileges:true','--memory','256m','--pids-limit','64',
        '--tmpfs','/tmp:rw,noexec,nosuid,size=16m']
    expected=set()
    targets={'/run/nats/input','/run/nats/template','/etc/nats/jwt','/etc/nats/tls','/run/nats-rendered'}
    if {m[1] for m in mounts}!=targets or len(mounts)!=5:
        raise Blocked('renderer_mount_scope_invalid')
    for path,target,writable in mounts:
        path=Path(path)
        if path.resolve(strict=True)!=path or not path.is_relative_to(runtime.base) or ',' in str(path) or writable!=(target=='/run/nats-rendered'):
            raise Blocked('renderer_mount_scope_invalid')
        expected.add((str(path),target,writable))
        args+=['--mount','type=bind,source='+str(path)+',target='+target+('' if writable else ',readonly')]
    args+=['--entrypoint','/nats-hub-config-renderer',image,*ARGS]
    cid=runtime.run(args)
    if not re.fullmatch(r'[a-f0-9]{64}',cid):raise Blocked('renderer_container_identity_invalid')
    runtime.owned[name]={'id':cid,'image':image,'image_id':rows[0]['Id'],'network':'none','mounts':expected}
    runtime.inspect(name)
    return name

def prove(base,descriptor,inputs,unchanged):
    """inputs contains four private captured directories, never a live mount."""
    import renderer_transition
    unchanged()
    runtime=DockerRuntime(base,'render'+uuid.uuid4().hex[:12]);outputs=[]
    try:
        for label in ('old','target'):
            with tempfile.TemporaryDirectory(prefix='renderer-'+label+'-',dir=runtime.base) as temp:
                out=Path(temp);os.chown(out,65532,65532);out.chmod(0o700)
                mounts=[(inputs[k],dest,False) for k,dest in (
                    ('input','/run/nats/input'),('template','/run/nats/template'),
                    ('jwt','/etc/nats/jwt'),('tls','/etc/nats/tls'))]+[(out,'/run/nats-rendered',True)]
                name=create(runtime,descriptor['images'][label],mounts,label)
                runtime.run(['start',name]);runtime.inspect(name)
                if runtime.run(['wait',name],timeout=30)!='0' or runtime.inspect(name)['State']['Running']:
                    raise Blocked('renderer_execution_refused')
                outputs.append(read_output(out/'config'/'nats.conf'))
                unchanged()
        return renderer_transition.compare_outputs(*outputs)
    finally:
        for name in reversed(list(runtime.owned)):
            row=runtime.inspect(name);runtime.run(['rm','-f',row['Id']])
            if runtime.run(['ps','-a','--filter','id='+row['Id'],'--format','{{.ID}}']):
                raise Blocked('renderer_cleanup_failed')
