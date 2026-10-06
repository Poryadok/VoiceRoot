"""Retained Space MVCC exporter and bounded private dump streaming.

No receipt from this adapter claims restore or off-node verification. The root
transaction must obtain both before using it as a migration prerequisite.
"""
import hashlib
import json
import os
from pathlib import Path
import re
import selectors
import signal
import subprocess
import time
import space_migration

class BackupError(RuntimeError):pass

def fail():raise BackupError('space_backup_rejected')

def snapshot_id(value):
    if not isinstance(value,str) or not re.fullmatch(r'[0-9A-F]{8}-[0-9A-F]{8}-[0-9]{1,10}',value):fail()
    return value

class Writer:
    def __init__(self,stream,limit):
        self.stream=stream;self.limit=limit;self.count=0;self.digest=hashlib.sha256();self.prefix=b''
    def write(self,raw):
        if not isinstance(raw,bytes) or len(raw)>1<<20 or self.count+len(raw)>self.limit:fail()
        if len(self.prefix)<5:self.prefix=(self.prefix+raw)[:5]
        self.count+=len(raw);self.digest.update(raw);self.stream.write(raw)

class Snapshot:
    def __init__(self,backend):self.backend=backend
    def __enter__(self):
        try:
            value,before=self.backend.begin()
            self.identifier=snapshot_id(value);self.before=space_migration.observed(before)
            if not self.backend.alive():fail()
            return self
        except BaseException:
            self.backend.close();raise
    def __exit__(self,*args):self.backend.close()
    def dump(self,target,limit):
        if type(limit) is not int or not 5<=limit<=64<<30 or not self.backend.alive():fail()
        target=Path(target);created=False
        try:
            fd=os.open(target,os.O_WRONLY|os.O_CREAT|os.O_EXCL|os.O_NOFOLLOW,0o600);created=True
            with os.fdopen(fd,'wb') as stream:
                writer=Writer(stream,limit);self.backend.dump(self.identifier,writer)
                if not self.backend.alive() or writer.prefix!=b'PGDMP' or writer.count<5:fail()
                stream.flush();os.fsync(stream.fileno())
            return {'schema':'voice-space-backup-v1','snapshot':self.identifier,'before':self.before,
                'dump_sha256':writer.digest.hexdigest(),'dump_bytes':writer.count,
                'restored':False,'offnode_verified':False}
        except BaseException:
            if created:target.unlink()
            raise

SQL="""BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY;
SET LOCAL idle_in_transaction_session_timeout = '600s';
SET LOCAL statement_timeout = '600s';
SELECT pg_export_snapshot();
SELECT json_build_object('database',current_database(),
 'version',(SELECT version FROM schema_migrations),
 'dirty',(SELECT dirty FROM schema_migrations),
 'allow_guests_true',(SELECT count(*) FROM spaces WHERE allow_guests=true),
 'column_default',(SELECT column_default FROM information_schema.columns WHERE table_schema='public' AND table_name='spaces' AND column_name='allow_guests'),
 'column_type',(SELECT data_type FROM information_schema.columns WHERE table_schema='public' AND table_name='spaces' AND column_name='allow_guests'));
"""

class Postgres:
    """Fixed installed pod commands. Caller validates its root-owned UID binding."""
    def __init__(self,timeout=600):
        if type(timeout) is not int or not 1<=timeout<=600:fail()
        self.timeout=timeout;self.child=None
    @staticmethod
    def argv(program,args,interactive=False):
        script='export PGPASSWORD="$POSTGRES_PASSWORD"; exec '+program+' -U "$POSTGRES_USER" "$@"'
        return ['/usr/local/bin/k3s','kubectl','--namespace','voice-staging','exec',
            *(['-i'] if interactive else []),'voice-postgres-0','--','sh','-ceu',script,'--',*args]
    def begin(self):
        self.deadline=time.monotonic()+self.timeout
        self.child=subprocess.Popen(self.argv('psql',['-X','-A','-t','-q','-v','ON_ERROR_STOP=1',
            '-h','127.0.0.1','-d','space_db'],True),stdin=subprocess.PIPE,
            stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,start_new_session=True,
            env={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root','LC_ALL':'C'})
        self.child.stdin.write(SQL.encode());self.child.stdin.flush()
        identifier=self.line();before=json.loads(self.line())
        return identifier,before
    def alive(self):return self.child is not None and self.child.poll() is None and time.monotonic()<self.deadline
    def line(self):
        output=bytearray()
        with selectors.DefaultSelector() as selector:
            selector.register(self.child.stdout,selectors.EVENT_READ)
            while len(output)<=16384:
                remaining=self.deadline-time.monotonic()
                if remaining<=0 or not selector.select(min(remaining,0.1)): 
                    if remaining<=0:fail()
                    continue
                raw=os.read(self.child.stdout.fileno(),1)
                if not raw:fail()
                if raw==b'\n':return output.decode('utf-8')
                output.extend(raw)
        fail()
    def dump(self,identifier,stream):
        snapshot_id(identifier)
        child=subprocess.Popen(self.argv('pg_dump',['-h','127.0.0.1',
            '-d','space_db','--format=custom','--snapshot='+identifier]),stdin=subprocess.DEVNULL,
            stdout=subprocess.PIPE,stderr=subprocess.DEVNULL,start_new_session=True,
            env={'PATH':'/usr/local/bin:/usr/bin:/bin','HOME':'/root','LC_ALL':'C'})
        try:
            with selectors.DefaultSelector() as selector:
                selector.register(child.stdout,selectors.EVENT_READ)
                while True:
                    if not self.alive():fail()
                    if not selector.select(0.1):continue
                    raw=os.read(child.stdout.fileno(),1<<20)
                    if not raw:break
                    stream.write(raw)
            if child.wait(timeout=max(0.01,self.deadline-time.monotonic()))!=0:fail()
        finally:
            if child.poll() is None:os.killpg(child.pid,signal.SIGKILL);child.wait(timeout=2)
            child.stdout.close()
    def close(self):
        if self.child is None:return
        try:
            if self.child.poll() is None:
                try:self.child.stdin.write(b'ROLLBACK;\n');self.child.stdin.flush()
                except (OSError,ValueError):pass
                self.child.stdin.close()
                try:self.child.wait(timeout=3)
                except subprocess.TimeoutExpired:os.killpg(self.child.pid,signal.SIGKILL);self.child.wait(timeout=2)
        finally:
            for stream in (self.child.stdin,self.child.stdout):
                if not stream.closed:stream.close()
