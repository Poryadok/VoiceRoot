"""Immutable private archive member reads for the shared CLOSED verifier.

No extraction, selected-store access, credential role or unbounded member read.
The caller must independently bind the manifest to the admitted durable cut.
"""
import copy
import hashlib
import os
from pathlib import Path
import stat
import tarfile
from controller import Blocked,READ,identity
from native_store import verify_archive,MAX_FILES

def fail():raise Blocked('closed_seal_archive_changed')

class Archive:
    PRIVATE_UID=65532
    PRIVATE_ANCHOR=Path('/var/lib/voice-staging-runner/operations')

    def __init__(self,base,path,manifest,authority,*,private_native=False):
        self.base=Path(base);self.path=Path(path);self.authority=authority
        if type(private_native) is not bool:fail()
        self.private_native=private_native
        if self.private_native and os.geteuid()!=self.PRIVATE_UID:fail()
        if self.private_native and self.base.parent!=self.PRIVATE_ANCHOR:fail()
        self.manifest=copy.deepcopy(manifest)
        if not self.path.is_relative_to(self.base) or self.path.resolve(strict=True)!=self.path:fail()
        if self.private_native and self.path.parent!=self.base:fail()
        self.rows={row['path']:row for row in self.manifest['files']}
        if len(self.rows)!=len(self.manifest['files']):fail()
        self.base_identity=identity(self.base.lstat())
        self.ancestor_identities={}
        if self.private_native:
            parent=self.base.parent
            reached_root=False
            while True:
                row=parent.lstat();mode=stat.S_IMODE(row.st_mode)
                if (not stat.S_ISDIR(row.st_mode) or
                    (row.st_uid==self.PRIVATE_UID and mode==0o700 and not reached_root) or
                    (row.st_uid==0 and mode&0o022==0)):
                    pass
                else:fail()
                self.ancestor_identities[str(parent)]=identity(row)
                if row.st_uid==0:reached_root=True
                if parent==Path('/'):
                    if row.st_uid!=0:fail()
                    break
                parent=parent.parent
        self.original=None;self.verify()

    def verify(self):
        self.authority()
        parent=self.path.parent
        if self.private_native:
            row=parent.lstat()
            if (parent!=self.base or identity(row)!=self.base_identity or not stat.S_ISDIR(row.st_mode)
                or row.st_uid!=self.PRIVATE_UID or row.st_gid!=self.PRIVATE_UID
                or stat.S_IMODE(row.st_mode)!=0o700):fail()
            for name,bound in self.ancestor_identities.items():
                ancestor=Path(name);row=ancestor.lstat()
                mode=stat.S_IMODE(row.st_mode)
                private_parent=row.st_uid==self.PRIVATE_UID and mode==0o700
                trusted_parent=row.st_uid==0 and mode&0o022==0
                if (identity(row)!=bound or not stat.S_ISDIR(row.st_mode)
                    or not (private_parent or trusted_parent)):
                    fail()
        else:
            while True:
                row=parent.lstat()
                modes={0o700,0o750} if parent==self.base else {0o700}
                if not stat.S_ISDIR(row.st_mode) or row.st_uid!=0 or stat.S_IMODE(row.st_mode) not in modes:fail()
                if parent==self.base:break
                parent=parent.parent
        row=self.path.lstat()
        expected_uid=self.PRIVATE_UID if self.private_native else 0
        if (not stat.S_ISREG(row.st_mode) or row.st_uid!=expected_uid
            or self.private_native and row.st_gid!=self.PRIVATE_UID or row.st_nlink!=1
            or stat.S_IMODE(row.st_mode) not in (0o400,0o600)):fail()
        bound=identity(row)
        if self.original is not None and bound!=self.original:fail()
        flags=os.O_RDONLY|os.O_NONBLOCK
        if self.private_native:flags|=os.O_NOFOLLOW
        fd=os.open(self.path,flags)
        try:
            if identity(os.fstat(fd))!=bound:fail()
        finally:os.close(fd)
        if verify_archive(self.path,self.manifest) is not True:fail()
        if identity(self.path.lstat())!=bound:fail()
        self.original=bound;self.authority()

    def member(self,path):
        row=self.rows.get(path)
        if (row is None or not path.endswith(('/o.dat','/meta.inf','/meta.sum'))
            or type(row['size']) is not int or not 0<=row['size']<=256<<10):fail()
        flags=READ|(os.O_NOFOLLOW if self.private_native else 0)
        self.verify();fd=os.open(self.path,flags|os.O_NONBLOCK)
        try:
            if identity(os.fstat(fd))!=self.original:fail()
            result=None;seen=set()
            with os.fdopen(fd,'rb',closefd=False) as source,tarfile.open(fileobj=source,mode='r|') as archive:
                for member in archive:
                    if member.name in seen or len(seen)>=MAX_FILES:fail()
                    seen.add(member.name)
                    if member.name!=path:continue
                    if not member.isreg() or member.size!=row['size']:fail()
                    with archive.extractfile(member) as stream:result=stream.read(row['size']+1)
            if (result is None or len(result)!=row['size']
                or hashlib.sha256(result).hexdigest()!=row['sha256']
                or identity(os.fstat(fd))!=self.original):fail()
        finally:os.close(fd)
        self.verify();return result
