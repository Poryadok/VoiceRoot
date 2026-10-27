#!/usr/bin/env python3
"""Opt-in real PostgreSQL16 regression. Docker Linux only, no Kubernetes/ports.

python scripts/dev/auth_user_adoption_pg.py [--fresh-baseline]
The baseline switch demonstrates why fresh-only issuance cannot be replayed.
Credentials and database diagnostics remain memory-only; output is sanitized.
"""
import argparse
import json
import pathlib
import re
import secrets
import subprocess
import tempfile
import hashlib
import hmac
import os
from unittest.mock import patch
import time
import uuid


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--fresh-baseline', action='store_true')
    args = parser.parse_args()
    root = pathlib.Path(__file__).resolve().parents[2]
    name = 'dimas16-adoption-pg-' + uuid.uuid4().hex[:10]
    password, auth, user = [secrets.token_hex(24) for _ in range(3)]
    created = False
    def docker(*cmd, input=None, check=True, timeout=60):
        # SQL runs in Linux. Native Windows text pipes otherwise rewrite LF
        # inside dollar-quoted function bodies to CRLF, unlike the real route.
        payload=input.encode('utf-8') if isinstance(input,str) else input
        result = subprocess.run(['docker', *cmd], input=payload, capture_output=True, timeout=timeout)
        result.stdout=result.stdout.decode('utf-8',errors='replace')
        result.stderr=result.stderr.decode('utf-8',errors='replace')
        if check and result.returncode:
            safe = result.stderr
            for value in (password, auth, user):
                safe = safe.replace(value, '[redacted]')
            raise RuntimeError('private Docker/PG fixture ' + cmd[0] + ' failed: ' + safe[-800:])
        return result
    def pg(sql, db='postgres', role='voice', credential=None, options='', application='', check=True, timeout=30):
        return docker('exec', '-i', '-e', 'PGPASSWORD=' + (credential or password),
                      '-e', 'PGOPTIONS=' + options, '-e', 'PGAPPNAME=' + application,
                      name, 'psql', '-h', '127.0.0.1', '-X', '-qAt', '-v', 'ON_ERROR_STOP=1',
                      '-U', role, '-d', db, input=sql, check=check, timeout=timeout)
    def roles(sql, a=auth, u=user):
        return docker('exec', '-i', '-e', 'PGPASSWORD=' + password,
                      '-e', 'AUTH_DB_PASSWORD=' + a, '-e', 'USER_DB_PASSWORD=' + u,
                      name, 'psql', '-h', '127.0.0.1', '-X', '-qAt', '-v', 'ON_ERROR_STOP=1',
                      '-U', 'voice', '-d', 'postgres', input=sql, check=False)
    def sessions(application):
        return json.loads(pg("SELECT coalesce(json_agg(json_build_object('pid',pid,'backend_start',backend_start,'state',state)), '[]'::json) FROM pg_stat_activity WHERE application_name='" + application + "'").stdout)
    def wait_session(application):
        for _ in range(60):
            actual = sessions(application)
            if actual:
                return actual
            time.sleep(.05)
        raise RuntimeError('private fixture backend not observed')
    try:
        if docker('info', '--format', '{{.OSType}}').stdout.strip() != 'linux':
            raise RuntimeError('Linux Docker required')
        docker('run', '-d', '--name', name, '--label', 'voice.test.owner=Dimas-16',
               '--network', 'none', '--memory', '512m', '--cpus', '1',
               '-e', 'POSTGRES_USER=voice', '-e', 'POSTGRES_PASSWORD=' + password,
               'postgres:16-alpine', '-c', 'cluster_name=voice-adoption-private-fixture')
        created = True
        for _ in range(80):
            if pg('SELECT 1', check=False).returncode == 0:
                break
            time.sleep(.1)
        assert pg("SHOW cluster_name").stdout.strip() == 'voice-adoption-private-fixture'
        compiled = docker('run', '--rm', '--network', 'none', '--label', 'voice.test.owner=Dimas-16',
            '--mount', 'type=bind,source=' + str(root) + ',target=/repo,readonly', '-w', '/repo/scripts/dev',
            'python:3.12-alpine', 'python', '-c',
            "import json;from auth_user_adoption import sql_phases;print(json.dumps(sql_phases('/repo')))")
        phases = json.loads(compiled.stdout)
        pg("CREATE DATABASE auth_db OWNER voice;")
        pg("CREATE DATABASE user_db OWNER voice;")
        pg("CREATE TABLE accounts(id serial PRIMARY KEY, marker text);INSERT INTO accounts(marker) VALUES('preserved');CREATE TABLE flyway_schema_history(installed_rank int,version text,script text,checksum int,success boolean);INSERT INTO flyway_schema_history VALUES(1,'26','synthetic-fixture-only',123,true);", 'auth_db')
        pg('BEGIN;\n' + (root/'src/backend/auth/src/main/resources/db/migration/V31__auth_identity_ddl_barrier.sql').read_text(encoding='utf-8') + '\nCOMMIT;', 'auth_db')
        pg("CREATE TABLE profiles(id serial PRIMARY KEY, marker text);INSERT INTO profiles(marker) VALUES('preserved');CREATE TABLE schema_migrations(version bigint, dirty boolean);INSERT INTO schema_migrations VALUES(22,false)", 'user_db')
        assert roles(phases['roles'], a=password).returncode != 0
        assert roles(phases['roles'], u=auth).returncode != 0
        assert pg("SELECT count(*) FROM pg_roles WHERE rolname IN ('auth_runtime','user_runtime')").stdout.strip() == '0'
        # A pre-existing foreign role is never adopted; no second role is created.
        pg("CREATE ROLE auth_runtime LOGIN PASSWORD '" + auth + "'")
        assert roles(phases['roles']).returncode != 0
        assert pg("SELECT count(*) FROM pg_roles WHERE rolname='user_runtime'").stdout.strip() == '0'
        pg('DROP ROLE auth_runtime')  # caller-owned disposable fixture only
        if args.fresh_baseline:
            baseline = (root/'docker/postgres/provision-auth-user-runtime-roles.sql').read_text(encoding='utf-8')
            assert roles(baseline).returncode == 0
            attempted = roles(baseline, a=secrets.token_hex(24), u=secrets.token_hex(24))
        else:
            assert roles(phases['roles']).returncode == 0
            pg(phases['grants_auth'], 'auth_db')
            pg(phases['grants_user'], 'user_db')
            attempted = roles(phases['roles'], a=secrets.token_hex(24), u=secrets.token_hex(24))
        if attempted.returncode == 0:
            raise RuntimeError('REGRESSION: existing runtime roles were accepted and rekeyed')
        assert pg('SELECT marker FROM accounts WHERE id=1', 'auth_db', 'auth_runtime', auth).stdout.strip() == 'preserved'
        assert pg('SELECT marker FROM profiles WHERE id=1', 'user_db', 'user_runtime', user).stdout.strip() == 'preserved'
        assert pg('SELECT 1', 'user_db', 'auth_runtime', auth, check=False).returncode != 0
        assert pg('SELECT 1', 'auth_db', 'user_runtime', user, check=False).returncode != 0
        for role, db, credential, sql in [('auth_runtime','auth_db',auth,'CREATE TABLE forbidden(id int)'),
                ('auth_runtime','auth_db',auth,"INSERT INTO flyway_schema_history(version) VALUES('99')"),
                ('auth_runtime','auth_db',auth,'INSERT INTO auth_identity_ddl_barrier VALUES(true)'),
                ('user_runtime','user_db',user,'UPDATE schema_migrations SET dirty=true')]:
            assert pg(sql, db, role, credential, check=False).returncode != 0
        gate = json.loads(pg((root/'src/backend/auth/src/main/resources/roleidentityauthority/gate-catalog.sql').read_text(encoding='utf-8'), 'auth_db', 'auth_runtime', auth).stdout)
        assert gate['acl_ok'] and gate['runtime_ok']
        validator="import json,os;from auth_user_adoption_runtime import validate_ddl_gate;validate_ddl_gate(json.loads(os.environ['FIXTURE_GATE']),'/repo')"
        def validate_actual_gate(expect_success):
            actual=json.loads(pg((root/'src/backend/auth/src/main/resources/roleidentityauthority/gate-catalog.sql').read_text(encoding='utf-8'),'auth_db','auth_runtime',auth).stdout)
            probe=docker('run','--rm','--network','none','--label','voice.test.owner=Dimas-16',
                '--mount','type=bind,source='+str(root)+',target=/repo,readonly','-w','/repo/scripts/dev',
                '-e','FIXTURE_GATE='+json.dumps(actual),'python:3.12-alpine','python','-c',validator,check=False)
            if (probe.returncode==0)!=expect_success:
                raise RuntimeError('private canonical gate mismatch: '+json.dumps(actual,sort_keys=True))
        validate_actual_gate(True)
        pg('ALTER EVENT TRIGGER auth_identity_ddl_gate DISABLE','auth_db')
        validate_actual_gate(False)
        pg('ALTER EVENT TRIGGER auth_identity_ddl_gate ENABLE','auth_db')
        validate_actual_gate(True)
        pg('BEGIN;CREATE TEMP TABLE preparation(id int) ON COMMIT DROP;COMMIT', 'user_db', 'user_runtime', user)
        pg('CREATE TABLE future(id serial PRIMARY KEY)', 'auth_db')
        pg('INSERT INTO future DEFAULT VALUES', 'auth_db', 'auth_runtime', auth)
        metadata_program="""import json
from auth_user_adoption_runtime import database_state
calls=[]
class Q:
 def query(self,db,sql):
  calls.append({'database':db,'sql':sql})
  return {'schema_owner':'voice','objects':{},'relation_acls':{},'defaults':[],'consumers':[],'unexpected_schemas':[],'event_triggers':[]}
database_state(Q())
print(json.dumps(calls))
"""
        metadata_queries=json.loads(docker('run','--rm','--network','none','--label','voice.test.owner=Dimas-16',
            '--mount','type=bind,source='+str(root)+',target=/repo,readonly','-w','/repo/scripts/dev','python:3.12-alpine',
            'python','-c',metadata_program).stdout)
        temp_owner=subprocess.Popen(['docker','exec','-i','-e','PGPASSWORD='+user,'-e','PGAPPNAME=adoption-fixture-temp-owner',
            name,'psql','-h','127.0.0.1','-X','-qAt','-U','user_runtime','-d','user_db'],
            stdin=subprocess.PIPE,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,text=True)
        temp_owner.stdin.write('BEGIN;CREATE TEMP TABLE association_preparation(id int) ON COMMIT DROP;SELECT pg_sleep(30);\n')
        temp_owner.stdin.flush();wait_session('adoption-fixture-temp-owner')
        for call in metadata_queries:
            actual=json.loads(pg(call['sql'],call['database']).stdout)
            if isinstance(actual,dict) and 'roles' in actual:
                assert actual['major']==16 and actual['admin']=={'user':'voice','superuser':True}
                assert all(props['owned_objects']==0 for props in actual['roles'].values()), 'legitimate TEMP ownership must not appear persistent'
        pg("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE application_name='adoption-fixture-temp-owner'")
        temp_owner.stdin.close();temp_owner.wait(timeout=5)
        # Real server lock deadline, not an asserted client timeout.
        blocker = subprocess.Popen(['docker','exec','-i','-e','PGPASSWORD='+password,'-e','PGAPPNAME=adoption-fixture-blocker',
            name,'psql','-h','127.0.0.1','-X','-qAt','-U','voice','-d','auth_db'],
            stdin=subprocess.PIPE,stdout=subprocess.DEVNULL,stderr=subprocess.DEVNULL,text=True)
        blocker.stdin.write('BEGIN;LOCK TABLE accounts IN ACCESS EXCLUSIVE MODE;SELECT pg_sleep(30);\n')
        blocker.stdin.flush()
        wait_session('adoption-fixture-blocker')
        victim = pg('ALTER TABLE accounts ADD COLUMN forbidden text', 'auth_db',
                    options='-c statement_timeout=500 -c lock_timeout=300', application='adoption-fixture-victim', check=False)
        assert victim.returncode != 0 and 'timeout' in victim.stderr
        assert sessions('adoption-fixture-victim') == []
        pg("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE application_name='adoption-fixture-blocker'")
        blocker.stdin.close(); blocker.wait(timeout=5)
        # Killing the CLI transport is not proof that its server backend stopped.
        try:
            pg('SELECT pg_sleep(10)', application='adoption-fixture-ambiguous', timeout=.8)
            raise RuntimeError('fixture client timeout missing')
        except subprocess.TimeoutExpired:
            active = wait_session('adoption-fixture-ambiguous')
            assert active[0]['state'] == 'active'
        pg("SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE application_name='adoption-fixture-ambiguous'")
        assert sessions('adoption-fixture-ambiguous') == []
        # Exercise encrypted archive -> independent empty cluster restoration.
        # This is a disposable Docker fixture, not physical DEV off-node proof.
        tools=json.loads(docker('image','inspect','rudeus321-a1-tools:local').stdout)[0]['Id']
        queries=json.loads(docker('run','--rm','--network','none','--label','voice.test.owner=Dimas-16',
            '--mount','type=bind,source='+str(root)+',target=/repo,readonly','-w','/repo/scripts/dev','python:3.12-alpine',
            'python','-c','import json;from auth_user_adoption_runtime import backup_catalog_queries;print(json.dumps(backup_catalog_queries()))').stdout)
        catalog={db:json.loads(pg(sql,db).stdout) for db,sql in queries.items()}
        with tempfile.TemporaryDirectory(prefix='dimas16-encrypted-restore-') as tmp:
            folder=pathlib.Path(tmp); key=secrets.token_hex(32)
            (folder/'backup.key').write_text(key,encoding='utf-8')
            files={}
            for db in ('auth_db','user_db','roles'):
                if db=='roles':
                    raw=subprocess.run(['docker','exec','-e','PGPASSWORD='+password,name,'pg_dumpall','-h','127.0.0.1','-U','voice','--roles-only','--no-role-passwords'],capture_output=True,check=True).stdout
                else:
                    raw=subprocess.run(['docker','exec','-e','PGPASSWORD='+password,name,'pg_dump','-h','127.0.0.1','-U','voice','-Fc','-d',db],capture_output=True,check=True).stdout
                encrypted=subprocess.run(['docker','run','--rm','-i','--network','none','--label','voice.test.owner=Dimas-16',
                    '-e','VOICE_BACKUP_KEY','--entrypoint','openssl',tools,'enc','-aes-256-cbc','-pbkdf2','-iter','200000','-salt','-pass','env:VOICE_BACKUP_KEY'],
                    input=raw,capture_output=True,env=dict(os.environ,VOICE_BACKUP_KEY=key),check=True).stdout
                member=db+'.enc'; (folder/member).write_bytes(encrypted)
                files[member]={'sha256':hashlib.sha256(encrypted).hexdigest(),'hmac':hmac.new(bytes.fromhex(key),encrypted,hashlib.sha256).hexdigest()}
            manifest={'format':1,'operation':name,'files':files,'catalog':catalog,'private_fixture':True,
                      'origin_host':docker('exec',name,'hostname').stdout.strip(),'binding':{'scope':'private-fixture-only'}}
            (folder/'backup-manifest.json').write_text(json.dumps(manifest),encoding='utf-8')
            from auth_user_adoption_restore import qualify
            real_run=subprocess.run;restore_volumes=[]
            def capture_owned_restore_volume(command,*arguments,**options):
                if (isinstance(command,list) and command[:3]==['docker','rm','-f']
                        and re.fullmatch(r'[0-9a-f]{64}',command[-1])):
                    inspected=real_run(['docker','inspect',command[-1]],capture_output=True)
                    if inspected.returncode==0:
                        obj=json.loads(inspected.stdout)[0]
                        assert obj['Id']==command[-1] and obj['Name'].startswith('/dimas16-adoption-restore-')
                        assert obj['Config']['Labels'].get('voice.test.owner')=='Dimas-16'
                        restore_volumes.extend(m['Name'] for m in obj['Mounts'] if m['Type']=='volume')
                return real_run(command,*arguments,**options)
            try:
                with patch('auth_user_adoption_restore.subprocess.run',side_effect=capture_owned_restore_volume):
                    report=qualify(folder,tools,name,root)
                if not restore_volumes:
                    raise RuntimeError('real owned PG16 anonymous volume was not observed')
                if any(real_run(['docker','volume','inspect',volume],capture_output=True).returncode==0
                       for volume in restore_volumes):
                    raise RuntimeError('owned plaintext restore volume remains after fixture_cleanup')
            finally:
                # Only volumes observed on this newly created labeled restore
                # container are eligible; never prune unattributed old data.
                for volume in restore_volumes:
                    real_run(['docker','volume','rm',volume],capture_output=True)
            assert report['result']=='PASS' and report['fixture_cleanup'] and report['scope']=='disposable-fixture-only'
            # Integrity corruption is rejected before pg_restore or acceptance.
            member=folder/'auth_db.enc'; member.write_bytes(member.read_bytes()+b'corrupt')
            try:
                qualify(folder,tools,name,root)
                raise AssertionError('tampered encrypted archive was accepted')
            except RuntimeError as error:
                assert 'integrity mismatch' in str(error)
        print(json.dumps({'result':'PASS','fixture':name,'checks':['strict duplicate/foreign role rejection, no credential rekey',
            'independent ownDB credentials; oppositeDB denied','data/owner/default grants and barrier preserved; ledger/DDL denied',
            'actual V31 gate accepted; actual disabled trigger denied; reenabled canonical contract accepted',
            'actual metadata queries evaluate; active legitimate User TEMP retains no persistent shared ownership',
            'real blocked DDL server timeout and backend closure','real client timeout leaves active server session until owned reconciliation',
            'encrypted/HMAC archive restores to independent empty PG16 with matching owner/defaultACL/history; tampering denied (private fixture scope only)',
            'actual owned plaintext restore volume absent before fixture cleanup is credited']}))
        return 0
    except BaseException as exc:
        print(json.dumps({'result':'FAIL','error':str(exc) if isinstance(exc, RuntimeError) else type(exc).__name__}))
        return 1
    finally:
        if created:
            docker('rm', '-f', '-v', name, check=False)


if __name__ == '__main__':
    raise SystemExit(main())
