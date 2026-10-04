#!/usr/bin/env bash
set -euo pipefail
[[ "${EUID}" -eq 0 ]] || { echo 'human root required' >&2; exit 1; }
exec /usr/bin/python3.13 -I -S - "$@" <<'PY'
import hashlib, io, json, os, pathlib, stat, sys, tarfile, uuid
EXPECTED_SHA = '__BUNDLE_SHA256__'
EXPECTED_BYTES = __BUNDLE_BYTES__
SOURCE = '/home/pmd/voice-known-baseline/known-baseline-bundle.tar'
ROOT = pathlib.Path('/var/lib/voice-nats-preservation')
FILES = {'root_main.py', 'controller.py', 'commands.py', 'docker_runtime.py',
         'stage_runtime.py', 'scenario.py', 'deployed-contract.json',
         'kernel', 'capture-manifest.json'}
try:
    args=sys.argv[1:]
    if args!=['--prepare'] and not (len(args)==4 and args[0]=='--resume') and not (len(args)==2 and args[0] in ('--refresh','--status')):
        raise ValueError('arguments')
    for parent in (pathlib.Path('/var'),pathlib.Path('/var/lib'),ROOT):
        s=os.lstat(parent)
        if not stat.S_ISDIR(s.st_mode) or s.st_uid!=0 or s.st_mode&0o022: raise ValueError('root custody')
    fd=os.open(SOURCE,os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK)
    try:
        before=os.fstat(fd)
        if not stat.S_ISREG(before.st_mode) or before.st_uid!=1000 or before.st_mode&0o022 or before.st_size!=EXPECTED_BYTES or not 1<=EXPECTED_BYTES<=32<<20:
            raise ValueError('source custody')
        with os.fdopen(fd,'rb',closefd=False) as f: raw=f.read(EXPECTED_BYTES+1)
        after=os.fstat(fd)
        if (before.st_dev,before.st_ino,before.st_size,before.st_mtime_ns,before.st_ctime_ns)!=(after.st_dev,after.st_ino,after.st_size,after.st_mtime_ns,after.st_ctime_ns) or len(raw)!=EXPECTED_BYTES or hashlib.sha256(raw).hexdigest()!=EXPECTED_SHA:
            raise ValueError('bundle hash')
    finally: os.close(fd)
    captured=ROOT/('known-baseline-code-'+uuid.uuid4().hex)
    captured.mkdir(mode=0o700)
    contents={}
    with tarfile.open(fileobj=io.BytesIO(raw),mode='r:') as archive:
        for member in archive:
            if member.name not in FILES or member.name in contents or not member.isreg() or not 1<=member.size<=32<<20:
                raise ValueError('bundle member')
            content=archive.extractfile(member).read(member.size+1)
            if len(content)!=member.size: raise ValueError('member size')
            contents[member.name]=content
    if set(contents)!=FILES: raise ValueError('bundle files')
    manifest=json.loads(contents['capture-manifest.json'])
    if set(manifest['files'])!=FILES-{'capture-manifest.json'}: raise ValueError('manifest files')
    for name, content in contents.items():
        if name!='capture-manifest.json' and hashlib.sha256(content).hexdigest()!=manifest['files'][name]:
            raise ValueError('member hash')
        out=os.open(captured/name,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o500 if name=='kernel' else 0o400)
        with os.fdopen(out,'wb') as f: f.write(content); f.flush(); os.fsync(f.fileno())
    os.execve('/usr/bin/python3.13',['/usr/bin/python3.13','-I','-S',str(captured/'root_main.py'),*args],
              {'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root'})
except Exception:
    print('KNOWN_NATS_CAPTURE=BLOCKED',file=sys.stderr);sys.exit(1)
PY
