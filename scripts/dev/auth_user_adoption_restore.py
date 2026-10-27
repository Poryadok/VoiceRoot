#!/usr/bin/env python3
"""Off-node Docker Linux restore verifier. No Kubernetes, ports or live DB.

Copy only the encrypted archive set/manifest and the separately custodied key
to a private off-node directory. --tools-image is an existing immutable Docker
image ID containing openssl (scripts/dev/Dockerfile.tools), not a mutable tag.
All decrypt/psql/pg_restore execution is inside caller-owned Linux containers.
Only sanitized receipt metadata is emitted. Plaintext remains memory/pipe-only.
"""
import argparse
import hashlib
import hmac
import json
import os
import pathlib
import platform
import re
import secrets
import subprocess
import time
import uuid


def qualify(directory,tools_image,operation,root):
    directory=pathlib.Path(directory)
    if not re.fullmatch(r'sha256:[0-9a-f]{64}',tools_image):
        raise RuntimeError('immutable local tools image ID required')
    manifest=json.loads((directory/'backup-manifest.json').read_text(encoding='utf-8'))
    if manifest.get('operation')!=operation or manifest.get('format')!=1:
        raise RuntimeError('backup operation identity changed')
    if not manifest.get('origin_host') or manifest['origin_host']==platform.node():
        raise RuntimeError('off-node copy not established')
    if set(manifest['files'])!={'auth_db.enc','user_db.enc','roles.enc'}:
        raise RuntimeError('backup member set incomplete')
    key=(directory/'backup.key').read_text(encoding='utf-8').strip()
    if not re.fullmatch(r'[0-9a-f]{64}',key):
        raise RuntimeError('backup key custody invalid')
    name='dimas16-adoption-restore-'+uuid.uuid4().hex[:10]
    password=secrets.token_hex(24); created=False; exits={}; owned_volumes=[]
    def docker(*args,input=None,env=None,check=True,timeout=90):
        p=subprocess.run(['docker',*args],input=input,capture_output=True,env=env,timeout=timeout)
        if check and p.returncode:
            raise RuntimeError('private Docker restore operation failed')
        return p
    def psql(database,sql):
        return docker('exec','-i','-e','PGPASSWORD='+password,name,'psql','-h','127.0.0.1',
                      '-X','-qAt','-v','ON_ERROR_STOP=1','-U','voice','-d',database,input=sql.encode())
    def decrypt(member):
        path=directory/member
        if path.is_symlink() or not path.is_file() or path.stat().st_size>256*1024*1024:
            raise RuntimeError('encrypted backup member unsafe or too large')
        raw=path.read_bytes()
        identity={'sha256':hashlib.sha256(raw).hexdigest(),
                  'hmac':hmac.new(bytes.fromhex(key),raw,hashlib.sha256).hexdigest()}
        if identity!=manifest['files'][member]:
            raise RuntimeError('encrypted off-node backup integrity mismatch')
        environment=dict(os.environ,VOICE_BACKUP_KEY=key)
        return docker('run','--rm','-i','--network','none','--label','voice.test.owner=Dimas-16',
            '-e','VOICE_BACKUP_KEY','--entrypoint','openssl',tools_image,'enc','-d','-aes-256-cbc',
            '-pbkdf2','-iter','200000','-pass','env:VOICE_BACKUP_KEY',input=raw,env=environment).stdout
    try:
        if docker('info','--format','{{.OSType}}').stdout.strip()!=b'linux':
            raise RuntimeError('Linux Docker required')
        if json.loads(docker('image','inspect',tools_image).stdout)[0]['Id']!=tools_image:
            raise RuntimeError('tools artifact identity changed')
        role_sql=decrypt('roles.enc').decode('utf-8')
        if re.search(r'\bPASSWORD\s',role_sql,re.IGNORECASE):
            raise RuntimeError('role-password dump forbidden')
        if role_sql.count('CREATE ROLE voice;')!=1:
            raise RuntimeError('expected migration owner restore declaration missing')
        role_sql=role_sql.replace('CREATE ROLE voice;','-- existing disposable migration owner')
        docker('run','-d','--name',name,'--network','none','--label','voice.test.owner=Dimas-16',
            '--memory','512m','--cpus','1','-e','POSTGRES_USER=voice','-e','POSTGRES_PASSWORD='+password,
            'postgres:16-alpine','-c','cluster_name=voice-adoption-private-restore')
        created=True
        inspected=json.loads(docker('inspect',name).stdout)[0]
        if inspected.get('Config',{}).get('Labels',{}).get('voice.test.owner')!='Dimas-16':
            raise RuntimeError('owned restore fixture identity changed')
        # No host/named/volumes-from mounts are supplied when creating this
        # fresh container. PostgreSQL's image VOLUME is therefore anonymous
        # and private to this creation, not any pre-existing Docker data.
        if any(m['Type']!='volume' for m in inspected.get('Mounts',[])):
            raise RuntimeError('unexpected shared restore fixture mount')
        owned_volumes=[m['Name'] for m in inspected.get('Mounts',[]) if m['Type']=='volume']
        for _ in range(80):
            p=docker('exec','-e','PGPASSWORD='+password,name,'psql','-h','127.0.0.1','-X','-qAt',
                     '-U','voice','-d','postgres','-c','SELECT 1',check=False)
            if p.returncode==0:
                break
            time.sleep(.1)
        assert psql('postgres','SHOW cluster_name').stdout.strip()==b'voice-adoption-private-restore'
        psql('postgres',role_sql); exits['roles']=0
        for db in ('auth_db','user_db'):
            psql('postgres','CREATE DATABASE '+db+' OWNER voice;')
            archive=decrypt(db+'.enc')
            docker('exec','-i','-e','PGPASSWORD='+password,name,'pg_restore','-h','127.0.0.1','-U','voice',
                   '--exit-on-error','--dbname='+db,input=archive)
            exits[db]=0
        # Get the exact verifier queries from the reviewed operator package,
        # avoiding a second implementation of owner/ACL/history normalization.
        queries=json.loads(docker('run','--rm','--network','none','--label','voice.test.owner=Dimas-16',
            '--mount','type=bind,source='+str(root)+',target=/repo,readonly','-w','/repo/scripts/dev',
            'python:3.12-alpine','python','-c',
            'import json;from auth_user_adoption_runtime import backup_catalog_queries;print(json.dumps(backup_catalog_queries()))').stdout)
        restored={db:json.loads(psql(db,sql).stdout) for db,sql in queries.items()}
        if restored!=manifest['catalog']:
            raise RuntimeError('restored ownership/default grants/ledger catalog differs')
        report={'protocol':'voice-auth-user-backup-v1','operation':operation,'result':'PASS',
                'files':manifest['files'],'restored_catalog':restored,'off_node':True,'restore_exit':exits,
                'binding':manifest.get('binding'),
                'scope':'disposable-fixture-only' if manifest.get('private_fixture') else 'actual-existing-dev-backup',
                'postgres_major':16,'verifier_sha256':hashlib.sha256(pathlib.Path(__file__).read_bytes()).hexdigest(),
                'tools_image':tools_image,'fixture_cleanup':False}
    finally:
        if created:
            result=docker('rm','-f','-v',name,check=False)
            if result.returncode:
                raise RuntimeError('owned restore fixture cleanup incomplete')
            # Successful daemon reads distinguish actual absence from a
            # transport/permission failure. Never prune unrelated volumes.
            remaining=set(docker('volume','ls','--format','{{.Name}}').stdout.decode().splitlines())
            if remaining.intersection(owned_volumes):
                raise RuntimeError('owned plaintext restore volume cleanup incomplete')
    report['fixture_cleanup']=True
    report['removed_owned_anonymous_volumes']=len(owned_volumes)
    return report


def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--backup-directory',required=True)
    parser.add_argument('--tools-image',required=True)
    parser.add_argument('--operation',required=True)
    parser.add_argument('--output',required=True)
    args=parser.parse_args()
    try:
        report=qualify(args.backup_directory,args.tools_image,args.operation,pathlib.Path(__file__).resolve().parents[2])
        output=pathlib.Path(args.output)
        fd=os.open(output,os.O_WRONLY|os.O_CREAT|os.O_EXCL,0o600)
        with os.fdopen(fd,'w') as stream:
            json.dump(report,stream,sort_keys=True);stream.flush();os.fsync(stream.fileno())
        print(json.dumps({'result':'PASS','operation':args.operation,'report_sha256':hashlib.sha256(output.read_bytes()).hexdigest()}))
        return 0
    except BaseException:
        print(json.dumps({'result':'INCOMPLETE','action':'Reconcile private off-node backup and owned restore fixture'}))
        return 1


if __name__=='__main__':
    raise SystemExit(main())
