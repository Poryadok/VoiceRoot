"""Restore a private Space dump only into an owned network-none scratch PG."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import time
import uuid
from encrypted_cut import regular
from space_backup import Postgres,Snapshot

class RestoreError(RuntimeError):pass
def fail():raise RestoreError('space_isolated_restore_rejected')

ENV={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root','LC_ALL':'C'}
def command(args,timeout=600,stdin=None):
    try:
        row=subprocess.run(['/usr/bin/docker',*args],stdin=stdin if stdin is not None else subprocess.DEVNULL,
            stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,timeout=timeout,env=ENV,check=True)
        if len(row.stdout)>16384:fail()
        return row.stdout.decode().strip()
    except (OSError,subprocess.SubprocessError,UnicodeError):fail()

class Scratch(Postgres):
    def __init__(self,name):super().__init__();self.name=name
    def argv(self,program,args,interactive=False):
        return ['/usr/bin/docker','exec',*(['-i'] if interactive else []),self.name,
            program,'-U','voice_restore',*args]

def restore(path,backup,image,operation):
    import re
    if not re.fullmatch(r'[a-f0-9]{12}',operation) or not re.fullmatch(r'docker.io/library/postgres@sha256:[a-f0-9]{64}',image):fail()
    fd,metadata=regular(Path(path),private=True)
    name='voice-space-restore-'+operation+'-'+uuid.uuid4().hex
    label='voice-space-restore='+name;volume=name;container=None;created_volume=False
    try:
        with os.fdopen(fd,'rb') as dump:
            sha=hashlib.sha256();count=0
            while chunk:=dump.read(1<<20):sha.update(chunk);count+=len(chunk)
            if count!=metadata.st_size or count!=backup['dump_bytes'] or sha.hexdigest()!=backup['dump_sha256']:fail()
            dump.seek(0)
            # No host path/store/app/port: only the newly owned volume is mounted.
            if command(['volume','create','--label',label,volume])!=volume:fail()
            created_volume=True
            if json.loads(command(['volume','inspect',volume]))[0].get('Labels',{}).get('voice-space-restore')!=name:fail()
            container=command(['create','--name',name,'--label',label,'--network','none',
                '--mount','type=volume,src='+volume+',dst=/var/lib/postgresql/data',
                '-e','POSTGRES_USER=voice_restore','-e','POSTGRES_PASSWORD=owned-disposable-restore-only',
                '-e','POSTGRES_DB=space_db',image])
            if not re.fullmatch(r'[a-f0-9]{64}',container):fail()
            info=json.loads(command(['inspect',container]))[0]
            if info['Config']['Labels'].get('voice-space-restore')!=name or info['HostConfig']['NetworkMode']!='none' or info['HostConfig'].get('PortBindings'):fail()
            command(['start',container]);deadline=time.monotonic()+60
            while True:
                try:command(['exec',container,'pg_isready','-U','voice_restore','-d','space_db'],timeout=3);break
                except RestoreError:
                    if time.monotonic()>=deadline:raise
                    time.sleep(.2)
            command(['exec','-i',container,'pg_restore','--exit-on-error','--no-owner','--no-privileges',
                '-U','voice_restore','-d','space_db'],stdin=dump)
            with Snapshot(Scratch(container)) as restored:
                if restored.before!=backup['before']:fail()
            dump.seek(0);after=hashlib.sha256()
            while chunk:=dump.read(1<<20):after.update(chunk)
            if after.hexdigest()!=backup['dump_sha256']:fail()
            return {'restored':True,'image':image,'dump_sha256':backup['dump_sha256'],
                'before':backup['before']}
    finally:
        if container is not None:
            # Removal is by captured immutable container ID, never a live name.
            command(['rm','-f',container],timeout=30)
        if created_volume:
            info=json.loads(command(['volume','inspect',volume]))[0]
            if info.get('Labels',{}).get('voice-space-restore')!=name:fail()
            command(['volume','rm',volume],timeout=30)
