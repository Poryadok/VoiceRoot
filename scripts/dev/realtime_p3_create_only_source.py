"""Bound the reviewed issuer cohort to the authenticated candidate source."""
import os
from pathlib import Path
import re
import stat
import subprocess

# Final independently reviewed and published issuer/exporter cohort.
REVIEWED_ISSUER_SHA = 'e63c0c8699e9a74624c3d423b5388d9b1630ee18'
PATHS = ('scripts/dev/realtime_p3_missing_issuer.py','scripts/dev/realtime_p3_root_export.py',
         'scripts/dev/credentials.py','scripts/dev/reset_credentials.py',
         'scripts/dev/realtime_p3_root_bundle.py','scripts/dev/realtime_p3_root_launch.template.sh')
LIMIT = 256 * 1024


class SourceError(RuntimeError): pass


def _blob(repo,commit,path):
    env = os.environ.copy(); env['GIT_NO_REPLACE_OBJECTS'] = '1'
    name = commit+':'+path
    size = subprocess.run(['git','-C',str(repo),'cat-file','-s',name],stdout=subprocess.PIPE,
                          stderr=subprocess.DEVNULL,env=env,timeout=20,check=True).stdout
    if not re.fullmatch(rb'[0-9]+\n?',size) or not 0 < int(size) <= LIMIT: raise ValueError()
    value = subprocess.run(['git','-C',str(repo),'cat-file','blob',name],stdout=subprocess.PIPE,
                           stderr=subprocess.DEVNULL,env=env,timeout=20,check=True).stdout
    if len(value) != int(size): raise ValueError()
    return value


def _disk(root,path):
    directory = os.open(root,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW)
    try:
        components = path.split('/')
        for part in components[:-1]:
            child = os.open(part,os.O_RDONLY|os.O_DIRECTORY|os.O_NOFOLLOW,dir_fd=directory)
            os.close(directory); directory = child
        fd = os.open(components[-1],os.O_RDONLY|os.O_NOFOLLOW|os.O_NONBLOCK,dir_fd=directory)
        try:
            info = os.fstat(fd)
            if not stat.S_ISREG(info.st_mode) or not 0 < info.st_size <= LIMIT: raise ValueError()
            raw = bytearray()
            while len(raw)<info.st_size:
                value = os.read(fd,info.st_size-len(raw))
                if not value: raise ValueError()
                raw.extend(value)
            after = os.fstat(fd)
            if (info.st_dev,info.st_ino,info.st_size,info.st_mtime_ns,info.st_ctime_ns) != (
                    after.st_dev,after.st_ino,after.st_size,after.st_mtime_ns,after.st_ctime_ns): raise ValueError()
            return bytes(raw)
        finally: os.close(fd)
    finally: os.close(directory)


def verify(repo, issuer_sha, candidate_sha):
    try:
        root = Path(repo)
        if (not root.is_absolute() or not isinstance(issuer_sha,str) or issuer_sha != REVIEWED_ISSUER_SHA
                or not re.fullmatch(r'[0-9a-f]{40}',issuer_sha)
                or not isinstance(candidate_sha,str) or not re.fullmatch(r'[0-9a-f]{40}',candidate_sha)):
            raise ValueError()
        for path in PATHS:
            reviewed = _blob(root,issuer_sha,path); candidate = _blob(root,candidate_sha,path)
            if reviewed != candidate or _disk(root,path) != candidate: raise ValueError()
    except Exception:
        raise SourceError('P3 issuer source equivalence unavailable') from None
