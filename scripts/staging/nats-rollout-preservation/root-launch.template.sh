#!/usr/bin/env bash
set -euo pipefail
[[ "${EUID}" -eq 0 ]] || { echo 'human root required' >&2; exit 1; }
exec /usr/bin/python3 -I -S - "$@" <<'PY'
import hashlib, io, json, os, pathlib, stat, sys, tarfile, uuid
EXPECTED_SHA='__BUNDLE_SHA256__'
EXPECTED_BYTES=__BUNDLE_BYTES__
FILES=__BUNDLE_FILES__
SOURCE=pathlib.Path('/home/pmd/voice-nats-rollout-v8/rollout-bundle.tar')
ROOT=pathlib.Path('/var/lib/voice-nats-preservation')
try:
    for parent in (*ROOT.parents[::-1],ROOT):
        s=parent.lstat()
        if not stat.S_ISDIR(s.st_mode) or s.st_uid!=0 or s.st_mode&0o022:raise ValueError('root custody')
    fd=os.open(SOURCE,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        before=os.fstat(fd)
        if not stat.S_ISREG(before.st_mode) or before.st_uid!=1000 or before.st_mode&0o022 or before.st_nlink!=1 or before.st_size!=EXPECTED_BYTES or not 1<=EXPECTED_BYTES<=32<<20:raise ValueError('source custody')
        raw=os.read(fd,EXPECTED_BYTES+1);after=os.fstat(fd)
        if (before.st_dev,before.st_ino,before.st_size,before.st_mtime_ns,before.st_ctime_ns)!=(after.st_dev,after.st_ino,after.st_size,after.st_mtime_ns,after.st_ctime_ns) or len(raw)!=EXPECTED_BYTES or hashlib.sha256(raw).hexdigest()!=EXPECTED_SHA:raise ValueError('bundle hash')
    finally:os.close(fd)
    contents={}
    with tarfile.open(fileobj=io.BytesIO(raw),mode='r:') as archive:
        for member in archive:
            if member.name not in FILES or member.name in contents or not member.isreg() or not 1<=member.size<=32<<20:raise ValueError('bundle member')
            content=archive.extractfile(member).read(member.size+1)
            if len(content)!=member.size:raise ValueError('member size')
            contents[member.name]=content
    if set(contents)!=set(FILES):raise ValueError('bundle files')
    manifest=json.loads(contents['capture-manifest.json'])
    if set(manifest)!=set(FILES)-{'capture-manifest.json'} or any(hashlib.sha256(contents[n]).hexdigest()!=v for n,v in manifest.items()):raise ValueError('member hash')
    captured=ROOT/('rollout-code-'+uuid.uuid4().hex);captured.mkdir(mode=0o700)
    for name,content in contents.items():
        outpath=captured/name;outpath.parent.mkdir(parents=True,exist_ok=True,mode=0o700)
        out=os.open(outpath,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o500 if name.endswith(('/kernel','/bootstrap-renewer')) else 0o400)
        with os.fdopen(out,'wb') as stream:stream.write(content);stream.flush();os.fsync(stream.fileno())
    arguments=sys.argv[1:]
    entry='root_cli.py'
    if len(arguments)==2 and arguments[0]=='--install':entry='installer.py';arguments=arguments[1:]
    os.execve('/usr/bin/python3',['/usr/bin/python3','-I','-S',str(captured/'nats-rollout-preservation'/entry),*arguments],
              {'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'})
except Exception:
    print('NATS_ROLLOUT_CAPTURE=BLOCKED',file=sys.stderr);sys.exit(1)
PY
