"""Fixed authenticated source comparison for the installed DEV peer signer."""
import base64
import hashlib
import os
from pathlib import Path
import re
import stat
import time

import canonical_application_package as canonical
import dev_peer_issuer_source as reviewed
import source_authority as github

INSTALLED = Path('/opt/voice-dev-peer-issuer')
FILES = tuple('scripts/dev/' + name for name in reviewed.FILES) + reviewed.ROOT_PATHS + (reviewed.SELF,)


def _installed(path):
    target = INSTALLED / path
    if target.resolve() != target:
        raise ValueError()
    for parent in (target.parent, *target.parent.parents):
        info = parent.lstat()
        if not stat.S_ISDIR(info.st_mode) or info.st_uid != 0 or info.st_mode & 0o022:
            raise ValueError()
    fd = os.open(target, os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK)
    try:
        before = os.fstat(fd)
        if (not stat.S_ISREG(before.st_mode) or before.st_uid != 0 or before.st_nlink != 1 or
                before.st_mode & 0o022 or not 0 < before.st_size <= 1024 * 1024):
            raise ValueError()
        data = bytearray()
        while len(data) < before.st_size:
            part = os.read(fd, min(65536, before.st_size - len(data)))
            if not part:
                raise ValueError()
            data.extend(part)
        def identity(value):
            return (value.st_dev, value.st_ino, value.st_size, value.st_mtime_ns, value.st_ctime_ns, value.st_mode, value.st_uid)
        if identity(before) != identity(os.fstat(fd)) or identity(before) != identity(target.lstat()):
            raise ValueError()
        return bytes(data)
    finally:
        os.close(fd)


def verify(token, source_sha, run_id, attempt):
    """Verify fixed master CI identity and fixed installed files against GitHub bytes."""
    if (not isinstance(token, str) or not token or len(token) > 8192 or
            not re.fullmatch(r'[a-f0-9]{40}', source_sha) or type(run_id) is not int or run_id <= 0 or
            type(attempt) is not int or attempt <= 0):
        raise ValueError()
    deadline = time.monotonic() + 120
    headers = github._headers(token)
    github._head(headers, source_sha, deadline)
    run = github._get(github.API + '/actions/runs/' + str(run_id), headers, deadline)
    canonical.validate_run(run, source_sha, run_id, attempt)
    jobs = github._pages(github.API + '/actions/runs/' + str(run_id) + '/attempts/' + str(attempt) + '/jobs',
                         'jobs', headers, deadline)
    canonical.validate_jobs(jobs, source_sha, run_id, attempt)
    names = FILES
    if len(names) != len(set(names)):
        raise ValueError()
    for name in names:
        obj = github._get(github.API + '/contents/' + name + '?ref=' + source_sha, headers, deadline)
        blob_sha = obj.get('sha', '')
        if (obj.get('path') != name or obj.get('type') != 'file' or
                not re.fullmatch(r'[a-f0-9]{40}', blob_sha) or obj.get('encoding') != 'base64' or
                type(obj.get('size')) is not int):
            raise ValueError()
        raw = base64.b64decode(obj['content'].replace('\n', ''), validate=True)
        if (len(raw) != obj['size'] or hashlib.sha1(b'blob ' + str(len(raw)).encode() + b'\0' + raw).hexdigest() != blob_sha
                or _installed(name) != raw):
            raise ValueError()
    github._head(headers, source_sha, deadline)
