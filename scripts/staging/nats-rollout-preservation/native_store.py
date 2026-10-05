"""Streaming rollout archives with explicit capacity and native-entry policy."""
import copy
import os
from pathlib import Path
import re
import shutil
import stat
import controller
from controller import Blocked

MAX_BYTES=48<<30
MAX_FILES=262144
MECHANISM='closed-jetstream-store-tar-v2'

def _legacy(manifest):
    if manifest.get('mechanism')!='closed-jetstream-store-tar-v1':
        if manifest.get('mechanism')!=MECHANISM or manifest.get('limits')!={'max_bytes':MAX_BYTES,'max_files':MAX_FILES}:raise Blocked('rollout_native_policy_invalid')
        manifest=copy.deepcopy(manifest);manifest.pop('limits');manifest['mechanism']='closed-jetstream-store-tar-v1'
    return manifest

def archive_closed_store(source,archive):
    row=controller.archive_closed_store(source,archive,max_bytes=MAX_BYTES,max_files=MAX_FILES,streaming=True)
    row['mechanism']=MECHANISM;row['limits']={'max_bytes':MAX_BYTES,'max_files':MAX_FILES}
    return row

def verify_archive(archive,manifest):
    return controller.verify_archive(archive,_legacy(manifest),max_bytes=MAX_BYTES,max_files=MAX_FILES)

def restore_closed_store(archive,target,manifest):
    return controller.restore_closed_store(archive,target,_legacy(manifest),max_bytes=MAX_BYTES,max_files=MAX_FILES,streaming=True)

def preflight(source,base,capacity,free_bytes=None):
    match=re.fullmatch(r'([1-9][0-9]*)(Ki|Mi|Gi|Ti)?',capacity)
    if not match:raise Blocked('rollout_pvc_capacity_invalid')
    requested=int(match[1])*{'Ki':1<<10,'Mi':1<<20,'Gi':1<<30,'Ti':1<<40,None:1}[match[2]]
    if requested>MAX_BYTES:raise Blocked('rollout_pvc_capacity_exceeds_backup_policy')
    source=Path(source);root=os.open(source,controller.READ|os.O_DIRECTORY);device=os.fstat(root).st_dev
    counts={'files':0,'dirs':0,'bytes':0}
    def walk(fd,depth=0):
        if depth>controller.MAX_DEPTH:raise Blocked('archive_depth_limit')
        with os.scandir(fd) as entries:
            for entry in entries:
                name=entry.name
                if not controller.SAFE.fullmatch(name) or name in ('.','..') or name.lower().endswith(('.creds','.key','.pem','.jwt')) or 'seed' in name.lower():raise Blocked('archive_input_invalid')
                info=os.stat(name,dir_fd=fd,follow_symlinks=False)
                if info.st_dev!=device:raise Blocked('archive_mount_boundary')
                if stat.S_ISREG(info.st_mode):counts['files']+=1;counts['bytes']+=info.st_size
                elif stat.S_ISDIR(info.st_mode):
                    counts['dirs']+=1;child=os.open(name,controller.READ|os.O_DIRECTORY,dir_fd=fd)
                    try:walk(child,depth+1)
                    finally:os.close(child)
                else:raise Blocked('archive_nonregular_input')
                if counts['files']+counts['dirs']>MAX_FILES or counts['bytes']>requested:raise Blocked('rollout_native_capacity_exceeded')
    try:walk(root)
    finally:os.close(root)
    # Before/after archives, two restored copies, CMS plaintext workspace,
    # ciphertext/private export and downloaded ZIP all need local space.
    # A one-GiB margin bounds active-write growth between this scan and fence.
    required=10*(counts['bytes']+1024*(counts['files']+counts['dirs']))+(1<<30)
    available=shutil.disk_usage(base).free if free_bytes is None else free_bytes
    if available<required:raise Blocked('rollout_backup_disk_space_insufficient')
    fs=os.statvfs(base)
    if fs.f_favail and fs.f_favail<4*(counts['files']+counts['dirs'])+1024:raise Blocked('rollout_backup_inodes_insufficient')
    return {**counts,'pvc_capacity_bytes':requested,'required_free_bytes':required,'max_bytes':MAX_BYTES,'max_files':MAX_FILES}
