"""Reconcile only exact DockerRuntime containers with mounts in one owned root."""
import json
import os
from pathlib import Path
import re
import subprocess
import sys

LABEL='voice.known-nats.operation'

def docker(*args):
    result=subprocess.run(['/usr/bin/docker','--host=unix:///var/run/docker.sock',*args],
        stdin=subprocess.DEVNULL,stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,timeout=30)
    if result.returncode:raise RuntimeError('owned_docker_unavailable')
    return result.stdout.decode('utf8').strip()

def listed():
    ids=docker('ps','-aq','--no-trunc','--filter','label='+LABEL).splitlines()
    if len(ids)!=len(set(ids)) or any(not re.fullmatch('[a-f0-9]{64}',item) for item in ids):
        raise ValueError('owned_container_inventory_invalid')
    return ids

def reconcile(root,*,require_empty=False):
    if (os.geteuid()!=0 or not root.is_absolute() or root.resolve(strict=True)!=root
        or root.stat().st_uid!=0 or root.stat().st_mode&0o777!=0o700):
        raise ValueError('owned_fixture_root_invalid')
    ids=listed()
    if require_empty and ids:raise ValueError('preexisting_nats_fixture_container')
    for cid in ids:
        rows=json.loads(docker('inspect',cid))
        if type(rows) is not list or len(rows)!=1:raise ValueError('owned_container_inspection_invalid')
        row=rows[0];labels=row.get('Config',{}).get('Labels') or {};operation=labels.get(LABEL,'')
        name=row.get('Name','').removeprefix('/')
        mounts=row.get('Mounts')
        if (row.get('Id')!=cid or not re.fullmatch(r'(?:actor|auth)[a-f0-9]{12}',operation)
            or not name.startswith('voice-known-'+operation+'-') or type(mounts) is not list or not mounts):
            raise ValueError('foreign_nats_fixture_container')
        for mount in mounts:
            source=mount.get('Source')
            if (mount.get('Type')!='bind' or type(source) is not str or not Path(source).is_absolute()
                or not Path(source).resolve(strict=False).is_relative_to(root)):
                raise ValueError('foreign_nats_fixture_mount')
        docker('rm','-f',cid)
    if listed():raise RuntimeError('owned_container_cleanup_unconfirmed')
    print('owned_nats_containers=EMPTY')

if __name__=='__main__':
    try:
        if len(sys.argv)!=3 or sys.argv[1] not in ('before','after'):raise ValueError('invalid_arguments')
        reconcile(Path(sys.argv[2]),require_empty=sys.argv[1]=='before')
    except Exception as error:
        print('owned_nats_cleanup=UNCONFIRMED '+type(error).__name__,file=sys.stderr)
        sys.exit(1)
