"""Human-root finite staging operation; no agent authenticates root.

Cold-store primitives are implemented first. Operator activation is deliberately
unavailable until the full manifest/fence/runtime integration has passed review.
"""

import hashlib
import io
import os
from pathlib import Path, PurePosixPath
import re
import stat
import tarfile

MAX_FILES = 8192
MAX_BYTES = 64 << 20
MAX_DEPTH = 12
SAFE = re.compile(r'^[A-Za-z0-9_$.-]{1,128}$')
READ = os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | os.O_NOATIME


class Blocked(Exception):
    pass


def archive_closed_store(source, archive, *, max_bytes=64 << 20, max_files=MAX_FILES, streaming=False):
    if type(max_bytes) is not int or type(max_files) is not int or not 0<max_bytes<=48<<30 or not 0<max_files<=262144:raise Blocked('archive_policy_invalid')
    files, dirs, total = [], [], 0
    root = os.open(source, READ | os.O_DIRECTORY)
    device = os.fstat(root).st_dev
    output = os.open(archive, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    try:
        with os.fdopen(output, 'wb') as stream, tarfile.open(fileobj=stream, mode='w') as tar:
            def visit(fd, prefix='', depth=0):
                nonlocal total
                if depth > MAX_DEPTH:
                    raise Blocked('archive_depth_limit')
                names = []
                with os.scandir(fd) as entries:
                    for entry in entries:
                        if len(names) >= max_files:
                            raise Blocked('archive_entry_limit')
                        names.append(entry.name)
                for name in sorted(names):
                    if not SAFE.fullmatch(name) or name in ('.', '..') or name.lower().endswith(('.creds', '.key', '.pem', '.jwt')) or 'seed' in name.lower():
                        raise Blocked('archive_input_invalid')
                    path = prefix + name
                    before = os.stat(name, dir_fd=fd, follow_symlinks=False)
                    if before.st_dev != device:
                        raise Blocked('archive_mount_boundary')
                    if stat.S_ISDIR(before.st_mode):
                        if len(files) + len(dirs) >= max_files:
                            raise Blocked('archive_entry_limit')
                        child = os.open(name, READ | os.O_DIRECTORY, dir_fd=fd)
                        try:
                            if identity(before) != identity(os.fstat(child)):
                                raise Blocked('archive_source_race')
                            dirs.append(path)
                            info = tarfile.TarInfo(path); info.type = tarfile.DIRTYPE; info.mode = 0o700
                            tar.addfile(info)
                            visit(child, path+'/', depth+1)
                        finally:
                            os.close(child)
                    elif stat.S_ISREG(before.st_mode):
                        if len(files) + len(dirs) >= max_files or total + before.st_size > max_bytes:
                            raise Blocked('archive_size_limit')
                        child = os.open(name, READ, dir_fd=fd)
                        try:
                            opened = os.fstat(child)
                            if identity(before) != identity(opened) or not stat.S_ISREG(opened.st_mode):
                                raise Blocked('archive_source_race')
                            if streaming:
                                class Reader:
                                    def __init__(self):self.digest=hashlib.sha256();self.count=0
                                    def read(self,size):
                                        if not 0<=size<=1<<20:raise Blocked('archive_read_bound')
                                        raw=os.read(child,size);self.digest.update(raw);self.count+=len(raw);return raw
                                data=Reader();info=tarfile.TarInfo(path);info.size=before.st_size;info.mode=0o600
                                tar.addfile(info,data)
                                if data.count!=before.st_size or os.read(child,1) or identity(opened)!=identity(os.fstat(child)):raise Blocked('archive_source_race')
                                sha=data.digest.hexdigest();size=data.count
                            else:
                                with os.fdopen(child,'rb',closefd=False) as data:raw=data.read(before.st_size+1)
                                if len(raw)!=before.st_size or identity(opened)!=identity(os.fstat(child)):raise Blocked('archive_source_race')
                                info=tarfile.TarInfo(path);info.size=len(raw);info.mode=0o600;tar.addfile(info,io.BytesIO(raw))
                                sha=hashlib.sha256(raw).hexdigest();size=len(raw)
                            files.append({'path':path,'size':size,'sha256':sha});total+=size
                        finally:
                            os.close(child)
                    else:
                        raise Blocked('archive_nonregular_input')
            visit(root)
        return {'mechanism':'closed-jetstream-store-tar-v1','archive_sha256':file_sha(archive,max_bytes=max_bytes,max_files=max_files),'file_count':len(files),'bytes':total,'dirs':dirs,'files':files}
    finally:
        os.close(root)


def restore_closed_store(archive, target, manifest, *, max_bytes=MAX_BYTES,max_files=MAX_FILES,streaming=False):
    if not verify_archive(archive, manifest,max_bytes=max_bytes,max_files=max_files):
        raise Blocked('archive_hash_mismatch')
    expected = {f['path']:f for f in manifest['files']}
    members, seen, total = [], set(), 0
    fd = os.open(archive, READ)
    with os.fdopen(fd, 'rb') as stream, tarfile.open(fileobj=stream, mode='r:') as tar:
        for member in tar:
            parts = PurePosixPath(member.name).parts
            if member.name.startswith('/') or not parts or len(parts)>MAX_DEPTH+1 or any(p in ('.','..') or not SAFE.fullmatch(p) for p in parts) or member.name in seen:
                raise Blocked('archive_path_invalid')
            seen.add(member.name)
            if len(seen)>max_files or not (member.isdir() or member.isreg()):
                raise Blocked('archive_member_invalid')
            if member.isdir():
                if member.name not in manifest['dirs']:
                    raise Blocked('archive_manifest_mismatch')
            else:
                row = expected.get(member.name)
                if row is None or member.size != row['size'] or member.size<0:
                    raise Blocked('archive_manifest_mismatch')
                total += member.size
                if total>max_bytes:
                    raise Blocked('archive_size_limit')
            members.append(member)
        if seen != set(manifest['dirs']) | set(expected) or total!=manifest['bytes']:
            raise Blocked('archive_manifest_mismatch')
        target = Path(target)
        target.mkdir(mode=0o700)
        for name in sorted(manifest['dirs'], key=lambda s:(s.count('/'),s)):
            (target / name).mkdir(mode=0o700)
        for member in members:
            if not member.isreg():
                continue
            out = os.open(target/member.name, os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600)
            with os.fdopen(out,'wb') as destination,tar.extractfile(member) as source:
                sha=hashlib.sha256();count=0
                while data:=source.read(1<<20 if streaming else member.size+1):
                    count+=len(data)
                    if count>member.size:raise Blocked('archive_file_hash_mismatch')
                    sha.update(data);destination.write(data)
                if count!=member.size or sha.hexdigest()!=expected[member.name]['sha256']:raise Blocked('archive_file_hash_mismatch')
                destination.flush(); os.fsync(destination.fileno())
    if not verify_archive(archive,manifest,max_bytes=max_bytes,max_files=max_files):
        raise Blocked('archive_changed_during_restore')


def file_sha(path, *, max_bytes=MAX_BYTES,max_files=MAX_FILES):
    fd = os.open(path, READ)
    try:
        before = os.fstat(fd)
        if not stat.S_ISREG(before.st_mode) or before.st_size>max_bytes+(max_files*1024)+(1<<20):
            raise Blocked('archive_file_invalid')
        digest = hashlib.sha256()
        while True:
            chunk = os.read(fd,1<<20)
            if not chunk: break
            digest.update(chunk)
        if identity(before)!=identity(os.fstat(fd)):
            raise Blocked('archive_changed_during_hash')
        return digest.hexdigest()
    finally:
        os.close(fd)


def verify_archive(archive, manifest, *, max_bytes=MAX_BYTES,max_files=MAX_FILES):
    if type(max_bytes) is not int or type(max_files) is not int or not 0<max_bytes<=48<<30 or not 0<max_files<=262144:raise Blocked('archive_policy_invalid')
    if set(manifest)!={'mechanism','archive_sha256','file_count','bytes','dirs','files'} or manifest['mechanism']!='closed-jetstream-store-tar-v1' or type(manifest['file_count']) is not int or type(manifest['bytes']) is not int or not 0<=manifest['bytes']<=max_bytes or not 0<=manifest['file_count']<=max_files or len(manifest['files'])!=manifest['file_count']:
        raise Blocked('archive_manifest_invalid')
    rows=manifest['files']; names=[r['path'] for r in rows]+manifest['dirs']
    if len(names)>max_files or len(names)!=len(set(names)) or sum(r['size'] for r in rows)!=manifest['bytes']:
        raise Blocked('archive_manifest_invalid')
    return file_sha(archive,max_bytes=max_bytes,max_files=max_files)==manifest['archive_sha256']


def identity(s):
    return (s.st_dev,s.st_ino,s.st_mode,s.st_size,s.st_mtime_ns,s.st_ctime_ns)
