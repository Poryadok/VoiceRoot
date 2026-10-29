#!/usr/bin/env bash
set -euo pipefail
[[ "${EUID}" -eq 0 ]] || { echo 'human root required' >&2; exit 1; }
exec /usr/bin/python3 -I -S - "$@" <<'PY'
import hashlib, io, json, os, pathlib, pwd, stat, sys, tarfile, uuid
EXPECTED_SHA = '__BUNDLE_SHA256__'
EXPECTED_BYTES = __BUNDLE_BYTES__
FILES = {'credentials.py', 'realtime_p3_missing_issuer.py', 'realtime_p3_root_export.py', 'reset_credentials.py', 'capture-manifest.json'}
SOURCE = pathlib.Path('/home/pmd/voice-realtime-p3/p3-source.tar')
ROOT = pathlib.Path('/home/voice-dev/issuance')
try:
    for parent in (*ROOT.parents[::-1], ROOT):
        info = parent.lstat()
        if not stat.S_ISDIR(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o022:
            raise ValueError()
    fd = os.open(SOURCE, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        before = os.fstat(fd)
        if (not stat.S_ISREG(before.st_mode) or before.st_uid != pwd.getpwnam('pmd').pw_uid
                or before.st_mode & 0o022 or before.st_nlink != 1 or before.st_size != EXPECTED_BYTES
                or not 1 <= EXPECTED_BYTES <= 2 << 20):
            raise ValueError()
        chunks, remaining = [], EXPECTED_BYTES + 1
        while remaining:
            chunk = os.read(fd, min(65536, remaining))
            if not chunk: break
            chunks.append(chunk)
            remaining -= len(chunk)
        raw = b''.join(chunks)
        after = os.fstat(fd)
        if ((before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns, before.st_ctime_ns) !=
            (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns, after.st_ctime_ns)
                or len(raw) != EXPECTED_BYTES or hashlib.sha256(raw).hexdigest() != EXPECTED_SHA):
            raise ValueError()
    finally:
        os.close(fd)
    contents = {}
    with tarfile.open(fileobj=io.BytesIO(raw), mode='r:') as archive:
        for member in archive:
            if member.name not in FILES or member.name in contents or not member.isreg() or not 1 <= member.size <= 1 << 20:
                raise ValueError()
            contents[member.name] = archive.extractfile(member).read(member.size + 1)
            if len(contents[member.name]) != member.size: raise ValueError()
    if set(contents) != FILES: raise ValueError()
    manifest = json.loads(contents['capture-manifest.json'])
    if (set(manifest) != FILES - {'capture-manifest.json'} or
            any(hashlib.sha256(contents[name]).hexdigest() != digest for name, digest in manifest.items())):
        raise ValueError()
    captured = ROOT / ('p3-source-' + uuid.uuid4().hex)
    captured.mkdir(mode=0o700)
    for name, data in contents.items():
        fd = os.open(captured / name, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o400)
        with os.fdopen(fd, 'wb') as stream:
            stream.write(data)
            stream.flush()
            os.fsync(stream.fileno())
    for directory in (captured, ROOT):
        fd = os.open(directory, os.O_RDONLY | os.O_DIRECTORY)
        try: os.fsync(fd)
        finally: os.close(fd)
    arguments = sys.argv[1:]
    module = 'realtime_p3_missing_issuer'
    if arguments and arguments[0] == '--export':
        module, arguments = 'realtime_p3_root_export', arguments[1:]
    loader = "import runpy,sys;sys.path.insert(0,sys.argv.pop(1));runpy.run_module(sys.argv.pop(1),run_name='__main__')"
    os.execve('/usr/bin/python3', ['/usr/bin/python3', '-I', '-S', '-c', loader, str(captured), module, *arguments],
              {'PATH': '/usr/local/bin:/usr/bin:/bin', 'HOME': '/root'})
except Exception:
    print('P3_SOURCE_CAPTURE=BLOCKED', file=sys.stderr)
    sys.exit(1)
PY
